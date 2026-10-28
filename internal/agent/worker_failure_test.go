package agent

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestWorkerFailureNotifiesBusyParent(t *testing.T) {
	for _, terminal := range []string{"error", "canceled"} {
		t.Run(terminal, func(t *testing.T) {
			env := testEnv(t)
			c := newJobCoordinator(t, env)
			c.workerLifecycle = t.Context()
			parent, err := env.sessions.Create(t.Context(), "parent")
			require.NoError(t, err)
			c.mainAgent = &wakeMockAgent{mockSessionAgent: &mockSessionAgent{}, busy: func(string) bool { return true }}
			c.mainAgentName = "test"
			notices := make(chan string, 2)
			var hidden atomic.Bool
			c.backgroundWakeRunner = func(ctx context.Context, sessionID, prompt string) {
				hidden.Store(message.HiddenUserMessage(ctx))
				notices <- sessionID + "\n" + prompt
			}
			release := make(chan struct{})
			a := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
				select {
				case <-release:
					return nil, errors.New("provider disconnected")
				case <-ctx.Done():
					return nil, ctx.Err()
				}
			})
			job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: a, SessionID: parent.ID, AgentMessageID: "failure", ToolCallID: terminal, Prompt: "work", SessionTitle: "Agent Session: code | Worker"}, "code")
			require.NoError(t, err)
			if terminal == "canceled" {
				require.NoError(t, c.StopAgentWorker(t.Context(), parent.ID, job.id))
			} else {
				close(release)
			}
			require.True(t, job.wait(t.Context(), time.Second))
			select {
			case notice := <-notices:
				require.True(t, strings.HasPrefix(notice, parent.ID+"\n"))
				require.Contains(t, notice, job.id)
				require.Contains(t, notice, "status "+terminal)
				require.True(t, hidden.Load())
				if terminal == "error" {
					require.Contains(t, notice, "provider disconnected")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("worker failure was held until the parent finishes its active turn")
			}
			c.wakeOnAgentJob(job)
			select {
			case notice := <-notices:
				t.Fatalf("failure repeated: %s", notice)
			case <-time.After(teamNoticeSettle + 50*time.Millisecond):
			}
		})
	}
}

func TestWorkerPanicFinishesJobAndReleasesQueue(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.cfg.Config().Options.MaxConcurrentAgents = 1
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	child, err := env.sessions.CreateTaskSession(t.Context(), env.sessions.CreateAgentToolSessionID("panic", "worker"), parent.ID, "Agent Session: code | Worker")
	require.NoError(t, err)
	a := newMockAgent("job-provider", 1024, func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error) {
		panic("worker transport panicked")
	})
	job := &agentJob{id: child.ID, sessionID: child.ID, parentSessionID: parent.ID, rootSessionID: parent.ID, status: "running", done: make(chan struct{}), startedAt: time.Now(), params: subAgentParams{Agent: a, SessionID: parent.ID, Prompt: "work"}}
	c.agentJobs[job.id] = job
	func() {
		defer func() { _ = recover() }()
		c.executeWorker(job, t.Context())
	}()
	status, output := job.snapshot()
	require.Equal(t, "error", status)
	require.Contains(t, output, "worker transport panicked")
	require.True(t, job.wait(t.Context(), time.Second))
	next, err := c.startAgentJob(t.Context(), subAgentParams{Agent: newMockAgent("job-provider", 1024, func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error) {
		return agentResultWithText("next completed"), nil
	}), SessionID: parent.ID, AgentMessageID: "next", ToolCallID: "worker", Prompt: "next"}, "code")
	require.NoError(t, err)
	require.True(t, next.wait(t.Context(), time.Second))
	nextStatus, _ := next.snapshot()
	require.Equal(t, "done", nextStatus)
}

func TestWorkerFailureJoinsActiveParentContext(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	c.workerLifecycle = t.Context()
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	a := NewSessionAgent(SessionAgentOptions{Sessions: env.sessions, Messages: env.messages}).(*sessionAgent)
	a.activeRequests.Set(parent.ID, &activeCancel{done: make(chan struct{})})
	c.mainAgent = a
	c.mainAgentName = "test"
	delivered := make(chan error, 1)
	c.backgroundWakeRunner = func(ctx context.Context, sessionID, prompt string) {
		_, err := a.Run(ctx, SessionAgentCall{SessionID: sessionID, Prompt: prompt, HiddenUserMessage: message.HiddenUserMessage(ctx)})
		delivered <- err
	}
	worker := newMockAgent("job-provider", 1024, func(context.Context, SessionAgentCall) (*fantasy.AgentResult, error) {
		return nil, errors.New("worker cannot continue")
	})
	job, err := c.startAgentJob(t.Context(), subAgentParams{Agent: worker, SessionID: parent.ID, AgentMessageID: "context", ToolCallID: "failure", Prompt: "work"}, "code")
	require.NoError(t, err)
	require.True(t, job.wait(t.Context(), time.Second))
	select {
	case err := <-delivered:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("failure was not queued into the active parent turn")
	}
	fold, canceled := a.drainQueueForStep(parent.ID)
	require.Empty(t, canceled)
	require.Len(t, fold, 1)
	require.True(t, fold[0].HiddenUserMessage)
	require.Contains(t, fold[0].Prompt, "worker cannot continue")
	require.Contains(t, fold[0].Prompt, "status error")
	require.Empty(t, a.QueuedPromptsList(parent.ID))
}

func TestWorkerNoticeSurvivesCoordinatorDispatchFailure(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	ctx, cancel := context.WithTimeout(t.Context(), 800*time.Millisecond)
	defer cancel()
	c.workerLifecycle = ctx
	c.interactive = true
	c.mainAgent = &mockSessionAgent{}
	c.mainAgentName = "test"
	c.readyWg.Go(func() error { return errors.New("initialization unavailable") })
	job := &agentJob{id: "failed-worker", rootSessionID: "parent", status: "error", output: "provider disconnected", wake: true, startedAt: time.Now()}
	c.agentJobs[job.id] = job
	c.deliverTeamNotices("parent")
	job.mu.Lock()
	seen := job.seen
	job.mu.Unlock()
	require.False(t, seen, "a failed dispatch must leave the worker result pending")
}
