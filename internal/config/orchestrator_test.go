package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func boolPtr(v bool) *bool { return &v }

func TestOrchestratorSectionRouting(t *testing.T) {
	t.Parallel()

	require.Equal(t, modelsSectionFile, sectionFileFor("orchestrator"))
	require.Equal(t, modelsSectionFile, sectionFileFor("orchestrator.enabled"))
	require.Equal(t, modelsSectionFile, sectionFileFor("orchestrator.planner"))
	require.Equal(t, modelsSectionFile, sectionFileFor("orchestrator.executor"))
	require.Equal(t, modelsSectionFile, sectionFileFor("session_orchestrator"))
	require.Equal(t, modelsSectionFile, sectionFileFor("session_orchestrator.chat-1"))
}

func TestOrchestratorHelpers_ResolutionAndFallbacks(t *testing.T) {
	t.Parallel()

	var nilCfg *Config
	require.False(t, nilCfg.IsOrchestratorEnabled("s1"))
	require.True(t, nilCfg.OrchestratorFinalReviewEnabled())
	require.Equal(t, SelectedModel{}, nilCfg.ResolveOrchestratorPlanner("s1"))
	require.Equal(t, SelectedModel{}, nilCfg.ResolveOrchestratorExecutor())

	large := SelectedModel{Provider: "anthropic", Model: "claude-3"}
	small := SelectedModel{Provider: "openai", Model: "gpt-4o-mini"}
	sessModel := SelectedModel{Provider: "openai", Model: "gpt-4"}
	plannerModel := SelectedModel{Provider: "anthropic", Model: "claude-opus", ReasoningMode: ReasoningModeHigh}
	executorModel := SelectedModel{Provider: "openai", Model: "gpt-5-mini", ReasoningMode: ReasoningModeLow}

	cfg := &Config{
		Models: map[SelectedModelType]SelectedModel{
			SelectedModelTypeLarge: large,
			SelectedModelTypeSmall: small,
		},
		SessionModels: map[string]SelectedModel{
			"sess-with-model": sessModel,
		},
		Orchestrator: OrchestratorConfig{
			Enabled: true,
		},
		SessionOrchestrator: map[string]bool{
			"sess-off": false,
		},
	}

	// Enabled checks per-session override first, then global default.
	require.True(t, cfg.IsOrchestratorEnabled(""))
	require.True(t, cfg.IsOrchestratorEnabled("sess-other"))
	require.False(t, cfg.IsOrchestratorEnabled("sess-off"))

	cfg.Orchestrator.Enabled = false
	cfg.SessionOrchestrator["sess-on"] = true
	require.False(t, cfg.IsOrchestratorEnabled(""))
	require.False(t, cfg.IsOrchestratorEnabled("sess-other"))
	require.True(t, cfg.IsOrchestratorEnabled("sess-on"))

	// FinalReview defaults to true when nil, otherwise follows explicit value.
	require.True(t, cfg.OrchestratorFinalReviewEnabled())
	cfg.Orchestrator.FinalReview = boolPtr(false)
	require.False(t, cfg.OrchestratorFinalReviewEnabled())
	cfg.Orchestrator.FinalReview = boolPtr(true)
	require.True(t, cfg.OrchestratorFinalReviewEnabled())

	// Planner fallback order: Orchestrator.Planner -> SessionModels[sid] -> Models[large].
	require.Equal(t, large, cfg.ResolveOrchestratorPlanner(""))
	require.Equal(t, large, cfg.ResolveOrchestratorPlanner("sess-other"))
	require.Equal(t, sessModel, cfg.ResolveOrchestratorPlanner("sess-with-model"))
	cfg.Orchestrator.Planner = &plannerModel
	require.Equal(t, plannerModel, cfg.ResolveOrchestratorPlanner("sess-with-model"))

	// Executor fallback order: Orchestrator.Executor -> Models[small] -> Models[large].
	require.Equal(t, small, cfg.ResolveOrchestratorExecutor())
	cfg.Models[SelectedModelTypeSmall] = SelectedModel{}
	require.Equal(t, large, cfg.ResolveOrchestratorExecutor())
	cfg.Orchestrator.Executor = &executorModel
	require.Equal(t, executorModel, cfg.ResolveOrchestratorExecutor())
}

func TestOrchestratorCloneForWrite_Isolation(t *testing.T) {
	t.Parallel()

	temp := 0.2
	topK := int64(40)
	orig := &Config{
		Providers: csync.NewMap[string, ProviderConfig](),
		Models:    map[SelectedModelType]SelectedModel{},
		SessionOrchestrator: map[string]bool{
			"s1": true,
		},
		Orchestrator: OrchestratorConfig{
			Enabled:     true,
			FinalReview: boolPtr(false),
			Planner: &SelectedModel{
				Provider:        "anthropic",
				Model:           "claude-3",
				Temperature:     &temp,
				ProviderOptions: map[string]any{"tier": "high"},
			},
			Executor: &SelectedModel{
				Provider:        "openai",
				Model:           "gpt-4",
				TopK:            &topK,
				ProviderOptions: map[string]any{"fast": true},
			},
		},
	}

	clone := orig.cloneForWrite()
	clone.SessionOrchestrator["s1"] = false
	clone.SessionOrchestrator["s2"] = true
	clone.Orchestrator.Enabled = false
	*clone.Orchestrator.FinalReview = true
	clone.Orchestrator.Planner.Model = "other-planner"
	*clone.Orchestrator.Planner.Temperature = 0.9
	clone.Orchestrator.Planner.ProviderOptions["tier"] = "low"
	clone.Orchestrator.Executor.Model = "other-executor"
	*clone.Orchestrator.Executor.TopK = 10
	clone.Orchestrator.Executor.ProviderOptions["fast"] = false

	require.True(t, orig.SessionOrchestrator["s1"])
	require.NotContains(t, orig.SessionOrchestrator, "s2")
	require.True(t, orig.Orchestrator.Enabled)
	require.False(t, *orig.Orchestrator.FinalReview)
	require.Equal(t, "claude-3", orig.Orchestrator.Planner.Model)
	require.Equal(t, 0.2, *orig.Orchestrator.Planner.Temperature)
	require.Equal(t, "high", orig.Orchestrator.Planner.ProviderOptions["tier"])
	require.Equal(t, "gpt-4", orig.Orchestrator.Executor.Model)
	require.Equal(t, int64(40), *orig.Orchestrator.Executor.TopK)
	require.Equal(t, true, orig.Orchestrator.Executor.ProviderOptions["fast"])
}

func TestUpdateOrchestratorConfig_PersistsAndPreservesModels(t *testing.T) {
	globalDir := t.TempDir()
	workDir := t.TempDir()
	t.Setenv("PRIME_GLOBAL_CONFIG", globalDir)
	t.Setenv("PRIME_GLOBAL_DATA", globalDir)
	resetProviderState()
	t.Cleanup(resetProviderState)

	require.NoError(t, os.WriteFile(
		filepath.Join(globalDir, "prime.json"),
		[]byte(twoProviderConfig("openai", "gpt-4")),
		0o600,
	))

	store, err := Load(workDir, "", false)
	require.NoError(t, err)

	planner := SelectedModel{Provider: "anthropic", Model: "claude-3", ReasoningMode: ReasoningModeHigh}
	executor := SelectedModel{Provider: "openai", Model: "gpt-4", ReasoningMode: ReasoningModeLow}
	require.NoError(t, store.UpdateOrchestratorConfig(ScopeGlobal, "", OrchestratorUpdate{
		Enabled:     boolPtr(true),
		Planner:     &planner,
		Executor:    &executor,
		FinalReview: boolPtr(false),
	}))

	cfg := store.Config()
	require.True(t, cfg.IsOrchestratorEnabled(""))
	require.False(t, cfg.OrchestratorFinalReviewEnabled())
	require.Equal(t, planner, cfg.ResolveOrchestratorPlanner(""))
	require.Equal(t, executor, cfg.ResolveOrchestratorExecutor())

	// Toggle off globally; Planner and Executor must remain intact.
	require.NoError(t, store.UpdateOrchestratorConfig(ScopeGlobal, "", OrchestratorUpdate{
		Enabled: boolPtr(false),
	}))
	cfg = store.Config()
	require.False(t, cfg.IsOrchestratorEnabled(""))
	require.Equal(t, planner, cfg.ResolveOrchestratorPlanner(""))
	require.Equal(t, executor, cfg.ResolveOrchestratorExecutor())
	require.False(t, cfg.OrchestratorFinalReviewEnabled())

	// Per-session override stored in workspace models.json.
	require.NoError(t, store.UpdateOrchestratorConfig(ScopeWorkspace, "chat-1", OrchestratorUpdate{
		Enabled: boolPtr(true),
	}))
	require.True(t, store.Config().IsOrchestratorEnabled("chat-1"))
	require.False(t, store.Config().IsOrchestratorEnabled("chat-2"))

	// Verify global and workspace writes landed in models.json.
	globalModels := readConfigJSON(t, filepath.Join(globalDir, "prime.json"), "orchestrator")
	orchMap, ok := globalModels["orchestrator"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, orchMap["enabled"])
	require.Equal(t, false, orchMap["final_review"])
	require.Contains(t, orchMap, "planner")
	require.Contains(t, orchMap, "executor")

	wsModels := readConfigJSON(t, filepath.Join(store.Config().Options.DataDirectory, "prime.json"), "session_orchestrator")
	sessMap, ok := wsModels["session_orchestrator"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, sessMap["chat-1"])

	// Reload from disk and verify everything survives.
	reloaded, err := Load(workDir, "", false)
	require.NoError(t, err)
	require.False(t, reloaded.Config().IsOrchestratorEnabled(""))
	require.True(t, reloaded.Config().IsOrchestratorEnabled("chat-1"))
	require.False(t, reloaded.Config().OrchestratorFinalReviewEnabled())
	require.Equal(t, planner, reloaded.Config().ResolveOrchestratorPlanner(""))
	require.Equal(t, executor, reloaded.Config().ResolveOrchestratorExecutor())

	// Clearing Planner removes only planner while leaving Executor intact.
	require.NoError(t, reloaded.UpdateOrchestratorConfig(ScopeGlobal, "", OrchestratorUpdate{
		Planner: &SelectedModel{},
	}))
	require.Nil(t, reloaded.Config().Orchestrator.Planner)
	require.Equal(t, SelectedModel{Provider: "openai", Model: "gpt-4"}, reloaded.Config().ResolveOrchestratorPlanner(""))
	require.Equal(t, executor, reloaded.Config().ResolveOrchestratorExecutor())

	reloadedAfterClear, err := Load(workDir, "", false)
	require.NoError(t, err)
	require.Nil(t, reloadedAfterClear.Config().Orchestrator.Planner)
	require.Equal(t, executor, reloadedAfterClear.Config().ResolveOrchestratorExecutor())

	// Incomplete model update returns an error.
	err = reloadedAfterClear.UpdateOrchestratorConfig(ScopeGlobal, "", OrchestratorUpdate{
		Planner: &SelectedModel{Provider: "openai"},
	})
	require.Error(t, err)
}

func TestResolveOrchestratorRoleReasoning(t *testing.T) {
	t.Parallel()

	capable := catwalk.Model{
		CanReason:       true,
		ReasoningLevels: []string{"none", "low", "medium", "high"},
	}
	nonCapable := catwalk.Model{
		CanReason: false,
	}

	require.Equal(t, ReasoningModeHigh, ResolveOrchestratorRoleReasoning("planner", SelectedModel{ReasoningMode: ReasoningModeOff}, capable))
	require.Equal(t, ReasoningModeLow, ResolveOrchestratorRoleReasoning("planner", SelectedModel{ReasoningMode: ReasoningModeLow}, capable))
	require.Equal(t, ReasoningModeOff, ResolveOrchestratorRoleReasoning("planner", SelectedModel{ReasoningMode: ReasoningModeOff}, nonCapable))
	require.Equal(t, ReasoningModeOff, ResolveOrchestratorRoleReasoning("executor", SelectedModel{ReasoningMode: ReasoningModeOff}, capable))
}
