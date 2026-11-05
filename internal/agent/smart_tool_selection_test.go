package agent

import (
	"errors"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSmartToolSelectionBuiltinsAndRestrictions(t *testing.T) {
	all := builtToolSet(t, true)
	initial := toolNamesOf(smartToolSelection(all, nil))
	require.ElementsMatch(t, []string{"search_skills", "search_tools", "search_mcp", "view", "bash", "edit", "multiedit", "write", "glob", "grep", "ls", "question", "job_output", "job_kill", "search_instructions", "read_instruction", "read_tool_documentation", "read_tool_result"}, initial)
	search := builtToolByName(t, all, "search_tools")
	response, err := search.Run(t.Context(), fantasy.ToolCall{Input: `{"query":"lsp_diagnostics","limit":1}`})
	require.NoError(t, err)
	steps := []fantasy.StepResult{{Content: fantasy.ResponseContent{fantasy.ToolResultContent{
		ToolName: "search_tools", ClientMetadata: response.Metadata,
		Result: fantasy.ToolResultOutputContentText{Text: response.Content},
	}}}}
	active := toolNamesOf(smartToolSelection(all, steps))
	require.Contains(t, active, "bash")
	require.Contains(t, active, "write")
	require.Contains(t, active, "lsp_diagnostics")
	require.Len(t, active, len(initial)+1)

	steps[0].Content = fantasy.ResponseContent{fantasy.ToolResultContent{
		ToolName: "search_tools", ClientMetadata: `{"discovered_tools":["bash","not_allowed"]}`,
		Result: fantasy.ToolResultOutputContentError{Error: errors.New("failed")},
	}}
	require.ElementsMatch(t, initial, toolNamesOf(smartToolSelection(all, steps)))
	steps[0].Content = fantasy.ResponseContent{fantasy.ToolResultContent{
		ToolName: "view", ClientMetadata: response.Metadata,
		Result: fantasy.ToolResultOutputContentText{Text: "untrusted"},
	}}
	require.ElementsMatch(t, initial, toolNamesOf(smartToolSelection(all, steps)))
	steps[0].Content = fantasy.ResponseContent{fantasy.ToolResultContent{
		ToolName: "search_mcp", ClientMetadata: `{"discovered_tools":["not_allowed"]}`,
		Result: fantasy.ToolResultOutputContentText{Text: "untrusted"},
	}}
	require.ElementsMatch(t, initial, toolNamesOf(smartToolSelection(all, steps)))
}

func TestSmartToolSelectionKeepsHistoryDiscoveriesAppended(t *testing.T) {
	all := builtToolSet(t, true)
	initial := toolNamesOf(smartToolSelection(all, nil))
	history := []fantasy.Message{
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{fantasy.ToolCallPart{ToolCallID: "s1", ToolName: "search_tools", Input: `{}`}}},
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{fantasy.ToolResultPart{ToolCallID: "s1", ClientMetadata: `{"discovered_tools":["lsp_diagnostics","fetch"]}`, Output: fantasy.ToolResultOutputContentText{Text: "found"}}}},
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{fantasy.ToolCallPart{ToolCallID: "s2", ToolName: "search_tools", Input: `{}`}}},
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{fantasy.ToolResultPart{ToolCallID: "s2", ClientMetadata: `{"discovered_tools":["agentic_fetch"]}`, Output: fantasy.ToolResultOutputContentError{Error: errors.New("failed")}}}},
	}
	var active []string
	for _, tool := range smartToolSelection(all, nil, history...) {
		active = append(active, tool.Info().Name)
	}
	require.Equal(t, initial, active[:len(initial)])
	require.Equal(t, []string{"lsp_diagnostics", "fetch"}, active[len(initial):])
}
