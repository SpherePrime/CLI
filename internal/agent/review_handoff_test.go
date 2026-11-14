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

func TestReviewerIgnoresMessagesUntilAuthorHandsOff(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	coder := newBlockingAgent("job-provider")
	authorJob, err := c.startAgentJob(t.Context(), subAgentParams{Agent: coder, SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "code", Prompt: "build", SessionTitle: "Agent Session: code | site", Reviewer: "site-review"}, config.AgentCode)
	require.NoError(t, err)
	reviewScript := &scriptedAgent{replies: []string{"VERDICT: APPROVED\nDEFECTS: none"}}
	reviewJob, err := c.startAgentJob(t.Context(), subAgentParams{Agent: newMockAgent("job-provider", 1024, reviewScript.run), SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "rev", Prompt: "review the site strictly", SessionTitle: "Agent Session: review | site-review", Dormant: true, AwaitHandoff: true}, config.AgentReview)
	require.NoError(t, err)

	_, err = c.SendAgentWorkerMessage(t.Context(), parent.ID, "site-review", "full review checklist from the coordinator")
	require.NoError(t, err)
	time.Sleep(50 * time.Millisecond)
	require.Equal(t, "waiting", jobStatus(reviewJob))
	require.Empty(t, reviewScript.calls())

	close(coder.blockOn)
	require.True(t, authorJob.wait(t.Context(), time.Second))
	waitUntil(t, func() bool { return jobStatus(reviewJob) == "done" })
	calls := reviewScript.calls()
	require.Len(t, calls, 1)
	require.Contains(t, calls[0], "review the site strictly")
	require.Contains(t, calls[0], "full review checklist from the coordinator")
	require.Contains(t, calls[0], "Automatic review handoff")
}

func TestUnlinkedReviewerStartsWhenNoCodeAgentsRemain(t *testing.T) {
	previous := orphanReviewDelay
	orphanReviewDelay = 20 * time.Millisecond
	t.Cleanup(func() { orphanReviewDelay = previous })
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	reviewScript := &scriptedAgent{replies: []string{"VERDICT: APPROVED"}}
	reviewJob, err := c.startAgentJob(t.Context(), subAgentParams{Agent: newMockAgent("job-provider", 1024, reviewScript.run), SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "rev", Prompt: "review the finished site", SessionTitle: "Agent Session: review | late-review", Dormant: true, AwaitHandoff: true}, config.AgentReview)
	require.NoError(t, err)
	waitUntil(t, func() bool { return jobStatus(reviewJob) == "done" })
	calls := reviewScript.calls()
	require.Len(t, calls, 1)
	require.Contains(t, calls[0], "review the finished site")
	require.Contains(t, calls[0], "Every code agent of the team has finished")
}

func TestUnlinkedReviewerWaitsForRunningCodeAgents(t *testing.T) {
	previous := orphanReviewDelay
	orphanReviewDelay = 20 * time.Millisecond
	t.Cleanup(func() { orphanReviewDelay = previous })
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	coder := newBlockingAgent("job-provider")
	authorJob, err := c.startAgentJob(t.Context(), subAgentParams{Agent: coder, SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "code", Prompt: "build"}, config.AgentCode)
	require.NoError(t, err)
	reviewScript := &scriptedAgent{replies: []string{"VERDICT: APPROVED"}}
	reviewJob, err := c.startAgentJob(t.Context(), subAgentParams{Agent: newMockAgent("job-provider", 1024, reviewScript.run), SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "rev", Prompt: "review", SessionTitle: "Agent Session: review | any-review", Dormant: true, AwaitHandoff: true}, config.AgentReview)
	require.NoError(t, err)
	time.Sleep(100 * time.Millisecond)
	require.Equal(t, "waiting", jobStatus(reviewJob))

	close(coder.blockOn)
	require.True(t, authorJob.wait(t.Context(), time.Second))
	waitUntil(t, func() bool { return jobStatus(reviewJob) == "done" })
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

func TestStrictReviewAllowsThreeRounds(t *testing.T) {
	t.Setenv("PRIME_DESKTOP", "1")
	c := &coordinator{}
	c.withTeamReasoning(WithReasoningMode(context.Background(), config.ReasoningModeMax), "root")
	minor := "VERDICT: CHANGES_REQUIRED\n- [major] header overlaps"
	require.Equal(t, "a", c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", rootSessionID: "root", reviewRound: 2}, "done", minor).target)
	route := c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", rootSessionID: "root", reviewRound: 3}, "done", minor)
	require.Empty(t, route.target)
	require.Contains(t, route.note, "limit (3) reached")
	require.Contains(t, c.reviewDepthNote("root"), "REVIEW DEPTH: strict")
	require.Contains(t, (&coordinator{}).reviewDepthNote("other"), "REVIEW DEPTH: quick")
}

func TestReviewRoundLimit(t *testing.T) {
	c := &coordinator{}
	minor := "VERDICT: CHANGES_REQUIRED\n- [minor] spacing"
	critical := "VERDICT: CHANGES_REQUIRED\n- [critical] (repeat 2) build fails"


	route := c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", reviewRound: 1}, "done", minor)
	require.Equal(t, "a", route.target)
	require.True(t, route.quiet)

	route = c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", reviewRound: 2}, "done", minor)
	require.Empty(t, route.target)
	require.Contains(t, route.note, "limit (2) reached")

	mixed := "VERDICT: CHANGES_REQUIRED\n- [critical] build fails\n- [minor] spacing"
	route = c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", reviewRound: 4}, "done", mixed)
	require.Equal(t, "a", route.target)
	require.False(t, route.quiet)
	require.Contains(t, route.note, "only the critical defects")
	require.Contains(t, route.text, "build fails")
	require.NotContains(t, route.text, "spacing")
	require.Equal(t, "a", c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", reviewRound: 5}, "done", critical).target)

	require.Nil(t, c.reviewRoute(&agentJob{agent: config.AgentReview, author: "a", reviewRound: 1}, "done", "VERDICT: APPROVED\nDEFECTS: none"))
}

func TestCollectTeamNoticesIncludesApprovedReminder(t *testing.T) {
	c := &coordinator{
		agentJobs: map[string]*agentJob{
			"rev-approved": {
				id:            "rev-approved",
				name:          "ui-review",
				agent:         config.AgentReview,
				rootSessionID: "root-1",
				wake:          true,
				status:        "done",
				output:        "VERDICT: APPROVED\nSCORE: 10/10\nDEFECTS: none",
			},
			"code-only": {
				id:            "code-only",
				name:          "api",
				agent:         config.AgentCode,
				rootSessionID: "root-2",
				wake:          true,
				status:        "done",
				output:        "implemented api",
			},
			"rev-changes": {
				id:            "rev-changes",
				name:          "api-review",
				agent:         config.AgentReview,
				rootSessionID: "root-3",
				wake:          true,
				status:        "done",
				output:        "VERDICT: CHANGES_REQUIRED\nDEFECTS:\n- [major] fails",
			},
		},
	}

	approvedNotice, claims := c.collectTeamNotices("root-1")
	require.Len(t, claims, 1)
	require.Contains(t, approvedNotice, "Parts with VERDICT: APPROVED are already verified")
	require.Contains(t, approvedNotice, "do NOT re-run their tests, builds, linters, or browser/app checks")

	codeNotice, _ := c.collectTeamNotices("root-2")
	require.NotContains(t, codeNotice, "Parts with VERDICT: APPROVED are already verified")

	changesNotice, _ := c.collectTeamNotices("root-3")
	require.NotContains(t, changesNotice, "Parts with VERDICT: APPROVED are already verified")
}
