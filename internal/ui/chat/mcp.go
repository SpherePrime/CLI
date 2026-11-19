package chat

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/ui/styles"
)

// MCPToolMessageItem is a message item that represents a bash tool call.
type MCPToolMessageItem struct {
	*baseToolMessageItem
}

var _ ToolMessageItem = (*MCPToolMessageItem)(nil)

// NewMCPToolMessageItem creates a new [MCPToolMessageItem].
func NewMCPToolMessageItem(
	sty *styles.Styles,
	toolCall message.ToolCall,
	result *message.ToolResult,
	canceled bool,
) ToolMessageItem {
	return newBaseToolMessageItem(sty, toolCall, result, &MCPToolRenderContext{}, canceled)
}

// MCPToolRenderContext renders bash tool messages.
type MCPToolRenderContext struct{}

func formatMCPToolTitle(sty *styles.Styles, opts *ToolRenderOpts) string {
	if opts.ToolCall.Name == "mcp_call" {
		var input struct {
			Server string `json:"server"`
			Tool   string `json:"tool"`
		}
		if json.Unmarshal([]byte(opts.ToolCall.Input), &input) == nil {
			if input.Server != "" && input.Tool != "" {
				mcpName := sty.Tool.MCPName.Render(humanizedToolName(input.Server))
				toolName := sty.Tool.MCPToolName.Render(humanizedToolName(input.Tool))
				return fmt.Sprintf("%s %s %s", mcpName, sty.Tool.MCPArrow.String(), toolName)
			}
			if input.Tool != "" {
				if parts := strings.SplitN(input.Tool, "_", 3); len(parts) == 3 && strings.HasPrefix(input.Tool, "mcp_") {
					mcpName := sty.Tool.MCPName.Render(humanizedToolName(parts[1]))
					toolName := sty.Tool.MCPToolName.Render(humanizedToolName(parts[2]))
					return fmt.Sprintf("%s %s %s", mcpName, sty.Tool.MCPArrow.String(), toolName)
				}
				return fmt.Sprintf(
					"%s %s %s",
					sty.Tool.MCPName.Render(prettifyToolName(opts.ToolCall.Name)),
					sty.Tool.MCPArrow.String(),
					sty.Tool.MCPToolName.Render(humanizedToolName(input.Tool)),
				)
			}
			if input.Server != "" {
				return fmt.Sprintf(
					"%s %s %s",
					sty.Tool.MCPName.Render(humanizedToolName(input.Server)),
					sty.Tool.MCPArrow.String(),
					sty.Tool.MCPToolName.Render(prettifyToolName(opts.ToolCall.Name)),
				)
			}
		}
		return prettifyToolName(opts.ToolCall.Name)
	}

	toolNameParts := strings.SplitN(opts.ToolCall.Name, "_", 3)
	if len(toolNameParts) != 3 {
		return prettifyToolName(opts.ToolCall.Name)
	}
	mcpName := humanizedToolName(toolNameParts[1])
	toolName := humanizedToolName(toolNameParts[2])

	mcpName = sty.Tool.MCPName.Render(mcpName)
	toolName = sty.Tool.MCPToolName.Render(toolName)

	return fmt.Sprintf("%s %s %s", mcpName, sty.Tool.MCPArrow.String(), toolName)
}

func (b *MCPToolRenderContext) RenderTool(sty *styles.Styles, width int, opts *ToolRenderOpts) string {
	cappedWidth := cappedMessageWidth(width)
	name := formatMCPToolTitle(sty, opts)

	if opts.IsPending() {
		return pendingTool(sty, name, opts.Anim, opts.Compact)
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(opts.ToolCall.Input), &params); err != nil {
		return toolErrorContent(sty, &message.ToolResult{Content: "Invalid parameters"}, cappedWidth)
	}

	var toolParams []string
	if len(params) > 0 {
		parsed, _ := json.Marshal(params)
		toolParams = append(toolParams, string(parsed))
	}

	header := toolHeader(sty, opts.Status, name, cappedWidth, opts, toolParams...)
	if opts.Compact {
		return header
	}

	if earlyState, ok := toolEarlyStateContent(sty, opts, cappedWidth); ok {
		return joinToolParts(header, earlyState)
	}

	if !opts.HasResult() || opts.Result.Content == "" {
		return header
	}

	bodyWidth := cappedWidth - toolBodyLeftPaddingTotal
	body := renderToolResultTextContent(sty, opts.Result.Content, toolResultContentWidths{Body: bodyWidth, Diff: cappedWidth}, opts.ExpandedContent)
	return joinToolParts(header, body)
}
