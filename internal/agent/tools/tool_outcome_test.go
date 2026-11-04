package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

func TestMCPResponseOutcome(t *testing.T) {
	for _, kind := range []string{"text", "image", "media"} {
		result := mcp.ToolResult{Type: kind, Content: "diagnostic", IsError: true}
		response := mcpResponse(context.WithValue(context.Background(), SupportsImagesContextKey, true), result)
		if !response.IsError || response.Outcome != fantasy.ToolOutcomeFailure {
			t.Fatalf("%s: failure lost: %+v", kind, response)
		}
	}
	response := mcpResponse(context.Background(), mcp.ToolResult{Type: "text", Content: "a data value containing error"})
	if response.Outcome != fantasy.ToolOutcomeUnknown {
		t.Fatalf("unknown output promoted: %+v", response)
	}
}

func TestShellOutputPreservesRawDiagnostics(t *testing.T) {
	raw := strings.Repeat("diagnostic\n", 10000)
	if output := formatOutput(raw, "last stderr", nil, t.TempDir()); !strings.Contains(output, raw) {
		t.Fatal("shell output truncated before projection")
	}
}
