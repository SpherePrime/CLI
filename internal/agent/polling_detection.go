package agent

import (
	"encoding/json"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	pollingWindowSize = 8
	pollingMaxRepeats = 3
)

var pollingTools = map[string]bool{
	"ls":            true,
	"view":          true,
	"glob":          true,
	"grep":          true,
	"bash":          true,
	"job_output":    true,
	"agent_jobs":    true,
	"agent_message": true,
}

var changingTools = map[string]bool{
	"edit":               true,
	"multiedit":          true,
	"write":              true,
	"lsp_rename":         true,
	"lsp_replace_symbol": true,
}

func hasPollingCalls(steps []fantasy.StepResult) bool {
	if len(steps) < pollingWindowSize {
		return false
	}
	counts := map[string]int{}
	for _, step := range steps[len(steps)-pollingWindowSize:] {
		for _, call := range step.Content.ToolCalls() {
			if changingTools[call.ToolName] {
				return false
			}
			if !pollingTools[call.ToolName] {
				continue
			}
			key := call.ToolName + "\x00" + normalizedToolInput(call.Input)
			counts[key]++
			if counts[key] > pollingMaxRepeats {
				return true
			}
		}
	}
	return false
}

func normalizedToolInput(input string) string {
	var arguments any
	if err := json.Unmarshal([]byte(input), &arguments); err != nil {
		return input
	}
	normalized, err := json.Marshal(arguments)
	if err != nil {
		return input
	}
	return string(normalized)
}
