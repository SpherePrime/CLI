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

func TestDiscoveredModelsDifferDetectsMetadataAndCapabilities(t *testing.T) {
	base := []catwalk.Model{
		{
			ID:               "m1",
			ContextWindow:    128000,
			DefaultMaxTokens: 8192,
			SupportsImages:   true,
			CanReason:        true,
			ReasoningLevels:  []string{"low", "high"},
			DefaultReasoningEffort: "low",
			Options: catwalk.ModelOptions{
				ProviderOptions: map[string]any{"web_search": false},
			},
		},
	}

	require.False(t, discoveredModelsDiffer(base, []catwalk.Model{
		{
			ID:                     "m1",
			ContextWindow:          128000,
			DefaultMaxTokens:       8192,
			SupportsImages:         true,
			CanReason:              true,
			ReasoningLevels:        []string{"low", "high"},
			DefaultReasoningEffort: "low",
			Options: catwalk.ModelOptions{
				ProviderOptions: map[string]any{"web_search": false},
			},
		},
	}))

	withWebSearch := []catwalk.Model{base[0]}
	withWebSearch[0].Options.ProviderOptions = map[string]any{"web_search": true}
	require.True(t, discoveredModelsDiffer(base, withWebSearch))

	withoutOptions := []catwalk.Model{base[0]}
	withoutOptions[0].Options.ProviderOptions = nil
	require.True(t, discoveredModelsDiffer(withoutOptions, withWebSearch))

	diffWindow := []catwalk.Model{base[0]}
	diffWindow[0].ContextWindow = 200000
	require.True(t, discoveredModelsDiffer(base, diffWindow))

	diffMaxTokens := []catwalk.Model{base[0]}
	diffMaxTokens[0].DefaultMaxTokens = 16384
	require.True(t, discoveredModelsDiffer(base, diffMaxTokens))

	diffImages := []catwalk.Model{base[0]}
	diffImages[0].SupportsImages = false
	require.True(t, discoveredModelsDiffer(base, diffImages))

	diffReason := []catwalk.Model{base[0]}
	diffReason[0].CanReason = false
	require.True(t, discoveredModelsDiffer(base, diffReason))

	diffLevels := []catwalk.Model{base[0]}
	diffLevels[0].ReasoningLevels = []string{"low", "medium", "high"}
	require.True(t, discoveredModelsDiffer(base, diffLevels))

	diffEffort := []catwalk.Model{base[0]}
	diffEffort[0].DefaultReasoningEffort = "high"
	require.True(t, discoveredModelsDiffer(base, diffEffort))
}

func TestFlushDiscoveredModelsPersistsWebSearchOption(t *testing.T) {
	dir := t.TempDir()
	server := discoveryServer(t, `{"data": [{"id": "m1", "supports_web_search": true}]}`)

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
	require.NoError(t, os.WriteFile(filepath.Join(dir, providersSectionFile), []byte(`{"providers":{"custom":{"id":"custom","type":"openai-compat","discover_models":true}}}`), 0o600))

	env := env.NewFromMap(map[string]string{})
	resolver := NewShellVariableResolver(env)
	require.NoError(t, cfg.configureProviders(context.Background(), store, env, resolver, []catwalk.Provider{}))
	store.flushDiscoveredModels()

	data, err := os.ReadFile(filepath.Join(dir, providersSectionFile))
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(data, &document))
	models := document["providers"].(map[string]any)["custom"].(map[string]any)["models"].([]any)
	require.Len(t, models, 1)
	modelObj := models[0].(map[string]any)
	require.Equal(t, "m1", modelObj["id"])
	opts := modelObj["options"].(map[string]any)
	providerOpts := opts["provider_options"].(map[string]any)
	require.Equal(t, true, providerOpts["web_search"])
}
