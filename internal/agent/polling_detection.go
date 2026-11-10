package agent

import (
	"encoding/json"
	"regexp"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	pollingWindowSize = 8
	pollingMaxRepeats = 3
	pollingMaxSleeps  = 1
)

var sleepCommand = regexp.MustCompile(`(?i)(\bsleep\s+-?[a-z]*\s*\d|start-sleep\b|\btimeout\s+/t\b|\bping\s+-n\s+\d+\s+127\.0\.0\.1)`)

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
	window := steps
	if len(window) > pollingWindowSize {
		window = window[len(window)-pollingWindowSize:]
	}
	counts := map[string]int{}
	sleeps := 0
	for _, step := range window {
		for _, call := range step.Content.ToolCalls() {
			if changingTools[call.ToolName] {
				return false
			}
			if !pollingTools[call.ToolName] {
				continue
			}
			if call.ToolName == "bash" && sleepCommand.MatchString(call.Input) {
				sleeps++
				if sleeps > pollingMaxSleeps {
					return true
				}
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
