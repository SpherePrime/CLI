package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

type catalogFixture struct {
	info fantasy.ToolInfo
}

func (t *catalogFixture) Info() fantasy.ToolInfo { return t.info }

func (t *catalogFixture) Run(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return fantasy.NewTextResponse("done"), nil
}

func (t *catalogFixture) ProviderOptions() fantasy.ProviderOptions { return nil }

func (t *catalogFixture) SetProviderOptions(fantasy.ProviderOptions) {}

func TestCompactToolRetainsPropertyNamesAndConstraints(t *testing.T) {
	schema := map[string]any{
		"type":        "object",
		"description": "long",
		"properties": map[string]any{
			"description": map[string]any{"type": "string", "description": "long", "default": "keep"},
			"other":       map[string]any{"enum": []any{map[string]any{"description": "data"}}},
		},
		"additionalProperties": false,
		"$defs":                map[string]any{"x": map[string]any{"type": "integer", "minimum": 1}},
	}
	tool := &catalogFixture{info: fantasy.ToolInfo{
		Name:        "mcp_test_change",
		Description: strings.Repeat("Long documentation. ", 50),
		InputSchema: schema,
	}}
	before, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	card, complete, err := compactToolDefinition(tool)
	if err != nil {
		t.Fatal(err)
	}
	if complete {
		t.Fatal("a compacted card must not report complete documentation")
	}
	props, ok := card.InputSchema["properties"].(map[string]any)
	if !ok {
		t.Fatal("properties were dropped")
	}
	field, ok := props["description"].(map[string]any)
	if !ok {
		t.Fatal("property named description was dropped")
	}
	if _, ok := field["description"]; ok {
		t.Fatal("annotation retained")
	}
	if field["default"] != "keep" || card.InputSchema["additionalProperties"] != false {
		t.Fatal("constraint lost")
	}
	after, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("canonical schema mutated")
	}
	if !strings.Contains(card.Description, "read_tool_documentation") || len(card.Description) > 400 {
		t.Fatal(card.Description)
	}
}

func TestCompactToolKeepsShortDocumentationComplete(t *testing.T) {
	tool := &catalogFixture{info: fantasy.ToolInfo{
		Name:        "short_tool",
		Description: "Do one small thing.",
		InputSchema: map[string]any{"type": "object", "properties": map[string]any{}},
	}}
	card, complete, err := compactToolDefinition(tool)
	if err != nil {
		t.Fatal(err)
	}
	if !complete {
		t.Fatal("a card that hides nothing must report complete documentation")
	}
	if card.Description != "Do one small thing." {
		t.Fatalf("unexpected description %q", card.Description)
	}
}
