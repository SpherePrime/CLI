package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/config"
	sdkmcp "github.com/SpherePrime/CLI/vendordeps/modelcontextprotocol/go-sdk/mcp"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestMCPInstructionsBlockIsOrderIndependent(t *testing.T) {
	first := mcpInstructionsBlock([]mcpInstruction{{Server: "docs", Text: "Docs."}, {Server: "desktop", Text: "Desktop."}})
	second := mcpInstructionsBlock([]mcpInstruction{{Server: "desktop", Text: "Desktop."}, {Server: "docs", Text: "Docs."}})
	require.Equal(t, first, second)
	require.Equal(t, "\n\n<mcp-instructions>\nDesktop.\n\nDocs.\n\n\n</mcp-instructions>", first)
	require.Empty(t, mcpInstructionsBlock(nil))
}

func TestConnectedMCPInstructionsFilteredByRole(t *testing.T) {
	newClientInfo := func(t *testing.T, name, instructions string) mcp.ClientInfo {
		t.Helper()
		serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
		server := sdkmcp.NewServer(&sdkmcp.Implementation{Name: name}, &sdkmcp.ServerOptions{Instructions: instructions})
		serverSession, err := server.Connect(context.Background(), serverTransport, nil)
		require.NoError(t, err)
		t.Cleanup(func() { _ = serverSession.Close() })

		ctx, cancel := context.WithCancel(context.Background())
		client := sdkmcp.NewClient(&sdkmcp.Implementation{Name: "test-client"}, nil)
		clientSession, err := client.Connect(ctx, clientTransport, nil)
		require.NoError(t, err)
		sess := &mcp.ClientSession{ClientSession: clientSession}
		t.Cleanup(func() {
			cancel()
			_ = clientSession.Close()
		})

		return mcp.ClientInfo{
			Name:   name,
			State:  mcp.StateConnected,
			Client: sess,
		}
	}

	states := map[string]mcp.ClientInfo{
		"browser":                 newClientInfo(t, "browser", "Browser instructions."),
		"apps":                    newClientInfo(t, "apps", "Apps instructions."),
		"windows":                 newClientInfo(t, "windows", "Windows instructions."),
		"prime-desktop-browser-2": newClientInfo(t, "prime-desktop-browser-2", "Browser 2 instructions."),
		"context7":                newClientInfo(t, "context7", "Context7 docs."),
	}

	codeBlock := mcpInstructionsBlock(connectedMCPInstructions(states, config.Agent{ID: config.AgentCode}))
	require.Contains(t, codeBlock, "Context7 docs.")
	require.NotContains(t, codeBlock, "Browser instructions.")
	require.NotContains(t, codeBlock, "Apps instructions.")
	require.NotContains(t, codeBlock, "Windows instructions.")
	require.NotContains(t, codeBlock, "Browser 2 instructions.")

	exploreBlock := mcpInstructionsBlock(connectedMCPInstructions(states, config.Agent{ID: config.AgentExplore, AllowedMCP: map[string][]string{}}))
	require.Empty(t, exploreBlock)

	reviewBlock := mcpInstructionsBlock(connectedMCPInstructions(states, config.Agent{ID: config.AgentReview}))
	require.Contains(t, reviewBlock, "Context7 docs.")
	require.Contains(t, reviewBlock, "Browser instructions.")
	require.Contains(t, reviewBlock, "Apps instructions.")
	require.Contains(t, reviewBlock, "Windows instructions.")
	require.Contains(t, reviewBlock, "Browser 2 instructions.")
}
