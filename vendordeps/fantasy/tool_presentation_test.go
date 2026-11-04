package fantasy

import (
	"context"
	"errors"
	"testing"
)

func TestToolPresentationGatePrecedesRepair(t *testing.T) {
	called := false
	a := &agent{}
	ctx := context.WithValue(context.Background(), toolGateContextKey{}, func(context.Context, ToolCall) error { return errors.New("tool_documentation_required") })
	tool := NewAgentTool("change", "full docs", func(context.Context, struct{}, ToolCall) (ToolResponse, error) {
		called = true
		return NewTextResponse("done"), nil
	})
	result := a.validateAndRepairToolCall(ctx, ToolCallContent{ToolName: "change", Input: `{`}, []AgentTool{tool}, nil, "", nil, func(context.Context, ToolCallRepairOptions) (*ToolCallContent, error) { called = true; return nil, nil })
	if !result.Invalid || called {
		t.Fatal("gate allowed repair or handler")
	}
}

func TestToolPresentationExplicitDefinitions(t *testing.T) {
	tool := NewAgentTool("change", "full", func(context.Context, struct{}, ToolCall) (ToolResponse, error) { return NewTextResponse(""), nil })
	definitions := []Tool{FunctionTool{Name: "change", Description: "short", InputSchema: map[string]any{"type": "object"}}}
	if err := validateToolDefinitions(definitions, []AgentTool{tool}, nil, false); err != nil {
		t.Fatal(err)
	}
	if err := validateToolDefinitions([]Tool{}, []AgentTool{tool}, nil, false); err == nil {
		t.Fatal("missing definition accepted")
	}
	_ = PrepareStepResult{ToolDefinitions: definitions, BeforeToolCall: func(context.Context, ToolCall) error { return nil }}
}
