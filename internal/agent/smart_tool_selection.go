package agent

import (
	"encoding/json"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

// smartToolSelection keeps every tool discovered earlier in the session.
// Always-available tools come first in name order and discovered tools
// follow in discovery order, so a new discovery only appends to the list and
// the request prefix that providers cache stays intact.
func smartToolSelection(all []fantasy.AgentTool, steps []fantasy.StepResult, messages ...fantasy.Message) []fantasy.AgentTool {
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
	base := map[string]bool{
		"view": true, "search_instructions": true, "read_instruction": true,
	}
	discovered := discoveredToolNames(steps, messages)
	byName := make(map[string]fantasy.AgentTool, len(all))
	var active []fantasy.AgentTool
	for _, tool := range all {
		name := tool.Info().Name
		byName[name] = tool
		if base[name] || basicTool(name) || isSmartSearch(name) || inlineDocumentedTool(name) {
			active = append(active, tool)
		}
	}
	slices.SortFunc(active, func(a, b fantasy.AgentTool) int {
		return strings.Compare(a.Info().Name, b.Info().Name)
	})
	included := make(map[string]bool, len(active))
	for _, tool := range active {
		included[tool.Info().Name] = true
	}
	for _, name := range discovered {
		if tool, ok := byName[name]; ok && !included[name] {
			active = append(active, tool)
			included[name] = true
		}
	}
	return active
}

func discoveredToolNames(steps []fantasy.StepResult, messages []fantasy.Message) []string {
	var names []string
	seen := map[string]bool{}
	add := func(toolName, metadata string) {
		if toolName != "search_tools" && toolName != "search_mcp" {
			return
		}
		var parsed struct {
			DiscoveredTools []string `json:"discovered_tools"`
		}
		if json.Unmarshal([]byte(metadata), &parsed) != nil {
			return
		}
		for _, name := range parsed.DiscoveredTools {
			if !seen[name] {
				seen[name] = true
				names = append(names, name)
			}
		}
	}
	callNames := toolCallNames(messages)
	for _, message := range messages {
		for _, part := range message.Content {
			result, ok := part.(fantasy.ToolResultPart)
			if !ok {
				continue
			}
			if _, isText := result.Output.(fantasy.ToolResultOutputContentText); isText {
				add(callNames[result.ToolCallID], result.ClientMetadata)
			}
		}
	}
	for _, step := range steps {
		for _, result := range step.Content.ToolResults() {
			if _, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result.Result); !ok {
				continue
			}
			add(result.ToolName, result.ClientMetadata)
		}
	}
	return names
}

func isSmartSearch(name string) bool {
	return name == "search_skills" || name == "search_tools" || name == "search_mcp"
}
