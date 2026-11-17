package customagents

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
)

const (
	defaultTickInterval = 15 * time.Second
	maxChainDepth       = 8
)

type ServiceOption func(*Service)

func WithTickInterval(d time.Duration) ServiceOption {
	return func(s *Service) {
		if d > 0 {
			s.tickInterval = d
		}
	}
}

type Service struct {
	store        *Store
	executor     AgentExecutor
	models       ModelCatalogSource
	events       EventSink
	tickInterval time.Duration

	mu           sync.Mutex
	activeRuns   map[string]context.CancelFunc
	activeAgents map[string]string
	schedCancel  context.CancelFunc
	wg           sync.WaitGroup
	started      bool
}

func NewService(
	store *Store,
	executor AgentExecutor,
	models ModelCatalogSource,
	events EventSink,
	opts ...ServiceOption,
) *Service {
	svc := &Service{
		store:        store,
		executor:     executor,
		models:       models,
		events:       events,
		tickInterval: defaultTickInterval,
		activeRuns:   make(map[string]context.CancelFunc),
		activeAgents: make(map[string]string),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}
	return svc
}

func (s *Service) Store() *Store {
	return s.store
}

func (s *Service) ListAvailableModels(ctx context.Context) ([]ModelOption, error) {
	if s.models == nil {
		return nil, errors.New("каталог моделей не настроен")
	}
	return s.models.ListAvailableModels(ctx)
}

func (s *Service) Start(ctx context.Context) {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return
	}
	schedCtx, cancel := context.WithCancel(ctx)
	s.schedCancel = cancel
	s.started = true
	s.wg.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.tickInterval)
		defer ticker.Stop()

		s.schedulerTick(schedCtx)
		for {
			select {
			case <-schedCtx.Done():
				return
			case <-ticker.C:
				s.schedulerTick(schedCtx)
			}
		}
	}()
}

func (s *Service) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	cancel := s.schedCancel
	s.started = false
	s.schedCancel = nil
	for _, runCancel := range s.activeRuns {
		runCancel()
	}
	s.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	s.wg.Wait()
}

func (s *Service) ProbeAgent(ctx context.Context, agentID string) (ProbeResult, error) {
	agent, err := s.store.GetAgent(ctx, agentID)
	if err != nil {
		return ProbeResult{}, err
	}
	return Probe(ctx, agent), nil
}

func (s *Service) Dispatch(ctx context.Context, req DispatchRequest) (CustomAgentRun, error) {
	agent, run, runCtx, err := s.prepareRun(ctx, context.Background(), req)
	if err != nil {
		return CustomAgentRun{}, err
	}

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.executeRun(runCtx, agent, run, req, false)
	}()
	return run, nil
}

func (s *Service) DispatchSync(ctx context.Context, req DispatchRequest) (CustomAgentRun, error) {
	agent, run, runCtx, err := s.prepareRun(ctx, ctx, req)
	if err != nil {
		return CustomAgentRun{}, err
	}
	return s.executeRun(runCtx, agent, run, req, true), nil
}

func (s *Service) CancelRun(runID string) error {
	s.mu.Lock()
	cancel, ok := s.activeRuns[runID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("запуск %s не найден среди активных", runID)
	}
	cancel()
	return nil
}

func (s *Service) prepareRun(lookupCtx, parentCtx context.Context, req DispatchRequest) (CustomAgent, CustomAgentRun, context.Context, error) {
	var (
		agent CustomAgent
		err   error
	)
	if strings.TrimSpace(req.AgentID) != "" {
		agent, err = s.store.GetAgent(lookupCtx, req.AgentID)
	} else if strings.TrimSpace(req.Slug) != "" {
		agent, err = s.store.GetAgentBySlug(lookupCtx, req.Slug)
	} else {
		return CustomAgent{}, CustomAgentRun{}, nil, errors.New("требуется agent_id или slug")
	}
	if err != nil {
		return CustomAgent{}, CustomAgentRun{}, nil, err
	}

	chainID := strings.TrimSpace(req.ChainID)
	if chainID == "" {
		chainID = uuid.NewString()
	}
	triggerReason := strings.TrimSpace(req.TriggerReason)
	if triggerReason == "" {
		triggerReason = TriggerManual
	}

	runID := uuid.NewString()

	s.mu.Lock()
	if activeID, busy := s.activeAgents[agent.ID]; busy {
		s.mu.Unlock()
		return CustomAgent{}, CustomAgentRun{}, nil, fmt.Errorf("агент %s уже выполняется (%s)", agent.Slug, activeID)
	}
	runCtx, cancel := context.WithCancel(parentCtx)
	s.activeRuns[runID] = cancel
	s.activeAgents[agent.ID] = runID
	s.mu.Unlock()

	run := CustomAgentRun{
		ID:            runID,
		AgentID:       agent.ID,
		AgentName:     agent.Name,
		AgentRole:     agent.Role,
		ChainID:       chainID,
		ParentRunID:   req.ParentRunID,
		TriggerReason: triggerReason,
		InputPrompt:   req.Prompt,
		Status:        RunRunning,
		Attempt:       1,
		StartedAt:     time.Now().Unix(),
	}

	created, err := s.store.CreateRun(lookupCtx, run)
	if err != nil {
		cancel()
		s.releaseActiveRun(runID, agent.ID)
		return CustomAgent{}, CustomAgentRun{}, nil, err
	}

	return agent, created, runCtx, nil
}

func (s *Service) releaseActiveRun(runID, agentID string) {
	s.mu.Lock()
	if cancel, ok := s.activeRuns[runID]; ok {
		delete(s.activeRuns, runID)
		cancel()
	}
	if cur, ok := s.activeAgents[agentID]; ok && cur == runID {
		delete(s.activeAgents, agentID)
	}
	s.mu.Unlock()
}

func (s *Service) isAgentActive(agentID string) bool {
	s.mu.Lock()
	_, ok := s.activeAgents[agentID]
	s.mu.Unlock()
	return ok
}

func (s *Service) executeRun(ctx context.Context, agent CustomAgent, run CustomAgentRun, req DispatchRequest, syncChain bool) CustomAgentRun {
	defer s.releaseActiveRun(run.ID, agent.ID)

	persistCtx := context.Background()
	s.publish(Event{
		Type:      "run.started",
		AgentID:   agent.ID,
		RunID:     run.ID,
		ChainID:   run.ChainID,
		Payload:   run,
		Timestamp: time.Now().Unix(),
	})

	isMonitor := agent.TriggerType == TriggerMonitor || (agent.TriggerType == TriggerInterval && agent.MonitorKind != "")
	if isMonitor && !req.Force {
		agent.Status = StatusMonitoring
		agent.LastError = ""
		_ = s.store.UpdateAgentRuntimeState(persistCtx, agent)
		s.publishStatus(agent)

		probe := Probe(ctx, agent)
		if probe.Error != "" {
			now := time.Now().Unix()
			run.Status = RunFailed
			run.Error = probe.Error
			run.FinishedAt = now
			run.Steps = append(run.Steps, RunStep{
				Kind:      "monitor",
				Title:     "Ошибка мониторинга",
				Detail:    probe.Error,
				Status:    "failed",
				Timestamp: now,
			})
			_ = s.store.UpdateRun(persistCtx, run)

			agent.Status = StatusError
			agent.LastError = probe.Error
			agent.LastRunAt = now
			agent.NextRunAt = computeNextRunAt(agent, now)
			_ = s.store.UpdateAgentRuntimeState(persistCtx, agent)
			s.publish(Event{
				Type:      "run.failed",
				AgentID:   agent.ID,
				RunID:     run.ID,
				ChainID:   run.ChainID,
				Payload:   run,
				Timestamp: now,
			})
			return run
		}

		if !probe.Changed {
			now := time.Now().Unix()
			run.Status = RunSkippedNoChange
			run.FinishedAt = now
			run.Steps = append(run.Steps, RunStep{
				Kind:      "monitor",
				Title:     "Изменений не обнаружено",
				Detail:    "Состояние объекта мониторинга не изменилось",
				Status:    "ok",
				Timestamp: now,
			})
			_ = s.store.UpdateRun(persistCtx, run)

			agent.Status = StatusIdle
			agent.LastError = ""
			agent.LastRunAt = now
			agent.NextRunAt = computeNextRunAt(agent, now)
			_ = s.store.UpdateAgentRuntimeState(persistCtx, agent)
			s.publish(Event{
				Type:      "run.skipped",
				AgentID:   agent.ID,
				RunID:     run.ID,
				ChainID:   run.ChainID,
				Payload:   run,
				Timestamp: now,
			})
			return run
		}

		run.MonitorDiff = probe.Diff
		agent.MonitorLastHash = probe.Hash
		agent.MonitorLastSnapshot = probe.Snapshot
		_ = s.store.UpdateAgentRuntimeState(persistCtx, agent)
		run.Steps = append(run.Steps, RunStep{
			Kind:      "monitor",
			Title:     "Обнаружены изменения",
			Detail:    probe.Diff,
			Status:    "ok",
			Timestamp: time.Now().Unix(),
		})
	}

	if s.executor == nil || s.models == nil {
		return s.failRun(persistCtx, agent, run, "не настроены исполнитель или каталог моделей")
	}

	prompt := buildInitialPrompt(agent, req.Prompt, run.MonitorDiff)
	run.InputPrompt = prompt

	memorySnapshot, _ := s.store.ReadMemory(persistCtx, agent.ID)
	blackboardContext := s.formatBlackboardContext(persistCtx, run.ChainID)

	availableModels, err := s.models.ListAvailableModels(ctx)
	if err != nil {
		return s.failRun(persistCtx, agent, run, fmt.Sprintf("не удалось получить список моделей: %v", err))
	}

	choice, err := SelectModelForTask(availableModels, agent, prompt)
	if err != nil {
		return s.failRun(persistCtx, agent, run, fmt.Sprintf("не удалось подобрать модель: %v", err))
	}

	run.ResolvedProvider = choice.ProviderID
	run.ResolvedModel = choice.ModelID
	run.ResolvedReasoning = choice.ReasoningMode
	run.AutoModelReason = choice.Reason
	_ = s.store.UpdateRun(persistCtx, run)

	maxAttempts := agent.MaxReworkAttempts
	if maxAttempts <= 0 {
		maxAttempts = 3
	}
	if agent.Role == RoleVerifier {
		maxAttempts = 1
	}

	needsVerification := agent.Role != RoleVerifier &&
		(len(agent.DeterministicChecks) > 0 ||
			strings.TrimSpace(agent.AcceptanceCriteria) != "" ||
			strings.TrimSpace(agent.VerifierAgentID) != "")

	currentPrompt := prompt
	var stepMu sync.Mutex

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return s.cancelRun(persistCtx, agent, run)
		}

		run.Attempt = attempt
		run.Status = RunRunning
		_ = s.store.UpdateRun(persistCtx, run)

		agent.Status = StatusRunning
		agent.LastError = ""
		_ = s.store.UpdateAgentRuntimeState(persistCtx, agent)
		s.publishStatus(agent)

		execReq := ExecuteRequest{
			Agent:             agent,
			ChainID:           run.ChainID,
			RunID:             run.ID,
			Prompt:            currentPrompt,
			MemorySnapshot:    memorySnapshot,
			BlackboardContext: blackboardContext,
			ProviderID:        choice.ProviderID,
			ModelID:           choice.ModelID,
			ReasoningMode:     choice.ReasoningMode,
			OnStep: func(step RunStep) {
				if step.Timestamp == 0 {
					step.Timestamp = time.Now().Unix()
				}
				stepMu.Lock()
				run.Steps = append(run.Steps, step)
				stepMu.Unlock()
				s.publish(Event{
					Type:      "run.step",
					AgentID:   agent.ID,
					RunID:     run.ID,
					ChainID:   run.ChainID,
					Payload:   step,
					Timestamp: step.Timestamp,
				})
			},
		}

		res, execErr := s.executor.Execute(ctx, execReq)
		stepMu.Lock()
		if len(res.Steps) > 0 {
			run.Steps = append(run.Steps, res.Steps...)
		}
		stepMu.Unlock()

		if execErr != nil {
			if errors.Is(ctx.Err(), context.Canceled) {
				return s.cancelRun(persistCtx, agent, run)
			}
			return s.failRun(persistCtx, agent, run, execErr.Error())
		}

		run.OutputText = res.OutputText

		if !needsVerification {
			run.Status = RunPassed
			break
		}

		run.Status = RunVerifying
		_ = s.store.UpdateRun(persistCtx, run)
		agent.Status = StatusVerifying
		_ = s.store.UpdateAgentRuntimeState(persistCtx, agent)
		s.publishStatus(agent)
		s.publish(Event{
			Type:      "run.verifying",
			AgentID:   agent.ID,
			RunID:     run.ID,
			ChainID:   run.ChainID,
			Payload:   run,
			Timestamp: time.Now().Unix(),
		})

		detPassed, detReport := RunDeterministicChecks(ctx, agent.DeterministicChecks, res.OutputText, agent.WorkspacePath)
		run.DeterministicReport = detReport

		var (
			verifiedOK       bool
			verifierFeedback string
		)

		if !detPassed {
			verifiedOK = false
			verifierFeedback = "Не пройдены детерминированные проверки:\n" + detReport
			run.Steps = append(run.Steps, RunStep{
				Kind:      "deterministic_check",
				Title:     "Детерминированные проверки не пройдены",
				Detail:    detReport,
				Status:    "failed",
				Timestamp: time.Now().Unix(),
			})
		} else {
			if detReport != "" {
				run.Steps = append(run.Steps, RunStep{
					Kind:      "deterministic_check",
					Title:     "Детерминированные проверки пройдены",
					Detail:    detReport,
					Status:    "ok",
					Timestamp: time.Now().Unix(),
				})
			}

			needsLLMVerifier := strings.TrimSpace(agent.AcceptanceCriteria) != "" || strings.TrimSpace(agent.VerifierAgentID) != ""
			if !needsLLMVerifier {
				verifiedOK = true
				verifierFeedback = "Детерминированные проверки пройдены."
			} else {
				verifierAgent := s.resolveVerifierAgent(persistCtx, agent)
				vPrompt := buildVerifierPrompt(agent, prompt, res.OutputText, detReport)
				vChoice, vSelErr := SelectModelForTask(availableModels, verifierAgent, vPrompt)
				if vSelErr != nil {
					verifiedOK = false
					verifierFeedback = fmt.Sprintf("Не удалось подобрать модель верификатора: %v", vSelErr)
				} else {
					vRes, vExecErr := s.executor.Execute(ctx, ExecuteRequest{
						Agent:             verifierAgent,
						ChainID:           run.ChainID,
						RunID:             run.ID,
						Prompt:            vPrompt,
						BlackboardContext: blackboardContext,
						ProviderID:        vChoice.ProviderID,
						ModelID:           vChoice.ModelID,
						ReasoningMode:     vChoice.ReasoningMode,
					})
					if vExecErr != nil {
						if errors.Is(ctx.Err(), context.Canceled) {
							return s.cancelRun(persistCtx, agent, run)
						}
						verifiedOK = false
						verifierFeedback = fmt.Sprintf("Ошибка верификатора: %v", vExecErr)
					} else {
						verifiedOK, verifierFeedback = ParseVerifierVerdict(vRes.OutputText)
					}
				}
			}
		}

		if verifiedOK {
			run.VerifierVerdict = "PASS"
			run.VerifierFeedback = verifierFeedback
			run.Status = RunPassed
			run.Steps = append(run.Steps, RunStep{
				Kind:      "verifier",
				Title:     "Верификация пройдена",
				Detail:    verifierFeedback,
				Status:    "ok",
				Timestamp: time.Now().Unix(),
			})
			break
		}

		run.VerifierVerdict = "FAIL"
		run.VerifierFeedback = verifierFeedback
		run.Steps = append(run.Steps, RunStep{
			Kind:      "verifier",
			Title:     fmt.Sprintf("Верификатор отклонил попытку #%d", attempt),
			Detail:    verifierFeedback,
			Status:    "failed",
			Timestamp: time.Now().Unix(),
		})

		if attempt < maxAttempts {
			run.Status = RunRework
			_ = s.store.UpdateRun(persistCtx, run)
			agent.Status = StatusRework
			_ = s.store.UpdateAgentRuntimeState(persistCtx, agent)
			s.publishStatus(agent)
			s.publish(Event{
				Type:      "run.rework",
				AgentID:   agent.ID,
				RunID:     run.ID,
				ChainID:   run.ChainID,
				Payload:   run,
				Timestamp: time.Now().Unix(),
			})
			currentPrompt = fmt.Sprintf(
				"%s\n\nПредыдущая попытка #%d отклонена верификатором.\nЗамечания:\n%s\n\nИсправь все замечания и предоставь полный обновлённый результат.",
				prompt, attempt, verifierFeedback,
			)
			continue
		}

		now := time.Now().Unix()
		run.Status = RunFailed
		run.Error = "отклонено верификатором: " + verifierFeedback
		run.FinishedAt = now
		_ = s.store.UpdateRun(persistCtx, run)

		agent.Status = StatusError
		agent.LastError = run.Error
		agent.LastRunAt = now
		agent.NextRunAt = computeNextRunAt(agent, now)
		_ = s.store.UpdateAgentRuntimeState(persistCtx, agent)
		s.publishStatus(agent)
		s.publish(Event{
			Type:      "run.failed",
			AgentID:   agent.ID,
			RunID:     run.ID,
			ChainID:   run.ChainID,
			Payload:   run,
			Timestamp: now,
		})
		return run
	}

	now := time.Now().Unix()
	run.Status = RunPassed
	run.FinishedAt = now
	_ = s.store.UpdateRun(persistCtx, run)

	agent.Status = StatusIdle
	agent.LastError = ""
	agent.LastRunAt = now
	agent.NextRunAt = computeNextRunAt(agent, now)
	_ = s.store.UpdateAgentRuntimeState(persistCtx, agent)
	s.publishStatus(agent)
	s.publish(Event{
		Type:      "run.passed",
		AgentID:   agent.ID,
		RunID:     run.ID,
		ChainID:   run.ChainID,
		Payload:   run,
		Timestamp: now,
	})

	if agent.NextAgentID != "" && req.Depth < maxChainDepth {
		handoffErrMsg := ""
		if _, err := s.store.GetAgent(persistCtx, agent.NextAgentID); err != nil {
			handoffErrMsg = fmt.Sprintf("следующий агент %s недоступен: %v", agent.NextAgentID, err)
		} else if s.isAgentActive(agent.NextAgentID) {
			handoffErrMsg = fmt.Sprintf("следующий агент %s уже выполняется", agent.NextAgentID)
		}

		if handoffErrMsg != "" {
			s.recordHandoffFailure(persistCtx, agent, &run, handoffErrMsg)
			return run
		}

		bbEntry, bbErr := s.store.PostBlackboard(persistCtx, BlackboardEntry{
			ChainID:       run.ChainID,
			RunID:         run.ID,
			FromAgentID:   agent.ID,
			FromAgentName: agent.Name,
			ToAgentID:     agent.NextAgentID,
			Topic:         "handoff",
			Payload:       run.OutputText,
			Status:        "open",
			CreatedAt:     now,
		})
		if bbErr != nil {
			s.recordHandoffFailure(persistCtx, agent, &run, fmt.Sprintf("не удалось записать передачу в blackboard: %v", bbErr))
			return run
		}

		nextReq := DispatchRequest{
			AgentID:       agent.NextAgentID,
			Prompt:        run.OutputText,
			TriggerReason: "chain:" + agent.Slug,
			ChainID:       run.ChainID,
			ParentRunID:   run.ID,
			Force:         true,
			Depth:         req.Depth + 1,
		}
		var dispatchErr error
		if syncChain {
			_, dispatchErr = s.DispatchSync(ctx, nextReq)
		} else {
			_, dispatchErr = s.Dispatch(context.Background(), nextReq)
		}
		if dispatchErr != nil {
			_ = s.store.MarkBlackboardConsumed(persistCtx, bbEntry.ID)
			s.recordHandoffFailure(persistCtx, agent, &run, fmt.Sprintf("не удалось запустить следующего агента %s: %v", agent.NextAgentID, dispatchErr))
		}
	}

	return run
}

func (s *Service) recordHandoffFailure(ctx context.Context, agent CustomAgent, run *CustomAgentRun, msg string) {
	ts := time.Now().Unix()
	step := RunStep{
		Kind:      "handoff",
		Title:     "Ошибка передачи цепочки",
		Detail:    msg,
		Status:    "failed",
		Timestamp: ts,
	}
	run.Steps = append(run.Steps, step)
	_ = s.store.UpdateRun(ctx, *run)
	s.publish(Event{
		Type:      "chain.handoff_failed",
		AgentID:   agent.ID,
		RunID:     run.ID,
		ChainID:   run.ChainID,
		Payload:   step,
		Timestamp: ts,
	})
}

func (s *Service) failRun(ctx context.Context, agent CustomAgent, run CustomAgentRun, errMsg string) CustomAgentRun {
	now := time.Now().Unix()
	run.Status = RunFailed
	run.Error = errMsg
	run.FinishedAt = now
	_ = s.store.UpdateRun(ctx, run)

	agent.Status = StatusError
	agent.LastError = errMsg
	agent.LastRunAt = now
	agent.NextRunAt = computeNextRunAt(agent, now)
	_ = s.store.UpdateAgentRuntimeState(ctx, agent)
	s.publishStatus(agent)
	s.publish(Event{
		Type:      "run.failed",
		AgentID:   agent.ID,
		RunID:     run.ID,
		ChainID:   run.ChainID,
		Payload:   run,
		Timestamp: now,
	})
	return run
}

func (s *Service) cancelRun(ctx context.Context, agent CustomAgent, run CustomAgentRun) CustomAgentRun {
	now := time.Now().Unix()
	run.Status = RunCancelled
	run.Error = "выполнение отменено"
	run.FinishedAt = now
	_ = s.store.UpdateRun(ctx, run)

	agent.Status = StatusIdle
	agent.LastRunAt = now
	agent.NextRunAt = computeNextRunAt(agent, now)
	_ = s.store.UpdateAgentRuntimeState(ctx, agent)
	s.publishStatus(agent)
	s.publish(Event{
		Type:      "run.cancelled",
		AgentID:   agent.ID,
		RunID:     run.ID,
		ChainID:   run.ChainID,
		Payload:   run,
		Timestamp: now,
	})
	return run
}

func (s *Service) resolveVerifierAgent(ctx context.Context, worker CustomAgent) CustomAgent {
	if strings.TrimSpace(worker.VerifierAgentID) != "" && worker.VerifierAgentID != worker.ID {
		if v, err := s.store.GetAgent(ctx, worker.VerifierAgentID); err == nil {
			return v
		}
	}

	agents, err := s.store.ListAgents(ctx)
	if err == nil {
		var firstEnabled *CustomAgent
		for i := range agents {
			a := agents[i]
			if a.Role == RoleVerifier && a.Enabled && a.ID != worker.ID {
				if a.Slug == "strict-verifier" {
					return a
				}
				if firstEnabled == nil {
					cp := a
					firstEnabled = &cp
				}
			}
		}
		if firstEnabled != nil {
			return *firstEnabled
		}
	}

	return CustomAgent{
		ID:           "builtin-verifier",
		Slug:         "builtin-verifier",
		Name:         "Встроенный верификатор",
		Role:         RoleVerifier,
		SystemPrompt: defaultVerifierPrompt,
		ModelMode:    ModelModeAuto,
		Enabled:      true,
	}
}

func (s *Service) formatBlackboardContext(ctx context.Context, chainID string) string {
	entries, err := s.store.ListBlackboard(ctx, chainID, 50)
	if err != nil || len(entries) == 0 {
		return ""
	}
	var b strings.Builder
	for _, e := range entries {
		if e.Status == "consumed" {
			continue
		}
		fmt.Fprintf(&b, "[%s -> %s | %s]\n%s\n\n", e.FromAgentName, e.ToAgentID, e.Topic, e.Payload)
	}
	return strings.TrimSpace(b.String())
}

func (s *Service) schedulerTick(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	agents, err := s.store.ListAgents(ctx)
	if err != nil {
		return
	}
	now := time.Now().Unix()
	for _, agent := range agents {
		if !agent.Enabled || agent.Status == StatusPaused {
			continue
		}
		if agent.TriggerType != TriggerInterval && agent.TriggerType != TriggerCron && agent.TriggerType != TriggerMonitor {
			continue
		}
		if agent.TriggerType == TriggerCron && agent.NextRunAt == 0 {
			agent.NextRunAt = computeNextRunAt(agent, now)
			_ = s.store.UpdateAgentRuntimeState(ctx, agent)
			continue
		}
		if agent.NextRunAt > now {
			continue
		}
		if s.isAgentActive(agent.ID) {
			continue
		}
		_, _ = s.Dispatch(ctx, DispatchRequest{
			AgentID:       agent.ID,
			TriggerReason: "schedule:" + agent.TriggerType,
		})
	}
}

func (s *Service) publish(ev Event) {
	if s.events != nil {
		s.events.Publish(ev)
	}
}

func (s *Service) publishStatus(agent CustomAgent) {
	s.publish(Event{
		Type:      "agent.status",
		AgentID:   agent.ID,
		Payload:   agent,
		Timestamp: time.Now().Unix(),
	})
}

func computeNextRunAt(agent CustomAgent, fromUnix int64) int64 {
	switch agent.TriggerType {
	case TriggerInterval, TriggerMonitor:
		mins := agent.IntervalMinutes
		if mins <= 0 {
			if agent.TriggerType == TriggerMonitor {
				mins = 5
			} else {
				mins = 1
			}
		}
		return fromUnix + int64(mins)*60
	case TriggerCron:
		sched, err := ParseCron(agent.CronExpr)
		if err != nil {
			return fromUnix + 60
		}
		next := sched.NextAfter(time.Unix(fromUnix, 0))
		if next.IsZero() {
			return fromUnix + 60
		}
		return next.Unix()
	default:
		return 0
	}
}

func buildInitialPrompt(agent CustomAgent, userPrompt, monitorDiff string) string {
	trimmedPrompt := strings.TrimSpace(userPrompt)
	if trimmedPrompt != "" && monitorDiff != "" {
		return trimmedPrompt + "\n\nИзменения мониторинга:\n" + monitorDiff
	}
	if trimmedPrompt != "" {
		return trimmedPrompt
	}
	if monitorDiff != "" {
		return "Обнаружены изменения объекта мониторинга:\n" + monitorDiff + "\n\nАктуальное состояние:\n" + truncateText(agent.MonitorLastSnapshot, 4000)
	}
	if agent.MonitorLastSnapshot != "" {
		return "Первичная проверка объекта мониторинга:\n" + truncateText(agent.MonitorLastSnapshot, 4000)
	}
	return "Выполни свою основную задачу согласно системной инструкции."
}

func buildVerifierPrompt(worker CustomAgent, taskPrompt, workerOutput, detReport string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Проверь результат работы агента %q (%s).\n\n", worker.Name, worker.Slug)
	fmt.Fprintf(&b, "Исходная задача:\n%s\n\n", taskPrompt)
	if strings.TrimSpace(worker.AcceptanceCriteria) != "" {
		fmt.Fprintf(&b, "Критерии приёмки:\n%s\n\n", worker.AcceptanceCriteria)
	}
	if detReport != "" {
		fmt.Fprintf(&b, "Отчёт детерминированных проверок:\n%s\n\n", detReport)
	}
	fmt.Fprintf(&b, "Результат исполнителя:\n%s\n\n", workerOutput)
	b.WriteString("Вынеси строгий вердикт: начни ответ с 'VERDICT: PASS' или 'VERDICT: FAIL'.")
	return b.String()
}

func truncateText(s string, maxLen int) string {
	return truncateUTF8(s, maxLen, "\n... (truncated)")
}
