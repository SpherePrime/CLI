package config

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestTeamRolesExistWithRestrictedTools(t *testing.T) {
	cfg := Config{Options: &Options{}}
	cfg.SetupAgents()

	explore, ok := cfg.Agents[AgentExplore]
	require.True(t, ok)
	require.Contains(t, explore.AllowedTools, "bash")
	require.Contains(t, explore.AllowedTools, "view")
	require.Contains(t, explore.AllowedTools, "agent_message")
	for _, tool := range []string{"edit", "multiedit", "write", "agent", "agent_jobs"} {
		require.NotContains(t, explore.AllowedTools, tool)
	}

	review, ok := cfg.Agents[AgentReview]
	require.True(t, ok)
	for _, tool := range []string{"bash", "view", "agent_message", "lsp_diagnostics"} {
		require.Contains(t, review.AllowedTools, tool)
	}
	for _, tool := range []string{"edit", "multiedit", "write", "lsp_rename", "lsp_replace_symbol", "agent", "agent_jobs"} {
		require.NotContains(t, review.AllowedTools, tool)
	}
	require.Nil(t, review.AllowedMCP)
}

func TestTeamRolesRespectDisabledTools(t *testing.T) {
	cfg := Config{Options: &Options{DisabledTools: []string{"bash"}}}
	cfg.SetupAgents()
	require.NotContains(t, cfg.Agents[AgentExplore].AllowedTools, "bash")
	require.NotContains(t, cfg.Agents[AgentReview].AllowedTools, "bash")
}

func TestExploreInheritsPinnedTaskModel(t *testing.T) {
	cfg := Config{
		Options: &Options{},
		Agents: map[string]Agent{
			AgentTask: {ID: AgentTask, ModelOverride: &SelectedModel{Provider: "p", Model: "task-pin"}},
		},
	}
	cfg.SetupAgents()
	require.NotNil(t, cfg.Agents[AgentExplore].ModelOverride)
	require.Equal(t, "task-pin", cfg.Agents[AgentExplore].ModelOverride.Model)

	cfg = Config{
		Options: &Options{},
		Agents: map[string]Agent{
			AgentTask:    {ID: AgentTask, ModelOverride: &SelectedModel{Provider: "p", Model: "task-pin"}},
			AgentExplore: {ID: AgentExplore, ModelOverride: &SelectedModel{Provider: "p", Model: "explore-pin"}},
		},
	}
	cfg.SetupAgents()
	require.Equal(t, "explore-pin", cfg.Agents[AgentExplore].ModelOverride.Model)
}
