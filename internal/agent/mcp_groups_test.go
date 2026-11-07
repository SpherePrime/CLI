package agent

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type fakeMCPTool struct {
	server string
	tool   string
	mode   string
	ran    *[]string
}

func (f fakeMCPTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{
		Name:        fmt.Sprintf("mcp_%s_%s", f.server, f.tool),
		Description: "Does " + f.tool + " with a long explanation. " + strings.Repeat("Details about parameters and behaviour. ", 20),
		Parameters: map[string]any{
			"query": map[string]any{"type": "string", "description": strings.Repeat("query text ", 30)},
		},
		Required: []string{"query"},
	}
}

func (f fakeMCPTool) Run(_ context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	*f.ran = append(*f.ran, call.Name+" "+call.Input)
	return fantasy.NewTextResponse("done"), nil
}

func (f fakeMCPTool) ProviderOptions() fantasy.ProviderOptions   { return nil }
func (f fakeMCPTool) SetProviderOptions(fantasy.ProviderOptions) {}
func (f fakeMCPTool) MCP() string                                { return f.server }
func (f fakeMCPTool) MCPToolName() string                        { return f.tool }
func (f fakeMCPTool) ContextMode() string                        { return f.mode }

func fakeServer(server, mode string, count int, ran *[]string) []fantasy.AgentTool {
	var tools []fantasy.AgentTool
	for i := range count {
		tools = append(tools, fakeMCPTool{server: server, tool: fmt.Sprintf("t%d", i), mode: mode, ran: ran})
	}
	return tools
}

func namesOf(tools []fantasy.AgentTool) []string {
	var names []string
	for _, tool := range tools {
		names = append(names, tool.Info().Name)
	}
	return names
}

func TestPresentMCPToolsGroupsLargeServers(t *testing.T) {
	var ran []string
	tools := append([]fantasy.AgentTool{newReadToolDocumentationTool()}, fakeServer("big", "", 8, &ran)...)
	tools = append(tools, fakeServer("small", "", 2, &ran)...)
	tools = append(tools, fakeServer("pinned", "inline", 6, &ran)...)
	tools = append(tools, fakeServer("forced", "grouped", 1, &ran)...)
	presentation := presentMCPTools(tools)
	require.NotNil(t, presentation.CallTool)
	visible := namesOf(presentation.Visible)
	require.Equal(t, mcpCallToolName, visible[len(visible)-1])
	require.Contains(t, visible, "mcp_small_t0")
	require.Contains(t, visible, "mcp_pinned_t5")
	require.NotContains(t, visible, "mcp_big_t0")
	require.NotContains(t, visible, "mcp_forced_t0")
	description := presentation.CallTool.Info().Description
	require.Contains(t, description, "big:\n- mcp_big_t0: Does t0 with a long explanation.")
	require.Contains(t, description, "forced:")
	require.Less(t, len(description), 2000)
}

func TestPresentMCPToolsLeavesSmartToolsAlone(t *testing.T) {
	var ran []string
	tools := append(fakeServer("big", "", 6, &ran), fantasy.NewAgentTool("search_tools", "Search.", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse(""), nil
	}))
	presentation := presentMCPTools(tools)
	require.Nil(t, presentation.CallTool)
	require.Len(t, presentation.Visible, 7)
}

func TestMCPCallValidatesAndDispatches(t *testing.T) {
	var ran []string
	call := newMCPCallTool(fakeServer("big", "", 4, &ran))
	response, err := call.Run(t.Context(), fantasy.ToolCall{ID: "1", Input: `{"tool":"mcp_big_t1","arguments":{"query":"x"}}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Equal(t, []string{`mcp_big_t1 {"query":"x"}`}, ran)

	response, err = call.Run(t.Context(), fantasy.ToolCall{ID: "2", Input: `{"tool":"mcp_big_t1","arguments":{}}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Contains(t, response.Content, readToolDocumentationName)

	response, err = call.Run(t.Context(), fantasy.ToolCall{ID: "3", Input: `{"tool":"mcp_other_x"}`})
	require.NoError(t, err)
	require.True(t, response.IsError)
	require.Len(t, ran, 1)
}

func TestMCPCallCardIsComplete(t *testing.T) {
	var ran []string
	definition, complete, err := compactToolDefinition(newMCPCallTool(fakeServer("big", "", 4, &ran)))
	require.NoError(t, err)
	require.True(t, complete)
	require.NotContains(t, definition.Description, "Full documentation is available")
}

func TestMCPCallListsGroupedServersWithBriefSummaries(t *testing.T) {
	var ran []string
	tools := append(fakeServer("apps", mcpContextGrouped, 3, &ran), fakeServer("big", "", 8, &ran)...)
	description := newMCPCallTool(tools).Info().Description
	require.Contains(t, description, "- mcp_apps_t0: Does t0 with a long explanation.")
	require.Contains(t, description, "- mcp_big_t0: Does t0")
	require.NotContains(t, description, "query text")

	presented := presentMCPTools(fakeServer("apps", mcpContextGrouped, 2, &ran))
	require.NotNil(t, presented.CallTool)
	require.Equal(t, []string{mcpCallToolName}, namesOf(presented.Visible))
}
