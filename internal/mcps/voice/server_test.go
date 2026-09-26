package voice

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/modelcontextprotocol/go-sdk/mcp"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestMCPServerListsVoiceTools(t *testing.T) {
	svc, err := newService()
	if err != nil {
		t.Skipf("prime config is not usable here: %v", err)
	}

	server := svc.mcpServer()
	client := mcp.NewClient(&mcp.Implementation{Name: "voice-test", Version: "v0"}, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcp.NewInMemoryTransports()
	serverSession, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)
	defer func() { _ = serverSession.Close() }()

	clientSession, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	defer func() { _ = clientSession.Close() }()

	listed, err := clientSession.ListTools(ctx, nil)
	require.NoError(t, err)

	var names []string
	for _, tool := range listed.Tools {
		names = append(names, tool.Name)
	}
	slices.Sort(names)
	require.Equal(t, []string{"dictate", "setup", "status", "stop_server", "warm"}, names)
}
