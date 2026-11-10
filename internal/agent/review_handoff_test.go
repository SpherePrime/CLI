package agent

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type scriptedAgent struct {
	mu      sync.Mutex
	replies []string
	prompts []string
}

func (s *scriptedAgent) run(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.prompts = append(s.prompts, call.Prompt)
	reply := s.replies[0]
	if len(s.replies) > 1 {
		s.replies = s.replies[1:]
	}
	return agentResultWithText(reply), nil
}

func (s *scriptedAgent) calls() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.prompts...)
}

func waitUntil(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("condition not reached")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func jobStatus(job *agentJob) string {
	status, _ := job.snapshot()
	return status
}

func TestDormantAgentWaitsForMessage(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	script := &scriptedAgent{replies: []string{"VERDICT: APPROVED"}}
	reviewer := newMockAgent("job-provider", 1024, script.run)
	job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: reviewer, SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "rev", Prompt: "review the page", SessionTitle: "Agent Session: review | rev", Dormant: true}, config.AgentReview)
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, "waiting", jobStatus(job))
	require.Empty(t, script.calls())

	_, err = c.SendAgentWorkerMessage(t.Context(), parent.ID, "rev", "page is ready")
	require.NoError(t, err)
	waitUntil(t, func() bool { return jobStatus(job) == "done" })
	calls := script.calls()
	require.Len(t, calls, 1)
	require.Contains(t, calls[0], "review the page")
	require.Contains(t, calls[0], "page is ready")
}

func TestStoppingDormantAgentCancelsIt(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	script := &scriptedAgent{replies: []string{"x"}}
	job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: newMockAgent("job-provider", 1024, script.run), SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "rev", Prompt: "review", Dormant: true}, config.AgentReview)
	require.NoError(t, err)
	require.NoError(t, c.StopAgentWorker(t.Context(), parent.ID, job.id))
	require.True(t, job.wait(t.Context(), time.Second))
	require.Equal(t, "canceled", jobStatus(job))
	require.Empty(t, script.calls())
}

func TestReviewerStartsDormantWhileCodeRuns(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	require.False(t, c.reviewStartsDormant(t.Context(), parent.ID))

	coder := newBlockingAgent("job-provider")
	job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: coder, SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "code", Prompt: "build"}, config.AgentCode)
	require.NoError(t, err)
	require.True(t, c.reviewStartsDormant(t.Context(), parent.ID))

	close(coder.blockOn)
	require.True(t, job.wait(t.Context(), time.Second))
	require.False(t, c.reviewStartsDormant(t.Context(), parent.ID))
}

func TestReviewLoopRunsBetweenAuthorAndReviewer(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	reviewScript := &scriptedAgent{replies: []string{"VERDICT: CHANGES_REQUIRED\nDEFECTS:\n- [major] header overlaps", "VERDICT: APPROVED\nDEFECTS: none"}}
	authorScript := &scriptedAgent{replies: []string{"built the page", "fixed the header"}}
	reviewJob, err := c.startAgentJob(t.Context(), subAgentParams{Agent: newMockAgent("job-provider", 1024, reviewScript.run), SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "rev", Prompt: "review the page", SessionTitle: "Agent Session: review | page-review", Dormant: true}, config.AgentReview)
	require.NoError(t, err)
	authorJob, err := c.startAgentJob(t.Context(), subAgentParams{Agent: newMockAgent("job-provider", 1024, authorScript.run), SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "code", Prompt: "build the page", SessionTitle: "Agent Session: code | page", Reviewer: "page-review"}, config.AgentCode)
	require.NoError(t, err)

	waitUntil(t, func() bool { return len(reviewScript.calls()) == 2 && jobStatus(reviewJob) == "done" })
	authorCalls := authorScript.calls()
	require.Len(t, authorCalls, 2)
	require.Contains(t, authorCalls[1], "Automatic review result, round 1")
	require.Contains(t, authorCalls[1], "header overlaps")
	reviewCalls := reviewScript.calls()
	require.Contains(t, reviewCalls[0], "Automatic review handoff")
	require.Contains(t, reviewCalls[0], "built the page")
	require.Contains(t, reviewCalls[1], "fixed the header")

	reviewJob.mu.Lock()
	require.Equal(t, 2, reviewJob.reviewRound)
	require.Equal(t, authorJob.id, reviewJob.author)
	reviewJob.mu.Unlock()
	authorJob.mu.Lock()
	require.True(t, authorJob.seen, "author reports routed to the reviewer do not wake the coordinator")
	authorJob.mu.Unlock()
}

func TestAuthorWaitingOnPeerIsNotHandedOff(t *testing.T) {
	c := &coordinator{}
	job := &agentJob{agent: config.AgentCode, reviewer: "rev"}
	require.Nil(t, c.reviewRoute(job, "done", "STATUS: WAITING\nneed the api types from api-agent"))
	require.NotNil(t, c.reviewRoute(job, "done", "built it"))
	require.Nil(t, c.reviewRoute(job, "error", "built it"))
	require.Nil(t, c.reviewRoute(&agentJob{agent: config.AgentCode}, "done", "built it"))
}

func TestReviewRoundLimit(t *testing.T) {
	c := &coordinator{}
	minor := "VERDICT: CHANGES_REQUIRED\n- [minor] spacing"
	critical := "VERDICT: CHANGES_REQUIRED\n- [critical] (repeat 2) build fails"

	route := c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", reviewRound: 3}, "done", minor)
	require.Equal(t, "a", route.target)
	require.True(t, route.quiet)

	route = c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", reviewRound: 10}, "done", minor)
	require.Empty(t, route.target)
	require.Contains(t, route.note, "limit (10) reached")

	route = c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", reviewRound: 12}, "done", critical)
	require.Equal(t, "a", route.target)
	require.False(t, route.quiet)
	require.Contains(t, route.note, "critical defects remain")

	require.Nil(t, c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", reviewRound: 1}, "done", "VERDICT: APPROVED\nDEFECTS: none"))
}
