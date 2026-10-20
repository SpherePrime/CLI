package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func providerEditFixture(t *testing.T, id string) ProviderEdit {
	t.Helper()
	var edit ProviderEdit
	require.NoError(t, json.Unmarshal([]byte(`{"id":"`+id+`","name":"Changed","base_url":"http://localhost:11434/v1","type":"openai-compat","discover_models":true,"models":[{"id":"partial","name":"Partial"},{"id":"zero","context_window":0,"default_max_tokens":0}]}`), &edit))
	return edit
}

func TestSaveProviderPreservesSecretsAndRenamesReferences(t *testing.T) {
	dir := t.TempDir()
	store := &ConfigStore{config: &Config{}, globalDataPath: filepath.Join(dir, "prime.json")}
	providers := []byte(`{"providers":{"old":{"id":"old","api_key":"keep-secret","headers":{"Authorization":"private"},"oauth":{"access_token":"oauth-private"},"flat_rate":true,"models":[{"id":"partial","supports_attachments":true,"cost_per_1m_in":3}]},"other":{"api_key":"other-private"}}}`)
	require.NoError(t, os.WriteFile(filepath.Join(dir, providersSectionFile), providers, 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, modelsSectionFile), []byte(`{"models":{"large":{"provider":"old","model":"partial"}},"recent_models":{"large":[{"provider":"old","model":"partial"}]},"model_settings":{"old|partial":{"temperature":0.8}}}`), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "prime.json"), []byte(`{"agents":{"custom":{"model_override":{"provider":"old","model":"partial"}}},"unrelated":true}`), 0600))
	emptyKey := ""
	require.NoError(t, store.SaveProvider(ScopeGlobal, "old", providerEditFixture(t, "renamed"), &emptyKey))
	data, err := os.ReadFile(filepath.Join(dir, providersSectionFile))
	require.NoError(t, err)
	var document map[string]any
	require.NoError(t, json.Unmarshal(data, &document))
	entries := document["providers"].(map[string]any)
	require.NotContains(t, entries, "old")
	saved := entries["renamed"].(map[string]any)
	require.Equal(t, "keep-secret", saved["api_key"])
	require.Equal(t, true, saved["flat_rate"])
	require.Contains(t, saved, "headers")
	require.Contains(t, saved, "oauth")
	require.Equal(t, true, saved["models"].([]any)[0].(map[string]any)["supports_attachments"])
	require.Equal(t, "other-private", entries["other"].(map[string]any)["api_key"])
	for _, name := range []string{modelsSectionFile, "prime.json"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		require.NoError(t, err)
		require.NotContains(t, string(data), `"old"`)
		require.Contains(t, string(data), "renamed")
	}
}

func TestSaveProviderRejectsInvalidAndCollisionWithoutWriting(t *testing.T) {
	dir := t.TempDir()
	store := &ConfigStore{config: &Config{}, globalDataPath: filepath.Join(dir, "prime.json")}
	original := []byte(`{"providers":{"old":{"api_key":"secret"},"taken":{"api_key":"different"}}}`)
	path := filepath.Join(dir, providersSectionFile)
	require.NoError(t, os.WriteFile(path, original, 0600))
	edit := providerEditFixture(t, "taken")
	require.ErrorContains(t, store.SaveProvider(ScopeGlobal, "old", edit, nil), "уже существует")
	edit.ID = "old"
	edit.Models[0]["context_window"] = float64(-1)
	require.ErrorContains(t, store.SaveProvider(ScopeGlobal, "old", edit, nil), "partial")
	edit.Models[0]["context_window"] = float64(10)
	edit.Models[0]["default_max_tokens"] = float64(20)
	require.ErrorContains(t, store.SaveProvider(ScopeGlobal, "old", edit, nil), "превышает")
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, original, data)
}

func TestSaveProviderWorkspaceInheritance(t *testing.T) {
	globalDir, projectDir := t.TempDir(), t.TempDir()
	providers := csync.NewMap[string, ProviderConfig]()
	providers.Set("inherited", ProviderConfig{ID: "inherited", APIKey: "inherited-secret"})
	store := &ConfigStore{config: &Config{Providers: providers}, globalDataPath: filepath.Join(globalDir, "prime.json"), workspacePath: filepath.Join(projectDir, "prime.json")}
	global := []byte(`{"providers":{"inherited":{"api_key":"inherited-secret"}}}`)
	require.NoError(t, os.WriteFile(filepath.Join(globalDir, providersSectionFile), global, 0600))
	require.ErrorContains(t, store.SaveProvider(ScopeWorkspace, "inherited", providerEditFixture(t, "different"), nil), "области")
	require.NoError(t, store.SaveProvider(ScopeWorkspace, "inherited", providerEditFixture(t, "inherited"), nil))
	local, err := os.ReadFile(filepath.Join(projectDir, providersSectionFile))
	require.NoError(t, err)
	require.Contains(t, string(local), "inherited-secret")
	unchanged, err := os.ReadFile(filepath.Join(globalDir, providersSectionFile))
	require.NoError(t, err)
	require.Equal(t, global, unchanged)
	require.ErrorContains(t, store.ValidateGlobalProviderRename("inherited"), "области проекта")
}

func TestSaveProviderWaitsForReloadAndPublishesFreshSnapshot(t *testing.T) {
	dir := t.TempDir()
	workDir, dataDir := t.TempDir(), t.TempDir()
	t.Setenv("PRIME_GLOBAL_CONFIG", dir)
	t.Setenv("PRIME_GLOBAL_DATA", t.TempDir())
	path := filepath.Join(dir, "prime.json")
	require.NoError(t, os.WriteFile(path, []byte(`{"options":{"disable_default_providers":true,"disable_provider_auto_update":true},"providers":{"old":{"id":"old","name":"Old","type":"openai-compat","api_key":"fixture-key","base_url":"http://localhost:11434/v1","discover_models":false,"models":[{"id":"partial","name":"Partial","context_window":64000,"default_max_tokens":8192}]}},"models":{"large":{"provider":"old","model":"partial"},"small":{"provider":"old","model":"partial"}}}`), 0600))
	store, err := Load(workDir, dataDir, false)
	require.NoError(t, err)
	store.globalDataPath = path
	store.writeMu.Lock()
	done := make(chan error, 1)
	go func() { done <- store.SaveProvider(ScopeGlobal, "old", providerEditFixture(t, "old"), nil) }()
	select {
	case err := <-done:
		store.writeMu.Unlock()
		t.Fatalf("save returned before pending reload completed: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	store.writeMu.Unlock()
	require.NoError(t, <-done)
	provider, exists := store.Config().Providers.Get("old")
	require.True(t, exists)
	require.Len(t, provider.Models, 2)
	require.Equal(t, int64(0), provider.Models[0].ContextWindow)
	require.NoError(t, store.SaveProvider(ScopeGlobal, "old", providerEditFixture(t, "renamed"), nil))
	_, exists = store.Config().Providers.Get("old")
	require.False(t, exists)
	require.Equal(t, "renamed", store.Config().Models[SelectedModelTypeLarge].Provider)
}
