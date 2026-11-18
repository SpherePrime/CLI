package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/app"
	"github.com/SpherePrime/CLI/internal/backend"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func newOrchestratorServerTestHarness(t *testing.T) (*Server, *backend.Workspace) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg-cfg"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "xdg-data"))
	t.Setenv("PRIME_GLOBAL_CONFIG", filepath.Join(root, "global-cfg"))
	t.Setenv("PRIME_GLOBAL_DATA", filepath.Join(root, "global-data"))

	workDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workDir, "prime.json"), []byte(`{
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

	cfg, err := config.Init(workDir, "", false)
	require.NoError(t, err)

	b := backend.New(context.Background(), nil, nil)
	appCtx, cancel := context.WithCancel(context.Background())
	a := app.NewForTest(appCtx)
	t.Cleanup(func() {
		cancel()
		a.ShutdownForTest()
	})
	ws := &backend.Workspace{
		ID:   uuid.New().String(),
		Path: workDir,
		App:  a,
		Cfg:  cfg,
	}
	backend.InsertWorkspaceForTest(b, ws)
	backend.SetWorkspaceShutdownFnForTest(ws, func() {})

	srv := &Server{backend: b}
	srv.installHandler()
	return srv, ws
}

func TestPostWorkspaceConfigOrchestrator_HTTP(t *testing.T) {
	srv, ws := newOrchestratorServerTestHarness(t)

	post := func(wsID string, body []byte) *httptest.ResponseRecorder {
		req := httptest.NewRequestWithContext(
			t.Context(),
			http.MethodPost,
			"/v1/workspaces/"+wsID+"/config/orchestrator",
			bytes.NewReader(body),
		)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		srv.Handler().ServeHTTP(rec, req)
		return rec
	}

	enabledTrue := true
	enabledFalse := false
	finalReviewFalse := false
	payload, err := json.Marshal(proto.ConfigOrchestratorRequest{
		Scope: config.ScopeGlobal,
		Enabled: &enabledTrue,
		Planner: &config.SelectedModel{
			Provider:      "anthropic",
			Model:         "claude-3",
			ReasoningMode: config.ReasoningModeHigh,
		},
		Executor: &config.SelectedModel{
			Provider: "openai",
			Model:    "gpt-4",
		},
		FinalReview: &finalReviewFalse,
	})
	require.NoError(t, err)

	rec := post(ws.ID, payload)
	require.Equal(t, http.StatusOK, rec.Code)

	cfg := ws.Cfg.Config()
	require.True(t, cfg.IsOrchestratorEnabled(""))
	require.False(t, cfg.OrchestratorFinalReviewEnabled())
	require.Equal(t, "claude-3", cfg.ResolveOrchestratorPlanner("").Model)
	require.Equal(t, "gpt-4", cfg.ResolveOrchestratorExecutor().Model)

	// Per-session disable via HTTP preserves global default and models.
	sessPayload, err := json.Marshal(proto.ConfigOrchestratorRequest{
		SessionID: "chat-1",
		Enabled:   &enabledFalse,
	})
	require.NoError(t, err)
	rec = post(ws.ID, sessPayload)
	require.Equal(t, http.StatusOK, rec.Code)
	require.False(t, ws.Cfg.Config().IsOrchestratorEnabled("chat-1"))
	require.True(t, ws.Cfg.Config().IsOrchestratorEnabled("chat-2"))
	require.Equal(t, "claude-3", ws.Cfg.Config().ResolveOrchestratorPlanner("chat-1").Model)

	// Bad JSON returns 400.
	rec = post(ws.ID, []byte("not-json"))
	require.Equal(t, http.StatusBadRequest, rec.Code)

	// Missing workspace returns 404.
	rec = post("no-such-workspace", payload)
	require.Equal(t, http.StatusNotFound, rec.Code)
}
