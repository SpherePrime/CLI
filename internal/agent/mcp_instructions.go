package agent

import (
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/config"
)

type mcpInstruction struct {
	Server string
	Text   string
}

func connectedMCPInstructions(states map[string]mcp.ClientInfo, agent config.Agent) []mcpInstruction {
	var instructions []mcpInstruction
	for name, server := range states {
		if !agentAllowsMCPServer(agent, name) {
			continue
		}
		if server.State != mcp.StateConnected || server.Client == nil {
			continue
		}
		result := server.Client.InitializeResult()
		if result == nil || result.Instructions == "" {
			continue
		}
		instructions = append(instructions, mcpInstruction{Server: name, Text: result.Instructions})
	}
	return instructions
}

func mcpInstructionsBlock(instructions []mcpInstruction) string {
	if len(instructions) == 0 {
		return ""
	}
	sorted := slices.Clone(instructions)
	slices.SortFunc(sorted, func(a, b mcpInstruction) int {
		return strings.Compare(a.Server, b.Server)
	})
	var builder strings.Builder
	for _, instruction := range sorted {
		builder.WriteString(instruction.Text)
		builder.WriteString("\n\n")
	}
	return "\n\n<mcp-instructions>\n" + builder.String() + "\n</mcp-instructions>"
}
