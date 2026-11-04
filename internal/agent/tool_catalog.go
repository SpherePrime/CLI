package agent

import (
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	readToolDocumentationName = "read_tool_documentation"
	readToolResultName        = "read_tool_result"
	toolCardSummaryRunes      = 240
)

func canonicalToolSchema(info fantasy.ToolInfo) (map[string]any, error) {
	return fantasy.CanonicalToolSchema(info)
}

func basicTool(name string) bool {
	switch name {
	case "view", "bash", "edit", "multiedit", "write", "glob", "grep", "ls", "question", "job_output", "job_kill":
		return true
	default:
		return false
	}
}

// inlineDocumentedTool reports whether a tool always ships its complete
// documentation in its own card. Runtime routing and discovery tools carry
// short, self-contained contracts, so a separate documentation read would
// only cost a round trip without adding information.
func inlineDocumentedTool(name string) bool {
	switch name {
	case readToolDocumentationName, readToolResultName,
		"search_skills", "search_tools", "search_mcp",
		"search_instructions", "read_instruction":
		return true
	default:
		return false
	}
}

// compactToolDefinition builds the provider-facing card for one tool. The
// second result reports whether the card still carries the complete
// documentation, in which case the model does not need a separate read.
func compactToolDefinition(tool fantasy.AgentTool) (fantasy.FunctionTool, bool, error) {
	info := tool.Info()
	canonical, err := canonicalToolSchema(info)
	if err != nil {
		return fantasy.FunctionTool{}, false, err
	}
	definition := fantasy.FunctionTool{
		Name:            info.Name,
		Description:     info.Description,
		InputSchema:     canonical,
		ProviderOptions: tool.ProviderOptions(),
	}
	if inlineDocumentedTool(info.Name) {
		return definition, true, nil
	}
	compacted, err := cloneSchema(canonical)
	if err != nil {
		return fantasy.FunctionTool{}, false, err
	}
	if !basicTool(info.Name) {
		stripSchemaAnnotations(compacted)
	}
	summary := shortToolSummary(info.Name, info.Description)
	complete := summary == strings.TrimSpace(info.Description) && schemasEqual(compacted, canonical)
	if !complete {
		summary += " Full documentation is available via " + readToolDocumentationName + " if needed."
	}
	definition.Description = summary
	definition.InputSchema = compacted
	return definition, complete, nil
}

func cloneSchema(schema map[string]any) (map[string]any, error) {
	encoded, err := json.Marshal(schema)
	if err != nil {
		return nil, err
	}
	var cloned map[string]any
	if err := json.Unmarshal(encoded, &cloned); err != nil {
		return nil, err
	}
	return cloned, nil
}

func schemasEqual(left, right map[string]any) bool {
	leftJSON, leftErr := json.Marshal(left)
	rightJSON, rightErr := json.Marshal(right)
	if leftErr != nil || rightErr != nil {
		return false
	}
	return string(leftJSON) == string(rightJSON)
}

func shortToolSummary(name, description string) string {
	summaries := map[string]string{
		"view":       "Read file contents.",
		"bash":       "Execute a shell command.",
		"edit":       "Replace exact file text.",
		"write":      "Write a complete file.",
		"glob":       "Find files by name.",
		"grep":       "Search file contents.",
		"ls":         "List directory contents.",
		"multiedit":  "Apply multiple exact text replacements.",
		"question":   "Ask the user questions.",
		"job_output": "Read output from a background command.",
		"job_kill":   "Stop a background command.",
	}
	if summary, ok := summaries[name]; ok {
		return summary
	}
	description = strings.TrimSpace(description)
	if end := strings.IndexAny(description, ".\n"); end >= 0 {
		description = description[:end+1]
	}
	if utf8.RuneCountInString(description) > toolCardSummaryRunes {
		description = string([]rune(description)[:toolCardSummaryRunes])
	}
	return description
}

func stripSchemaAnnotations(node map[string]any) {
	for _, key := range []string{"description", "title", "examples", "$comment"} {
		delete(node, key)
	}
	for _, key := range []string{"properties", "patternProperties", "$defs", "definitions", "dependentSchemas"} {
		children, ok := node[key].(map[string]any)
		if !ok {
			continue
		}
		for _, child := range children {
			if schema, ok := child.(map[string]any); ok {
				stripSchemaAnnotations(schema)
			}
		}
	}
	for _, key := range []string{"items", "additionalProperties", "unevaluatedProperties", "unevaluatedItems", "contains", "propertyNames", "not", "if", "then", "else", "contentSchema"} {
		if child, ok := node[key].(map[string]any); ok {
			stripSchemaAnnotations(child)
		}
	}
	for _, key := range []string{"allOf", "anyOf", "oneOf", "prefixItems"} {
		children, ok := node[key].([]any)
		if !ok {
			continue
		}
		for _, child := range children {
			if schema, ok := child.(map[string]any); ok {
				stripSchemaAnnotations(schema)
			}
		}
	}
}
