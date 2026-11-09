package agent

import (
	"context"
	"fmt"
	"regexp"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

// agedPlaceholderPattern matches the text history aging puts in place of long
// arguments of old tool calls, in its current and earlier wording.
var agedPlaceholderPattern = regexp.MustCompile(`\[\d+ bytes omitted from an earlier call\]|\[history placeholder: \d+ bytes removed`)

func agedPlaceholder(size int) string {
	return fmt.Sprintf("[history placeholder: %d bytes removed from this old call, not real content]", size)
}

// placeholderGuardTool refuses calls that copy an aged placeholder into their
// arguments. Models sometimes repeat an old call from history verbatim, and
// executing it would write the placeholder into a file instead of its content.
type placeholderGuardTool struct {
	inner fantasy.AgentTool
}

func wrapToolsWithPlaceholderGuard(tools []fantasy.AgentTool) []fantasy.AgentTool {
	out := make([]fantasy.AgentTool, len(tools))
	for i, tool := range tools {
		out[i] = &placeholderGuardTool{inner: tool}
	}
	return out
}

func (g *placeholderGuardTool) Info() fantasy.ToolInfo { return g.inner.Info() }

func (g *placeholderGuardTool) ProviderOptions() fantasy.ProviderOptions {
	return g.inner.ProviderOptions()
}

func (g *placeholderGuardTool) SetProviderOptions(opts fantasy.ProviderOptions) {
	g.inner.SetProviderOptions(opts)
}

func (g *placeholderGuardTool) Run(ctx context.Context, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
	if match := agedPlaceholderPattern.FindString(call.Input); match != "" {
		return fantasy.NewTextErrorResponse(fmt.Sprintf(
			"Rejected: the arguments contain %q. That text is a placeholder for content trimmed from an old call in the conversation history, not real data, so nothing was executed. Send the complete real content instead. If you need the current content of a file, read it first.",
			match)), nil
	}
	return g.inner.Run(ctx, call)
}
