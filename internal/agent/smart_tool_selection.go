package agent

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

func smartToolSelection(all []fantasy.AgentTool, steps []fantasy.StepResult) []fantasy.AgentTool {
	smart := false
	for _, tool := range all {
		if isSmartSearch(tool.Info().Name) {
			smart = true
			break
		}
	}
	if !smart {
		return all
	}
	selected := map[string]bool{
		"view": true, "search_instructions": true, "read_instruction": true,
	}
	for _, step := range steps {
		for _, result := range step.Content.ToolResults() {
			if result.ToolName != "search_tools" && result.ToolName != "search_mcp" {
				continue
			}
			if _, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result.Result); !ok {
				continue
			}
			var metadata struct {
				DiscoveredTools []string `json:"discovered_tools"`
			}
			if json.Unmarshal([]byte(result.ClientMetadata), &metadata) != nil {
				continue
			}
			for _, name := range metadata.DiscoveredTools {
				selected[name] = true
			}
		}
	}
	active := make([]fantasy.AgentTool, 0, len(selected)+3)
	for _, tool := range all {
		if selected[tool.Info().Name] || basicTool(tool.Info().Name) || isSmartSearch(tool.Info().Name) || inlineDocumentedTool(tool.Info().Name) {
			active = append(active, tool)
		}
	}
	slices.SortFunc(active, func(a, b fantasy.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})
	return active
}

func isSmartSearch(name string) bool {
	return name == "search_skills" || name == "search_tools" || name == "search_mcp"
}
