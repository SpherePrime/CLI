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
	require.ElementsMatch(t, []string{"search_skills", "search_tools", "search_mcp", "view", "search_instructions", "read_instruction"}, initial)
	search := builtToolByName(t, all, "search_tools")
	response, err := search.Run(t.Context(), fantasy.ToolCall{Input: `{"query":"bash","limit":1}`})
	require.NoError(t, err)
	steps := []fantasy.StepResult{{Content: fantasy.ResponseContent{fantasy.ToolResultContent{
		ToolName: "search_tools", ClientMetadata: response.Metadata,
		Result: fantasy.ToolResultOutputContentText{Text: response.Content},
	}}}}
	active := toolNamesOf(smartToolSelection(all, steps))
	require.Contains(t, active, "bash")
	require.NotContains(t, active, "write")
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
