package backend

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/google/uuid"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestConfiguredWorkspaceDefersAgentAndPreservesSelectedMode(t *testing.T) {
	xdgIsolated(t)
	t.Setenv("USERPROFILE", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())
	t.Setenv("PRIME_GLOBAL_CONFIG", t.TempDir())
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	project := t.TempDir()
	configuration := fmt.Sprintf(`{
		"options":{"disable_default_providers":true,"disable_provider_auto_update":true,"disable_metrics":true},
		"providers":{"fixture":{"id":"fixture","type":"openai-compat","base_url":%q,"api_key":"fixture-key","discover_models":false,"models":[{"id":"fixture-model","name":"Fixture","context_window":128000,"default_max_tokens":8192},{"id":"fixture-small","name":"Small","context_window":128000,"default_max_tokens":4096}]}},
		"models":{"large":{"provider":"fixture","model":"fixture-model"},"small":{"provider":"fixture","model":"fixture-small"}},
		"mcp":{"lazy-runtime-test":{"type":"http","url":%q}}
	}`, server.URL, server.URL)
	require.NoError(t, os.WriteFile(filepath.Join(project, "prime.json"), []byte(configuration), 0o600))
	b := New(context.Background(), nil, func() {})
	t.Cleanup(func() { drainBackend(t, b) })
	ws, _, err := b.CreateWorkspace(protoWS(project, t.TempDir(), uuid.New().String()))
	require.NoError(t, err)
	require.True(t, ws.Cfg.Config().IsConfigured())
	require.Nil(t, ws.AgentCoordinator)
	require.NoError(t, b.SetMainAgent(ws.ID, config.AgentPlan))
	require.Nil(t, ws.AgentCoordinator)
	require.NoError(t, b.SetConfigField(ws.ID, config.ScopeWorkspace, "options.disable_metrics", true))
	plan := ws.Cfg.Config().Agents[config.AgentPlan]
	planModel := ws.Cfg.Config().Models[config.SelectedModelTypeSmall]
	plan.ModelOverride = &planModel
	ws.Cfg.Config().Agents[config.AgentPlan] = plan
	require.Eventually(t, func() bool { return requests.Load() > 0 }, 3*time.Second, 10*time.Millisecond)
	require.Nil(t, ws.AgentCoordinator)
	session, err := ws.Sessions.Create(context.Background(), "Existing chat")
	require.NoError(t, err)
	require.NoError(t, b.SummarizeSession(context.Background(), ws.ID, session.ID))
	require.NotNil(t, ws.AgentCoordinator)
	require.Equal(t, "fixture-small", ws.AgentCoordinator.Model().ModelCfg.Model)
	require.NoError(t, b.InitAgent(context.Background(), ws.ID, true))
	require.NotNil(t, ws.AgentCoordinator)
	require.Equal(t, "fixture-small", ws.AgentCoordinator.Model().ModelCfg.Model)
}
