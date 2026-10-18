package agent

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestAddMessagePartsCategorizesHistory(t *testing.T) {
	t.Parallel()

	totals := map[string]int64{}
	add := func(key string, tokens int64) { totals[key] += tokens }

	addMessageParts(message.Message{Role: message.User, Parts: []message.ContentPart{
		message.TextContent{Text: "12345678"},
		message.BinaryContent{MIMEType: "image/png", Data: make([]byte, 40)},
	}}, add)
	addMessageParts(message.Message{Role: message.Assistant, Parts: []message.ContentPart{
		message.ReasoningContent{Thinking: "1234"},
		message.TextContent{Text: "1234"},
		message.ToolCall{Name: "view", Input: "{}"},
	}}, add)
	addMessageParts(message.Message{Role: message.Tool, Parts: []message.ContentPart{
		message.ToolResult{Content: "12345678"},
	}}, add)

	require.Equal(t, int64(2), totals[ContextUserMessages])
	require.Equal(t, int64(1), totals[ContextAssistant])
	require.Equal(t, int64(1), totals[ContextReasoning])
	require.Positive(t, totals[ContextToolCalls])
	require.Equal(t, int64(2), totals[ContextToolResults])
	require.Positive(t, totals[ContextAttachments])
}
