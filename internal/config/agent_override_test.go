package config

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// agentPinConfig selects openai/gpt-4 as the large model and defines a second
// provider so a pinned agent model can be a different one.
func agentPinConfig() string {
	return `{
		"models": {
			"large": {"provider": "openai", "model": "gpt-4"},
			"small": {"provider": "openai", "model": "gpt-4"}
		},
		"providers": {
			"openai": {
				"api_key": "test-key",
				"models": [{"id": "gpt-4", "name": "GPT-4"}]
			},
			"anthropic": {
				"api_key": "test-key-2",
				"models": [{"id": "claude-3", "name": "Claude 3"}]
			}
		}
	}`
}

func loadAgentPinStore(t *testing.T) (*ConfigStore, string) {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "prime.json")

	t.Setenv("PRIME_GLOBAL_CONFIG", dir)
	t.Setenv("PRIME_GLOBAL_DATA", dir)
	resetProviderState()
	t.Cleanup(resetProviderState)

	require.NoError(t, os.WriteFile(configPath, []byte(agentPinConfig()), 0o600))

	store, err := Load(dir, dir, false)
	require.NoError(t, err)
	store.globalDataPath = configPath
	store.CaptureStalenessSnapshot([]string{configPath})

	return store, configPath
}

// TestAgentModelOverrideRoundTrip mirrors what the agents dialog does when a
// model is pinned to a subagent: write the whole agent under agents.<id>,
// then read it back after the reload the write triggers.
func TestAgentModelOverrideRoundTrip(t *testing.T) {
	store, configPath := loadAgentPinStore(t)

	agent, ok := store.Config().Agents[AgentCoder]
	require.True(t, ok, "agents should be set up for a configured store")

	agent.ModelOverride = &SelectedModel{Provider: "anthropic", Model: "claude-3"}
	require.NoError(t, store.SetConfigField(ScopeGlobal, "agents."+AgentCoder, agent))

	stored := store.Config().Agents[AgentCoder]
	require.NotNil(t, stored.ModelOverride, "pin lost right after SetConfigField")

	require.NoError(t, store.ReloadFromDisk(context.Background()))

	reread := store.Config().Agents[AgentCoder]
	require.NotNil(t, reread.ModelOverride, "pin lost after reload from disk")
	require.Equal(t, "anthropic", reread.ModelOverride.Provider)
	require.Equal(t, "claude-3", reread.ModelOverride.Model)

	pinned := store.Config().GetModelForAgent(reread)
	require.NotNil(t, pinned)
	require.Equal(t, "Claude 3", pinned.Name, "agent should resolve to the pinned model")

	disk, err := os.ReadFile(configPath)
	require.NoError(t, err)
	require.Contains(t, string(disk), "claude-3", "pin should be written to the config file")
}
