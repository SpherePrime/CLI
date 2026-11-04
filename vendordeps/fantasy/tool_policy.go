package fantasy

import (
	"context"
	"fmt"
	"slices"
)

type toolGateContextKey struct{}

func checkToolGate(ctx context.Context, call ToolCallContent, tools []AgentTool) error {
	gate, _ := ctx.Value(toolGateContextKey{}).(func(context.Context, ToolCall) error)
	if gate == nil {
		return nil
	}
	if !slices.ContainsFunc(tools, func(t AgentTool) bool { return t.Info().Name == call.ToolName }) {
		return fmt.Errorf("tool not active: %s", call.ToolName)
	}
	return gate(ctx, ToolCall{ID: call.ToolCallID, Name: call.ToolName, Input: call.Input})
}

func validateToolDefinitions(definitions []Tool, tools []AgentTool, active []string, disabled bool) error {
	expected := map[string]bool{}
	if !disabled {
		for _, tool := range tools {
			name := tool.Info().Name
			if len(active) == 0 || slices.Contains(active, name) {
				expected[name] = true
			}
		}
	}
	seen := map[string]bool{}
	for _, definition := range definitions {
		name := definition.GetName()
		if seen[name] || !expected[name] {
			return fmt.Errorf("invalid tool presentation name: %s", name)
		}
		seen[name] = true
	}
	if len(seen) != len(expected) {
		return fmt.Errorf("tool presentation must contain every active canonical tool")
	}
	return nil
}
