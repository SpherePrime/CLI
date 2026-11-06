package agent

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestUserQueueExcludesInternalCallsWithoutChangingDelivery(t *testing.T) {
	a, _ := newStreamTestAgent(t)
	a.enqueueCall(SessionAgentCall{SessionID: "session", Prompt: "[Team notice] legitimate user text", RunID: "user-first"})
	a.enqueueCall(SessionAgentCall{SessionID: "session", Prompt: "worker finished", HiddenUserMessage: true})
	a.enqueueCall(SessionAgentCall{SessionID: "session", Prompt: "follow-up", RunID: "user-last", Attachments: []message.Attachment{{}}})
	a.enqueueCall(SessionAgentCall{SessionID: "session", Prompt: "peer message", HiddenUserMessage: true})

	entries := a.QueueEntries("session")
	require.Len(t, entries, 2)
	require.Equal(t, "[Team notice] legitimate user text", entries[0].Prompt)
	require.Equal(t, "follow-up", entries[1].Prompt)
	require.Equal(t, 1, entries[1].Attachments)
	require.Equal(t, entries, a.QueueEntries("session"))
	require.Equal(t, 2, a.QueuedPrompts("session"))
	require.Equal(t, []string{"[Team notice] legitimate user text", "follow-up"}, a.QueuedPromptsList("session"))

	fold, canceled := a.drainQueueForStep("session")
	require.Empty(t, canceled)
	require.Len(t, fold, 2)
	require.Equal(t, "worker finished", fold[0].Prompt)
	require.Equal(t, "peer message", fold[1].Prompt)
	require.True(t, fold[0].HiddenUserMessage)
	require.True(t, fold[1].HiddenUserMessage)
	require.Equal(t, entries, a.QueueEntries("session"))
}

func TestClearUserQueuePreservesInternalDelivery(t *testing.T) {
	a, _ := newStreamTestAgent(t)
	a.enqueueCall(SessionAgentCall{SessionID: "session", Prompt: "first notice", HiddenUserMessage: true})
	a.enqueueCall(SessionAgentCall{SessionID: "session", Prompt: "user prompt"})
	a.enqueueCall(SessionAgentCall{SessionID: "session", Prompt: "second notice", HiddenUserMessage: true})
	a.ClearQueue("session")
	a.ClearQueue("session")
	require.Empty(t, a.QueueEntries("session"))
	require.Zero(t, a.QueuedPrompts("session"))
	require.Empty(t, a.QueuedPromptsList("session"))
	fold, _ := a.drainQueueForStep("session")
	require.Len(t, fold, 2)
	require.Equal(t, "first notice", fold[0].Prompt)
	require.Equal(t, "second notice", fold[1].Prompt)
}

func TestInternalQueueCannotBeSteeredByUserControl(t *testing.T) {
	a, _ := newStreamTestAgent(t)
	a.enqueueCall(SessionAgentCall{SessionID: "session", queueID: "internal", Prompt: "notice", HiddenUserMessage: true})
	_, err := a.RunQueuedNow(t.Context(), "session", "internal")
	require.ErrorIs(t, err, ErrQueuedPromptMissing)
	fold, _ := a.drainQueueForStep("session")
	require.Len(t, fold, 1)
	require.False(t, fold[0].steering)
}

func TestCancelStillClearsInternalOnlyQueue(t *testing.T) {
	a, _ := newStreamTestAgent(t)
	a.enqueueCall(SessionAgentCall{SessionID: "session", Prompt: "notice", HiddenUserMessage: true})
	a.Cancel("session")
	fold, _ := a.drainQueueForStep("session")
	require.Empty(t, fold)
}

func TestInternalQueuedCallPersistsHiddenWhenAutomaticallyRun(t *testing.T) {
	a, env := newStreamTestAgent(t)
	session, err := env.sessions.Create(t.Context(), "queue")
	require.NoError(t, err)
	a.activeRequests.Set(session.ID, &activeCancel{})
	_, err = a.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "internal notice", HiddenUserMessage: true})
	require.NoError(t, err)
	require.Empty(t, a.QueueEntries(session.ID))
	a.ClearQueue(session.ID)
	a.activeRequests.Del(session.ID)
	_, err = a.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "continue"})
	require.NoError(t, err)
	history, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	notices := 0
	for _, entry := range history {
		if entry.Role == message.User && entry.Content().Text == "internal notice" {
			notices++
			require.True(t, entry.Content().Hidden)
		}
	}
	require.Equal(t, 1, notices)
}
