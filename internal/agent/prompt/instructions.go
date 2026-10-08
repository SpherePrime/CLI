package prompt

import (
	"slices"

	"github.com/SpherePrime/CLI/internal/config"
)

func instructionToolsAvailable(cfg config.Config, promptName string) bool {
	agentName := map[string]string{
		"coder": config.AgentGeneral,
		"code":  config.AgentCode,
		"task":  config.AgentTask,
		"plan":  config.AgentPlan,
	}[promptName]
	allowed := cfg.Agents[agentName].AllowedTools
	return slices.Contains(allowed, "search_instructions") && slices.Contains(allowed, "read_instruction")
}
