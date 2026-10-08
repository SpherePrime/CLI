package model

import (
	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/lsp"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"strings"
	"testing"
)

func TestResourceListsHaveExplicitStates(t *testing.T) {
	for _, name := range styles.AllThemeNames() {
		theme := styles.ThemeForName(name)
		for _, row := range []struct {
			state mcp.State
			label string
		}{
			{mcp.StateConnected, "[ON]"}, {mcp.StateDisabled, "[OFF]"}, {mcp.StateStarting, "[WAIT]"}, {mcp.StateError, "[ERR]"}, {mcp.StateNeedsAuth, "[AUTH]"},
		} {
			require.Contains(t, mcpList(&theme, []mcp.ClientInfo{{Name: "service", State: row.state}}, 30, 1), row.label, name)
		}
		rendered := lspList(&theme, []LSPInfo{{LSPClientInfo: workspace.LSPClientInfo{Name: "go", State: lsp.StateReady}}, {LSPClientInfo: workspace.LSPClientInfo{Name: "lua", State: lsp.StateDisabled}}}, 30, 2)
		require.True(t, strings.Contains(rendered, "[ON]") && strings.Contains(rendered, "[OFF]"), name)
	}
}
