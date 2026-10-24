package session

import (
	"context"
	"github.com/SpherePrime/CLI/internal/db"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"testing"
)

func TestListChildrenIncludesQueuedSessionsAndIsolatesParents(t *testing.T) {
	dataDir := t.TempDir()
	t.Cleanup(func() { require.NoError(t, db.Release(dataDir)); db.ResetPool() })
	conn, err := db.Connect(t.Context(), dataDir)
	require.NoError(t, err)
	svc := NewService(db.New(conn), conn)
	parent, err := svc.Create(t.Context(), "parent")
	require.NoError(t, err)
	other, err := svc.Create(t.Context(), "other")
	require.NoError(t, err)
	child, err := svc.CreateTaskSession(t.Context(), "queued-child", parent.ID, "queued")
	require.NoError(t, err)
	_, err = svc.CreateTaskSession(t.Context(), "other-child", other.ID, "other")
	require.NoError(t, err)
	children, err := svc.(interface {
		ListChildren(context.Context, string) ([]Session, error)
	}).ListChildren(t.Context(), parent.ID)
	require.NoError(t, err)
	require.Len(t, children, 1)
	require.Equal(t, child.ID, children[0].ID)
	require.Zero(t, children[0].MessageCount)
}
