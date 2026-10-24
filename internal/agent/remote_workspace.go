package agent

import (
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

func (c *coordinator) buildRemoteWorkspaceTools(agent config.Agent, isSubAgent bool) []fantasy.AgentTool {
	candidates := []fantasy.AgentTool{
		tools.NewTodosTool(c.sessions),
		tools.NewFetchTool(c.permissions, c.cfg.WorkingDir(), nil),
		tools.NewSourcegraphTool(nil),
		tools.NewMCPJobTool(c.cfg),
	}
	if !isSubAgent && c.interactive {
		candidates = append(candidates, tools.NewQuestionTool(c.questions))
	}
	var available []fantasy.AgentTool
	for _, tool := range candidates {
		if slices.Contains(agent.AllowedTools, tool.Info().Name) {
			available = append(available, tool)
		}
	}
	var mcpEntries []tools.SmartSearchEntry
	for _, tool := range tools.GetMCPTools(c.permissions, c.cfg, c.cfg.WorkingDir(), c.wakeOnMCPJob) {
		if !c.cfg.Config().IsDesktopMCP(tool.MCP(), c.cfg.WorkingDir()) || !agentAllowsMCPTool(agent, tool) {
			continue
		}
		available = append(available, tool)
		mcpEntries = append(mcpEntries, tool.SmartSearchEntry())
	}
	if c.cfg.Config().Options.SmartTools {
		toolEntries := tools.SmartSearchEntriesFromTools(available)
		if slices.Contains(agent.AllowedTools, "search_tools") {
			available = append(available, tools.NewSearchToolsTool(toolEntries))
		}
		if slices.Contains(agent.AllowedTools, "search_mcp") {
			available = append(available, tools.NewSearchMCPTool(mcpEntries))
		}
	}
	slices.SortFunc(available, func(a, b fantasy.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})
	return available
}
