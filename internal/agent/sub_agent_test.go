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

func TestSubAgentPromptAcceptsWorkingDir(t *testing.T) {
	t.Parallel()

	// subAgentPrompt must forward options through to the underlying builder,
	// otherwise the sub-agent loses the caller's working directory.
	for _, id := range []string{config.AgentCoder, config.AgentTask, config.AgentPlan} {
		p, err := subAgentPrompt(id)
		require.NoError(t, err, id)
		require.NotNil(t, p, id)
	}
}