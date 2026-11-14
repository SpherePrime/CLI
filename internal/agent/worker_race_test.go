package agent

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestExecuteWorkerCancelsContextOnCompletionErrorAndPanic(t *testing.T) {
	t.Run("normal_completion", func(t *testing.T) {
		env := testEnv(t)
		c := newJobCoordinator(t, env)
		parent, err := env.sessions.Create(t.Context(), "parent")
		require.NoError(t, err)

		var mu sync.Mutex
		var runCtxs []context.Context
		a := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			mu.Lock()
			runCtxs = append(runCtxs, ctx)
			mu.Unlock()
			return agentResultWithText("completed"), nil
		})
		job, err := c.startAgentJob(t.Context(), subAgentParams{
			Agent:          a,
			SessionID:      parent.ID,
			AgentMessageID: "m-done",
			ToolCallID:     "t-done",
			Prompt:         "work",
		}, "code")
		require.NoError(t, err)
		require.True(t, job.wait(t.Context(), time.Second))
		mu.Lock()
		defer mu.Unlock()
		require.NotEmpty(t, runCtxs)
		for _, runCtx := range runCtxs {
			require.ErrorIs(t, runCtx.Err(), context.Canceled)
		}
	})

	t.Run("error_completion", func(t *testing.T) {
		env := testEnv(t)
		c := newJobCoordinator(t, env)
		parent, err := env.sessions.Create(t.Context(), "parent")
		require.NoError(t, err)

		var mu sync.Mutex
		var runCtxs []context.Context
		a := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			mu.Lock()
			runCtxs = append(runCtxs, ctx)
			mu.Unlock()
			return nil, errors.New("boom")
		})
		job, err := c.startAgentJob(t.Context(), subAgentParams{
			Agent:          a,
			SessionID:      parent.ID,
			AgentMessageID: "m-err",
			ToolCallID:     "t-err",
			Prompt:         "work",
		}, "code")
		require.NoError(t, err)
		require.True(t, job.wait(t.Context(), time.Second))
		mu.Lock()
		defer mu.Unlock()
		require.NotEmpty(t, runCtxs)
		for _, runCtx := range runCtxs {
			require.ErrorIs(t, runCtx.Err(), context.Canceled)
		}
	})

	t.Run("panic_completion", func(t *testing.T) {
		env := testEnv(t)
		c := newJobCoordinator(t, env)
		parent, err := env.sessions.Create(t.Context(), "parent")
		require.NoError(t, err)

		var mu sync.Mutex
		var runCtxs []context.Context
		a := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			mu.Lock()
			runCtxs = append(runCtxs, ctx)
			mu.Unlock()
			panic("boom")
		})
		job, err := c.startAgentJob(t.Context(), subAgentParams{
			Agent:          a,
			SessionID:      parent.ID,
			AgentMessageID: "m-panic",
			ToolCallID:     "t-panic",
			Prompt:         "work",
		}, "code")
		require.NoError(t, err)
		require.True(t, job.wait(t.Context(), time.Second))
		status, _ := job.snapshot()
		require.Equal(t, "error", status)
		mu.Lock()
		defer mu.Unlock()
		require.NotEmpty(t, runCtxs)
		for _, runCtx := range runCtxs {
			require.ErrorIs(t, runCtx.Err(), context.Canceled)
		}
	})

	t.Run("inactive_before_reserve", func(t *testing.T) {
		env := testEnv(t)
		c := newJobCoordinator(t, env)
		c.cfg.Config().Options.MaxConcurrentAgents = 1
		parent, err := env.sessions.Create(t.Context(), "parent")
		require.NoError(t, err)

		blocker := newBlockingAgent("job-provider")
		_, err = c.startAgentJob(t.Context(), subAgentParams{
			Agent:          blocker,
			SessionID:      parent.ID,
			AgentMessageID: "m-block",
			ToolCallID:     "t-block",
			Prompt:         "hold slot",
		}, "code")
		require.NoError(t, err)

		jobCtx, cancel := context.WithCancel(t.Context())
		job := &agentJob{
			id:              "inactive-job",
			sessionID:       "inactive-session",
			parentSessionID: parent.ID,
			rootSessionID:   parent.ID,
			status:          "canceled",
			cancel:          cancel,
			done:            make(chan struct{}),
			startedAt:       time.Now(),
			params: subAgentParams{
				Agent:     newBlockingAgent("job-provider"),
				SessionID: parent.ID,
				Prompt:    "work",
			},
		}
		c.executeWorker(job, jobCtx)
		require.ErrorIs(t, jobCtx.Err(), context.Canceled)
		close(blocker.blockOn)
	})
}

func TestWorkerStopRejectsConcurrentSendUntilStoppedThenRestartsCleanly(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	entered := make(chan struct{}, 2)
	releaseFirst := make(chan struct{})
	var runs atomic.Int32
	followUpSeen := make(chan string, 1)

	a := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
		n := runs.Add(1)
		if n == 1 {
			entered <- struct{}{}
			<-releaseFirst
			return nil, ctx.Err()
		}
		followUpSeen <- call.Prompt
		return agentResultWithText("restarted"), nil
	})

	job, err := c.startAgentJob(t.Context(), subAgentParams{
		Agent:          a,
		SessionID:      parent.ID,
		AgentMessageID: "stop-race",
		ToolCallID:     "1",
		Prompt:         "initial",
	}, "code")
	require.NoError(t, err)

	<-entered
	require.NoError(t, c.StopAgentWorker(t.Context(), parent.ID, job.id))

	_, err = c.SendAgentWorkerMessage(t.Context(), parent.ID, job.id, "while-stopping")
	require.Error(t, err)

	job.mu.Lock()
	require.Empty(t, job.pending)
	job.mu.Unlock()

	close(releaseFirst)
	require.True(t, job.wait(t.Context(), time.Second))
	status, _ := job.snapshot()
	require.Equal(t, "canceled", status)

	_, err = c.SendAgentWorkerMessage(t.Context(), parent.ID, job.id, "after-stopped")
	require.NoError(t, err)
	select {
	case prompt := <-followUpSeen:
		require.Contains(t, prompt, "after-stopped")
	case <-time.After(time.Second):
		t.Fatal("follow-up after stop did not execute")
	}
	require.True(t, job.wait(t.Context(), time.Second))
}

func TestWorkerStopAndSendMessageConcurrentHammerNoStranding(t *testing.T) {
	env := testEnv(t)
	c := newJobCoordinator(t, env)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	for i := range 20 {
		a := newMockAgent("job-provider", 1024, func(ctx context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(5 * time.Millisecond):
				return agentResultWithText("ok"), nil
			}
		})
		job, err := c.startAgentJob(t.Context(), subAgentParams{
			Agent:          a,
			SessionID:      parent.ID,
			AgentMessageID: "hammer",
			ToolCallID:     string(rune('a' + i)),
			Prompt:         "work",
		}, "code")
		require.NoError(t, err)

		var wg sync.WaitGroup
		wg.Go(func() {
			_ = c.StopAgentWorker(t.Context(), parent.ID, job.id)
		})
		wg.Go(func() {
			_, _ = c.SendAgentWorkerMessage(t.Context(), parent.ID, job.id, "follow-up")
		})
		wg.Wait()

		require.True(t, job.wait(t.Context(), 2*time.Second))
		job.mu.Lock()
		pendingLen := len(job.pending)
		stopping := job.stopping
		status := job.status
		job.mu.Unlock()
		require.False(t, stopping)
		require.Zero(t, pendingLen, "message stranded in pending when job reached status %s", status)
	}
}
