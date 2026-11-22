package agent

import (
	"context"
	"os"
	"slices"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

func desktopHost() bool {
	return os.Getenv("PRIME_DESKTOP") == "1"
}

func reviewPolicyBlock(_ bool, _ string) string {
	return ""
}

func hasAgentTool(agentTools []fantasy.AgentTool) bool {
	return slices.ContainsFunc(agentTools, func(tool fantasy.AgentTool) bool {
		return tool.Info().Name == AgentToolName
	})
}

func (c *coordinator) activeCodeAgents(rootSessionID string) int {
	active := 0
	for _, job := range c.registrySnapshot() {
		job.mu.Lock()
		if job.rootSessionID == rootSessionID && job.agent == config.AgentCode && (job.status == "running" || job.status == "queued") {
			active++
		}
		job.mu.Unlock()
	}
	return active
}

func (c *coordinator) reviewStartsDormant(ctx context.Context, sessionID string) bool {
	root, err := c.teamRoot(ctx, sessionID)
	if err != nil {
		return false
	}
	return c.activeCodeAgents(root) > 0
}
