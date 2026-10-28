package agent

import (
	"context"
	"errors"
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestRunAcceptanceConfirmsQueuedPrompt(t *testing.T) {
	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{Sessions: env.sessions, Messages: env.messages}).(*sessionAgent)
	a.activeRequests.Set("parent", &activeCancel{})
	marker := &runAcceptance{accepted: make(chan struct{})}
	ctx := context.WithValue(t.Context(), runAcceptanceKey{}, marker)
	_, err := a.Run(ctx, SessionAgentCall{SessionID: "parent", Prompt: "worker failed", HiddenUserMessage: true})
	require.NoError(t, err)
	require.True(t, marker.confirmed())
	fold, _ := a.drainQueueForStep("parent")
	require.Len(t, fold, 1)
	require.True(t, fold[0].HiddenUserMessage)
}

func TestRunAcceptanceRejectsInvalidPrompt(t *testing.T) {
	env := testEnv(t)
	a := NewSessionAgent(SessionAgentOptions{Sessions: env.sessions, Messages: env.messages}).(*sessionAgent)
	marker := &runAcceptance{accepted: make(chan struct{})}
	ctx := context.WithValue(t.Context(), runAcceptanceKey{}, marker)
	_, err := a.Run(ctx, SessionAgentCall{SessionID: "parent"})
	require.ErrorIs(t, err, ErrEmptyPrompt)
	require.False(t, marker.confirmed())
}

func TestRunAcceptanceSurvivesProviderFailureAfterMessageCreation(t *testing.T) {
	env := testEnv(t)
	model := &fakeLanguageModel{stream: func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: errors.New("provider disconnected")})
	}}
	a := testSessionAgent(env, model, model, "system").(*sessionAgent)
	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	marker := &runAcceptance{accepted: make(chan struct{})}
	ctx := context.WithValue(t.Context(), runAcceptanceKey{}, marker)
	_, err = a.Run(ctx, SessionAgentCall{SessionID: parent.ID, Prompt: "worker failed", HiddenUserMessage: true, NonInteractive: true})
	require.ErrorContains(t, err, "provider disconnected")
	require.True(t, marker.confirmed())
	history, err := env.messages.List(t.Context(), parent.ID)
	require.NoError(t, err)
	var noticeCount int
	for _, entry := range history {
		if entry.Role == message.User && entry.Content().Text == "worker failed" {
			noticeCount++
		}
	}
	require.Equal(t, 1, noticeCount)
}

func TestDispatchTeamNoticeCanRetryCoordinatorRefusal(t *testing.T) {
	c := newGateTestCoordinator(t, true)
	parent, err := c.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)
	a := c.currentAgent().(*sessionAgent)
	a.activeRequests.Set(parent.ID, &activeCancel{})
	provider, ok := c.cfg.Config().Providers.Get("mock")
	require.True(t, ok)
	c.cfg.Config().Providers.Del("mock")
	require.False(t, c.dispatchTeamNotice(t.Context(), parent.ID, "worker failed"))
	require.Zero(t, a.QueuedPrompts(parent.ID))
	c.cfg.Config().Providers.Set("mock", provider)
	require.True(t, c.dispatchTeamNotice(t.Context(), parent.ID, "worker failed"))
	fold, _ := a.drainQueueForStep(parent.ID)
	require.Len(t, fold, 1)
	require.Equal(t, "worker failed", fold[0].Prompt)
}
