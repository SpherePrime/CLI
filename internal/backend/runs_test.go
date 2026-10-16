package backend

import (
	"context"
	"github.com/SpherePrime/CLI/internal/agent/notify"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"sync"
	"testing"
	"time"
)

func TestRunIDPreventsDuplicateDispatch(t *testing.T) {
	b, _ := newTestBackend(t)
	c := newBlockingCoordinator()
	ws := insertAgentWorkspace(t, b, c)
	defer close(c.release)
	msg := proto.AgentMessage{SessionID: "s", RunID: "id", Prompt: "hello"}
	var callers sync.WaitGroup
	for range 12 {
		callers.Go(func() { require.NoError(t, b.SendMessage(ws.ID, msg)) })
	}
	callers.Wait()
	<-c.entered
	require.EqualValues(t, 1, c.runCount.Load())
	run, err := b.GetRun(ws.ID, "id")
	require.NoError(t, err)
	require.Equal(t, "running", run.Status)
	msg.Prompt = "different"
	require.ErrorIs(t, b.SendMessage(ws.ID, msg), ErrRunConflict)
}

func TestRunRegistryRetainsAuthoritativeTerminalResult(t *testing.T) {
	b, _ := newTestBackend(t)
	c := newBlockingCoordinator()
	ws := insertRunCompleteWorkspace(t, b, context.Background(), c)
	defer close(c.release)
	msg := proto.AgentMessage{SessionID: "s", RunID: "terminal", Prompt: "hello"}
	require.NoError(t, b.SendMessage(ws.ID, msg))
	<-c.entered
	ws.RunCompletions().PublishMustDeliver(context.Background(), pubsub.UpdatedEvent, notify.RunComplete{SessionID: "s", RunID: "terminal", MessageID: "reply", Text: "done", Cancelled: true})
	require.Eventually(t, func() bool {
		run, err := b.GetRun(ws.ID, "terminal")
		return err == nil && run.Status == "cancelled" && run.Text == "done" && run.MessageID == "reply" && run.FinishedAt != nil
	}, time.Second, time.Millisecond)
	require.NoError(t, b.SendMessage(ws.ID, msg))
	require.EqualValues(t, 1, c.runCount.Load())
}
