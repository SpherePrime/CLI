package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	mcpCallToolName      = "mcp_call"
	mcpGroupMinTokens    = 1000
	mcpGroupMinTools     = 4
	mcpGroupManyTools    = 8
	mcpGroupSummaryRunes = 100
	mcpContextInline     = "inline"
	mcpContextGrouped    = "grouped"
)

type mcpServerTool interface {
	MCP() string
	MCPToolName() string
	ContextMode() string
}

type mcpPresentation struct {
	Visible  []fantasy.AgentTool
	CallTool fantasy.AgentTool
}

func mcpIdentity(tool fantasy.AgentTool) (mcpServerTool, bool) {
	if hooked, ok := tool.(*hookedTool); ok {
		tool = hooked.inner
	}
	server, ok := tool.(mcpServerTool)
	return server, ok
}

// presentMCPTools collapses large MCP servers into a single mcp_call card
// that lists their tools by name. Small servers and servers configured as
// inline keep their own cards. Smart Tools already hides MCP schemas behind
// search, so it is left unchanged.
func presentMCPTools(tools []fantasy.AgentTool) mcpPresentation {
	for _, tool := range tools {
		if isSmartSearch(tool.Info().Name) {
			return mcpPresentation{Visible: tools}
		}
	}
	servers := map[string][]fantasy.AgentTool{}
	modes := map[string]string{}
	for _, tool := range tools {
		if server, ok := mcpIdentity(tool); ok {
			servers[server.MCP()] = append(servers[server.MCP()], tool)
			modes[server.MCP()] = server.ContextMode()
		}
	}
	grouped := map[string]bool{}
	var groupedTools []fantasy.AgentTool
	for name, serverTools := range servers {
		if !shouldGroupMCPServer(modes[name], serverTools) {
			continue
		}
		for _, tool := range serverTools {
			grouped[tool.Info().Name] = true
			groupedTools = append(groupedTools, tool)
		}
	}
	if len(groupedTools) == 0 {
		return mcpPresentation{Visible: tools}
	}
	visible := make([]fantasy.AgentTool, 0, len(tools)-len(groupedTools)+1)
	for _, tool := range tools {
		if !grouped[tool.Info().Name] {
			visible = append(visible, tool)
		}
	}
	call := newMCPCallTool(groupedTools)
	return mcpPresentation{Visible: append(visible, call), CallTool: call}
}

func shouldGroupMCPServer(mode string, tools []fantasy.AgentTool) bool {
	switch mode {
	case mcpContextInline:
		return false
	case mcpContextGrouped:
		return true
	}
	if len(tools) < mcpGroupMinTools {
		return false
	}
	if len(tools) >= mcpGroupManyTools {
		return true
	}
	var tokens int64
	for _, tool := range tools {
		definition, _, err := compactToolDefinition(tool)
		if err != nil {
			return false
		}
		tokens += estimateToolDefinitionsTokens([]fantasy.Tool{definition})
	}
	return tokens >= mcpGroupMinTokens
}

type mcpCallTool struct {
	tools           map[string]fantasy.AgentTool
	names           []string
	description     string
	providerOptions fantasy.ProviderOptions
}

type mcpCallParams struct {
	Tool      string          `json:"tool"`
	Arguments json.RawMessage `json:"arguments"`
}

func newMCPCallTool(grouped []fantasy.AgentTool) *mcpCallTool {
	call := &mcpCallTool{tools: make(map[string]fantasy.AgentTool, len(grouped))}
	byServer := map[string][]string{}
	for _, tool := range grouped {
		name := tool.Info().Name
		call.tools[name] = tool
		call.names = append(call.names, name)
		if server, ok := mcpIdentity(tool); ok {
			byServer[server.MCP()] = append(byServer[server.MCP()], name)
		}
	}
	slices.Sort(call.names)
	servers := make([]string, 0, len(byServer))
	for server := range byServer {
		servers = append(servers, server)
	}
	slices.Sort(servers)
	var builder strings.Builder
	builder.WriteString("Call a tool of an MCP server that is listed here by name instead of as a separate card. Pass the full tool name and its arguments object. Read a tool's input schema with ")
	builder.WriteString(readToolDocumentationName)
	builder.WriteString(" when its arguments are not obvious.")
	for _, server := range servers {
		names := byServer[server]
		slices.Sort(names)
		fmt.Fprintf(&builder, "\n\n%s:", server)
		for _, name := range names {
			fmt.Fprintf(&builder, "\n- %s: %s", name, groupSummary(call.tools[name].Info().Description))
		}
	}
	call.description = builder.String()
	return call
}

func groupSummary(description string) string {
	summary := shortToolSummary("", description)
	if utf8.RuneCountInString(summary) > mcpGroupSummaryRunes {
		summary = string([]rune(summary)[:mcpGroupSummaryRunes]) + "…"
	}
	return summary
}

func (t *mcpCallTool) Info() fantasy.ToolInfo {
	return fantasy.ToolInfo{
		Name:        mcpCallToolName,
		Description: t.description,
		Parameters: map[string]any{
			"tool": map[string]any{
				"type":        "string",
				"enum":        t.names,
				"description": "Full name of the MCP tool to call",
			},
			"arguments": map[string]any{
				"type":        "object",
				"description": "Arguments for the tool, matching its input schema",
			},
		},
		Required: []string{"tool"},
	}
}

func (t *mcpCallTool) ProviderOptions() fantasy.ProviderOptions { return t.providerOptions }

func (t *mcpCallTool) SetProviderOptions(options fantasy.ProviderOptions) {
	t.providerOptions = options
}

func (t *mcpCallTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	var params mcpCallParams
	if err := json.Unmarshal([]byte(call.Input), &params); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("Invalid %s input: %v", mcpCallToolName, err)), nil
	}
	target, ok := t.tools[params.Tool]
	if !ok {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("No grouped MCP tool named %q. Available: %s.", params.Tool, strings.Join(t.names, ", "))), nil
	}
	arguments := strings.TrimSpace(string(params.Arguments))
	if arguments == "" || arguments == "null" {
		arguments = "{}"
	}
	if err := fantasy.ValidateToolInput(target.Info(), arguments); err != nil {
		return fantasy.NewTextErrorResponse(fmt.Sprintf("%v. Read the schema with %s tool_name=%q.", err, readToolDocumentationName, params.Tool)), nil
	}
	return target.Run(ctx, fantasy.ToolCall{ID: call.ID, Name: params.Tool, Input: arguments})
}
