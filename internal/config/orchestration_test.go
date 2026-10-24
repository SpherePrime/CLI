package config

import (
	"encoding/json"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestOrchestrationDefaultsAndSettings(t *testing.T) {
	options := Options{}
	require.False(t, options.DisableMultiAgent)
	require.Equal(t, 10, options.GetMaxConcurrentAgents())
	require.NoError(t, json.Unmarshal([]byte(`{"disable_multi_agent":true,"max_concurrent_agents":3}`), &options))
	require.True(t, options.DisableMultiAgent)
	require.Equal(t, 3, options.GetMaxConcurrentAgents())
	options.MaxConcurrentAgents = -1
	require.Equal(t, 10, options.GetMaxConcurrentAgents())
}

func TestWorkerMessagingKeepsReadOnlyRestrictions(t *testing.T) {
	cfg := Config{Options: &Options{}}
	cfg.SetupAgents()
	for _, id := range []string{AgentGeneral, AgentCode, AgentTask, AgentPlan} {
		require.Contains(t, cfg.Agents[id].AllowedTools, "agent_message")
	}
	for _, id := range []string{AgentTask, AgentPlan} {
		require.NotContains(t, cfg.Agents[id].AllowedTools, "edit")
		require.NotContains(t, cfg.Agents[id].AllowedTools, "agent")
	}
}
