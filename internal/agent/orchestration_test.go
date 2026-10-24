package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestWorkerLimitQueueCancellationAndTeamIsolation(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.cfg.Config().Options.MaxConcurrentAgents = 1
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	other, err := env.sessions.Create(t.Context(), "other")
	require.NoError(t, err)
	var jobs []*agentJob
	for i := 0; i < 3; i++ {
		worker := newBlockingAgent("job-provider")
		job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: worker, SessionID: parent.ID, AgentMessageID: "m", ToolCallID: fmt.Sprint(i), Prompt: "work", SessionTitle: "Agent Session: code"}, "code")
		require.NoError(t, err)
		jobs = append(jobs, job)
	}
	require.Eventually(t, func() bool { status, _ := jobs[0].snapshot(); return status == "running" }, time.Second, time.Millisecond)
	status, _ := jobs[1].snapshot()
	require.Equal(t, "queued", status)
	require.NoError(t, c.StopAgentWorker(t.Context(), parent.ID, jobs[1].id))
	require.NoError(t, c.StopAgentWorker(t.Context(), parent.ID, jobs[0].id))
	require.Eventually(t, func() bool { status, _ := jobs[2].snapshot(); return status == "running" }, time.Second, time.Millisecond)
	_, err = c.SendAgentWorkerMessage(t.Context(), other.ID, jobs[2].id, "cross-team")
	require.Error(t, err)
	require.NoError(t, c.StopAgentWorker(t.Context(), parent.ID, jobs[2].id))
}

func TestWorkerDefaultLimitAndIndependentRoots(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.cfg.Config().Options.MaxConcurrentAgents = 0
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	other, err := env.sessions.Create(t.Context(), "other")
	require.NoError(t, err)
	jobs := []*agentJob{}
	for i := 0; i < 12; i++ {
		source := parent.ID
		if i == 11 {
			source = other.ID
		}
		job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: newBlockingAgent("job-provider"), SessionID: source, AgentMessageID: "default", ToolCallID: fmt.Sprint(i), Prompt: "go"}, "code")
		require.NoError(t, err)
		jobs = append(jobs, job)
	}
	for i, job := range jobs {
		status, _ := job.snapshot()
		if i == 10 {
			require.Equal(t, "queued", status)
		} else {
			require.Equal(t, "running", status)
		}
	}
	for _, job := range jobs {
		require.NoError(t, c.StopAgentWorker(t.Context(), job.parentSessionID, job.id))
		require.True(t, job.wait(t.Context(), time.Second))
	}
}

func TestWorkerFollowUpReusesSessionAndSerializesBusyDelivery(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	calls := make(chan SessionAgentCall, 4)
	release := make(chan struct{}, 4)
	var active atomic.Int32
	var maximum atomic.Int32
	a := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		n := active.Add(1)
		if n > maximum.Load() {
			maximum.Store(n)
		}
		defer active.Add(-1)
		calls <- call
		select {
		case <-release:
			return agentResultWithText("complete"), nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: a, SessionID: parent.ID, AgentMessageID: "follow", ToolCallID: "up", Prompt: "initial"}, "code")
	require.NoError(t, err)
	first := <-calls
	_, err = c.SendAgentWorkerMessage(t.Context(), parent.ID, job.id, "second assignment")
	require.NoError(t, err)
	select {
	case <-calls:
		t.Fatal("a second worker turn started while the first was busy")
	case <-time.After(30 * time.Millisecond):
	}
	release <- struct{}{}
	second := <-calls
	require.Equal(t, first.SessionID, second.SessionID)
	require.Contains(t, second.Prompt, "second assignment")
	release <- struct{}{}
	require.True(t, job.wait(t.Context(), time.Second))
	_, err = c.SendAgentWorkerMessage(t.Context(), parent.ID, job.id, "third assignment")
	require.NoError(t, err)
	third := <-calls
	require.Equal(t, first.SessionID, third.SessionID)
	release <- struct{}{}
	require.True(t, job.wait(t.Context(), time.Second))
	require.Equal(t, int32(1), maximum.Load())
	c.cfg.Config().Options.DisableMultiAgent = true
	_, err = c.SendAgentWorkerMessage(t.Context(), parent.ID, job.id, "disabled assignment")
	require.Error(t, err)
}

func TestWorkerToolsEnforceTeamScopeAndNoNestedLaunches(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	other, err := env.sessions.Create(t.Context(), "other")
	require.NoError(t, err)
	job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: newBlockingAgent("job-provider"), SessionID: parent.ID, AgentMessageID: "scope", ToolCallID: "child", Prompt: "secret assignment"}, "code")
	require.NoError(t, err)
	_, err = c.startAgentJob(t.Context(), subAgentParams{Agent: newBlockingAgent("job-provider"), SessionID: job.sessionID, AgentMessageID: "nested", ToolCallID: "child", Prompt: "nested"}, "code")
	require.ErrorContains(t, err, "nested")
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, other.ID)
	for _, op := range []string{"get", "kill"} {
		input, _ := json.Marshal(agentJobsParams{Op: op, JobID: job.id})
		result, err := c.agentJobsTool().Run(ctx, fantasy.ToolCall{Input: string(input)})
		require.NoError(t, err)
		require.True(t, result.IsError)
	}
	result, err := c.agentJobsTool().Run(ctx, fantasy.ToolCall{Input: `{"op":"list"}`})
	require.NoError(t, err)
	require.NotContains(t, result.Content, job.id)
	require.Error(t, c.StopAgentWorker(t.Context(), other.ID, job.id))
	require.NoError(t, c.StopAgentWorker(t.Context(), parent.ID, job.id))
	require.True(t, job.wait(t.Context(), time.Second))
}

func TestWorkerCompletionWaitsForParentAndSkipsConsumedResult(t *testing.T) {
	for _, consume := range []bool{false, true} {
		t.Run(fmt.Sprint(consume), func(t *testing.T) {
			env := testEnv(t)
			c := newJobCoordinator(t, env)
			parent, err := env.sessions.Create(t.Context(), "parent")
			require.NoError(t, err)
			var busy atomic.Bool
			busy.Store(true)
			c.mainAgent = &wakeMockAgent{mockSessionAgent: &mockSessionAgent{}, busy: func(string) bool { return busy.Load() }}
			c.mainAgentName = "test"
			wake := make(chan string, 2)
			c.backgroundWakeRunner = func(ctx context.Context, sessionID, prompt string) { wake <- sessionID }
			a := newBlockingAgent("job-provider")
			job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: a, SessionID: parent.ID, AgentMessageID: "wake", ToolCallID: "child", Prompt: "work"}, "code")
			require.NoError(t, err)
			close(a.blockOn)
			require.True(t, job.wait(t.Context(), time.Second))
			if consume {
				require.False(t, c.getAgentJob(agentJobsParams{JobID: job.id}).IsError)
			}
			select {
			case <-wake:
				t.Fatal("parent woke during an active turn")
			case <-time.After(20 * time.Millisecond):
			}
			busy.Store(false)
			if consume {
				select {
				case <-wake:
					t.Fatal("consumed result replayed")
				case <-time.After(backgroundWakeIdlePoll + 100*time.Millisecond):
				}
			} else {
				select {
				case sessionID := <-wake:
					require.Equal(t, parent.ID, sessionID)
				case <-time.After(time.Second):
					t.Fatal("missing completion wake")
				}
			}
		})
	}
}

func TestTeamNoticesArriveOnceAsOneBatch(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	var busy atomic.Bool
	busy.Store(true)
	c.mainAgent = &wakeMockAgent{mockSessionAgent: &mockSessionAgent{}, busy: func(string) bool { return busy.Load() }}
	c.mainAgentName = "test"
	prompts := make(chan string, 4)
	c.backgroundWakeRunner = func(ctx context.Context, sessionID, prompt string) { prompts <- prompt }
	var jobs []*agentJob
	for _, call := range []string{"first", "second"} {
		a := newBlockingAgent("job-provider")
		job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: a, SessionID: parent.ID, AgentMessageID: "batch", ToolCallID: call, Prompt: call, SessionTitle: "Agent Session: code | " + call}, "code")
		require.NoError(t, err)
		close(a.blockOn)
		require.True(t, job.wait(t.Context(), time.Second))
		jobs = append(jobs, job)
	}
	busy.Store(false)
	select {
	case prompt := <-prompts:
		for _, job := range jobs {
			require.Contains(t, prompt, job.id)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("missing team notice")
	}
	c.wakeOnAgentJob(jobs[0])
	select {
	case prompt := <-prompts:
		t.Fatalf("delivered result repeated: %s", prompt)
	case <-time.After(teamNoticeSettle + backgroundWakeIdlePoll + 100*time.Millisecond):
	}
}

func TestDisabledWorkersAndHistoricalRoster(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	c.cfg.Config().Options.DisableMultiAgent = true
	_, err = c.startAgentJob(t.Context(), subAgentParams{Agent: newBlockingAgent("job-provider"), SessionID: parent.ID, AgentMessageID: "m", ToolCallID: "c", Prompt: "work"}, "code")
	require.Error(t, err)
	child, err := env.sessions.CreateTaskSession(t.Context(), env.sessions.CreateAgentToolSessionID("old", "call"), parent.ID, "Agent Session: task")
	require.NoError(t, err)
	workers, err := c.AgentWorkers(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Len(t, workers, 1)
	require.Equal(t, child.ID, workers[0].SessionID)
	require.Equal(t, "idle", workers[0].Status)
}

func TestSynchronousWorkerUsesSharedQueueAndDefaultCode(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.cfg.Config().Options.MaxConcurrentAgents = 1
	c.cfg.Config().SetupAgents()
	a := newBlockingAgent("job-provider")
	c.subAgents = map[string]SessionAgent{config.AgentCode: a}
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	first, err := c.startAgentJob(t.Context(), subAgentParams{Agent: a, SessionID: parent.ID, AgentMessageID: "sync", ToolCallID: "first", Prompt: "first"}, "code")
	require.NoError(t, err)
	tool, err := c.agentTool(t.Context())
	require.NoError(t, err)
	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, parent.ID)
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "sync")
	result := make(chan fantasy.ToolResponse, 1)
	go func() {
		response, _ := tool.Run(ctx, fantasy.ToolCall{ID: "second", Input: `{"prompt":"second","name":"Second worker"}`})
		result <- response
	}()
	require.Eventually(t, func() bool {
		jobs := c.registrySnapshot()
		if len(jobs) != 2 {
			return false
		}
		status, _ := jobs[1].snapshot()
		return status == "queued"
	}, time.Second, time.Millisecond)
	close(a.blockOn)
	select {
	case response := <-result:
		require.False(t, response.IsError)
		require.Contains(t, response.Content, "done-after-release")
	case <-time.After(time.Second):
		t.Fatal("synchronous worker did not drain")
	}
	require.True(t, first.wait(t.Context(), time.Second))
	workers, err := c.AgentWorkers(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Len(t, workers, 2)
	require.Equal(t, "code", workers[1].Agent)
	require.Equal(t, "Second worker", workers[1].Name)
}

func TestWorkerFollowUpRefreshesParentPermissions(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.permissions = permission.NewPermissionService(t.TempDir(), true, nil)
	permissions := c.permissions.(permission.SessionSkipService)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	permissions.SetSessionSkipRequests(parent.ID, true)
	observed := make(chan bool, 2)
	a := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		observed <- permissions.SessionSkipRequests(call.SessionID)
		return agentResultWithText("done"), nil
	})
	job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: a, SessionID: parent.ID, AgentMessageID: "permissions", ToolCallID: "first", Prompt: "first"}, "code")
	require.NoError(t, err)
	require.True(t, job.wait(t.Context(), time.Second))
	require.True(t, <-observed)
	permissions.SetSessionSkipRequests(parent.ID, false)
	_, err = c.SendAgentWorkerMessage(t.Context(), parent.ID, job.id, "follow-up")
	require.NoError(t, err)
	require.True(t, job.wait(t.Context(), time.Second))
	require.False(t, <-observed)
}

func TestWorkerLifecycleCancelsQueuedRunsAndSuppressesWake(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	ctx, cancel := context.WithCancel(t.Context())
	c.workerLifecycle = ctx
	c.cfg.Config().Options.MaxConcurrentAgents = 1
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	wakes := make(chan string, 2)
	c.backgroundWakeRunner = func(ctx context.Context, sessionID, prompt string) { wakes <- sessionID }
	jobs := []*agentJob{}
	for i := 0; i < 2; i++ {
		job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: newBlockingAgent("job-provider"), SessionID: parent.ID, AgentMessageID: "lifecycle", ToolCallID: fmt.Sprint(i), Prompt: "work"}, "code")
		require.NoError(t, err)
		jobs = append(jobs, job)
	}
	cancel()
	for _, job := range jobs {
		require.True(t, job.wait(t.Context(), time.Second))
		status, _ := job.snapshot()
		require.Equal(t, "canceled", status)
	}
	select {
	case <-wakes:
		t.Fatal("worker woke a closed workspace")
	case <-time.After(20 * time.Millisecond):
	}
}
