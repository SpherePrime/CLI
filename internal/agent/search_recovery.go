package agent

import (
	"encoding/json"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

type emptySearchRecovery struct {
	active bool
}

func (recovery *emptySearchRecovery) prepare(steps []fantasy.StepResult, prepared *fantasy.PrepareStepResult) {
	if !recovery.active && hasRepeatedEmptySearches(steps) {
		recovery.active = true
	}
	if !recovery.active {
		return
	}
	prepared.Messages = append(prepared.Messages, fantasy.NewUserMessage("Repeated searches returned no matches without making progress. Grep is unavailable for the rest of this turn. Check the working directory and file list with glob or a directory listing, read likely files, and use a different search strategy. Continue the task using the remaining tools. If the required source is absent, explain the concrete blocker instead of repeating the same searches."))
	availableTools := make([]fantasy.AgentTool, 0, len(prepared.Tools))
	for _, tool := range prepared.Tools {
		if tool.Info().Name != "grep" {
			availableTools = append(availableTools, tool)
		}
	}
	prepared.Tools = availableTools
}

func hasRepeatedEmptySearches(steps []fantasy.StepResult) bool {
	const windowSize = 6
	if len(steps) < windowSize {
		return false
	}
	counts := make(map[string]int)
	for _, step := range steps[len(steps)-windowSize:] {
		signature, empty := emptySearchSignature(step.Content)
		if !empty {
			return false
		}
		counts[signature]++
	}
	for _, count := range counts {
		if count >= 3 {
			return true
		}
	}
	return false
}

func emptySearchSignature(content fantasy.ResponseContent) (string, bool) {
	calls := content.ToolCalls()
	results := content.ToolResults()
	if len(calls) != 1 || len(results) != 1 {
		return "", false
	}
	call, result := calls[0], results[0]
	if call.ToolName != "grep" || result.ToolCallID != call.ToolCallID {
		return "", false
	}
	text, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](result.Result)
	if !ok || strings.TrimSpace(text.Text) != "No files found" {
		return "", false
	}
	var arguments any
	if err := json.Unmarshal([]byte(call.Input), &arguments); err != nil {
		return "", false
	}
	normalized, err := json.Marshal(arguments)
	if err != nil {
		return "", false
	}
	return string(normalized), true
}
