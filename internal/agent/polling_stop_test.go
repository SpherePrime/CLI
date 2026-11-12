package agent

import (
	"context"
	"fmt"
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type pollingStopModel struct {
	fastModel
	calls int
}

func (m *pollingStopModel) Stream(context.Context, fantasy.Call) (fantasy.StreamResponse, error) {
	m.calls++
	id := fmt.Sprintf("poll-%d", m.calls)
	return func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: id, ToolCallName: "ls", ToolCallInput: `{}`})
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
	}, nil
}

func TestPollingStopPersistsExplicitFailure(t *testing.T) {
	env := testEnv(t)
	sess, err := env.sessions.Create(t.Context(), "Polling regression")
	require.NoError(t, err)
	model := &pollingStopModel{}
	tool := fantasy.NewAgentTool("ls", "list", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("unchanged"), nil
	})
	a := testSessionAgent(env, model, fastModel{}, "system", tool)
	_, err = a.Run(t.Context(), SessionAgentCall{SessionID: sess.ID, Prompt: "Inspect files"})
	require.ErrorContains(t, err, "repeated tool calls")
	messages, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var last message.Message
	for _, msg := range messages {
		if msg.Role == message.Assistant {
			last = msg
		}
	}
	require.Equal(t, message.FinishReasonError, last.FinishPart().Reason)
	require.False(t, a.IsSessionBusy(sess.ID))
}
