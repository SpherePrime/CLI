package agent

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/jsonrepair"
)

// commonToolNameAliases maps tool names models frequently hallucinate from
// other agent frameworks onto the real Prime tool names.
var commonToolNameAliases = map[string]string{
	"read_file":     "view",
	"read":          "view",
	"read_files":    "view",
	"view_file":     "view",
	"file_reader":   "view",
	"open_file":     "view",
	"cat":           "view",
	"show_file":     "view",
	"write_file":    "write",
	"create_file":   "write",
	"new_file":      "write",
	"str_replace":   "edit",
	"str_replace_editor": "edit",
	"replace_in_file":    "edit",
	"edit_file":          "edit",
	"apply_patch":        "edit",
	"apply_diff":         "edit",
	"multi_edit":         "multiedit",
	"multiple_edits":     "multiedit",
	"run_command":        "bash",
	"execute_command":    "bash",
	"shell":              "bash",
	"terminal":           "bash",
	"list_dir":           "ls",
	"list_directory":     "ls",
	"directory":          "ls",
	"search_files":       "glob",
	"find_files":         "glob",
	"glob_files":         "glob",
	"grep_search":        "grep",
	"search_text":        "grep",
	"code_search":        "grep",
	"web_fetch":          "fetch",
	"http_get":           "fetch",
	"todo":               "todos",
	"todo_list":          "todos",
	"update_todo_list":   "todos",
	"kill_terminal":      "job_kill",
	"kill_shell":         "job_kill",
	"view_terminal":      "job_output",
	"terminal_output":    "job_output",
	"get_diagnostics":    "lsp_diagnostics",
}

// repairToolCall fixes a tool call that failed validation before giving up:
// renames hallucinated or mistyped tool names to the closest available tool
// and repairs malformed JSON arguments. Returns nil when nothing was changed.
func repairToolCall(
	ctx context.Context,
	options fantasy.ToolCallRepairOptions,
) (*fantasy.ToolCallContent, error) {
	available := make([]string, 0, len(options.AvailableTools))
	for _, tool := range options.AvailableTools {
		available = append(available, tool.Info().Name)
	}

	repaired := options.OriginalToolCall
	changed := false

	if !slices.Contains(available, repaired.ToolName) {
		if name, ok := resolveToolName(repaired.ToolName, available); ok {
			repaired.ToolName = name
			changed = true
		}
	}

	var input map[string]any
	if json.Unmarshal([]byte(repaired.Input), &input) != nil {
		if fixed, err := jsonrepair.RepairJSON(repaired.Input); err == nil && fixed != repaired.Input {
			repaired.Input = fixed
			changed = true
		}
	}

	if !changed {
		return nil, nil
	}
	return &repaired, nil
}

// resolveToolName finds the best matching tool for a hallucinated or
// misspelled name: exact case-insensitive match, known alias, mcp_ prefix or
// suffix match, then the closest name within a small edit distance.
func resolveToolName(called string, available []string) (string, bool) {
	normalized := normalizeToolName(called)

	var candidates []string
	for _, name := range available {
		if normalizeToolName(name) == normalized {
			return name, true
		}
		candidates = append(candidates, name)
	}

	if alias, ok := commonToolNameAliases[normalized]; ok {
		for _, name := range candidates {
			if normalizeToolName(name) == normalizeToolName(alias) {
				return name, true
			}
		}
	}

	if !strings.HasPrefix(normalized, "mcp_") {
		var suffixMatches []string
		for _, name := range candidates {
			lower := normalizeToolName(name)
			if lower == "mcp_"+normalized || strings.HasSuffix(lower, "_"+normalized) {
				suffixMatches = append(suffixMatches, name)
			}
		}
		if len(suffixMatches) == 1 {
			return suffixMatches[0], true
		}
	}

	bestName := ""
	bestDistance := -1
	for _, name := range candidates {
		distance := editDistance(normalized, normalizeToolName(name))
		if distance <= editDistanceLimit(len(normalized)) && (bestDistance == -1 || distance < bestDistance) {
			bestDistance = distance
			bestName = name
		}
	}
	if bestName != "" {
		return bestName, true
	}

	return "", false
}

func normalizeToolName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.NewReplacer("-", "_", ".", "_", " ", "_").Replace(name)
	return name
}

func editDistanceLimit(length int) int {
	switch {
	case length <= 4:
		return 1
	case length <= 8:
		return 2
	default:
		return 3
	}
}

func editDistance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(cur[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
