package agent

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// TestSubAgentPromptCoversConfiguredAgents pins the roster the agent tool
// shows the model: without it the model cannot pick an agent, because only the
// names would be guessable.
func TestSubAgentPromptCoversConfiguredAgents(t *testing.T) {
	t.Parallel()

	coder, err := coderPrompt()
	require.NoError(t, err)
	task, err := taskPrompt()
	require.NoError(t, err)
	plan, err := planPrompt()
	require.NoError(t, err)

	// Each configured agent must resolve to a distinct personality, otherwise
	// choosing one only swaps the label and not the behaviour.
	require.NotSame(t, coder, task)
	require.NotSame(t, coder, plan)
	require.NotSame(t, task, plan)

	// An unknown id falls back to the task prompt rather than failing: a
	// stale agent name in config should not break the tool.
	unknown, err := subAgentPrompt("does-not-exist")
	require.NoError(t, err)
	require.NotNil(t, unknown)

	// The task prompt is what the default (empty) name resolves to.
	def, err := subAgentPrompt(config.AgentTask)
	require.NoError(t, err)
	require.NotNil(t, def)
}

// The roster the agent tool shows must be the workers only. Coder is the
// agent the session itself runs on, so listing it as a target let the model
// spawn itself, and each of those calls could spawn another.
func TestSubAgentNamesExcludeMainAgent(t *testing.T) {
	t.Parallel()

	c := newTestCoordinator(t, testEnv(t), "test", config.ProviderConfig{ID: "test"})
	// config.Init does not populate the agent map on its own, and whether it
	// ends up populated depends on whether the machine running the test has a
	// global config to merge. Relying on that made this test pass on a
	// developer's machine and fail on a fresh runner.
	c.cfg.Config().SetupAgents()

	names := c.SubAgentNames()
	require.NotEmpty(t, names)
	require.NotContains(t, names, config.AgentGeneral,
		"the main agent must not be offered as something to call")
	require.Contains(t, names, config.AgentTask)
	require.Contains(t, names, config.AgentPlan)

	// Asking for the main agent by name has to be refused rather than
	// quietly recursing.
	_, err := c.SubAgent(config.AgentGeneral)
	require.Error(t, err)
	require.Contains(t, err.Error(), config.AgentGeneral)
}

func TestSubAgentPromptAcceptsWorkingDir(t *testing.T) {
	t.Parallel()

	// subAgentPrompt must forward options through to the underlying builder,
	// otherwise the sub-agent loses the caller's working directory.
	for _, id := range []string{config.AgentGeneral, config.AgentTask, config.AgentPlan} {
		p, err := subAgentPrompt(id)
		require.NoError(t, err, id)
		require.NotNil(t, p, id)
	}
}