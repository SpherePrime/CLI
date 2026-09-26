package model

import (
	"fmt"

	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/mcps"
	"github.com/SpherePrime/CLI/internal/ui/util"
)

// installBuiltinMCP writes the config section Prime's own server ships with.
// The backend watches config writes and reconciles MCP servers right away, so
// the new tools show up without a restart.
func (m *UI) installBuiltinMCP(name string) tea.Msg {
	server, ok := mcps.Lookup(name)
	if !ok {
		return util.ReportError(fmt.Errorf("unknown built-in mcp server %q", name))()
	}
	entry, err := server.BuildConfig()
	if err != nil {
		return util.ReportError(err)()
	}
	if err := m.com.Workspace.SetConfigField(config.ScopeGlobal, "mcp."+name, entry); err != nil {
		return util.ReportError(fmt.Errorf("cannot install %s: %w", name, err))()
	}
	return util.NewInfoMsg(m.com.LSprintf("info.mcp_installed", name))
}

// removeBuiltinMCP drops the server's config section; reconciliation stops
// the running process on its own.
func (m *UI) removeBuiltinMCP(name string) tea.Msg {
	if err := m.com.Workspace.RemoveConfigField(config.ScopeGlobal, "mcp."+name); err != nil {
		return util.ReportError(fmt.Errorf("cannot remove %s: %w", name, err))()
	}
	return util.NewInfoMsg(m.com.LSprintf("info.mcp_removed", name))
}
