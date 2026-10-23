package agent

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestRunQueuedNowPreservesCallsAndRejectsStaleID(t *testing.T) {
	a, _ := newStreamTestAgent(t)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	var interrupted atomic.Bool
	a.activeRequests.Set("session", &activeCancel{cancel: func() { interrupted.Store(true) }})
	a.enqueueCall(SessionAgentCall{SessionID: "session", RunID: "first", Prompt: "same"})
	a.enqueueCall(SessionAgentCall{SessionID: "session", RunID: "second", Prompt: "same", Attachments: []message.Attachment{{}}})
	entries := a.QueueEntries("session")
	require.Len(t, entries, 2)
	require.NotEqual(t, entries[0].ID, entries[1].ID)
	done, err := a.RunQueuedNow(ctx, "session", entries[1].ID)
	require.NoError(t, err)
	require.False(t, interrupted.Load())
	calls, _ := a.messageQueue.Get("session")
	require.Equal(t, "second", calls[0].RunID)
	require.Len(t, calls[0].Attachments, 1)
	require.Equal(t, "first", calls[1].RunID)
	require.False(t, a.hasPendingCancel("session"))
	listed := a.QueueEntries("session")
	require.True(t, listed[0].Steering)
	require.False(t, listed[1].Steering)
	interrupted.Store(false)
	_, err = a.RunQueuedNow(ctx, "session", "stale")
	require.ErrorIs(t, err, ErrQueuedPromptMissing)
	require.False(t, interrupted.Load())
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("queue monitor did not stop")
	}
}

func TestRunQueuedNowStartsAfterActiveRunFinishes(t *testing.T) {
	a, env := newStreamTestAgent(t)
	session, err := env.sessions.Create(t.Context(), "queue")
	require.NoError(t, err)
	a.activeRequests.Set(session.ID, &activeCancel{cancel: func() {}})
	a.enqueueCall(SessionAgentCall{SessionID: session.ID, RunID: "selected", Prompt: "selected prompt"})
	entries := a.QueueEntries(session.ID)
	done, err := a.RunQueuedNow(t.Context(), session.ID, entries[0].ID)
	require.NoError(t, err)
	a.activeRequests.Del(session.ID)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("selected prompt never ran")
	}
	messages, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	users := 0
	for _, m := range messages {
		if m.Role == message.User {
			users++
			require.Equal(t, "selected prompt", m.Content().String())
		}
	}
	require.Equal(t, 1, users)
	require.Zero(t, a.QueuedPrompts(session.ID))
}
