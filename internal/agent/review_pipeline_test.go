package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools"
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
	require.Contains(t, block, "Never launch a review agent while any code agent is running")
	require.Contains(t, block, "sends its complete report straight to the author")
	require.Contains(t, block, "do not relay defects yourself")
	require.Contains(t, block, "agree interfaces directly with agent_message")
	require.Contains(t, block, "reaches 5 rounds")
	require.Contains(t, block, "If the project directory is empty or creating a new project from scratch, skip explore")
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

	_, ok = runReasoningMode(c.withTeamReasoning(context.Background(), "other"))
	require.False(t, ok)

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

func TestReviewWaitsForCodeAgentsInStrictMode(t *testing.T) {
	t.Setenv("PRIME_DESKTOP", "1")
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	coder := newBlockingAgent("job-provider")
	job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: coder, SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "code", Prompt: "build"}, config.AgentCode)
	require.NoError(t, err)
	require.Equal(t, 1, c.activeCodeAgents(parent.ID))

	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, parent.ID)
	require.Empty(t, c.reviewLaunchBlocked(ctx, parent.ID), "without Max the reviewer is never blocked")

	maxCtx := WithReasoningMode(ctx, config.ReasoningModeMax)
	require.Contains(t, c.reviewLaunchBlocked(maxCtx, parent.ID), "1 code agent(s) are still running")

	tool, err := c.agentTool(maxCtx)
	require.NoError(t, err)
	input, _ := json.Marshal(AgentParams{Prompt: "review it", Agent: config.AgentReview, Background: true})
	response, err := tool.Run(context.WithValue(maxCtx, tools.MessageIDContextKey, "msg"), fantasy.ToolCall{ID: "review", Input: string(input)})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, "Strict review pipeline")

	close(coder.blockOn)
	require.True(t, job.wait(t.Context(), time.Second))
	require.Zero(t, c.activeCodeAgents(parent.ID))
	require.Empty(t, c.reviewLaunchBlocked(maxCtx, parent.ID))
}
