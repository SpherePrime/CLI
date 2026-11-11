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
	maxModel := config.SelectedModel{ReasoningMode: config.ReasoningModeMax}
	highModel := config.SelectedModel{ReasoningMode: config.ReasoningModeHigh}

	t.Setenv("PRIME_DESKTOP", "1")
	block := strictReviewBlock(true, maxModel)
	require.Contains(t, block, "<strict_review_pipeline>")
	require.Contains(t, block, "vertical slices only")
	require.Contains(t, block, "reviewer=<that review agent's name>")
	require.Contains(t, block, "always start as waiting")
	require.Contains(t, block, "delivered to the reviewer together with the author")
	require.Contains(t, block, "Do not relay defects")
	require.Contains(t, block, "After 3 rounds")
	require.Contains(t, block, "only critical defects go back to the author")
	require.Contains(t, block, "skip explore")
	require.NotContains(t, block, "scaffolding, components and styling")
	require.Contains(t, block, "an APPROVED part is verified")
	require.NotContains(t, block, "final check of the integrated whole")
	require.Empty(t, strictReviewBlock(false, maxModel))
	require.Empty(t, strictReviewBlock(true, highModel))

	t.Setenv("PRIME_DESKTOP", "")
	require.Empty(t, strictReviewBlock(true, maxModel))
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

