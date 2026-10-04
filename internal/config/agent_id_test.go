package config

import (
	"slices"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/assert"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestConfig_AgentIDs(t *testing.T) {
	cfg := &Config{
		Options: &Options{
			DisabledTools: []string{},
		},
	}
	cfg.SetupAgents()

	t.Run("Coder agent should have correct ID", func(t *testing.T) {
		coderAgent, ok := cfg.Agents[AgentCoder]
		require.True(t, ok)
		assert.Equal(t, AgentCoder, coderAgent.ID, "Coder agent ID should be '%s'", AgentCoder)
	})

	t.Run("Task agent should have correct ID", func(t *testing.T) {
		taskAgent, ok := cfg.Agents[AgentTask]
		require.True(t, ok)
		assert.Equal(t, AgentTask, taskAgent.ID, "Task agent ID should be '%s'", AgentTask)
	})

	t.Run("Plan agent should have correct ID", func(t *testing.T) {
		planAgent, ok := cfg.Agents[AgentPlan]
		require.True(t, ok)
		assert.Equal(t, AgentPlan, planAgent.ID, "Plan agent ID should be '%s'", AgentPlan)
	})

	t.Run("General agent should have correct ID", func(t *testing.T) {
		generalAgent, ok := cfg.Agents[AgentGeneral]
		require.True(t, ok)
		assert.Equal(t, AgentGeneral, generalAgent.ID, "General agent ID should be '%s'", AgentGeneral)
	})
}

// Task reads and plan reads, because a sub-agent that could write would change
// files while the model was still deciding what to do about them. General does
// the writing, which is the whole point of having it.
func TestConfig_AgentToolScopes(t *testing.T) {
	cfg := &Config{Options: &Options{DisabledTools: []string{}}}
	cfg.SetupAgents()

	has := func(id string, tool string) bool {
		agent, ok := cfg.Agents[id]
		require.True(t, ok, id)
		return slices.Contains(agent.AllowedTools, tool)
	}

	for _, tool := range []string{"edit", "write", "bash", "multiedit"} {
		require.False(t, has(AgentTask, tool), "task must not have %s", tool)
		require.False(t, has(AgentPlan, tool), "plan must not have %s", tool)
		require.True(t, has(AgentGeneral, tool), "general must have %s", tool)
	}

	// One level of delegation. A worker that could call a worker could grow a
	// chain with nothing to stop it.
	require.False(t, has(AgentGeneral, "agent"),
		"general must not be able to delegate further")
	require.True(t, has(AgentCoder, "agent"),
		"the main agent is the one that delegates")

	// Research agents keep the read-only lookups.
	for _, tool := range []string{"glob", "grep", "ls", "view"} {
		require.True(t, has(AgentTask, tool), "task must keep %s", tool)
	}
}

// The default agent map is rebuilt on every load and only model overrides are
// carried over from the file, so an agent added in a later version reaches
// existing installations instead of being invisible until the config is reset.
func TestConfig_NewAgentsReachExistingConfigs(t *testing.T) {
	cfg := &Config{
		Options: &Options{DisabledTools: []string{}},
		Agents: map[string]Agent{
			// An older config: no general, and a pinned model on task.
			AgentTask: {
				ID:            AgentTask,
				ModelOverride: &SelectedModel{Provider: "p", Model: "m"},
			},
		},
	}
	cfg.SetupAgents()

	require.Contains(t, cfg.Agents, AgentGeneral,
		"an agent added in a later version must appear for existing users")
	require.Equal(t, "m", cfg.Agents[AgentTask].ModelOverride.Model,
		"a pinned model must survive the defaults being rebuilt")
}
