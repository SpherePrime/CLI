package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const (
	repeatedLineCollapseMin = 3
	maxBlankLineRun         = 1
)

var ansiEscapePattern = regexp.MustCompile(`\x1b(?:\[[0-?]*[ -/]*[@-~]|\][^\x07\x1b]*(?:\x07|\x1b\\)|[@-Z\\-_])`)

func noisyOutputTool(name string) bool {
	switch name {
	case "bash", "job_output", "fetch", "agentic_fetch", "diagnostics":
		return true
	default:
		return strings.HasPrefix(name, "mcp_")
	}
}

func compactToolOutput(toolName, text string) string {
	if text == "" || !noisyOutputTool(toolName) {
		return text
	}
	if strings.HasPrefix(toolName, "mcp_") {
		if compacted, ok := compactJSONText(text); ok {
			return compacted
		}
	}
	return compactTerminalText(text)
}

func compactJSONText(text string) (string, bool) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" || (trimmed[0] != '{' && trimmed[0] != '[') || !strings.Contains(trimmed, "\n") {
		return text, false
	}
	var buffer bytes.Buffer
	if err := json.Compact(&buffer, []byte(trimmed)); err != nil {
		return text, false
	}
	return buffer.String(), true
}

func compactTerminalText(text string) string {
	text = ansiEscapePattern.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if index := strings.LastIndexByte(line, '\r'); index >= 0 {
			line = line[index+1:]
		}
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(collapseRepeatedLines(lines), "\n")
}

func collapseRepeatedLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	blankRun := 0
	for i := 0; i < len(lines); {
		line := lines[i]
		if line == "" {
			if blankRun < maxBlankLineRun {
				result = append(result, "")
			}
			blankRun++
			i++
			continue
		}
		run := 1
		for i+run < len(lines) && lines[i+run] == line {
			run++
		}
		blankRun = 0
		result = append(result, line)
		if run >= repeatedLineCollapseMin {
			result = append(result, fmt.Sprintf("[previous line repeated %d more times]", run-1))
		} else {
			for range run - 1 {
				result = append(result, line)
			}
		}
		i += run
	}
	return result
}
