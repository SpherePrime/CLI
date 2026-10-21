package agent

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/notify"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type steeringStepModel struct {
	finishStreamModel
	calls      atomic.Int64
	lastPrompt []byte
}

func (m *steeringStepModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.lastPrompt, _ = json.Marshal(call.Prompt)
	if m.calls.Add(1) == 1 {
		return func(yield func(fantasy.StreamPart) bool) {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: "step", ToolCallName: "wait", ToolCallInput: "{}"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
		}, nil
	}
	return m.finishStreamModel.Stream(ctx, call)
}

func TestRunQueuedNowSteersAtNextStepWithoutCancelingOriginalTask(t *testing.T) {
	env := testEnv(t)
	entered := make(chan struct{})
	release := make(chan struct{})
	var toolCanceled atomic.Bool
	tool := fantasy.NewAgentTool("wait", "wait", func(ctx context.Context, _ struct{}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		close(entered)
		select {
		case <-release:
		case <-ctx.Done():
			toolCanceled.Store(true)
			return fantasy.ToolResponse{}, ctx.Err()
		}
		return fantasy.NewTextResponse("original task progress"), nil
	})
	model := &steeringStepModel{finishStreamModel: finishStreamModel{text: "both tasks completed"}}
	a := testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system", tool).(*sessionAgent)
	broker := pubsub.NewBroker[notify.RunComplete]()
	t.Cleanup(broker.Shutdown)
	a.runComplete = broker
	completionCtx, cancel := context.WithCancel(t.Context())
	defer cancel()
	completions := broker.Subscribe(completionCtx)
	session, err := env.sessions.Create(t.Context(), "queue steering")
	require.NoError(t, err)
	finished := make(chan error, 1)
	go func() {
		_, runErr := a.Run(t.Context(), SessionAgentCall{SessionID: session.ID, RunID: "original", Prompt: "finish original task"})
		finished <- runErr
	}()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("original tool did not start")
	}
	_, err = a.Run(t.Context(), SessionAgentCall{
		SessionID: session.ID, RunID: "addition", Prompt: "also include follow-up",
		Attachments: []message.Attachment{{FileName: "notes.txt", MimeType: "text/plain", Content: []byte("follow-up attachment")}},
	})
	require.NoError(t, err)
	entries := a.QueueEntries(session.ID)
	require.Len(t, entries, 1)
	done, err := a.RunQueuedNow(t.Context(), session.ID, entries[0].ID)
	require.NoError(t, err)
	close(release)
	select {
	case err := <-finished:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("original run did not complete")
	}
	<-done
	require.False(t, toolCanceled.Load())
	require.EqualValues(t, 2, model.calls.Load())
	require.Contains(t, string(model.lastPrompt), "finish original task")
	require.Contains(t, string(model.lastPrompt), "also include follow-up")
	require.Contains(t, string(model.lastPrompt), "follow-up attachment")
	require.Contains(t, string(model.lastPrompt), steeringInstruction)
	seen := map[string]notify.RunComplete{}
	for len(seen) < 2 {
		select {
		case event := <-completions:
			seen[event.Payload.RunID] = event.Payload
		case <-time.After(5 * time.Second):
			t.Fatal("missing run completion")
		}
	}
	require.False(t, seen["original"].Cancelled)
	require.False(t, seen["addition"].Cancelled)
	require.Equal(t, seen["original"].MessageID, seen["addition"].MessageID)
	messages, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	for _, stored := range messages {
		require.NotContains(t, stored.Content().String(), steeringInstruction)
	}
}

func TestSteeringContinuationEarlyCancellationCompletesAllRunIDs(t *testing.T) {
	for _, path := range []string{"cancel-on-entry", "queue-drop"} {
		t.Run(path, func(t *testing.T) {
			a, env := newStreamTestAgent(t)
			session, err := env.sessions.Create(t.Context(), "continuation")
			require.NoError(t, err)
			var completed []notify.RunComplete
			onComplete := func(event notify.RunComplete) { completed = append(completed, event) }
			call := SessionAgentCall{
				SessionID: session.ID, RunID: "original", Prompt: "continue", OnComplete: onComplete,
				steeringCalls: []SessionAgentCall{{SessionID: session.ID, RunID: "addition", steering: true, OnComplete: onComplete}},
			}
			if path == "cancel-on-entry" {
				call.Accepted = a.BeginAccepted(session.ID)
				a.Cancel(session.ID)
				_, err = a.Run(t.Context(), call)
				require.NoError(t, err)
			} else {
				a.messageQueue.Set(session.ID, []SessionAgentCall{call})
				a.ClearQueue(session.ID)
			}
			require.Len(t, completed, 2)
			seen := map[string]int{}
			for _, event := range completed {
				seen[event.RunID]++
				require.True(t, event.Cancelled)
			}
			require.Equal(t, map[string]int{"original": 1, "addition": 1}, seen)
		})
	}
}
