package model

import (
	"fmt"
	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/lsp"
	"strings"
)

func (m *UI) resourceSummary() string {
	groups := []string{}
	mcpCounts := map[string]int{}
	if m.com.Workspace != nil && m.com.Config() != nil {
		for _, server := range m.com.Config().MCP.Sorted() {
			state := "WAIT"
			if server.MCP.Disabled {
				state = "OFF"
			} else if info, ok := m.mcpStates[server.Name]; ok {
				switch info.State {
				case mcp.StateConnected:
					state = "ON"
				case mcp.StateDisabled:
					state = "OFF"
				case mcp.StateError:
					state = "ERR"
				case mcp.StateNeedsAuth:
					state = "AUTH"
				}
			}
			mcpCounts[state]++
		}
	}
	lspCounts := map[string]int{}
	for _, info := range m.lspStates {
		state := "WAIT"
		switch info.State {
		case lsp.StateReady:
			state = "ON"
		case lsp.StateDisabled:
			state = "OFF"
		case lsp.StateError:
			state = "ERR"
		}
		lspCounts[state]++
	}
	skillCounts := map[string]int{}
	for _, item := range m.skillStatusItems() {
		for _, state := range []string{"ON", "OFF", "ERR"} {
			if strings.Contains(item.icon, "["+state+"]") {
				skillCounts[state]++
				break
			}
		}
	}
	for index, counts := range []map[string]int{mcpCounts, lspCounts, skillCounts} {
		parts := []string{}
		for _, state := range []string{"ON", "OFF", "WAIT", "ERR", "AUTH"} {
			if counts[state] > 0 {
				parts = append(parts, resourceStateBadge(m.com.Styles, state)+fmt.Sprint(counts[state]))
			}
		}
		if len(parts) == 0 {
			parts = append(parts, resourceStateBadge(m.com.Styles, "OFF"))
		}
		groups = append(groups, []string{"MCP", "LSP", "Skills"}[index]+" "+strings.Join(parts, " "))
	}
	return m.com.Styles.Resource.RowTitleBase.Render(strings.Join(groups, " · "))
}
