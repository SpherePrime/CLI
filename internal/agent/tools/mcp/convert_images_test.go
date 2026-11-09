package mcp

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	sdk "github.com/SpherePrime/CLI/vendordeps/modelcontextprotocol/go-sdk/mcp"
)

func TestConvertToolResultKeepsEveryImage(t *testing.T) {
	result := convertToolResult(&sdk.CallToolResult{Content: []sdk.Content{
		&sdk.TextContent{Text: "Scenario PASSED"},
		&sdk.ImageContent{Data: []byte{0x89, 0x50, 0x4e, 0x47, 1}, MIMEType: "image/png"},
		&sdk.TextContent{Text: "Screenshot mobile"},
		&sdk.ImageContent{Data: []byte{0x89, 0x50, 0x4e, 0x47, 2}, MIMEType: "image/png"},
	}})
	require.Equal(t, "image", result.Type)
	require.Equal(t, []byte{0x89, 0x50, 0x4e, 0x47, 1}, result.Data)
	require.Len(t, result.Images, 2)
	require.Equal(t, []byte{0x89, 0x50, 0x4e, 0x47, 2}, result.Images[1].Data)
	require.Contains(t, result.Content, "Screenshot mobile")
}
