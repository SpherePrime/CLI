package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/SpherePrime/CLI/internal/instructions"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestInstructionSearchReturnsMetadataWithoutLoadingBodies(t *testing.T) {
	tool := NewSearchInstructionsTool()
	response, err := tool.Run(context.Background(), fantasy.ToolCall{Input: `{"query":"debugging","limit":2}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	var result struct {
		Matches []struct {
			ID      string `json:"id"`
			Content string `json:"content"`
			Read    string `json:"read"`
		} `json:"matches"`
	}
	require.NoError(t, json.Unmarshal([]byte(response.Content), &result))
	require.NotEmpty(t, result.Matches)
	require.LessOrEqual(t, len(result.Matches), 2)
	for _, match := range result.Matches {
		require.NotEmpty(t, match.ID)
		require.Empty(t, match.Content)
		require.Contains(t, match.Read, "read_instruction")
	}
}

func TestInstructionReadRejectsUnknownIDs(t *testing.T) {
	tool := NewReadInstructionTool()
	for _, input := range []string{`{"id":"../../README.md"}`, `{"id":"not-a-module"}`, `{"id":""}`} {
		response, err := tool.Run(context.Background(), fantasy.ToolCall{Input: input})
		require.NoError(t, err)
		require.True(t, response.IsError)
		require.Contains(t, response.Content, "search_instructions")
	}
}

func TestInstructionReadLoadsOnlyOneSelectedModule(t *testing.T) {
	tool := NewReadInstructionTool()
	expected, err := instructions.Read("debugging")
	require.NoError(t, err)
	response, err := tool.Run(context.Background(), fantasy.ToolCall{Input: `{"id":"debugging"}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Contains(t, response.Content, expected)
	for _, entry := range instructions.Catalog() {
		if entry.ID == "debugging" {
			continue
		}
		other, err := instructions.Read(entry.ID)
		require.NoError(t, err)
		require.NotContains(t, response.Content, other, entry.ID)
	}
}

func TestInstructionReadSupportsBoundedContinuation(t *testing.T) {
	body, err := instructions.Read("debugging")
	require.NoError(t, err)
	response, err := NewReadInstructionTool().Run(context.Background(), fantasy.ToolCall{Input: `{"id":"debugging","offset":5,"limit":40}`})
	require.NoError(t, err)
	require.False(t, response.IsError)
	require.Contains(t, response.Content, string([]rune(body)[5:45]))
	require.NotContains(t, response.Content, body)
	require.Contains(t, response.Content, "offset=45")
}
