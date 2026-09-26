package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

func stubTools(names ...string) []fantasy.AgentTool {
	tools := make([]fantasy.AgentTool, 0, len(names))
	for _, name := range names {
		tools = append(tools, fantasy.NewAgentTool(
			name,
			"stub tool",
			func(ctx context.Context, input map[string]any, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
				return fantasy.NewTextResponse("ok"), nil
			},
		))
	}
	return tools
}

func repairName(t *testing.T, called string, available ...string) (string, bool) {
	t.Helper()
	result, _ := repairToolCall(context.Background(), fantasy.ToolCallRepairOptions{
		OriginalToolCall: fantasy.ToolCallContent{
			ToolCallID: "call_1",
			ToolName:   called,
			Input:      `{}`,
		},
		AvailableTools: stubTools(available...),
	})
	if result == nil {
		return "", false
	}
	return result.ToolName, true
}

func TestRepairToolCallRenamesHallucinatedNames(t *testing.T) {
	cases := []struct {
		called    string
		available []string
		want      string
	}{
		{"read_file", []string{"view", "edit", "bash"}, "view"},
		{"Read-File", []string{"view", "edit"}, "view"},
		{"str_replace", []string{"view", "edit", "multiedit"}, "edit"},
		{"apply_patch", []string{"view", "edit"}, "edit"},
		{"Run-Command", []string{"bash", "view"}, "bash"},
		{"browser_navigate", []string{"mcp_playwright_browser_navigate", "bash"}, "mcp_playwright_browser_navigate"},
		{"gloob", []string{"glob", "grep", "ls"}, "glob"},
	}
	for _, tc := range cases {
		got, ok := repairName(t, tc.called, tc.available...)
		require.True(t, ok, "call %q should be repaired", tc.called)
		require.Equal(t, tc.want, got, "call %q", tc.called)
	}
}

func TestRepairToolCallRenamesWhenTargetMissing(t *testing.T) {
	_, ok := repairName(t, "read_file", "bash", "web_fetch")
	require.False(t, ok, "alias target view is not available, must not invent a tool")
}

func TestRepairToolCallGivesUpOnUnrelatedName(t *testing.T) {
	_, ok := repairName(t, "quantum_flux_capacitor", "view", "edit", "bash")
	require.False(t, ok)
}

func TestRepairToolCallRepairsBrokenJSON(t *testing.T) {
	result, _ := repairToolCall(context.Background(), fantasy.ToolCallRepairOptions{
		OriginalToolCall: fantasy.ToolCallContent{
			ToolCallID: "call_1",
			ToolName:   "bash",
			Input:      `{"command": "ls -la",}`,
		},
		AvailableTools: stubTools("bash", "view"),
	})
	require.NotNil(t, result)
	require.Equal(t, "bash", result.ToolName)
	var input map[string]any
	require.NoError(t, json.Unmarshal([]byte(result.Input), &input))
	require.Equal(t, "ls -la", input["command"])
}

func TestRepairToolCallKeepsValidCallUnchanged(t *testing.T) {
	result, _ := repairToolCall(context.Background(), fantasy.ToolCallRepairOptions{
		OriginalToolCall: fantasy.ToolCallContent{
			ToolCallID: "call_1",
			ToolName:   "bash",
			Input:      `{"command": "ls"}`,
		},
		AvailableTools: stubTools("bash", "view"),
	})
	require.Nil(t, result, "valid call needs no repair")
}
