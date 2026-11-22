package agent

import (
	"context"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestReviewPolicyBlockReturnsEmpty(t *testing.T) {
	t.Setenv("PRIME_DESKTOP", "1")
	for _, mode := range []string{
		config.ReasoningModeMax,
		config.ReasoningModeHigh,
		config.ReasoningModeMedium,
		config.ReasoningModeLow,
		config.ReasoningModeOff,
		config.ReasoningModeXHigh,
		config.ReasoningModeDefault,
	} {
		require.Empty(t, reviewPolicyBlock(true, mode))
		require.Empty(t, reviewPolicyBlock(false, mode))
	}

	t.Setenv("PRIME_DESKTOP", "")
	require.Empty(t, reviewPolicyBlock(true, config.ReasoningModeMax))
}

func TestReviewDepthFollowsReasoningLevel(t *testing.T) {
	require.Equal(t, reviewDepthLight, reviewDepthFor(config.ReasoningModeOff))
	require.Equal(t, reviewDepthQuick, reviewDepthFor(""))
	t.Setenv("PRIME_DESKTOP", "1")
	require.Equal(t, reviewDepthStrict, reviewDepthFor(config.ReasoningModeMax))
	require.Equal(t, 1, reviewDepthLight.rounds())
	require.Equal(t, 3, reviewDepthStrict.rounds())
	require.Contains(t, reviewDepthLight.note(), "REVIEW DEPTH: light")
	require.Contains(t, reviewDepthLight.note(), "at most 1 review round;")
	require.Contains(t, reviewDepthThorough.note(), "REVIEW DEPTH: thorough")
}

func TestCoordinatorReasoningPrefersTheRunLevel(t *testing.T) {
	model := config.SelectedModel{ReasoningMode: config.ReasoningModeHigh}
	require.Equal(t, config.ReasoningModeHigh, coordinatorReasoningMode(context.Background(), model))
	ctx := WithReasoningMode(context.Background(), config.ReasoningModeDefault)
	require.Equal(t, config.ReasoningModeDefault, coordinatorReasoningMode(ctx, model))
}

func TestTeamReasoningSurvivesWakeRuns(t *testing.T) {
	c := &coordinator{}
	userRun := c.withTeamReasoning(WithReasoningMode(context.Background(), config.ReasoningModeMax), "root")
	mode, ok := runReasoningMode(userRun)
	require.True(t, ok)
	require.Equal(t, config.ReasoningModeMax, mode)

	wakeRun := c.withTeamReasoning(context.Background(), "root")
	mode, ok = runReasoningMode(wakeRun)
	require.True(t, ok)
	require.Equal(t, config.ReasoningModeMax, mode)

	mode, ok = runReasoningMode(c.withTeamReasoning(context.Background(), "other"))
	require.True(t, ok)
	require.Equal(t, config.ReasoningModeMax, mode)

	c.withTeamReasoning(WithReasoningMode(context.Background(), config.ReasoningModeLow), "root")
	require.Equal(t, config.ReasoningModeLow, c.teamReasoningMode("root"))
}

func TestWorkersRunOnTheTeamReasoningLevel(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	calls := make(chan SessionAgentCall, 2)
	worker := newMockAgent("job-provider", 1024, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		calls <- call
		return agentResultWithText("done"), nil
	})

	job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: worker, SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "plain", Prompt: "work"}, config.AgentCode)
	require.NoError(t, err)
	require.Nil(t, (<-calls).ModelOverride)
	require.True(t, job.wait(t.Context(), time.Second))

	c.withTeamReasoning(WithReasoningMode(t.Context(), config.ReasoningModeMax), parent.ID)
	job, err = c.startAgentJob(t.Context(), subAgentParams{Agent: worker, SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "max", Prompt: "work"}, config.AgentCode)
	require.NoError(t, err)
	call := <-calls
	require.NotNil(t, call.ModelOverride)
	require.Equal(t, config.ReasoningModeMax, call.ModelOverride.ModelCfg.EffectiveReasoningMode())
	require.True(t, job.wait(t.Context(), time.Second))
}

