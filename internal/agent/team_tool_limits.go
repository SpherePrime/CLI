package agent

import "github.com/SpherePrime/CLI/internal/config"

func roleDeniesMCP(agent config.Agent, mcpName string) bool {
	return agent.ID == config.AgentCode && mcpName == config.DesktopBrowserMCPName
}
