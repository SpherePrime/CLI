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
		coderAgent, ok := cfg.Agents[AgentGeneral]
		require.True(t, ok)
		assert.Equal(t, AgentGeneral, coderAgent.ID, "Coder agent ID should be '%s'", AgentGeneral)
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
		require.True(t, has(AgentGeneral, tool), "the main agent must have %s", tool)
		require.True(t, has(AgentCode, tool), "code must have %s", tool)
		require.False(t, has(AgentTask, tool), "task must not have %s", tool)
		require.False(t, has(AgentPlan, tool), "plan must not have %s", tool)
	}

	// One level of delegation: the main agent is the one that delegates, and
	// no worker may. A worker that could call a worker could grow a chain with
	// nothing to bound it.
	require.True(t, has(AgentGeneral, "agent"),
		"the main agent is the one that delegates")
	for _, worker := range []string{AgentCode, AgentTask, AgentPlan} {
		require.False(t, has(worker, "agent"),
			"%s must not be able to delegate further", worker)
	}

	// Research agents keep the read-only lookups.
	for _, tool := range []string{"glob", "grep", "ls", "view"} {
		require.True(t, has(AgentTask, tool), "task must keep %s", tool)
		require.True(t, has(AgentPlan, tool), "plan must keep %s", tool)
	}
}

// general is the agent the session runs on, so it must not be offered as a
// delegation target, while the workers must be.
func TestConfig_MainAgentIsDistinctFromWorkers(t *testing.T) {
	cfg := &Config{Options: &Options{DisabledTools: []string{}}}
	cfg.SetupAgents()

	require.Contains(t, cfg.Agents, AgentGeneral)
	require.Contains(t, cfg.Agents, AgentCode)
	require.Contains(t, cfg.Agents, AgentTask)
	require.Contains(t, cfg.Agents, AgentPlan)

	general := cfg.Agents[AgentGeneral]
	for _, worker := range []string{AgentCode, AgentTask, AgentPlan} {
		require.NotEqual(t, general.ID, cfg.Agents[worker].ID,
			"the main agent and %s must be different agents", worker)
		require.NotEqual(t, general.Name, cfg.Agents[worker].Name)
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

// The main agent was called "coder" until it was renamed to "general". A
// config written before the rename carries its settings under the old id, and
// silently dropping them would lose a pinned model somebody chose.
func TestConfig_CoderSettingsMigrateToGeneral(t *testing.T) {
	cfg := &Config{
		Options: &Options{DisabledTools: []string{}},
		Agents: map[string]Agent{
			LegacyAgentCoder: {
				ID:            LegacyAgentCoder,
				Disabled:      true,
				ModelOverride: &SelectedModel{Provider: "p", Model: "main-pin"},
			},
		},
	}
	cfg.SetupAgents()

	require.NotContains(t, cfg.Agents, LegacyAgentCoder,
		"the old id must not linger")
	require.Equal(t, "main-pin", cfg.Agents[AgentGeneral].ModelOverride.Model,
		"the main agent's pinned model must follow the rename")
	require.True(t, cfg.Agents[AgentGeneral].Disabled,
		"a disabled main agent must stay disabled")
}

// "general" named a full-tools worker for one release before the main agent
// took the name. A config from that window has both ids, and the old general
// settings belong on the code worker.
func TestConfig_OldGeneralWorkerSettingsMigrateToCode(t *testing.T) {
	cfg := &Config{
		Options: &Options{DisabledTools: []string{}},
		Agents: map[string]Agent{
			LegacyAgentCoder: {ID: LegacyAgentCoder},
			AgentGeneral: {
				ID:            AgentGeneral,
				ModelOverride: &SelectedModel{Provider: "p", Model: "worker-pin"},
			},
		},
	}
	cfg.SetupAgents()

	require.Equal(t, "worker-pin", cfg.Agents[AgentCode].ModelOverride.Model,
		"the old worker pin belongs on code")
	require.Nil(t, cfg.Agents[AgentGeneral].ModelOverride,
		"and must not also be pinned onto the main agent")
	require.Nil(t, cfg.Agents[AgentGeneral].ModelOverride)
}

// A config from after the rename has no "coder" entry, so its "general" is the
// main agent and must be left alone.
func TestConfig_NewGeneralIsNotTreatedAsTheOldWorker(t *testing.T) {
	cfg := &Config{
		Options: &Options{DisabledTools: []string{}},
		Agents: map[string]Agent{
			AgentGeneral: {
				ID:            AgentGeneral,
				ModelOverride: &SelectedModel{Provider: "p", Model: "main-pin"},
			},
		},
	}
	cfg.SetupAgents()

	require.Equal(t, "main-pin", cfg.Agents[AgentGeneral].ModelOverride.Model)
	require.Nil(t, cfg.Agents[AgentCode].ModelOverride,
		"code must not inherit the main agent's pin")
}
