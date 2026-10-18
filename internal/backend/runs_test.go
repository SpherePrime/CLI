package backend

import (
	"context"
	"github.com/SpherePrime/CLI/internal/agent/notify"
	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/internal/runregistry"
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

func TestDurableRunRetryAfterRestartNeverDispatchesAgain(t *testing.T) {
	dir := t.TempDir()
	connection, err := db.Connect(context.Background(), dir)
	require.NoError(t, err)
	b, _ := newTestBackend(t)
	coordinator := newBlockingCoordinator()
	ws := insertAgentWorkspace(t, b, coordinator)
	ws.runStore, err = runregistry.New(context.Background(), connection, "project", "previous-process", ws.ID)
	require.NoError(t, err)
	msg := proto.AgentMessage{SessionID: "session", RunID: "durable", Prompt: "work"}
	require.NoError(t, b.SendMessage(ws.ID, msg))
	<-coordinator.entered
	close(coordinator.release)
	ws.runWG.Wait()
	require.NoError(t, db.Release(dir))
	connection, err = db.Connect(context.Background(), dir)
	require.NoError(t, err)
	t.Cleanup(func() { db.Release(dir) })
	restarted, _ := newTestBackend(t)
	nextCoordinator := newBlockingCoordinator()
	defer close(nextCoordinator.release)
	nextWS := insertAgentWorkspace(t, restarted, nextCoordinator)
	nextWS.runStore, err = runregistry.New(context.Background(), connection, "project", "next-process", nextWS.ID)
	require.NoError(t, err)
	require.NoError(t, restarted.SendMessage(nextWS.ID, msg))
	require.EqualValues(t, 0, nextCoordinator.runCount.Load())
	record, err := restarted.GetRun(nextWS.ID, "durable")
	require.NoError(t, err)
	require.Equal(t, "interrupted", record.Status)
}

func TestDurableAcceptanceFailureDoesNotDispatch(t *testing.T) {
	dir := t.TempDir()
	connection, err := db.Connect(context.Background(), dir)
	require.NoError(t, err)
	b, _ := newTestBackend(t)
	coordinator := newBlockingCoordinator()
	defer close(coordinator.release)
	ws := insertAgentWorkspace(t, b, coordinator)
	ws.runStore, err = runregistry.New(context.Background(), connection, "project", "process", ws.ID)
	require.NoError(t, err)
	require.NoError(t, db.Release(dir))
	require.Error(t, b.SendMessage(ws.ID, proto.AgentMessage{SessionID: "s", RunID: "not-durable", Prompt: "work"}))
	require.Zero(t, coordinator.runCount.Load())
	require.Empty(t, ws.runs)
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
