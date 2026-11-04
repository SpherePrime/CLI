package fantasy

import "testing"

func TestCanonicalSchemaRejectsExtraArguments(t *testing.T) {
	info := ToolInfo{Name: "change", InputSchema: map[string]any{"type": "object", "properties": map[string]any{"path": map[string]any{"type": "string"}}, "required": []string{"path"}, "additionalProperties": false}}
	if err := ValidateToolInput(info, `{"path":"a","unexpected":true}`); err == nil {
		t.Fatal("extra argument was accepted")
	}
	if err := ValidateToolInput(info, `{"path":"a"}`); err != nil {
		t.Fatal(err)
	}
}

func TestValidateToolInputConstraints(t *testing.T) {
	schema := map[string]any{"type": "object", "$defs": map[string]any{"item": map[string]any{"type": "integer", "minimum": 2}}, "properties": map[string]any{"n": map[string]any{"$ref": "#/$defs/item"}, "nested": map[string]any{"type": "object", "required": []string{"x"}}, "choice": map[string]any{"enum": []any{"a", "b"}}}, "required": []string{"n"}}
	for _, input := range []string{`null`, `[]`, `{`, `{}`, `{"n":1}`, `{"n":2,"nested":{}}`, `{"n":2,"choice":"c"}`} {
		if err := ValidateToolInput(ToolInfo{Name: "test", InputSchema: schema}, input); err == nil {
			t.Errorf("accepted %s", input)
		}
	}
	if err := ValidateToolInput(ToolInfo{Name: "test", InputSchema: schema}, `{"n":3,"nested":{"x":true},"choice":"a"}`); err != nil {
		t.Fatal(err)
	}
}

func TestToolOutcomeContract(t *testing.T) {
	response := NewTextErrorResponse("failure")
	if response.Outcome != ToolOutcomeFailure {
		t.Fatal("error response must have failure outcome")
	}
	receipt := &ToolDocumentationReceipt{Version: 1, Scope: "scope"}
	result := ToolResultContent{Outcome: ToolOutcomeSuccess, Documentation: receipt}
	part := ToolResultPart{Outcome: result.Outcome, Documentation: result.Documentation}
	if part.Documentation.Scope != "scope" {
		t.Fatal("receipt lost")
	}
}
