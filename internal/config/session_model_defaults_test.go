package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSessionModelRestoresSavedContextSettings(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PRIME_GLOBAL_CONFIG", dir)
	t.Setenv("PRIME_GLOBAL_DATA", dir)
	resetProviderState()
	t.Cleanup(resetProviderState)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "prime.json"), []byte(twoProviderConfig("openai", "gpt-4")), 0o600))
	store, err := Load(dir, dir, false)
	require.NoError(t, err)
	selected := SelectedModel{Provider: "anthropic", Model: "claude-3", ContextWindow: 250000}
	require.NoError(t, store.UpdatePreferredModel(ScopeGlobal, SelectedModelTypeLarge, selected))
	require.NoError(t, store.SetSessionModel("chat", &SelectedModel{Provider: selected.Provider, Model: selected.Model}))
	require.EqualValues(t, 250000, store.Config().SessionModels["chat"].ContextWindow)
}
