package backend

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestProjectSessionsWithoutDatabaseIsEmpty(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	sessions, err := b.ProjectSessions(context.Background(), t.TempDir())
	require.NoError(t, err)
	require.Empty(t, sessions)
}

func TestProjectSessionsReadsClosedProjectDatabase(t *testing.T) {
	t.Parallel()
	t.Cleanup(CloseAllProjectMetaConns)

	project := t.TempDir()
	dataDir := filepath.Join(project, ".prime")
	require.NoError(t, os.MkdirAll(dataDir, 0o755))

	conn, err := db.Connect(context.Background(), dataDir)
	require.NoError(t, err)
	queries := db.New(conn)
	_, err = queries.CreateSession(context.Background(), db.CreateSessionParams{
		ID:    "test-session-id",
		Title: "Closed project chat",
	})
	require.NoError(t, err)
	require.NoError(t, db.Release(dataDir))

	b, _ := newTestBackend(t)
	sessions, err := b.ProjectSessions(context.Background(), project)
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	require.Equal(t, "Closed project chat", sessions[0].Title)
	require.NotEmpty(t, sessions[0].ID)
	CloseAllProjectMetaConns()
}

func TestProjectSessionsBatchedAgainstWorkspace(t *testing.T) {
	t.Parallel()

	b, _ := newTestBackend(t)
	sessions, err := b.ProjectSessions(context.Background(), t.TempDir())
	require.NoError(t, err)
	require.Empty(t, sessions)
}
