package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestTeamReasoningFallsBackToLastChosenMode(t *testing.T) {
	t.Parallel()

	c := &coordinator{}
	require.Equal(t, "", c.teamReasoningMode("child"))

	chosen := c.withTeamReasoning(WithReasoningMode(context.Background(), "max"), "root")
	mode, ok := runReasoningMode(chosen)
	require.True(t, ok)
	require.Equal(t, "max", mode)

	continued := c.withTeamReasoning(context.Background(), "root")
	mode, _ = runReasoningMode(continued)
	require.Equal(t, "max", mode, "a continuation of the same session keeps its mode")

	other := c.withTeamReasoning(context.Background(), "another-session")
	mode, _ = runReasoningMode(other)
	require.Equal(t, "max", mode, "a run without a mode of its own uses the last chosen mode")
	require.Equal(t, "max", c.teamReasoningMode("worker-root"))

	c.withTeamReasoning(WithReasoningMode(context.Background(), "low"), "second")
	mode, _ = runReasoningMode(c.withTeamReasoning(context.Background(), "root"))
	require.Equal(t, "max", mode, "a session keeps its own stored mode")
}
