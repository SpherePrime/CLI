package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func boolPtr(v bool) *bool { return &v }

func insertOrchestratorWorkspace(t *testing.T, b *Backend, coord *forgetTrackingCoordinator) *Workspace {
	t.Helper()
	dir := t.TempDir()
	primeJSON := `{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {
    "openai": {
      "id": "openai", "name": "OpenAI", "type": "openai-compat",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "k1", "discover_models": false,
      "models": [{"id": "gpt-4", "name": "GPT-4"}]
    },
    "anthropic": {
      "id": "anthropic", "name": "Anthropic", "type": "anthropic",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "k2", "discover_models": false,
      "models": [{"id": "claude-3", "name": "Claude 3", "can_reason": true, "reasoning_levels": ["low", "medium", "high"]}]
    }
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "prime.json"), []byte(primeJSON), 0o600))
	cfg, err := config.Init(dir, "", false)
	require.NoError(t, err)

	ws := insertRunCompleteWorkspace(t, b, context.Background(), coord)
	ws.Cfg = cfg
	return ws
}

func TestUpdateOrchestratorConfig_PublishesAndPreservesModels(t *testing.T) {
	b, ws, evc := newPublishingWorkspace(t)

	// Seed provider catalog so reasoning mode normalization succeeds.
	globalDir := filepath.Dir(config.GlobalConfig())
	require.NoError(t, os.MkdirAll(globalDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, "prime.json"), []byte(`{
  "options": {"disable_default_providers": true, "disable_provider_auto_update": true},
  "providers": {
    "openai": {
      "id": "openai", "name": "OpenAI", "type": "openai-compat",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "k1", "discover_models": false,
      "models": [{"id": "gpt-4", "name": "GPT-4"}]
    },
    "anthropic": {
      "id": "anthropic", "name": "Anthropic", "type": "anthropic",
      "base_url": "http://127.0.0.1:9/v1", "api_key": "k2", "discover_models": false,
      "models": [{"id": "claude-3", "name": "Claude 3", "can_reason": true, "reasoning_levels": ["low", "medium", "high"]}]
    }
  }
}`), 0o600))
	cfg, err := config.Init(ws.Path, ws.Cfg.Config().Options.DataDirectory, false)
	require.NoError(t, err)
	ws.Cfg = cfg
	drainEvents(evc, 50*time.Millisecond)

	planner := config.SelectedModel{
		Provider:      "anthropic",
		Model:         "claude-3",
		ReasoningMode: config.ReasoningModeHigh,
	}
	executor := config.SelectedModel{
		Provider: "openai",
		Model:    "gpt-4",
	}

	require.NoError(t, b.UpdateOrchestratorConfig(ws.ID, proto.ConfigOrchestratorRequest{
		Scope:       config.ScopeGlobal,
		Enabled:     boolPtr(true),
		Planner:     &planner,
		Executor:    &executor,
		FinalReview: boolPtr(false),
	}))
	awaitConfigChanged(t, evc, ws.ID)

	cur := ws.Cfg.Config()
	require.True(t, cur.IsOrchestratorEnabled(""))
	require.False(t, cur.OrchestratorFinalReviewEnabled())
	require.Equal(t, planner, cur.ResolveOrchestratorPlanner(""))
	require.Equal(t, executor, cur.ResolveOrchestratorExecutor())

	// Toggle off for one session only; global default and models remain intact.
	require.NoError(t, b.UpdateOrchestratorConfig(ws.ID, proto.ConfigOrchestratorRequest{
		SessionID: "chat-1",
		Enabled:   boolPtr(false),
	}))
	awaitConfigChanged(t, evc, ws.ID)

	cur = ws.Cfg.Config()
	require.False(t, cur.IsOrchestratorEnabled("chat-1"))
	require.True(t, cur.IsOrchestratorEnabled("chat-2"))
	require.Equal(t, planner, cur.ResolveOrchestratorPlanner("chat-1"))
	require.Equal(t, executor, cur.ResolveOrchestratorExecutor())
}

func TestUpdateOrchestratorConfig_ForgetsSubAgents(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("PRIME_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())

	b, _ := newTestBackend(t)
	c1 := &forgetTrackingCoordinator{}
	c2 := &forgetTrackingCoordinator{}
	ws1 := insertOrchestratorWorkspace(t, b, c1)
	_ = insertOrchestratorWorkspace(t, b, c2)

	// Per-session / workspace-scoped update forgets sub-agents only on ws1.
	require.NoError(t, b.UpdateOrchestratorConfig(ws1.ID, proto.ConfigOrchestratorRequest{
		Scope:     config.ScopeWorkspace,
		SessionID: "chat-1",
		Enabled:   boolPtr(true),
	}))
	require.Equal(t, int32(1), c1.forgetCalls.Load())
	require.Equal(t, int32(0), c2.forgetCalls.Load())

	// Global update forgets sub-agents across open workspaces.
	require.NoError(t, b.UpdateOrchestratorConfig(ws1.ID, proto.ConfigOrchestratorRequest{
		Scope: config.ScopeGlobal,
		Planner: &config.SelectedModel{
			Provider:      "anthropic",
			Model:         "claude-3",
			ReasoningMode: config.ReasoningModeMedium,
		},
	}))
	require.Equal(t, int32(2), c1.forgetCalls.Load())
	require.Equal(t, int32(1), c2.forgetCalls.Load())
}

func TestUpdateOrchestratorConfig_ValidationErrors(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("PRIME_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())

	b, _ := newTestBackend(t)
	c1 := &forgetTrackingCoordinator{}
	ws := insertOrchestratorWorkspace(t, b, c1)

	err := b.UpdateOrchestratorConfig("missing-ws", proto.ConfigOrchestratorRequest{
		Enabled: boolPtr(true),
	})
	require.ErrorIs(t, err, ErrWorkspaceNotFound)

	err = b.UpdateOrchestratorConfig(ws.ID, proto.ConfigOrchestratorRequest{
		Scope: config.ScopeGlobal,
		Planner: &config.SelectedModel{
			Provider:      "anthropic",
			Model:         "does-not-exist",
			ReasoningMode: config.ReasoningModeHigh,
		},
	})
	require.Error(t, err)
	require.Equal(t, int32(0), c1.forgetCalls.Load())

	err = b.UpdateOrchestratorConfig(ws.ID, proto.ConfigOrchestratorRequest{
		Scope: config.ScopeGlobal,
		Planner: &config.SelectedModel{
			Provider:      "anthropic",
			Model:         "claude-3",
			ReasoningMode: "bogus-mode",
		},
	})
	require.Error(t, err)
	require.Equal(t, int32(0), c1.forgetCalls.Load())

	// Ensure no ConfigChanged was emitted on validation failures.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	evc := ws.Events(ctx)
	select {
	case ev := <-evc:
		if _, ok := ev.Payload.(pubsub.Event[proto.ConfigChanged]); ok {
			t.Fatal("unexpected ConfigChanged event on failed validation")
		}
	case <-time.After(50 * time.Millisecond):
	}
}
