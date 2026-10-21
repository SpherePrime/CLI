package permission

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSessionSkipIsolation(t *testing.T) {
	service := NewPermissionService(t.TempDir(), false, nil)
	scoped, ok := service.(interface {
		SetSessionSkipRequests(string, bool)
		SessionSkipRequests(string) bool
	})
	require.True(t, ok, "permission service must support session access modes")
	scoped.SetSessionSkipRequests("full", true)
	require.True(t, scoped.SessionSkipRequests("full"))
	require.False(t, scoped.SessionSkipRequests("ask"))
	require.False(t, service.SkipRequests())

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	granted, err := service.Request(ctx, CreatePermissionRequest{SessionID: "full", ToolName: "bash"})
	require.NoError(t, err)
	require.True(t, granted)
	granted, err = service.Request(ctx, CreatePermissionRequest{SessionID: "ask", ToolName: "bash"})
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, granted)
}

func TestSessionSkipFalseOverridesGlobalAndRevokesAutoApproval(t *testing.T) {
	service := NewPermissionService(t.TempDir(), true, nil)
	scoped, ok := service.(interface {
		SetSessionSkipRequests(string, bool)
		SessionSkipRequests(string) bool
	})
	require.True(t, ok, "permission service must support session access modes")
	service.AutoApproveSession("ask")
	scoped.SetSessionSkipRequests("ask", false)
	require.False(t, scoped.SessionSkipRequests("ask"))
	require.True(t, scoped.SessionSkipRequests("other"))
	require.True(t, service.SkipRequests())

	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	granted, err := service.Request(ctx, CreatePermissionRequest{SessionID: "ask", ToolName: "bash"})
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, granted)

	service.SetSkipRequests(false)
	service.AutoApproveSession("delegated")
	granted, err = service.Request(ctx, CreatePermissionRequest{SessionID: "delegated", ToolName: "bash"})
	require.NoError(t, err)
	require.True(t, granted)
}
