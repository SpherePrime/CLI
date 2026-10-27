package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/env"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func discoveryServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server
}

func TestFlushDiscoveredModelsPersistsToGlobalProvidersFile(t *testing.T) {
	dir := t.TempDir()
	server := discoveryServer(t, `{"data": [{"id": "m1"}, {"id": "m2"}]}`)

	cfg := &Config{
		Providers: csync.NewMapFrom(map[string]ProviderConfig{
			"custom": {
				APIKey:  "test-key",
				BaseURL: server.URL + "/v1",
			},
		}),
	}
	cfg.setDefaults("/tmp", "")

	store := &ConfigStore{config: cfg, globalDataPath: filepath.Join(dir, "prime.json")}
	require.NoError(t, os.WriteFile(filepath.Join(dir, providersSectionFile), []byte(`{"providers":{"custom":{"id":"custom","type":"openai-compat","discover_models":true}}}`), 0600))

	env := env.NewFromMap(map[string]string{})
	resolver := NewShellVariableResolver(env)
	require.NoError(t, cfg.configureProviders(context.Background(), store, env, resolver, []catwalk.Provider{}))
	store.flushDiscoveredModels()

	data, err := os.ReadFile(filepath.Join(dir, providersSectionFile))
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(data, &document))
	models := document["providers"].(map[string]any)["custom"].(map[string]any)["models"].([]any)
	require.Len(t, models, 2)
	require.Equal(t, "m1", models[0].(map[string]any)["id"])
	require.Equal(t, "m2", models[1].(map[string]any)["id"])
}

func TestFlushDiscoveredModelsPersistsToWorkspaceProvidersFile(t *testing.T) {
	globalDir, workspaceDir := t.TempDir(), t.TempDir()
	server := discoveryServer(t, `{"data": [{"id": "ws-model"}]}`)

	cfg := &Config{
		Providers: csync.NewMapFrom(map[string]ProviderConfig{
			"local": {
				APIKey:  "test-key",
				BaseURL: server.URL + "/v1",
			},
		}),
	}
	cfg.setDefaults("/tmp", "")

	store := &ConfigStore{
		config:         cfg,
		globalDataPath: filepath.Join(globalDir, "prime.json"),
		workspacePath:  filepath.Join(workspaceDir, "prime.json"),
	}
	require.NoError(t, os.WriteFile(filepath.Join(workspaceDir, providersSectionFile), []byte(`{"providers":{"local":{"id":"local","type":"openai-compat","discover_models":true}}}`), 0600))

	env := env.NewFromMap(map[string]string{})
	resolver := NewShellVariableResolver(env)
	require.NoError(t, cfg.configureProviders(context.Background(), store, env, resolver, []catwalk.Provider{}))
	store.flushDiscoveredModels()

	workspaceData, err := os.ReadFile(filepath.Join(workspaceDir, providersSectionFile))
	require.NoError(t, err)
	require.Contains(t, string(workspaceData), "ws-model")

	globalData, err := os.ReadFile(filepath.Join(globalDir, providersSectionFile))
	require.True(t, os.IsNotExist(err) || err == nil)
	if err == nil {
		require.NotContains(t, string(globalData), "ws-model")
	}
}

func TestFlushDiscoveredModelsSkipsUnknownProviders(t *testing.T) {
	dir := t.TempDir()
	server := discoveryServer(t, `{"data": [{"id": "m1"}]}`)

	cfg := &Config{
		Providers: csync.NewMapFrom(map[string]ProviderConfig{
			"ghost": {
				APIKey:  "test-key",
				BaseURL: server.URL + "/v1",
			},
		}),
	}
	cfg.setDefaults("/tmp", "")

	store := &ConfigStore{config: cfg, globalDataPath: filepath.Join(dir, "prime.json")}

	env := env.NewFromMap(map[string]string{})
	resolver := NewShellVariableResolver(env)
	require.NoError(t, cfg.configureProviders(context.Background(), store, env, resolver, []catwalk.Provider{}))
	store.flushDiscoveredModels()

	_, err := os.Stat(filepath.Join(dir, providersSectionFile))
	require.True(t, os.IsNotExist(err))
}

func TestFlushDiscoveredModelsWithoutPathsIsNoop(t *testing.T) {
	dir := t.TempDir()
	server := discoveryServer(t, `{"data": [{"id": "m1"}]}`)

	cfg := &Config{
		Providers: csync.NewMapFrom(map[string]ProviderConfig{
			"custom": {
				APIKey:  "test-key",
				BaseURL: server.URL + "/v1",
			},
		}),
	}
	cfg.setDefaults("/tmp", "")

	store := &ConfigStore{config: cfg}

	env := env.NewFromMap(map[string]string{})
	resolver := NewShellVariableResolver(env)
	require.NoError(t, cfg.configureProviders(context.Background(), store, env, resolver, []catwalk.Provider{}))
	store.flushDiscoveredModels()

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}
