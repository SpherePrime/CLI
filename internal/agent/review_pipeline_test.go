package agent

import (
	"context"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestStrictReviewBlockOnlyForDesktopMaxCoordinator(t *testing.T) {
	t.Setenv("PRIME_DESKTOP", "1")
	block := reviewPolicyBlock(true, config.ReasoningModeMax)
	require.Contains(t, block, "<strict_review_pipeline>")
	require.Contains(t, block, "vertical slices only")
	require.Contains(t, block, "reviewer=<that review agent's name>")
	require.Contains(t, block, "always start as waiting")
	require.Contains(t, block, "delivered to the reviewer together with the author")
	require.Contains(t, block, "Do not relay defects")
	require.Contains(t, block, "After 3 rounds")
	require.Contains(t, block, "only critical defects go back to the author")
	require.Contains(t, block, "skip explore")
	require.Contains(t, block, "acceptance criteria as a numbered list")
	require.Contains(t, block, "checked in the browser without launching the native app")
	require.NotContains(t, block, "scaffolding, components and styling")
	require.Contains(t, block, "an APPROVED part is verified")
	require.NotContains(t, block, "final check of the integrated whole")
	require.Empty(t, reviewPolicyBlock(false, config.ReasoningModeMax))
	require.NotContains(t, reviewPolicyBlock(true, config.ReasoningModeHigh), "<strict_review_pipeline>")

	t.Setenv("PRIME_DESKTOP", "")
	require.NotContains(t, reviewPolicyBlock(true, config.ReasoningModeMax), "<strict_review_pipeline>")
	require.Contains(t, reviewPolicyBlock(true, config.ReasoningModeMax), "Review depth for this task: thorough")
}

func TestReviewPolicyFollowsReasoningLevel(t *testing.T) {
	t.Setenv("PRIME_DESKTOP", "1")
	light := reviewPolicyBlock(true, config.ReasoningModeLow)
	require.Contains(t, light, "Review depth for this task: light")
	require.Contains(t, light, "Do not launch review agents unless the user asks")
	require.Contains(t, light, "close-up")
	require.Equal(t, light, reviewPolicyBlock(true, config.ReasoningModeOff))

	quick := reviewPolicyBlock(true, config.ReasoningModeMedium)
	require.Contains(t, quick, "Review depth for this task: quick")
	require.Contains(t, quick, "at most 2 review rounds")
	require.Equal(t, quick, reviewPolicyBlock(true, config.ReasoningModeHigh))
	require.Equal(t, quick, reviewPolicyBlock(true, config.ReasoningModeDefault))

	thorough := reviewPolicyBlock(true, config.ReasoningModeXHigh)
	require.Contains(t, thorough, "Review depth for this task: thorough")
	require.Contains(t, thorough, "reviewer=<that name>")

	require.Equal(t, reviewDepthLight, reviewDepthFor(config.ReasoningModeOff))
	require.Equal(t, reviewDepthQuick, reviewDepthFor(""))
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

