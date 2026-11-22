package agent

import (
	"github.com/SpherePrime/CLI/internal/config"
)

func roleDeniesMCP(_ config.Agent, _ string) bool {
	return false
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
