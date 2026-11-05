package agent

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestMCPInstructionsBlockIsOrderIndependent(t *testing.T) {
	first := mcpInstructionsBlock([]mcpInstruction{{Server: "docs", Text: "Docs."}, {Server: "desktop", Text: "Desktop."}})
	second := mcpInstructionsBlock([]mcpInstruction{{Server: "desktop", Text: "Desktop."}, {Server: "docs", Text: "Docs."}})
	require.Equal(t, first, second)
	require.Equal(t, "\n\n<mcp-instructions>\nDesktop.\n\nDocs.\n\n\n</mcp-instructions>", first)
	require.Empty(t, mcpInstructionsBlock(nil))
}
