package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSetSessionModel_PersistsAndClears(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "prime.json")

	t.Setenv("PRIME_GLOBAL_CONFIG", dir)
	t.Setenv("PRIME_GLOBAL_DATA", dir)
	resetProviderState()
	t.Cleanup(resetProviderState)

	require.NoError(t, os.WriteFile(configPath, []byte(twoProviderConfig("openai", "gpt-4")), 0o600))

	store, err := Load(dir, dir, false)
	require.NoError(t, err)
	store.globalDataPath = configPath

	require.NoError(t, store.SetSessionModel("session-1", &SelectedModel{Provider: "anthropic", Model: "claude-3"}))
	stored := store.Config().SessionModels["session-1"]
	require.Equal(t, "anthropic", stored.Provider)
	require.Equal(t, "claude-3", stored.Model)
	require.Equal(t, "gpt-4", store.Config().Models[SelectedModelTypeLarge].Model)

	onDisk := readConfigJSON(t, configPath, "session_models")
	entries, ok := onDisk["session_models"].(map[string]any)
	require.True(t, ok)
	require.Contains(t, entries, "session-1")

	require.NoError(t, store.SetSessionModel("session-1", nil))
	require.NotContains(t, store.Config().SessionModels, "session-1")
	onDisk = readConfigJSON(t, configPath, "session_models")
	require.Empty(t, onDisk["session_models"])
	reloaded, err := Load(dir, dir, false)
	require.NoError(t, err)
	require.NotContains(t, reloaded.Config().SessionModels, "session-1")
}
