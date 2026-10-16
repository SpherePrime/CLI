package runregistry

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/internal/proto"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestAcceptedRunSurvivesRestartWithoutRedispatch(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	connection, err := db.Connect(ctx, dir)
	require.NoError(t, err)
	store, err := New(ctx, connection, "project", "process-one", "workspace-one")
	require.NoError(t, err)
	message := proto.AgentMessage{SessionID: "session", RunID: "run", Prompt: "edit files"}
	accepted, err := store.Accept(ctx, message)
	require.NoError(t, err)
	require.True(t, accepted)
	require.NoError(t, db.Release(dir))
	connection, err = db.Connect(ctx, dir)
	require.NoError(t, err)
	t.Cleanup(func() { db.Release(dir) })
	restarted, err := New(ctx, connection, "project", "process-two", "workspace-two")
	require.NoError(t, err)
	record, err := restarted.Get(ctx, "run")
	require.NoError(t, err)
	require.Equal(t, "interrupted", record.Status)
	require.NotNil(t, record.FinishedAt)
	accepted, err = restarted.Accept(ctx, message)
	require.NoError(t, err)
	require.False(t, accepted)
	message.Prompt = "another action"
	_, err = restarted.Accept(ctx, message)
	require.ErrorIs(t, err, ErrConflict)
}

func TestActiveProcessAndProjectIsolation(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	connection, err := db.Connect(ctx, dir)
	require.NoError(t, err)
	t.Cleanup(func() { db.Release(dir) })
	first, err := New(ctx, connection, "first-project", "same-process", "first-workspace")
	require.NoError(t, err)
	message := proto.AgentMessage{SessionID: "session", RunID: "run", Prompt: "work"}
	_, err = first.Accept(ctx, message)
	require.NoError(t, err)
	_, err = New(ctx, connection, "first-project", "same-process", "duplicate-workspace")
	require.NoError(t, err)
	record, err := first.Get(ctx, "run")
	require.NoError(t, err)
	require.Equal(t, "running", record.Status)
	second, err := New(ctx, connection, "other-project", "other-process", "second-workspace")
	require.NoError(t, err)
	_, err = second.Get(ctx, "run")
	require.ErrorIs(t, err, ErrNotFound)
	_, err = second.Accept(ctx, message)
	require.NoError(t, err)
}

func TestCompletedRunRemainsCompletedAfterRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	connection, err := db.Connect(ctx, dir)
	require.NoError(t, err)
	t.Cleanup(func() { db.Release(dir) })
	store, err := New(ctx, connection, "project", "first-process", "workspace")
	require.NoError(t, err)
	_, err = store.Accept(ctx, proto.AgentMessage{SessionID: "session", RunID: "completed", Prompt: "work"})
	require.NoError(t, err)
	require.NoError(t, store.Finish(ctx, proto.RunRecord{RunID: "completed", Status: "completed", Text: "saved", MessageID: "reply"}))
	restarted, err := New(ctx, connection, "project", "second-process", "workspace-two")
	require.NoError(t, err)
	record, err := restarted.Get(ctx, "completed")
	require.NoError(t, err)
	require.Equal(t, "completed", record.Status)
	require.Equal(t, "saved", record.Text)
	require.Equal(t, "reply", record.MessageID)
}
