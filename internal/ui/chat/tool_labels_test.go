package chat

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestPrettifyToolNameAndFormatToolDisplayName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		input       string
		wantPretty  string
		wantDisplay string
	}{
		{name: "custom_agent_memory", input: "custom_agent_memory", wantPretty: "Agent Memory", wantDisplay: "Agent Memory"},
		{name: "custom_agent_list", input: "custom_agent_list", wantPretty: "List Agents", wantDisplay: "List Agents"},
		{name: "custom_agent_list_models", input: "custom_agent_list_models", wantPretty: "Agent Models", wantDisplay: "Agent Models"},
		{name: "custom_agent_create", input: "custom_agent_create", wantPretty: "Create Agent", wantDisplay: "Create Agent"},
		{name: "custom_agent_update", input: "custom_agent_update", wantPretty: "Update Agent", wantDisplay: "Update Agent"},
		{name: "custom_agent_delete", input: "custom_agent_delete", wantPretty: "Delete Agent", wantDisplay: "Delete Agent"},
		{name: "custom_agent_dispatch", input: "custom_agent_dispatch", wantPretty: "Dispatch Agent", wantDisplay: "Dispatch Agent"},
		{name: "custom_agent_swarm", input: "custom_agent_swarm", wantPretty: "Agent Swarm", wantDisplay: "Agent Swarm"},
		{name: "custom_agent_workflow", input: "custom_agent_workflow", wantPretty: "Agent Workflow", wantDisplay: "Agent Workflow"},
		{name: "custom_agent_runs", input: "custom_agent_runs", wantPretty: "Agent Runs", wantDisplay: "Agent Runs"},
		{name: "custom_agent_blackboard", input: "custom_agent_blackboard", wantPretty: "Agent Blackboard", wantDisplay: "Agent Blackboard"},
		{name: "handoff_to_executor", input: "handoff_to_executor", wantPretty: "Handoff to Executor", wantDisplay: "Handoff to Executor"},
		{name: "escalate_to_planner", input: "escalate_to_planner", wantPretty: "Escalate to Planner", wantDisplay: "Escalate to Planner"},
		{name: "agent_jobs", input: "agent_jobs", wantPretty: "Agent Jobs", wantDisplay: "Agent Jobs"},
		{name: "agent_message", input: "agent_message", wantPretty: "Agent Message", wantDisplay: "Agent Message"},
		{name: "goal_complete", input: "goal_complete", wantPretty: "Goal Complete", wantDisplay: "Goal Complete"},
		{name: "prime_info", input: "prime_info", wantPretty: "Prime Info", wantDisplay: "Prime Info"},
		{name: "prime_logs", input: "prime_logs", wantPretty: "Prime Logs", wantDisplay: "Prime Logs"},
		{name: "mcp_job", input: "mcp_job", wantPretty: "MCP Job", wantDisplay: "MCP Job"},
		{name: "mcp_call", input: "mcp_call", wantPretty: "MCP Call", wantDisplay: "MCP Call"},
		{name: "generate_image", input: "generate_image", wantPretty: "Generate Image", wantDisplay: "Generate Image"},
		{name: "list_mcp_resources", input: "list_mcp_resources", wantPretty: "MCP Resources", wantDisplay: "MCP Resources"},
		{name: "read_mcp_resource", input: "read_mcp_resource", wantPretty: "Read MCP Resource", wantDisplay: "Read MCP Resource"},
		{name: "search_instructions", input: "search_instructions", wantPretty: "Search Instructions", wantDisplay: "Search Instructions"},
		{name: "read_instruction", input: "read_instruction", wantPretty: "Read Instruction", wantDisplay: "Read Instruction"},
		{name: "search_skills", input: "search_skills", wantPretty: "Search Skills", wantDisplay: "Search Skills"},
		{name: "search_mcp", input: "search_mcp", wantPretty: "Search MCP", wantDisplay: "Search MCP"},
		{name: "search_tools", input: "search_tools", wantPretty: "Search Tools", wantDisplay: "Search Tools"},
		{name: "read_tool_documentation", input: "read_tool_documentation", wantPretty: "Tool Documentation", wantDisplay: "Tool Documentation"},
		{name: "read_tool_result", input: "read_tool_result", wantPretty: "Full Tool Result", wantDisplay: "Full Tool Result"},
		{name: "sudo", input: "sudo", wantPretty: "Sudo", wantDisplay: "Sudo"},
		{name: "question", input: "question", wantPretty: "Question", wantDisplay: "Question"},
		{name: "lsp_diagnostics", input: "lsp_diagnostics", wantPretty: "Diagnostics", wantDisplay: "Diagnostics"},
		{name: "lsp_references", input: "lsp_references", wantPretty: "Find References", wantDisplay: "Find References"},
		{name: "lsp_definition", input: "lsp_definition", wantPretty: "Find Definition", wantDisplay: "Find Definition"},
		{name: "lsp_rename", input: "lsp_rename", wantPretty: "Rename Symbol", wantDisplay: "Rename Symbol"},
		{name: "lsp_replace_symbol", input: "lsp_replace_symbol", wantPretty: "Replace Symbol", wantDisplay: "Replace Symbol"},
		{name: "lsp_call_hierarchy", input: "lsp_call_hierarchy", wantPretty: "Call Hierarchy", wantDisplay: "Call Hierarchy"},
		{name: "lsp_symbols", input: "lsp_symbols", wantPretty: "List Symbols", wantDisplay: "List Symbols"},
		{name: "lsp_restart", input: "lsp_restart", wantPretty: "Restart LSP", wantDisplay: "Restart LSP"},
		{name: "mcp server tool", input: "mcp_github_create_issue", wantPretty: "Mcp Github Create Issue", wantDisplay: "Github " + styles.ArrowRightIcon + " Create Issue"},
		{name: "unknown fallback", input: "custom_unknown_tool", wantPretty: "Custom Unknown Tool", wantDisplay: "Custom Unknown Tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.wantPretty, prettifyToolName(tt.input))
			require.Equal(t, tt.wantDisplay, FormatToolDisplayName(tt.input))
		})
	}
}

func TestNewToolMessageItem_CleanLabels(t *testing.T) {
	t.Parallel()

	sty := styles.ColorTonePantera()

	tests := []struct {
		name     string
		toolName string
		input    string
		want     string
	}{
		{name: "custom_agent_memory", toolName: "custom_agent_memory", input: `{"action":"read"}`, want: "Agent Memory"},
		{name: "handoff_to_executor", toolName: "handoff_to_executor", input: `{"task":"implement"}`, want: "Handoff to Executor"},
		{name: "escalate_to_planner", toolName: "escalate_to_planner", input: `{"reason":"blocked"}`, want: "Escalate to Planner"},
		{name: "mcp_call", toolName: "mcp_call", input: `{"tool":"mcp_github_get"}`, want: "MCP Call"},
		{name: "mcp_job", toolName: "mcp_job", input: `{"action":"status"}`, want: "MCP Job"},
		{name: "unknown_fallback", toolName: "mystery_fallback_tool", input: `{"k":"v"}`, want: "Mystery Fallback Tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			tc := message.ToolCall{
				ID:       "tc-" + tt.name,
				Name:     tt.toolName,
				Input:    tt.input,
				Finished: true,
			}
			res := &message.ToolResult{
				ToolCallID: tc.ID,
				Content:    "ok",
			}
			item := NewToolMessageItem(&sty, "msg-1", tc, res, false, "")
			rendered := ansi.Strip(item.Render(120))
			require.Contains(t, rendered, tt.want)
			require.NotContains(t, rendered, "Invalid tool name")
			require.NotContains(t, rendered, tt.toolName)
		})
	}
}

func TestMCPToolRenderContext_MCPCall(t *testing.T) {
	t.Parallel()

	sty := styles.ColorTonePantera()
	renderer := &MCPToolRenderContext{}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "server and tool",
			input: `{"server":"github","tool":"create_issue"}`,
			want:  "Github " + styles.ArrowRightIcon + " Create Issue",
		},
		{
			name:  "prefixed tool only",
			input: `{"tool":"mcp_github_get_repo"}`,
			want:  "Github " + styles.ArrowRightIcon + " Get Repo",
		},
		{
			name:  "plain tool only",
			input: `{"tool":"get_repo"}`,
			want:  "MCP Call " + styles.ArrowRightIcon + " Get Repo",
		},
		{
			name:  "empty object",
			input: `{}`,
			want:  "MCP Call",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			out := ansi.Strip(renderer.RenderTool(&sty, 120, &ToolRenderOpts{
				ToolCall: message.ToolCall{
					ID:       "mcp-call-" + tt.name,
					Name:     "mcp_call",
					Input:    tt.input,
					Finished: true,
				},
				Result: &message.ToolResult{
					ToolCallID: "mcp-call-" + tt.name,
					Content:    "done",
				},
				Status: ToolStatusSuccess,
			}))
			require.Contains(t, out, tt.want)
			require.NotContains(t, out, "Invalid tool name")
		})
	}
}
