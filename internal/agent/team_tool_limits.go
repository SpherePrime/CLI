package agent

import (
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
)

func roleDeniesMCP(agent config.Agent, mcpName string) bool {
	if agent.ID != config.AgentCode {
		return false
	}
	return isDeniedCodeDesktopMCP(mcpName)
}

func isDeniedCodeDesktopMCP(mcpName string) bool {
	if mcpName == "prime-desktop" {
		return true
	}
	if rest, ok := strings.CutPrefix(mcpName, "prime-desktop-"); ok {
		if isDigits(rest) {
			return true
		}
		mcpName = rest
	}
	for _, area := range []string{config.DesktopBrowserMCPName, "apps", "windows"} {
		if mcpName == area {
			return true
		}
		if rest, ok := strings.CutPrefix(mcpName, area+"-"); ok && isDigits(rest) {
			return true
		}
	}
	return false
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func agentAllowsMCPServer(agent config.Agent, mcpName string) bool {
	if roleDeniesMCP(agent, mcpName) {
		return false
	}
	if agent.AllowedMCP == nil {
		return true
	}
	if len(agent.AllowedMCP) == 0 {
		return false
	}
	_, ok := agent.AllowedMCP[mcpName]
	return ok
}
