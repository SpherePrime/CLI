package customagents

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
)

type resolvedSwarmWorker struct {
	agent CustomAgent
	task  string
}

func (s *Service) DispatchSwarm(ctx context.Context, req SwarmRequest) (SwarmResult, error) {
	goal := strings.TrimSpace(req.Goal)
	if goal == "" {
		return SwarmResult{}, errors.New("требуется цель роя (goal)")
	}
	if len(req.Workers) == 0 {
		return SwarmResult{}, errors.New("требуется хотя бы один воркер в рое")
	}
	if s.store == nil || s.executor == nil || s.models == nil {
		return SwarmResult{}, errors.New("сервис роя не настроен (отсутствуют хранилище, исполнитель или каталог моделей)")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	chainID := strings.TrimSpace(req.ChainID)
	if chainID == "" {
		chainID = "swarm-" + uuid.NewString()
	}

	persistCtx := context.Background()
	availableModels, err := s.models.ListAvailableModels(ctx)
	if err != nil {
		return SwarmResult{}, fmt.Errorf("не удалось получить список моделей: %w", err)
	}

	resolvedWorkers, err := s.resolveSwarmWorkers(ctx, req)
	if err != nil {
		return SwarmResult{}, err
	}

	res := SwarmResult{
		ChainID:  chainID,
		Goal:     goal,
		Status:   RunRunning,
		Attempts: 1,
		Workers:  make([]SwarmWorkerResult, len(resolvedWorkers)),
	}

	_, _ = s.store.PostBlackboard(persistCtx, BlackboardEntry{
		ChainID:       chainID,
		FromAgentID:   "swarm-coordinator",
		FromAgentName: "Swarm Coordinator",
		Topic:         "swarm.goal",
		Payload:       formatSwarmTopology(goal, resolvedWorkers),
		Status:        "open",
		CreatedAt:     time.Now().Unix(),
	})

	s.publish(Event{
		Type:      "swarm.started",
		ChainID:   chainID,
		Payload:   map[string]any{"goal": goal, "workers": len(resolvedWorkers)},
		Timestamp: time.Now().Unix(),
	})

	workerResults := s.runSwarmWorkersParallel(ctx, persistCtx, chainID, goal, resolvedWorkers, nil, "", availableModels)
	res.Workers = workerResults
	if ctx.Err() != nil {
		res.Status = RunCancelled
		res.Error = ctx.Err().Error()
		res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
		return res, ctx.Err()
	}

	maxAttempts := req.MaxReworkAttempts
	if maxAttempts <= 0 {
		maxAttempts = 2
	}

	if !req.SkipVerify {
		verifierAgent := s.resolveSwarmVerifier(persistCtx, req.VerifierAgentID)

		for attempt := 1; attempt <= maxAttempts; attempt++ {
			res.Attempts = attempt
			if ctx.Err() != nil {
				res.Status = RunCancelled
				res.Error = ctx.Err().Error()
				res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
				return res, ctx.Err()
			}

			s.publish(Event{
				Type:      "swarm.verifying",
				ChainID:   chainID,
				Payload:   map[string]any{"attempt": attempt},
				Timestamp: time.Now().Unix(),
			})

			combinedOutputs := formatSwarmAggregation(workerResults)
			detWorkDir := resolveSwarmWorkDir(resolvedWorkers)
			detPassed, detReport := RunDeterministicChecks(ctx, req.DeterministicChecks, combinedOutputs, detWorkDir)
			res.DeterministicReport = detReport
			if ctx.Err() != nil {
				res.Status = RunCancelled
				res.Error = ctx.Err().Error()
				res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
				return res, ctx.Err()
			}

			var (
				verifiedOK       bool
				verifierFeedback string
			)

			failedWorkerFeedback := collectFailedWorkerFeedback(workerResults)
			if !detPassed {
				verifiedOK = false
				verifierFeedback = "Не пройдены детерминированные проверки роя:\n" + detReport
				if failedWorkerFeedback != "" {
					verifierFeedback += "\n\nОшибки воркеров:\n" + failedWorkerFeedback
				}
			} else if failedWorkerFeedback != "" {
				verifiedOK = false
				verifierFeedback = "Часть воркеров завершилась с ошибкой:\n" + failedWorkerFeedback
			} else {
				bbContext := s.formatBlackboardContext(persistCtx, chainID)
				vPrompt := buildSwarmVerifierPrompt(goal, req.AcceptanceCriteria, detReport, workerResults, bbContext)
				vChoice, vSelErr := SelectModelForTask(availableModels, verifierAgent, vPrompt)
				if vSelErr != nil {
					verifiedOK = false
					verifierFeedback = fmt.Sprintf("Не удалось подобрать модель верификатора роя: %v", vSelErr)
				} else {
					vRes, vExecErr := s.executor.Execute(ctx, ExecuteRequest{
						Agent:             verifierAgent,
						ChainID:           chainID,
						Prompt:            vPrompt,
						MemorySnapshot:    s.buildMemorySnapshot(persistCtx, verifierAgent.ID),
						BlackboardContext: bbContext,
						ProviderID:        vChoice.ProviderID,
						ModelID:           vChoice.ModelID,
						ReasoningMode:     vChoice.ReasoningMode,
					})
					if vExecErr != nil {
						if errors.Is(ctx.Err(), context.Canceled) || ctx.Err() != nil {
							res.Status = RunCancelled
							res.Error = ctx.Err().Error()
							res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
							return res, ctx.Err()
						}
						verifiedOK = false
						verifierFeedback = fmt.Sprintf("Ошибка верификатора роя: %v", vExecErr)
					} else {
						verifiedOK, verifierFeedback = ParseVerifierVerdict(vRes.OutputText)
					}
				}
			}

			verdictLabel := "FAIL"
			if verifiedOK {
				verdictLabel = "PASS"
			}
			res.VerifierVerdict = verdictLabel
			res.VerifierFeedback = verifierFeedback

			_, _ = s.store.PostBlackboard(persistCtx, BlackboardEntry{
				ChainID:       chainID,
				FromAgentID:   verifierAgent.ID,
				FromAgentName: verifierAgent.Name,
				Topic:         "swarm.verdict",
				Payload:       fmt.Sprintf("VERDICT: %s\n%s", verdictLabel, verifierFeedback),
				Status:        "open",
				CreatedAt:     time.Now().Unix(),
			})

			if verifiedOK {
				break
			}

			if attempt < maxAttempts {
				s.publish(Event{
					Type:      "swarm.rework",
					ChainID:   chainID,
					Payload:   map[string]any{"attempt": attempt, "feedback": verifierFeedback},
					Timestamp: time.Now().Unix(),
				})

				reworkIndices := selectSwarmReworkIndices(workerResults, verifierFeedback)
				s.consumeSupersededBlackboard(persistCtx, chainID, workerResults, reworkIndices)
				reworkPromptSuffix := fmt.Sprintf(
					"Предыдущий результат роя отклонён верификатором (попытка #%d).\nЗамечания:\n%s\n\nИсправь замечания в своей части задачи и предоставь полный обновлённый результат.",
					attempt, verifierFeedback,
				)
				updated := s.runSwarmWorkersParallel(ctx, persistCtx, chainID, goal, resolvedWorkers, reworkIndices, reworkPromptSuffix, availableModels)
				for _, idx := range reworkIndices {
					workerResults[idx] = updated[idx]
				}
				res.Workers = workerResults
				if ctx.Err() != nil {
					res.Status = RunCancelled
					res.Error = ctx.Err().Error()
					res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
					return res, ctx.Err()
				}
				continue
			}

			res.Status = RunFailed
			res.Error = "отклонено верификатором роя: " + verifierFeedback
			res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
			s.publish(Event{
				Type:      "swarm.failed",
				ChainID:   chainID,
				Payload:   res,
				Timestamp: time.Now().Unix(),
			})
			return res, nil
		}
	} else {
		if failed := collectFailedWorkerFeedback(workerResults); failed != "" {
			res.Status = RunFailed
			res.Error = failed
			res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
			return res, nil
		}
	}

	if req.SkipSynthesize {
		res.FinalOutput = formatSwarmAggregation(workerResults)
		res.Status = RunPassed
		res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
		s.publish(Event{
			Type:      "swarm.completed",
			ChainID:   chainID,
			Payload:   res,
			Timestamp: time.Now().Unix(),
		})
		return res, nil
	}

	s.publish(Event{
		Type:      "swarm.synthesizing",
		ChainID:   chainID,
		Timestamp: time.Now().Unix(),
	})

	synthAgent := s.resolveSwarmSynthesizer(persistCtx, req.SynthesizerAgentID)
	bbContext := s.formatBlackboardContext(persistCtx, chainID)
	synthPrompt := buildSwarmSynthesizerPrompt(goal, workerResults, bbContext)

	synthChoice, err := SelectModelForTask(availableModels, synthAgent, synthPrompt)
	if err != nil {
		res.Status = RunFailed
		res.Error = fmt.Sprintf("не удалось подобрать модель синтезатора: %v", err)
		res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
		return res, nil
	}

	synthRun := CustomAgentRun{
		ID:                uuid.NewString(),
		AgentID:           synthAgent.ID,
		AgentName:         synthAgent.Name,
		AgentRole:         synthAgent.Role,
		ChainID:           chainID,
		TriggerReason:     "swarm:synthesize",
		InputPrompt:       synthPrompt,
		Status:            RunRunning,
		Attempt:           1,
		ResolvedProvider:  synthChoice.ProviderID,
		ResolvedModel:     synthChoice.ModelID,
		ResolvedReasoning: synthChoice.ReasoningMode,
		AutoModelReason:   synthChoice.Reason,
		StartedAt:         time.Now().Unix(),
	}
	synthRun, _ = s.store.CreateRun(persistCtx, synthRun)

	synthExecRes, synthErr := s.executor.Execute(ctx, ExecuteRequest{
		Agent:             synthAgent,
		ChainID:           chainID,
		RunID:             synthRun.ID,
		Prompt:            synthPrompt,
		MemorySnapshot:    s.buildMemorySnapshot(persistCtx, synthAgent.ID),
		BlackboardContext: bbContext,
		ProviderID:        synthChoice.ProviderID,
		ModelID:           synthChoice.ModelID,
		ReasoningMode:     synthChoice.ReasoningMode,
	})
	if len(synthExecRes.Steps) > 0 {
		synthRun.Steps = append(synthRun.Steps, synthExecRes.Steps...)
	}

	if synthErr != nil {
		now := time.Now().Unix()
		synthRun.FinishedAt = now
		if ctx.Err() != nil {
			synthRun.Status = RunCancelled
			synthRun.Error = ctx.Err().Error()
			_ = s.store.UpdateRun(persistCtx, synthRun)
			res.SynthesizerRun = &synthRun
			res.Status = RunCancelled
			res.Error = ctx.Err().Error()
			res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
			return res, ctx.Err()
		}
		synthRun.Status = RunFailed
		synthRun.Error = synthErr.Error()
		_ = s.store.UpdateRun(persistCtx, synthRun)
		res.SynthesizerRun = &synthRun
		res.Status = RunFailed
		res.Error = "ошибка синтезатора: " + synthErr.Error()
		res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
		return res, nil
	}

	if valErr := ValidateWorkerOutput(synthExecRes.OutputText); valErr != nil {
		now := time.Now().Unix()
		synthRun.Status = RunFailed
		synthRun.Error = valErr.Error()
		synthRun.FinishedAt = now
		_ = s.store.UpdateRun(persistCtx, synthRun)
		res.SynthesizerRun = &synthRun
		res.Status = RunFailed
		res.Error = "некорректный вывод синтезатора: " + valErr.Error()
		res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)
		return res, nil
	}

	now := time.Now().Unix()
	synthRun.OutputText = synthExecRes.OutputText
	synthRun.Status = RunPassed
	synthRun.FinishedAt = now
	_ = s.store.UpdateRun(persistCtx, synthRun)

	_, _ = s.store.PostBlackboard(persistCtx, BlackboardEntry{
		ChainID:       chainID,
		RunID:         synthRun.ID,
		FromAgentID:   synthAgent.ID,
		FromAgentName: synthAgent.Name,
		Topic:         "swarm.final",
		Payload:       synthExecRes.OutputText,
		Status:        "open",
		CreatedAt:     now,
	})

	res.FinalOutput = synthExecRes.OutputText
	res.SynthesizerRun = &synthRun
	res.Status = RunPassed
	res.Blackboard, _ = s.store.ListBlackboard(persistCtx, chainID, 200)

	s.publish(Event{
		Type:      "swarm.completed",
		ChainID:   chainID,
		Payload:   res,
		Timestamp: now,
	})
	return res, nil
}

func (s *Service) resolveSwarmWorkers(ctx context.Context, req SwarmRequest) ([]resolvedSwarmWorker, error) {
	resolved := make([]resolvedSwarmWorker, 0, len(req.Workers))
	for idx, spec := range req.Workers {
		var (
			agent CustomAgent
			found bool
		)
		if id := strings.TrimSpace(spec.AgentID); id != "" {
			if existing, err := s.store.GetAgent(ctx, id); err == nil {
				agent = existing
				found = true
			}
		}
		if !found {
			if slug := strings.TrimSpace(spec.Slug); slug != "" {
				if existing, err := s.store.GetAgentBySlug(ctx, slug); err == nil {
					agent = existing
					found = true
				}
			}
		}

		if !found {
			id := strings.TrimSpace(spec.AgentID)
			if id == "" {
				id = "swarm-worker-" + uuid.NewString()[:8]
			}
			slug := strings.TrimSpace(spec.Slug)
			if slug == "" {
				slug = normalizeSlug("", spec.Name, id)
				if spec.Name == "" {
					slug = fmt.Sprintf("swarm-worker-%d-%s", idx+1, id[len(id)-min(4, len(id)):])
				}
			}
			name := cmp.Or(strings.TrimSpace(spec.Name), slug)
			role := cmp.Or(strings.TrimSpace(spec.Role), RoleWorker)
			modelMode := cmp.Or(strings.TrimSpace(spec.ModelMode), ModelModeAuto)
			maxRework := spec.MaxReworkAttempts
			if maxRework <= 0 {
				maxRework = 2
			}
			agent = CustomAgent{
				ID:                  id,
				Slug:                slug,
				Name:                name,
				Role:                role,
				SystemPrompt:        spec.SystemPrompt,
				ModelMode:           modelMode,
				ProviderID:          spec.ProviderID,
				ModelID:             spec.ModelID,
				ReasoningMode:       spec.ReasoningMode,
				AutoModelHint:       spec.AutoModelHint,
				AllowedTools:        slices.Clone(spec.AllowedTools),
				WorkspacePath:       spec.WorkspacePath,
				Enabled:             true,
				CreatedBy:           "swarm",
				TriggerType:         TriggerManual,
				AcceptanceCriteria:  spec.AcceptanceCriteria,
				DeterministicChecks: slices.Clone(spec.DeterministicChecks),
				VerifierAgentID:     spec.VerifierAgentID,
				MaxReworkAttempts:   maxRework,
				StrictVerify:        spec.StrictVerify,
				Status:              StatusIdle,
			}
			if req.PersistWorkers {
				persisted, err := s.store.CreateAgent(ctx, agent)
				if err != nil {
					return nil, fmt.Errorf("не удалось сохранить воркера роя %s: %w", agent.Slug, err)
				}
				agent = persisted
			}
		} else {
			if strings.TrimSpace(spec.SystemPrompt) != "" {
				agent.SystemPrompt = spec.SystemPrompt
			}
			if strings.TrimSpace(spec.ModelMode) != "" {
				agent.ModelMode = spec.ModelMode
			}
			if strings.TrimSpace(spec.ProviderID) != "" {
				agent.ProviderID = spec.ProviderID
			}
			if strings.TrimSpace(spec.ModelID) != "" {
				agent.ModelID = spec.ModelID
			}
			if strings.TrimSpace(spec.ReasoningMode) != "" {
				agent.ReasoningMode = spec.ReasoningMode
			}
			if strings.TrimSpace(spec.AutoModelHint) != "" {
				agent.AutoModelHint = spec.AutoModelHint
			}
			if len(spec.AllowedTools) > 0 {
				agent.AllowedTools = slices.Clone(spec.AllowedTools)
			}
			if strings.TrimSpace(spec.WorkspacePath) != "" {
				agent.WorkspacePath = spec.WorkspacePath
			}
			if strings.TrimSpace(spec.AcceptanceCriteria) != "" {
				agent.AcceptanceCriteria = spec.AcceptanceCriteria
			}
			if len(spec.DeterministicChecks) > 0 {
				agent.DeterministicChecks = slices.Clone(spec.DeterministicChecks)
			}
			if strings.TrimSpace(spec.VerifierAgentID) != "" {
				agent.VerifierAgentID = spec.VerifierAgentID
			}
			if spec.MaxReworkAttempts > 0 {
				agent.MaxReworkAttempts = spec.MaxReworkAttempts
			}
			if spec.StrictVerify {
				agent.StrictVerify = true
			}
		}

		task := cmp.Or(strings.TrimSpace(spec.Task), strings.TrimSpace(spec.Prompt))
		if task == "" {
			task = strings.TrimSpace(req.Goal)
		}
		resolved = append(resolved, resolvedSwarmWorker{
			agent: agent,
			task:  task,
		})
	}
	return resolved, nil
}

func (s *Service) runSwarmWorkersParallel(
	ctx context.Context,
	persistCtx context.Context,
	chainID string,
	goal string,
	workers []resolvedSwarmWorker,
	indices []int,
	reworkNote string,
	availableModels []ModelOption,
) []SwarmWorkerResult {
	results := make([]SwarmWorkerResult, len(workers))
	targetIndices := indices
	if len(targetIndices) == 0 {
		targetIndices = make([]int, len(workers))
		for i := range workers {
			targetIndices[i] = i
		}
	}

	var wg sync.WaitGroup
	for _, idx := range targetIndices {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i] = s.executeSwarmWorker(ctx, persistCtx, chainID, goal, workers[i], reworkNote, availableModels)
		}(idx)
	}
	wg.Wait()
	return results
}

func (s *Service) executeSwarmWorker(
	ctx context.Context,
	persistCtx context.Context,
	chainID string,
	goal string,
	w resolvedSwarmWorker,
	reworkNote string,
	availableModels []ModelOption,
) SwarmWorkerResult {
	agent := w.agent
	basePrompt := w.task
	if w.task != goal {
		basePrompt = fmt.Sprintf("Общая цель роя:\n%s\n\nТвоя подзадача:\n%s", goal, w.task)
	}
	if strings.TrimSpace(reworkNote) != "" {
		basePrompt += "\n\n" + reworkNote
	}

	choice, selErr := SelectModelForTask(availableModels, agent, basePrompt)
	if selErr != nil {
		return SwarmWorkerResult{
			AgentID: agent.ID,
			Slug:    agent.Slug,
			Name:    agent.Name,
			Task:    w.task,
			Status:  RunFailed,
			Error:   selErr.Error(),
		}
	}

	run := CustomAgentRun{
		ID:                uuid.NewString(),
		AgentID:           agent.ID,
		AgentName:         agent.Name,
		AgentRole:         agent.Role,
		ChainID:           chainID,
		TriggerReason:     "swarm:worker",
		InputPrompt:       basePrompt,
		Status:            RunRunning,
		Attempt:           1,
		ResolvedProvider:  choice.ProviderID,
		ResolvedModel:     choice.ModelID,
		ResolvedReasoning: choice.ReasoningMode,
		AutoModelReason:   choice.Reason,
		StartedAt:         time.Now().Unix(),
	}
	run, _ = s.store.CreateRun(persistCtx, run)

	maxAttempts := agent.MaxReworkAttempts
	if maxAttempts <= 0 {
		maxAttempts = 2
	}
	needsWorkerVerify := agent.Role != RoleVerifier &&
		(agent.StrictVerify ||
			len(agent.DeterministicChecks) > 0 ||
			strings.TrimSpace(agent.AcceptanceCriteria) != "" ||
			strings.TrimSpace(agent.VerifierAgentID) != "")

	memorySnapshot := s.buildMemorySnapshot(persistCtx, agent.ID)
	currentPrompt := basePrompt

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			run.Status = RunCancelled
			run.Error = ctx.Err().Error()
			run.FinishedAt = time.Now().Unix()
			_ = s.store.UpdateRun(persistCtx, run)
			return SwarmWorkerResult{
				AgentID: agent.ID,
				Slug:    agent.Slug,
				Name:    agent.Name,
				Task:    w.task,
				Run:     run,
				Status:  RunCancelled,
				Error:   run.Error,
			}
		}

		run.Attempt = attempt
		run.Status = RunRunning
		run.VerifierVerdict = ""
		run.VerifierFeedback = ""
		run.DeterministicReport = ""
		_ = s.store.UpdateRun(persistCtx, run)

		bbContext := s.formatBlackboardContext(persistCtx, chainID)
		execRes, execErr := s.executor.Execute(ctx, ExecuteRequest{
			Agent:             agent,
			ChainID:           chainID,
			RunID:             run.ID,
			Prompt:            currentPrompt,
			MemorySnapshot:    memorySnapshot,
			BlackboardContext: bbContext,
			ProviderID:        choice.ProviderID,
			ModelID:           choice.ModelID,
			ReasoningMode:     choice.ReasoningMode,
		})
		if len(execRes.Steps) > 0 {
			run.Steps = append(run.Steps, execRes.Steps...)
		}

		if execErr != nil {
			run.FinishedAt = time.Now().Unix()
			if ctx.Err() != nil {
				run.Status = RunCancelled
				run.Error = ctx.Err().Error()
				_ = s.store.UpdateRun(persistCtx, run)
				return SwarmWorkerResult{
					AgentID: agent.ID,
					Slug:    agent.Slug,
					Name:    agent.Name,
					Task:    w.task,
					Run:     run,
					Status:  RunCancelled,
					Error:   run.Error,
				}
			}
			run.Status = RunFailed
			run.Error = execErr.Error()
			_ = s.store.UpdateRun(persistCtx, run)
			return SwarmWorkerResult{
				AgentID: agent.ID,
				Slug:    agent.Slug,
				Name:    agent.Name,
				Task:    w.task,
				Run:     run,
				Status:  RunFailed,
				Error:   run.Error,
			}
		}

		run.OutputText = execRes.OutputText

		if valErr := ValidateWorkerOutput(execRes.OutputText); valErr != nil {
			guardFeedback := "Вывод исполнителя не прошёл проверку качества: " + valErr.Error()
			run.VerifierVerdict = "FAIL"
			run.VerifierFeedback = guardFeedback
			run.Steps = append(run.Steps, RunStep{
				Kind:      "output_guard",
				Title:     "Проверка вывода не пройдена",
				Detail:    valErr.Error(),
				Status:    "failed",
				Timestamp: time.Now().Unix(),
			})
			if attempt < maxAttempts {
				currentPrompt = fmt.Sprintf("%s\n\nПредыдущая попытка #%d отклонена проверкой качества.\nЗамечания:\n%s\n\nИсправь замечания и предоставь полный обновлённый результат.", basePrompt, attempt, guardFeedback)
				continue
			}
			run.Status = RunFailed
			run.Error = "отклонено проверкой вывода: " + valErr.Error()
			run.FinishedAt = time.Now().Unix()
			_ = s.store.UpdateRun(persistCtx, run)
			return SwarmWorkerResult{
				AgentID: agent.ID,
				Slug:    agent.Slug,
				Name:    agent.Name,
				Task:    w.task,
				Run:     run,
				Output:  run.OutputText,
				Status:  RunFailed,
				Error:   run.Error,
			}
		}

		if !needsWorkerVerify {
			run.Status = RunPassed
			break
		}

		detPassed, detReport := RunDeterministicChecks(ctx, agent.DeterministicChecks, execRes.OutputText, agent.WorkspacePath)
		run.DeterministicReport = detReport

		var (
			verifiedOK       bool
			verifierFeedback string
		)
		if !detPassed {
			verifiedOK = false
			verifierFeedback = "Не пройдены детерминированные проверки:\n" + detReport
		} else {
			needsLLMVerifier := agent.StrictVerify || strings.TrimSpace(agent.AcceptanceCriteria) != "" || strings.TrimSpace(agent.VerifierAgentID) != ""
			if !needsLLMVerifier {
				verifiedOK = true
				verifierFeedback = "Детерминированные проверки пройдены."
			} else {
				vAgent := s.resolveVerifierAgent(persistCtx, agent)
				vPrompt := buildVerifierPrompt(agent, basePrompt, execRes.OutputText, detReport)
				vChoice, vSelErr := SelectModelForTask(availableModels, vAgent, vPrompt)
				if vSelErr != nil {
					verifiedOK = false
					verifierFeedback = fmt.Sprintf("Не удалось подобрать модель верификатора: %v", vSelErr)
				} else {
					vRes, vExecErr := s.executor.Execute(ctx, ExecuteRequest{
						Agent:             vAgent,
						ChainID:           chainID,
						RunID:             run.ID,
						Prompt:            vPrompt,
						BlackboardContext: bbContext,
						ProviderID:        vChoice.ProviderID,
						ModelID:           vChoice.ModelID,
						ReasoningMode:     vChoice.ReasoningMode,
					})
					if vExecErr != nil {
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
			break
		}

		run.VerifierVerdict = "FAIL"
		run.VerifierFeedback = verifierFeedback
		if attempt < maxAttempts {
			currentPrompt = fmt.Sprintf("%s\n\nПредыдущая попытка #%d отклонена верификатором.\nЗамечания:\n%s\n\nИсправь все замечания и предоставь полный обновлённый результат.", basePrompt, attempt, verifierFeedback)
			continue
		}
		run.Status = RunFailed
		run.Error = "отклонено верификатором: " + verifierFeedback
		run.FinishedAt = time.Now().Unix()
		_ = s.store.UpdateRun(persistCtx, run)
		return SwarmWorkerResult{
			AgentID: agent.ID,
			Slug:    agent.Slug,
			Name:    agent.Name,
			Task:    w.task,
			Run:     run,
			Output:  run.OutputText,
			Status:  RunFailed,
			Error:   run.Error,
		}
	}

	now := time.Now().Unix()
	run.Status = RunPassed
	run.FinishedAt = now
	_ = s.store.UpdateRun(persistCtx, run)

	_, _ = s.store.PostBlackboard(persistCtx, BlackboardEntry{
		ChainID:       chainID,
		RunID:         run.ID,
		FromAgentID:   agent.ID,
		FromAgentName: agent.Name,
		Topic:         "worker.result",
		Payload:       run.OutputText,
		Status:        "open",
		CreatedAt:     now,
	})

	s.publish(Event{
		Type:      "swarm.worker_done",
		AgentID:   agent.ID,
		RunID:     run.ID,
		ChainID:   chainID,
		Payload:   run,
		Timestamp: now,
	})

	return SwarmWorkerResult{
		AgentID: agent.ID,
		Slug:    agent.Slug,
		Name:    agent.Name,
		Task:    w.task,
		Run:     run,
		Output:  run.OutputText,
		Status:  RunPassed,
	}
}

func (s *Service) resolveSwarmVerifier(ctx context.Context, verifierID string) CustomAgent {
	if id := strings.TrimSpace(verifierID); id != "" {
		if a, err := s.store.GetAgent(ctx, id); err == nil {
			return a
		}
		if a, err := s.store.GetAgentBySlug(ctx, id); err == nil {
			return a
		}
	}
	return s.resolveVerifierAgent(ctx, CustomAgent{Role: RoleOrchestrator})
}

func (s *Service) resolveSwarmSynthesizer(ctx context.Context, synthesizerID string) CustomAgent {
	if id := strings.TrimSpace(synthesizerID); id != "" {
		if a, err := s.store.GetAgent(ctx, id); err == nil {
			return a
		}
		if a, err := s.store.GetAgentBySlug(ctx, id); err == nil {
			return a
		}
	}
	return CustomAgent{
		ID:            "swarm-synthesizer",
		Slug:          "swarm-synthesizer",
		Name:          "Синтезатор роя",
		Role:          RoleOrchestrator,
		SystemPrompt:  defaultOrchestratorPrompt,
		ModelMode:     ModelModeAuto,
		AutoModelHint: TierBalanced,
		Enabled:       true,
	}
}

func resolveSwarmWorkDir(workers []resolvedSwarmWorker) string {
	for _, w := range workers {
		if strings.TrimSpace(w.agent.WorkspacePath) != "" {
			return w.agent.WorkspacePath
		}
	}
	return ""
}

func collectFailedWorkerFeedback(results []SwarmWorkerResult) string {
	var failed []string
	for _, r := range results {
		if r.Status != RunPassed {
			label := cmp.Or(r.Slug, r.Name, r.AgentID)
			failed = append(failed, fmt.Sprintf("- %s (%s): %s", label, r.Status, r.Error))
		}
	}
	return strings.Join(failed, "\n")
}

func (s *Service) consumeSupersededBlackboard(ctx context.Context, chainID string, results []SwarmWorkerResult, reworkIndices []int) {
	if s.store == nil || len(reworkIndices) == 0 {
		return
	}
	entries, err := s.store.ListBlackboard(ctx, chainID, 500)
	if err != nil || len(entries) == 0 {
		return
	}
	reworkAgents := make(map[string]bool, len(reworkIndices))
	reworkRuns := make(map[string]bool, len(reworkIndices))
	for _, idx := range reworkIndices {
		if idx < 0 || idx >= len(results) {
			continue
		}
		if results[idx].AgentID != "" {
			reworkAgents[results[idx].AgentID] = true
		}
		if results[idx].Run.ID != "" {
			reworkRuns[results[idx].Run.ID] = true
		}
	}
	for _, e := range entries {
		if e.Status != "open" {
			continue
		}
		if e.Topic == "swarm.verdict" ||
			(e.Topic == "worker.result" && (reworkRuns[e.RunID] || reworkAgents[e.FromAgentID])) {
			_ = s.store.MarkBlackboardConsumed(ctx, e.ID)
		}
	}
}

func selectSwarmReworkIndices(results []SwarmWorkerResult, feedback string) []int {
	lower := strings.ToLower(feedback)
	var selected []int
	for i, r := range results {
		if r.Status != RunPassed {
			selected = append(selected, i)
			continue
		}
		if (r.Slug != "" && strings.Contains(lower, strings.ToLower(r.Slug))) ||
			(r.Name != "" && strings.Contains(lower, strings.ToLower(r.Name))) ||
			(r.AgentID != "" && strings.Contains(lower, strings.ToLower(r.AgentID))) {
			selected = append(selected, i)
		}
	}
	if len(selected) == 0 {
		for i := range results {
			selected = append(selected, i)
		}
	}
	return selected
}

func formatSwarmTopology(goal string, workers []resolvedSwarmWorker) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Цель: %s\nВоркеры (%d):\n", goal, len(workers))
	for i, w := range workers {
		fmt.Fprintf(&b, "%d. %s (%s): %s\n", i+1, w.agent.Name, w.agent.Slug, w.task)
	}
	return strings.TrimSpace(b.String())
}

func formatSwarmAggregation(results []SwarmWorkerResult) string {
	var b strings.Builder
	for _, r := range results {
		title := cmp.Or(r.Name, r.Slug, r.AgentID)
		fmt.Fprintf(&b, "## %s\n%s\n\n", title, strings.TrimSpace(r.Output))
	}
	return strings.TrimSpace(b.String())
}

func buildSwarmVerifierPrompt(goal, criteria, detReport string, results []SwarmWorkerResult, bbContext string) string {
	var b strings.Builder
	b.WriteString("Проверь совокупный результат работы роя агентов.\n\n")
	fmt.Fprintf(&b, "Общая цель роя:\n%s\n\n", goal)
	if strings.TrimSpace(criteria) != "" {
		fmt.Fprintf(&b, "Критерии приёмки:\n%s\n\n", criteria)
	} else {
		fmt.Fprintf(&b, "%s\n\n", strictCompletenessChecklist())
	}
	if detReport != "" {
		fmt.Fprintf(&b, "Отчёт детерминированных проверок:\n%s\n\n", detReport)
	}
	b.WriteString("Результаты воркеров:\n")
	for _, r := range results {
		fmt.Fprintf(&b, "### %s (%s) [%s]\nЗадача: %s\nВывод:\n%s\n\n", r.Name, r.Slug, r.Status, r.Task, r.Output)
	}
	if strings.TrimSpace(bbContext) != "" {
		fmt.Fprintf(&b, "Содержимое Blackboard:\n%s\n\n", bbContext)
	}
	b.WriteString("Вынеси строгий вердикт: начни ответ с 'VERDICT: PASS' или 'VERDICT: FAIL'.")
	return b.String()
}

func buildSwarmSynthesizerPrompt(goal string, results []SwarmWorkerResult, bbContext string) string {
	var b strings.Builder
	b.WriteString("Объедини проверенные результаты работы воркеров роя в единый, точный и лаконичный итоговый ответ.\n\n")
	fmt.Fprintf(&b, "Цель:\n%s\n\n", goal)
	b.WriteString("Проверенные результаты воркеров:\n")
	for _, r := range results {
		fmt.Fprintf(&b, "### %s (%s)\n%s\n\n", r.Name, r.Slug, r.Output)
	}
	if strings.TrimSpace(bbContext) != "" {
		fmt.Fprintf(&b, "Контекст Blackboard:\n%s\n\n", bbContext)
	}
	return strings.TrimSpace(b.String())
}
