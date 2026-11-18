package customagents

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestWorkflowSchemaMigrationAndEnsureDefaults(t *testing.T) {
	ctx := context.Background()

	legacyDir := t.TempDir()
	legacyDB, err := db.OpenFile(filepath.Join(legacyDir, "custom_agents.db"))
	require.NoError(t, err)
	_, err = legacyDB.Exec(`CREATE TABLE custom_agents (
		id TEXT PRIMARY KEY,
		slug TEXT NOT NULL UNIQUE,
		name TEXT NOT NULL,
		description TEXT NOT NULL DEFAULT '',
		role TEXT NOT NULL DEFAULT 'worker',
		system_prompt TEXT NOT NULL DEFAULT '',
		memory TEXT NOT NULL DEFAULT '',
		model_mode TEXT NOT NULL DEFAULT 'auto',
		provider_id TEXT NOT NULL DEFAULT '',
		model_id TEXT NOT NULL DEFAULT '',
		reasoning_mode TEXT NOT NULL DEFAULT '',
		auto_model_hint TEXT NOT NULL DEFAULT '',
		allowed_tools TEXT NOT NULL DEFAULT '[]',
		workspace_path TEXT NOT NULL DEFAULT '',
		enabled INTEGER NOT NULL DEFAULT 1,
		created_by TEXT NOT NULL DEFAULT 'user',
		trigger_type TEXT NOT NULL DEFAULT 'manual',
		interval_minutes INTEGER NOT NULL DEFAULT 0,
		cron_expr TEXT NOT NULL DEFAULT '',
		monitor_kind TEXT NOT NULL DEFAULT '',
		monitor_target TEXT NOT NULL DEFAULT '',
		monitor_last_hash TEXT NOT NULL DEFAULT '',
		monitor_last_snapshot TEXT NOT NULL DEFAULT '',
		acceptance_criteria TEXT NOT NULL DEFAULT '',
		deterministic_checks TEXT NOT NULL DEFAULT '[]',
		verifier_agent_id TEXT NOT NULL DEFAULT '',
		next_agent_id TEXT NOT NULL DEFAULT '',
		max_rework_attempts INTEGER NOT NULL DEFAULT 3,
		status TEXT NOT NULL DEFAULT 'idle',
		last_error TEXT NOT NULL DEFAULT '',
		last_run_at INTEGER NOT NULL DEFAULT 0,
		next_run_at INTEGER NOT NULL DEFAULT 0,
		created_at INTEGER NOT NULL DEFAULT 0,
		updated_at INTEGER NOT NULL DEFAULT 0
	);`)
	require.NoError(t, err)
	_, err = legacyDB.Exec(`INSERT INTO custom_agents (id, slug, name, role, created_by) VALUES
		('legacy-orch-id', 'orchestrator', 'Legacy Orch', 'orchestrator', 'system'),
		('legacy-ver-id', 'strict-verifier', 'Legacy Verifier', 'verifier', 'system'),
		('custom-worker-id', 'custom-worker', 'Custom Worker', 'worker', 'user');`)
	require.NoError(t, err)
	require.NoError(t, legacyDB.Close())

	migratedStore, err := OpenStore(legacyDir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = migratedStore.Close() })

	require.NoError(t, migratedStore.EnsureDefaults(ctx))

	defWF, err := migratedStore.GetWorkflow(ctx, "default")
	require.NoError(t, err)
	require.Equal(t, "default", defWF.ID)
	require.Equal(t, "default", defWF.Slug)
	require.Equal(t, "Основная цепочка", defWF.Name)
	require.Equal(t, "agent-orchestrator", defWF.EntryAgentID)
	require.Equal(t, "system", defWF.CreatedBy)
	require.True(t, defWF.Enabled)

	orch, err := migratedStore.GetAgent(ctx, "legacy-orch-id")
	require.NoError(t, err)
	require.Equal(t, "default", orch.WorkflowID)

	ver, err := migratedStore.GetAgent(ctx, "legacy-ver-id")
	require.NoError(t, err)
	require.Equal(t, "default", ver.WorkflowID)

	customWorker, err := migratedStore.GetAgent(ctx, "custom-worker-id")
	require.NoError(t, err)
	require.Equal(t, "", customWorker.WorkflowID)

	freshStore, err := OpenStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = freshStore.Close() })

	require.NoError(t, freshStore.EnsureDefaults(ctx))
	require.NoError(t, freshStore.EnsureDefaults(ctx))

	freshOrch, err := freshStore.GetAgent(ctx, "agent-orchestrator")
	require.NoError(t, err)
	require.Equal(t, "default", freshOrch.WorkflowID)

	freshVer, err := freshStore.GetAgent(ctx, "agent-strict-verifier")
	require.NoError(t, err)
	require.Equal(t, "default", freshVer.WorkflowID)

	defAgents, err := freshStore.ListAgentsByWorkflow(ctx, "default")
	require.NoError(t, err)
	require.Len(t, defAgents, 2)
}

func TestWorkflowCRUDLayoutAndDeleteSemantics(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.EnsureDefaults(ctx))

	sink := &mockEventSink{}
	svc := NewService(store, &mockExecutor{}, &mockModelCatalog{models: testCatalog()}, sink)

	_, err = svc.CreateWorkflow(ctx, CreateWorkflowInput{
		Name:       "Bad Layout",
		LayoutJSON: `["not-an-object"]`,
	})
	require.Error(t, err)

	wf1, err := svc.CreateWorkflow(ctx, CreateWorkflowInput{
		Name:        "Release Pipeline",
		Description: "Checks releases",
		LayoutJSON:  `{"a1":{"x":10,"y":20}}`,
	})
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(wf1.ID, "wf-"))
	require.Equal(t, "release-pipeline", wf1.Slug)
	require.True(t, wf1.Enabled)
	require.Equal(t, `{"a1":{"x":10,"y":20}}`, wf1.LayoutJSON)

	wf2, err := svc.CreateWorkflow(ctx, CreateWorkflowInput{
		Name: "Release Pipeline",
	})
	require.NoError(t, err)
	require.Equal(t, "release-pipeline-2", wf2.Slug)

	allWFs, err := svc.ListWorkflows(ctx)
	require.NoError(t, err)
	require.Len(t, allWFs, 3)
	require.Equal(t, "default", allWFs[0].ID)

	newName := "Release Pipeline v2"
	newLayout := `{"a1":{"x":100,"y":200}}`
	updatedWF, err := svc.UpdateWorkflow(ctx, wf1.ID, UpdateWorkflowPatch{
		Name:       &newName,
		LayoutJSON: &newLayout,
	})
	require.NoError(t, err)
	require.Equal(t, newName, updatedWF.Name)
	require.Equal(t, newLayout, updatedWF.LayoutJSON)

	a1, err := svc.CreateAgent(ctx, CreateAgentInput{
		Slug:       "wf1-worker",
		Name:       "WF1 Worker",
		Role:       RoleWorker,
		WorkflowID: wf1.ID,
		Memory:     "keep me",
	})
	require.NoError(t, err)
	require.Equal(t, wf1.ID, a1.WorkflowID)

	updatedDesc := "updated description"
	a1Updated, err := svc.UpdateAgent(ctx, a1.ID, UpdateAgentPatch{
		Description: &updatedDesc,
	})
	require.NoError(t, err)
	require.Equal(t, wf1.ID, a1Updated.WorkflowID)
	require.Equal(t, updatedDesc, a1Updated.Description)

	a2, err := svc.CreateAgent(ctx, CreateAgentInput{
		Slug:       "wf2-worker",
		Name:       "WF2 Worker",
		Role:       RoleWorker,
		WorkflowID: wf2.ID,
		Memory:     "ephemeral memory",
	})
	require.NoError(t, err)

	aExternal, err := svc.CreateAgent(ctx, CreateAgentInput{
		Slug:        "external-worker",
		Name:        "External Worker",
		Role:        RoleWorker,
		NextAgentID: a2.ID,
	})
	require.NoError(t, err)

	require.Error(t, svc.DeleteWorkflow(ctx, "default", false))

	require.NoError(t, svc.DeleteWorkflow(ctx, wf1.ID, false))
	_, err = svc.GetWorkflow(ctx, wf1.ID)
	require.True(t, errors.Is(err, sql.ErrNoRows))
	a1After, err := store.GetAgent(ctx, a1.ID)
	require.NoError(t, err)
	require.Equal(t, "", a1After.WorkflowID)

	require.NoError(t, svc.DeleteWorkflow(ctx, wf2.ID, true))
	_, err = store.GetAgent(ctx, a2.ID)
	require.True(t, errors.Is(err, sql.ErrNoRows))
	_, err = os.Stat(store.memoryPath(a2.ID))
	require.True(t, errors.Is(err, os.ErrNotExist))

	aExtAfter, err := store.GetAgent(ctx, aExternal.ID)
	require.NoError(t, err)
	require.Equal(t, "", aExtAfter.NextAgentID)

	sink.mu.Lock()
	var updatedEvents, deletedEvents int
	for _, ev := range sink.events {
		switch ev.Type {
		case EventWorkflowUpdated:
			updatedEvents++
			require.NotNil(t, ev.Workflow)
		case EventWorkflowDeleted:
			deletedEvents++
			require.NotEmpty(t, ev.WorkflowID)
		}
	}
	sink.mu.Unlock()
	require.GreaterOrEqual(t, updatedEvents, 3)
	require.Equal(t, 2, deletedEvents)
}

func TestRunWorkflowAutoAndExplicitEntryExecution(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.EnsureDefaults(ctx))

	var step1Calls atomic.Int32
	var step2Calls atomic.Int32
	exec := &mockExecutor{
		handler: func(_ context.Context, req ExecuteRequest, _ int) (ExecuteResult, error) {
			switch req.Agent.Slug {
			case "wf-step-1":
				step1Calls.Add(1)
				return ExecuteResult{OutputText: "step-1-output"}, nil
			case "wf-step-2":
				step2Calls.Add(1)
				return ExecuteResult{OutputText: "step-2-final: " + req.BlackboardContext}, nil
			default:
				return ExecuteResult{OutputText: "ok"}, nil
			}
		},
	}
	svc := NewService(store, exec, &mockModelCatalog{models: testCatalog()}, &mockEventSink{})

	wf, err := svc.CreateWorkflow(ctx, CreateWorkflowInput{
		Slug: "auto-chain",
		Name: "Auto Chain",
	})
	require.NoError(t, err)

	step2, err := svc.CreateAgent(ctx, CreateAgentInput{
		Slug:       "wf-step-2",
		Name:       "Step 2",
		Role:       RoleWorker,
		WorkflowID: wf.ID,
	})
	require.NoError(t, err)

	step1, err := svc.CreateAgent(ctx, CreateAgentInput{
		Slug:        "wf-step-1",
		Name:        "Step 1",
		Role:        RoleWorker,
		WorkflowID:  wf.ID,
		NextAgentID: step2.ID,
	})
	require.NoError(t, err)

	entryRun, err := svc.RunWorkflow(ctx, wf.ID, "execute auto chain")
	require.NoError(t, err)
	require.Equal(t, step1.ID, entryRun.AgentID)
	require.Equal(t, "workflow:auto-chain", entryRun.TriggerReason)
	require.True(t, strings.HasPrefix(entryRun.ChainID, "wf-auto-chain-"))

	var runs2 []CustomAgentRun
	require.Eventually(t, func() bool {
		if step1Calls.Load() != 1 || step2Calls.Load() != 1 {
			return false
		}
		var listErr error
		runs2, listErr = store.ListRuns(ctx, step2.ID, 10)
		return listErr == nil && len(runs2) == 1 && runs2[0].Status == RunPassed && !svc.isAgentActive(step2.ID) && !svc.isAgentActive(step1.ID)
	}, 2*time.Second, 10*time.Millisecond)

	require.Len(t, runs2, 1)
	require.Equal(t, RunPassed, runs2[0].Status)
	require.Contains(t, runs2[0].OutputText, "step-1-output")

	wfExplicit, err := svc.CreateWorkflow(ctx, CreateWorkflowInput{
		Slug:         "explicit-chain",
		Name:         "Explicit Chain",
		EntryAgentID: step2.ID,
	})
	require.NoError(t, err)

	_, err = svc.UpdateAgent(ctx, step2.ID, UpdateAgentPatch{
		WorkflowID: &wfExplicit.ID,
	})
	require.NoError(t, err)

	explicitRun, err := svc.RunWorkflow(ctx, wfExplicit.ID, "run explicit")
	require.NoError(t, err)
	require.Equal(t, step2.ID, explicitRun.AgentID)

	require.Eventually(t, func() bool {
		if step2Calls.Load() != 2 {
			return false
		}
		r, listErr := store.ListRuns(ctx, step2.ID, 10)
		return listErr == nil && len(r) == 2 && r[0].Status == RunPassed && !svc.isAgentActive(step2.ID)
	}, 2*time.Second, 10*time.Millisecond)

	disabled := false
	_, err = svc.UpdateWorkflow(ctx, wfExplicit.ID, UpdateWorkflowPatch{
		Enabled: &disabled,
	})
	require.NoError(t, err)

	_, err = svc.RunWorkflow(ctx, wfExplicit.ID, "should fail")
	require.Error(t, err)
}
