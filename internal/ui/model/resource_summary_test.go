package model

import (
	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/lsp"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"testing"
)

func TestResourceSummaryIncludesWaitingAndDisabledServices(t *testing.T) {
	u, _ := newAppearanceFlowUI(t)
	u.com.Config().MCP = config.MCPs{}
	u.com.Config().MCP["ready"] = config.MCPConfig{}
	u.com.Config().MCP["disabled"] = config.MCPConfig{Disabled: true}
	u.mcpStates = map[string]mcp.ClientInfo{"ready": {Name: "ready", State: mcp.StateConnected}}
	u.lspStates = map[string]workspace.LSPClientInfo{"go": {Name: "go", State: lsp.StateUnstarted}}
	summary := ansi.Strip(u.resourceSummary())
	require.Contains(t, summary, "MCP ")
	require.Contains(t, summary, "[ON]")
	require.Contains(t, summary, "[OFF]")
	require.Contains(t, summary, "[WAIT]")
	require.Contains(t, summary, "Skills ")
}
