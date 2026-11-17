package customagents

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type mockModelCatalog struct {
	models []ModelOption
	err    error
}

func (m *mockModelCatalog) ListAvailableModels(_ context.Context) ([]ModelOption, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.models, nil
}

type mockExecutor struct {
	mu       sync.Mutex
	calls    []ExecuteRequest
	handler  func(ctx context.Context, req ExecuteRequest, callIndex int) (ExecuteResult, error)
}

func (m *mockExecutor) Execute(ctx context.Context, req ExecuteRequest) (ExecuteResult, error) {
	m.mu.Lock()
	idx := len(m.calls)
	m.calls = append(m.calls, req)
	h := m.handler
	m.mu.Unlock()

	if h != nil {
		return h(ctx, req, idx)
	}
	return ExecuteResult{OutputText: "done"}, nil
}

func (m *mockExecutor) snapshotCalls() []ExecuteRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ExecuteRequest, len(m.calls))
	copy(out, m.calls)
	return out
}

type mockEventSink struct {
	mu     sync.Mutex
	events []Event
}

func (s *mockEventSink) Publish(ev Event) {
	s.mu.Lock()
	s.events = append(s.events, ev)
	s.mu.Unlock()
}

func testCatalog() []ModelOption {
	return []ModelOption{
		{
			ProviderID:           "p1",
			ProviderName:         "Provider One",
			ModelID:              "fast-mini",
			Name:                 "Fast Mini",
			Tier:                 TierFast,
			CanReason:            true,
			ReasoningModes:       []string{"off", "low", "medium"},
			DefaultReasoningMode: "low",
			ContextWindow:        64000,
			CostPer1MIn:          0.15,
			CostPer1MOut:         0.6,
		},
		{
			ProviderID:           "p1",
			ProviderName:         "Provider One",
			ModelID:              "balanced-std",
			Name:                 "Balanced Standard",
			Tier:                 TierBalanced,
			CanReason:            true,
			ReasoningModes:       []string{"off", "low", "medium", "high"},
			DefaultReasoningMode: "medium",
			ContextWindow:        128000,
			CostPer1MIn:          1.5,
			CostPer1MOut:         6.0,
		},
		{
			ProviderID:           "p1",
			ProviderName:         "Provider One",
			ModelID:              "reason-pro",
			Name:                 "Reasoning Pro",
			Tier:                 TierReasoning,
			CanReason:            true,
			ReasoningModes:       []string{"low", "medium", "high"},
			DefaultReasoningMode: "high",
			ContextWindow:        200000,
			CostPer1MIn:          3.0,
			CostPer1MOut:         15.0,
		},
		{
			ProviderID:           "p2",
			ProviderName:         "Provider Two",
			ModelID:              "flagship-max",
			Name:                 "Flagship Max",
			Tier:                 TierFlagship,
			CanReason:            true,
			ReasoningModes:       []string{"medium", "high", "xhigh"},
			DefaultReasoningMode: "high",
			ContextWindow:        400000,
			CostPer1MIn:          15.0,
			CostPer1MOut:         60.0,
			Locked:               true,
			PlanLabel:            "Enterprise",
		},
	}
}

func TestStoreCRUDDefaultsAndMemoryPersistence(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()

	store, err := OpenStore(dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	require.NoError(t, store.EnsureDefaults(ctx))
	require.NoError(t, store.EnsureDefaults(ctx))

	agents, err := store.ListAgents(ctx)
	require.NoError(t, err)
	require.Len(t, agents, 2)

	orch, err := store.GetAgentBySlug(ctx, "orchestrator")
	require.NoError(t, err)
	require.Equal(t, RoleOrchestrator, orch.Role)
	require.Equal(t, "Главный координатор", orch.Name)
	require.True(t, orch.Enabled)

	verifier, err := store.GetAgentBySlug(ctx, "strict-verifier")
	require.NoError(t, err)
	require.Equal(t, RoleVerifier, verifier.Role)
	require.Equal(t, "Строгий верификатор", verifier.Name)

	created, err := store.CreateAgent(ctx, CustomAgent{
		Slug:         "log-watcher",
		Name:         "Наблюдатель логов",
		Role:         RoleWorker,
		SystemPrompt: "Следи за ошибками",
		Memory:       "initial fact",
		AllowedTools: []string{"view", "grep"},
		DeterministicChecks: []DeterministicCheck{
			{ID: "c1", Kind: CheckRegex, Pattern: "OK", Required: true},
		},
		Enabled: true,
	})
	require.NoError(t, err)
	require.NotEmpty(t, created.ID)

	memFile := filepath.Join(dir, "memories", created.ID+".md")
	diskBytes, err := os.ReadFile(memFile)
	require.NoError(t, err)
	require.Equal(t, "initial fact", string(diskBytes))

	require.NoError(t, store.AppendMemory(ctx, created.ID, "second fact"))
	mem, err := store.ReadMemory(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "initial fact\n\nsecond fact", mem)

	reloaded, err := store.GetAgent(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "initial fact\n\nsecond fact", reloaded.Memory)
	require.Equal(t, []string{"view", "grep"}, reloaded.AllowedTools)
	require.Len(t, reloaded.DeterministicChecks, 1)

	reloaded.Description = "updated desc"
	updated, err := store.UpdateAgent(ctx, reloaded)
	require.NoError(t, err)
	require.Equal(t, "updated desc", updated.Description)

	entry, err := store.PostBlackboard(ctx, BlackboardEntry{
		ChainID:       "chain-1",
		RunID:         "run-1",
		FromAgentID:   created.ID,
		FromAgentName: created.Name,
		ToAgentID:     orch.ID,
		Topic:         "report",
		Payload:       "all clear",
	})
	require.NoError(t, err)
	bbList, err := store.ListBlackboard(ctx, "chain-1", 10)
	require.NoError(t, err)
	require.Len(t, bbList, 1)
	require.Equal(t, "open", bbList[0].Status)

	require.NoError(t, store.MarkBlackboardConsumed(ctx, entry.ID))
	bbList, err = store.ListBlackboard(ctx, "chain-1", 10)
	require.NoError(t, err)
	require.Equal(t, "consumed", bbList[0].Status)

	require.NoError(t, store.DeleteAgent(ctx, created.ID))
	_, err = os.Stat(memFile)
	require.True(t, errors.Is(err, os.ErrNotExist))
	_, err = store.GetAgent(ctx, created.ID)
	require.True(t, errors.Is(err, sql.ErrNoRows))
}

func TestSelectModelForTask(t *testing.T) {
	models := testCatalog()

	monitorAgent := CustomAgent{
		Role:        RoleWorker,
		ModelMode:   ModelModeAuto,
		TriggerType: TriggerMonitor,
	}
	fastChoice, err := SelectModelForTask(models, monitorAgent, "проверь статус сервиса")
	require.NoError(t, err)
	require.Equal(t, "fast-mini", fastChoice.ModelID)
	require.Equal(t, "low", fastChoice.ReasoningMode)
	require.Equal(t, TierFast, fastChoice.Tier)
	require.NotEmpty(t, fastChoice.Reason)

	workerAgent := CustomAgent{
		Role:        RoleWorker,
		ModelMode:   ModelModeAuto,
		TriggerType: TriggerManual,
	}
	balancedChoice, err := SelectModelForTask(models, workerAgent, "подготовь еженедельный отчёт по продажам")
	require.NoError(t, err)
	require.Equal(t, "balanced-std", balancedChoice.ModelID)
	require.Equal(t, "medium", balancedChoice.ReasoningMode)
	require.Equal(t, TierBalanced, balancedChoice.Tier)

	verifierAgent := CustomAgent{
		Role:      RoleVerifier,
		ModelMode: ModelModeAuto,
	}
	reasonChoice, err := SelectModelForTask(models, verifierAgent, "проверь соответствие критериям")
	require.NoError(t, err)
	require.Equal(t, "reason-pro", reasonChoice.ModelID)
	require.Equal(t, "high", reasonChoice.ReasoningMode)
	require.Equal(t, TierReasoning, reasonChoice.Tier)

	flagshipAgent := CustomAgent{
		Role:          RoleOrchestrator,
		ModelMode:     ModelModeAuto,
		AutoModelHint: TierFlagship,
	}
	fallbackFromLocked, err := SelectModelForTask(models, flagshipAgent, "архитектура и безопасность")
	require.NoError(t, err)
	require.NotEqual(t, "flagship-max", fallbackFromLocked.ModelID)
	require.Equal(t, "reason-pro", fallbackFromLocked.ModelID)
	require.Equal(t, "high", fallbackFromLocked.ReasoningMode)

	manualAgent := CustomAgent{
		Role:          RoleWorker,
		ModelMode:     ModelModeManual,
		ProviderID:    "p1",
		ModelID:       "balanced-std",
		ReasoningMode: "low",
	}
	manualChoice, err := SelectModelForTask(models, manualAgent, "любая задача")
	require.NoError(t, err)
	require.Equal(t, "p1", manualChoice.ProviderID)
	require.Equal(t, "balanced-std", manualChoice.ModelID)
	require.Equal(t, "low", manualChoice.ReasoningMode)

	manualLocked := CustomAgent{
		Role:       RoleWorker,
		ModelMode:  ModelModeManual,
		ProviderID: "p2",
		ModelID:    "flagship-max",
	}
	manualFallbackChoice, err := SelectModelForTask(models, manualLocked, "проверь статус")
	require.NoError(t, err)
	require.NotEqual(t, "flagship-max", manualFallbackChoice.ModelID)
	require.Contains(t, manualFallbackChoice.Reason, "недоступна или заблокирована")
}

func TestCronParserAndNextAfter(t *testing.T) {
	_, err := ParseCron("invalid")
	require.Error(t, err)
	_, err = ParseCron("65 * * * *")
	require.Error(t, err)
	_, err = ParseCron("*/0 * * * *")
	require.Error(t, err)

	sched, err := ParseCron("15,45 9-17 * * 1-5")
	require.NoError(t, err)

	fridayLate := time.Date(2026, 4, 17, 17, 45, 0, 0, time.UTC)
	next := sched.NextAfter(fridayLate)
	expectedMonday := time.Date(2026, 4, 20, 9, 15, 0, 0, time.UTC)
	require.Equal(t, expectedMonday, next)

	stepSched, err := ParseCron("*/10 * * * *")
	require.NoError(t, err)
	base := time.Date(2026, 4, 17, 10, 3, 25, 0, time.UTC)
	require.Equal(t, time.Date(2026, 4, 17, 10, 10, 0, 0, time.UTC), stepSched.NextAfter(base))
}

func TestMonitorModeFileAndHTTPProbes(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	watchedFile := filepath.Join(t.TempDir(), "app.log")
	require.NoError(t, os.WriteFile(watchedFile, []byte("line1\nline2\n"), 0o600))

	exec := &mockExecutor{
		handler: func(_ context.Context, req ExecuteRequest, _ int) (ExecuteResult, error) {
			return ExecuteResult{OutputText: "обработано: " + req.Prompt}, nil
		},
	}
	svc := NewService(store, exec, &mockModelCatalog{models: testCatalog()}, &mockEventSink{})

	agent, err := store.CreateAgent(ctx, CustomAgent{
		Slug:          "file-monitor",
		Name:          "Монитор файла",
		Role:          RoleWorker,
		ModelMode:     ModelModeAuto,
		TriggerType:   TriggerMonitor,
		MonitorKind:   MonitorFile,
		MonitorTarget: watchedFile,
		Enabled:       true,
	})
	require.NoError(t, err)

	run1, err := svc.DispatchSync(ctx, DispatchRequest{AgentID: agent.ID})
	require.NoError(t, err)
	require.Equal(t, RunPassed, run1.Status)
	require.Len(t, exec.snapshotCalls(), 1)

	run2, err := svc.DispatchSync(ctx, DispatchRequest{AgentID: agent.ID})
	require.NoError(t, err)
	require.Equal(t, RunSkippedNoChange, run2.Status)
	require.Len(t, exec.snapshotCalls(), 1)

	require.NoError(t, os.WriteFile(watchedFile, []byte("line1\nline2\nERROR panic\n"), 0o600))
	run3, err := svc.DispatchSync(ctx, DispatchRequest{AgentID: agent.ID})
	require.NoError(t, err)
	require.Equal(t, RunPassed, run3.Status)
	require.Contains(t, run3.MonitorDiff, "+ERROR panic")
	require.Len(t, exec.snapshotCalls(), 2)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("status=ok"))
	}))
	defer srv.Close()

	httpProbe := Probe(ctx, CustomAgent{
		MonitorKind:   MonitorHTTP,
		MonitorTarget: srv.URL,
	})
	require.Empty(t, httpProbe.Error)
	require.True(t, httpProbe.Changed)
	require.Contains(t, httpProbe.Snapshot, "status=ok")

	blockedProbe := Probe(ctx, CustomAgent{
		MonitorKind:   MonitorHTTP,
		MonitorTarget: "http://169.254.169.254/latest/meta-data/",
	})
	require.NotEmpty(t, blockedProbe.Error)
}

func TestDeterministicChecksShortCircuitBeforeLLMVerifier(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.EnsureDefaults(ctx))

	exec := &mockExecutor{
		handler: func(_ context.Context, req ExecuteRequest, _ int) (ExecuteResult, error) {
			if req.Agent.Role == RoleVerifier {
				return ExecuteResult{}, errors.New("LLM verifier must not be called when deterministic checks fail")
			}
			return ExecuteResult{OutputText: "невалидный текст без JSON и маркера"}, nil
		},
	}
	svc := NewService(store, exec, &mockModelCatalog{models: testCatalog()}, &mockEventSink{})

	worker, err := store.CreateAgent(ctx, CustomAgent{
		Slug:               "strict-worker",
		Name:               "Строгий исполнитель",
		Role:               RoleWorker,
		AcceptanceCriteria: "Должен быть валидный JSON и слово DONE",
		MaxReworkAttempts:  1,
		DeterministicChecks: []DeterministicCheck{
			{ID: "has-done", Kind: CheckRegex, Pattern: "DONE", Required: true},
			{ID: "has-json", Kind: CheckJSONFields, Pattern: "status,result.id", Required: true},
		},
		Enabled: true,
	})
	require.NoError(t, err)

	run, err := svc.DispatchSync(ctx, DispatchRequest{AgentID: worker.ID, Prompt: "сгенерируй отчёт"})
	require.NoError(t, err)
	require.Equal(t, RunFailed, run.Status)
	require.Equal(t, "FAIL", run.VerifierVerdict)
	require.Contains(t, run.DeterministicReport, "[FAIL] has-done")
	require.Contains(t, run.DeterministicReport, "[FAIL] has-json")

	calls := exec.snapshotCalls()
	require.Len(t, calls, 1)
	require.Equal(t, RoleWorker, calls[0].Agent.Role)
}

func TestVerifierReworkLoopAndChainHandoff(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.EnsureDefaults(ctx))

	var workerAttempts int
	exec := &mockExecutor{
		handler: func(_ context.Context, req ExecuteRequest, _ int) (ExecuteResult, error) {
			switch req.Agent.Slug {
			case "worker-a":
				workerAttempts++
				if workerAttempts == 1 {
					return ExecuteResult{OutputText: "черновик без деталей"}, nil
				}
				return ExecuteResult{OutputText: "полный отчёт с деталями"}, nil
			case "strict-verifier":
				if strings.Contains(req.Prompt, "черновик без деталей") {
					return ExecuteResult{OutputText: "VERDICT: FAIL\n1. Добавьте конкретные детали."}, nil
				}
				return ExecuteResult{OutputText: "VERDICT: PASS\nВсе требования выполнены."}, nil
			case "worker-b":
				return ExecuteResult{OutputText: "финал: " + req.BlackboardContext}, nil
			default:
				return ExecuteResult{OutputText: "ok"}, nil
			}
		},
	}

	sink := &mockEventSink{}
	svc := NewService(store, exec, &mockModelCatalog{models: testCatalog()}, sink)

	workerB, err := store.CreateAgent(ctx, CustomAgent{
		Slug:    "worker-b",
		Name:    "Получатель цепочки",
		Role:    RoleWorker,
		Enabled: true,
	})
	require.NoError(t, err)

	workerA, err := store.CreateAgent(ctx, CustomAgent{
		Slug:               "worker-a",
		Name:               "Первый исполнитель",
		Role:               RoleWorker,
		AcceptanceCriteria: "Должен быть полный отчёт с деталями",
		NextAgentID:        workerB.ID,
		MaxReworkAttempts:  3,
		Enabled:            true,
	})
	require.NoError(t, err)

	runA, err := svc.DispatchSync(ctx, DispatchRequest{
		AgentID: workerA.ID,
		Prompt:  "подготовь отчёт",
		ChainID: "chain-pass",
	})
	require.NoError(t, err)
	require.Equal(t, RunPassed, runA.Status)
	require.Equal(t, 2, runA.Attempt)
	require.Equal(t, "PASS", runA.VerifierVerdict)

	bb, err := store.ListBlackboard(ctx, "chain-pass", 10)
	require.NoError(t, err)
	require.Len(t, bb, 1)
	require.Equal(t, workerA.ID, bb[0].FromAgentID)
	require.Equal(t, workerB.ID, bb[0].ToAgentID)
	require.Equal(t, "полный отчёт с деталями", bb[0].Payload)

	runsB, err := store.ListRuns(ctx, workerB.ID, 10)
	require.NoError(t, err)
	require.Len(t, runsB, 1)
	require.Equal(t, RunPassed, runsB[0].Status)
	require.Contains(t, runsB[0].OutputText, "полный отчёт с деталями")

	alwaysFailExec := &mockExecutor{
		handler: func(_ context.Context, req ExecuteRequest, _ int) (ExecuteResult, error) {
			if req.Agent.Role == RoleVerifier {
				return ExecuteResult{OutputText: "VERDICT: FAIL\n1. Всё ещё недостаточно полно."}, nil
			}
			return ExecuteResult{OutputText: "слабый результат"}, nil
		},
	}
	failSvc := NewService(store, alwaysFailExec, &mockModelCatalog{models: testCatalog()}, &mockEventSink{})

	blockedWorker, err := store.CreateAgent(ctx, CustomAgent{
		Slug:               "blocked-worker",
		Name:               "Блокируемый исполнитель",
		Role:               RoleWorker,
		AcceptanceCriteria: "Строгий критерий",
		NextAgentID:        workerB.ID,
		MaxReworkAttempts:  2,
		Enabled:            true,
	})
	require.NoError(t, err)

	failedRun, err := failSvc.DispatchSync(ctx, DispatchRequest{
		AgentID: blockedWorker.ID,
		Prompt:  "сделай работу",
		ChainID: "chain-blocked",
	})
	require.NoError(t, err)
	require.Equal(t, RunFailed, failedRun.Status)
	require.Equal(t, 2, failedRun.Attempt)
	require.Equal(t, "FAIL", failedRun.VerifierVerdict)

	bbBlocked, err := store.ListBlackboard(ctx, "chain-blocked", 10)
	require.NoError(t, err)
	require.Empty(t, bbBlocked)

	runsBAfter, err := store.ListRuns(ctx, workerB.ID, 10)
	require.NoError(t, err)
	require.Len(t, runsBAfter, 1)
}

func TestSchedulerDispatchesWithoutOverlap(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })

	var inFlight atomic.Int32
	var maxInFlight atomic.Int32
	var totalCalls atomic.Int32
	gate := make(chan struct{})

	exec := &mockExecutor{
		handler: func(ctx context.Context, _ ExecuteRequest, _ int) (ExecuteResult, error) {
			cur := inFlight.Add(1)
			for {
				prev := maxInFlight.Load()
				if cur <= prev || maxInFlight.CompareAndSwap(prev, cur) {
					break
				}
			}
			totalCalls.Add(1)
			select {
			case <-gate:
			case <-ctx.Done():
			}
			inFlight.Add(-1)
			return ExecuteResult{OutputText: "tick ok"}, nil
		},
	}

	svc := NewService(
		store,
		exec,
		&mockModelCatalog{models: testCatalog()},
		&mockEventSink{},
		WithTickInterval(20*time.Millisecond),
	)

	_, err = store.CreateAgent(ctx, CustomAgent{
		Slug:            "interval-agent",
		Name:            "Интервальный агент",
		Role:            RoleWorker,
		TriggerType:     TriggerInterval,
		IntervalMinutes: 5,
		NextRunAt:       time.Now().Unix() - 10,
		Enabled:         true,
	})
	require.NoError(t, err)

	svc.Start(ctx)

	require.Eventually(t, func() bool {
		return totalCalls.Load() == 1
	}, 2*time.Second, 10*time.Millisecond)

	time.Sleep(100 * time.Millisecond)
	require.Equal(t, int32(1), totalCalls.Load())
	require.Equal(t, int32(1), maxInFlight.Load())

	close(gate)
	svc.Stop()

	runs, err := store.ListRuns(ctx, "", 10)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	require.Equal(t, RunPassed, runs[0].Status)
}

func TestEdgeCasesMemoryPreservationEmptyMonitorAndUTF8Diff(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.EnsureDefaults(ctx))

	modelsWithEmpty := append([]ModelOption{{}, {ProviderID: "p1", ModelID: ""}}, testCatalog()...)
	choice, err := SelectModelForTask(modelsWithEmpty, CustomAgent{
		Role:        RoleWorker,
		ModelMode:   ModelModeAuto,
		TriggerType: TriggerMonitor,
	}, "проверь статус сервиса")
	require.NoError(t, err)
	require.Equal(t, "fast-mini", choice.ModelID)

	emptyFile := filepath.Join(t.TempDir(), "empty.log")
	require.NoError(t, os.WriteFile(emptyFile, []byte(""), 0o600))
	firstProbe := Probe(ctx, CustomAgent{MonitorKind: MonitorFile, MonitorTarget: emptyFile})
	require.Empty(t, firstProbe.Error)
	require.True(t, firstProbe.Changed)
	require.NotEmpty(t, firstProbe.Hash)

	require.NoError(t, os.WriteFile(emptyFile, []byte("новая запись\n"), 0o600))
	secondProbe := Probe(ctx, CustomAgent{
		MonitorKind:         MonitorFile,
		MonitorTarget:       emptyFile,
		MonitorLastHash:     firstProbe.Hash,
		MonitorLastSnapshot: firstProbe.Snapshot,
	})
	require.True(t, secondProbe.Changed)
	require.Contains(t, secondProbe.Diff, "+новая запись")

	cyrillicDiff := UnifiedLineDiff("", strings.Repeat("ПриветМир", 50), 25)
	require.True(t, utf8.ValidString(cyrillicDiff))
	require.LessOrEqual(t, utf8.RuneCountInString(cyrillicDiff), 25)

	exec := &mockExecutor{
		handler: func(ctx context.Context, req ExecuteRequest, _ int) (ExecuteResult, error) {
			if req.Agent.Slug == "mem-worker" {
				if err := store.AppendMemory(ctx, req.Agent.ID, "learned fact"); err != nil {
					return ExecuteResult{}, err
				}
				return ExecuteResult{OutputText: "готово"}, nil
			}
			return ExecuteResult{OutputText: "VERDICT: PASS"}, nil
		},
	}
	svc := NewService(store, exec, &mockModelCatalog{models: testCatalog()}, &mockEventSink{}, WithTickInterval(20*time.Millisecond))

	worker, err := store.CreateAgent(ctx, CustomAgent{
		Slug:        "mem-worker",
		Name:        "Агент с памятью",
		Role:        RoleWorker,
		Memory:      "base fact",
		NextAgentID: "non-existent-agent",
		Enabled:     true,
	})
	require.NoError(t, err)
	worker.VerifierAgentID = worker.ID
	worker, err = store.UpdateAgent(ctx, worker)
	require.NoError(t, err)

	resolvedVerifier := svc.resolveVerifierAgent(ctx, worker)
	require.NotEqual(t, worker.ID, resolvedVerifier.ID)

	run, err := svc.DispatchSync(ctx, DispatchRequest{
		AgentID: worker.ID,
		Prompt:  "выполни задачу",
		ChainID: "chain-missing-next",
	})
	require.NoError(t, err)
	require.Equal(t, RunPassed, run.Status)
	require.NotEmpty(t, run.Steps)
	require.Equal(t, "handoff", run.Steps[len(run.Steps)-1].Kind)
	require.Equal(t, "failed", run.Steps[len(run.Steps)-1].Status)

	mem, err := store.ReadMemory(ctx, worker.ID)
	require.NoError(t, err)
	require.Equal(t, "base fact\n\nlearned fact", mem)

	bb, err := store.ListBlackboard(ctx, "chain-missing-next", 10)
	require.NoError(t, err)
	require.Empty(t, bb)

	cronAgent, err := store.CreateAgent(ctx, CustomAgent{
		Slug:        "cron-future",
		Name:        "Крон агент",
		Role:        RoleWorker,
		TriggerType: TriggerCron,
		CronExpr:    "59 23 31 12 *",
		Enabled:     true,
	})
	require.NoError(t, err)
	require.Greater(t, cronAgent.NextRunAt, time.Now().Unix())
}
