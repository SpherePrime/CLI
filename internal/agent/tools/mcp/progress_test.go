package mcp

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/modelcontextprotocol/go-sdk/mcp"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestProgressNotificationsAreTrackedByToken(t *testing.T) {
	recordProgress(context.Background(), &mcp.ProgressNotificationClientRequest{Params: &mcp.ProgressNotificationParams{ProgressToken: "mcp-7", Message: "loading", Progress: 2, Total: 5}})
	progress, ok := LastProgress("mcp-7")
	require.True(t, ok)
	require.Equal(t, "loading", progress.Message)
	require.Equal(t, 2.0, progress.Value)
	ForgetProgress("mcp-7")
	_, ok = LastProgress("mcp-7")
	require.False(t, ok)
	require.Error(t, Ping(context.Background(), "missing-server", 0))
	require.False(t, HasTool("missing-server", "browser_health"))
}
