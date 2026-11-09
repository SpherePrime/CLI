package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func agingTurn(index int, toolName, input, output string) []fantasy.Message {
	id := "call-" + string(rune('a'+index))
	return []fantasy.Message{
		{Role: fantasy.MessageRoleUser, Content: []fantasy.MessagePart{fantasy.TextPart{Text: "turn"}}},
		{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{fantasy.ToolCallPart{ToolCallID: id, ToolName: toolName, Input: input}}},
		{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{fantasy.ToolResultPart{ToolCallID: id, Outcome: fantasy.ToolOutcomeSuccess, Output: fantasy.ToolResultOutputContentText{Text: output}}}},
	}
}

func agingStore(t *testing.T) toolArtifactStore {
	t.Helper()
	store, err := newToolArtifactStore(t.TempDir())
	require.NoError(t, err)
	return store
}

func resultText(t *testing.T, message fantasy.Message) string {
	t.Helper()
	result := message.Content[0].(fantasy.ToolResultPart)
	return result.Output.(fantasy.ToolResultOutputContentText).Text
}

func TestAgingBoundaryMovesInBatches(t *testing.T) {
	policy := defaultHistoryAgingPolicy()
	var messages []fantasy.Message
	for i := range 6 {
		messages = append(messages, agingTurn(i, "bash", `{}`, "x")...)
	}
	require.Zero(t, agingBoundary(messages, policy))
	for i := 6; i < 7; i++ {
		messages = append(messages, agingTurn(i, "bash", `{}`, "x")...)
	}
	require.Equal(t, 12, agingBoundary(messages, policy))
	for i := 7; i < 10; i++ {
		messages = append(messages, agingTurn(i, "bash", `{}`, "x")...)
	}
	require.Equal(t, 12, agingBoundary(messages, policy))
}

func TestAgingReplacesOldResultsAndKeepsRecent(t *testing.T) {
	store := agingStore(t)
	large := strings.Repeat("output line\n", 200)
	var messages []fantasy.Message
	for i := range 7 {
		messages = append(messages, agingTurn(i, "bash", `{"command":"go test"}`, large)...)
	}
	aged := ageToolHistory(context.Background(), "scope", store, messages, defaultHistoryAgingPolicy())
	old := resultText(t, aged[2])
	require.Contains(t, old, "Earlier result of bash")
	require.Contains(t, old, readToolResultName)
	require.Equal(t, large, resultText(t, aged[len(aged)-1]))
	require.Equal(t, large, resultText(t, messages[2]))

	var id string
	for _, field := range strings.Fields(old) {
		if len(field) == 49 && strings.HasSuffix(field, ".") {
			id = strings.TrimSuffix(field, ".")
		}
	}
	require.NotEmpty(t, id)
	data, _, total, err := store.Read(context.Background(), "scope", id, 0, len(large))
	require.NoError(t, err)
	require.Equal(t, len(large), total)
	require.Equal(t, large, string(data))
}

func TestAgingMarksViewsOfModifiedFilesOutdated(t *testing.T) {
	store := agingStore(t)
	large := strings.Repeat("content\n", 300)
	messages := agingTurn(0, "view", `{"file_path":"main.go"}`, large)
	messages = append(messages, agingTurn(1, "edit", `{"file_path":"main.go","old_string":"a","new_string":"b"}`, "ok")...)
	for i := 2; i < 7; i++ {
		messages = append(messages, agingTurn(i, "bash", `{}`, "x")...)
	}
	aged := ageToolHistory(context.Background(), "scope", store, messages, defaultHistoryAgingPolicy())
	require.Contains(t, resultText(t, aged[2]), "Outdated result: the file was modified later")
	require.Contains(t, resultText(t, aged[2]), "main.go")
}

func TestAgingShrinksLongCallArguments(t *testing.T) {
	content := strings.Repeat("x", 5000)
	input, err := json.Marshal(map[string]string{"file_path": "a.go", "content": content})
	require.NoError(t, err)
	shrunk := shrinkCallInput(string(input))
	var decoded map[string]string
	require.NoError(t, json.Unmarshal([]byte(shrunk), &decoded))
	require.Equal(t, "a.go", decoded["file_path"])
	require.Equal(t, agedPlaceholder(5000), decoded["content"])
	require.True(t, agedPlaceholderPattern.MatchString(decoded["content"]))
	require.Equal(t, `{"a":"<b>"}`, shrinkCallInput(`{"a":"<b>"}`))
}

func TestAgingDropsOldMediaInBatches(t *testing.T) {
	var messages []fantasy.Message
	for i := range 7 {
		id := "shot-" + string(rune('a'+i))
		messages = append(messages,
			fantasy.Message{Role: fantasy.MessageRoleAssistant, Content: []fantasy.MessagePart{fantasy.ToolCallPart{ToolCallID: id, ToolName: "mcp_browser_screenshot", Input: `{}`}}},
			fantasy.Message{Role: fantasy.MessageRoleTool, Content: []fantasy.MessagePart{fantasy.ToolResultPart{ToolCallID: id, Output: fantasy.ToolResultOutputContentMedia{Data: "AAAA", MediaType: "image/png"}}}},
		)
	}
	aged := ageToolHistory(context.Background(), "scope", agingStore(t), messages, defaultHistoryAgingPolicy())
	for i := 1; i < 10; i += 2 {
		require.Contains(t, resultText(t, aged[i]), "removed from context")
	}
	for i := 11; i < 14; i += 2 {
		_, isMedia := aged[i].Content[0].(fantasy.ToolResultPart).Output.(fantasy.ToolResultOutputContentMedia)
		require.True(t, isMedia)
	}
}
