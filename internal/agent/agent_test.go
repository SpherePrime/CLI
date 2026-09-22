package agent

import (
	"encoding/base64"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"

	_ "github.com/SpherePrime/CLI/vendordeps/joho/godotenv/autoload"
)

func TestMain(m *testing.M) {
	slog.SetLogLoggerLevel(slog.LevelError)
	m.Run()
}

func makeTestTodos(n int) []session.Todo {
	todos := make([]session.Todo, n)
	for i := range n {
		todos[i] = session.Todo{
			Status:  session.TodoStatusPending,
			Content: fmt.Sprintf("Task %d: Implement feature with some description that makes it realistic", i),
		}
	}
	return todos
}

func BenchmarkBuildSummaryPrompt(b *testing.B) {
	cases := []struct {
		name     string
		numTodos int
	}{
		{"0todos", 0},
		{"5todos", 5},
		{"10todos", 10},
		{"50todos", 50},
	}

	for _, tc := range cases {
		todos := makeTestTodos(tc.numTodos)

		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				_ = buildSummaryPrompt(todos)
			}
		})
	}
}

func TestPreparePrompt_FiltersImageAttachments(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	// User message with text, a text attachment, and an image attachment.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "hello world"},
			message.BinaryContent{Path: "notes.txt", MIMEType: "text/plain", Data: []byte("important notes")},
			message.BinaryContent{Path: "image.png", MIMEType: "image/png", Data: []byte("fake-image-data")},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	// New-turn image attachment (not yet stored in the DB).
	imageAtt := message.Attachment{
		FileName: "screenshot.png",
		MimeType: "image/png",
		Content:  []byte("fake-screenshot"),
	}

	// When supportsImages is false, image attachments should be stripped
	// from history AND from the files list.
	history, files := agent.preparePrompt(msgs, false, imageAtt)
	// First message is the system reminder, second is the user message.
	require.Len(t, history, 2)
	require.Len(t, history[1].Content, 1)
	text, ok := fantasy.AsMessagePart[fantasy.TextPart](history[1].Content[0])
	require.True(t, ok)
	require.Contains(t, text.Text, "hello world")
	require.Contains(t, text.Text, "important notes")
	require.Empty(t, files, "image files should be excluded when model does not support images")

	// When supportsImages is true, image attachments should remain in
	// history and be included in the files list.
	history, files = agent.preparePrompt(msgs, true, imageAtt)
	require.Len(t, history, 2)
	require.Len(t, history[1].Content, 2)
	text, ok = fantasy.AsMessagePart[fantasy.TextPart](history[1].Content[0])
	require.True(t, ok)
	require.Contains(t, text.Text, "hello world")
	file, ok := fantasy.AsMessagePart[fantasy.FilePart](history[1].Content[1])
	require.True(t, ok)
	require.Equal(t, "image.png", file.Filename)
	require.Len(t, files, 1, "new-turn image attachment should be included when model supports images")
	require.Equal(t, "screenshot.png", files[0].Filename)
}

func TestCreateUserMessage_RetainsAllAttachments(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	// Mix of text and image attachments вЂ” all should be stored.
	call := SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "look at this image",
		Attachments: []message.Attachment{
			{FileName: "notes.txt", FilePath: "notes.txt", MimeType: "text/plain", Content: []byte("notes")},
			{FileName: "photo.png", FilePath: "photo.png", MimeType: "image/png", Content: []byte("fake-png")},
		},
	}

	msg, err := agent.createUserMessage(ctx, call)
	require.NoError(t, err)

	// All attachments should be present as BinaryContent parts.
	binaryParts := msg.BinaryContent()
	require.Len(t, binaryParts, 2, "both text and image attachments should be stored in the user message")
	require.Equal(t, "notes.txt", binaryParts[0].Path)
	require.Equal(t, "text/plain", binaryParts[0].MIMEType)
	require.Equal(t, "photo.png", binaryParts[1].Path)
	require.Equal(t, "image/png", binaryParts[1].MIMEType)

	// Reload from DB to verify persistence.
	reloaded, err := env.messages.Get(ctx, msg.ID)
	require.NoError(t, err)
	binaryParts = reloaded.BinaryContent()
	require.Len(t, binaryParts, 2, "attachments should survive DB round-trip")
	require.Equal(t, "photo.png", binaryParts[1].Path)
}

func TestPreparePrompt_OrphanedToolUse(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	// Create a user message.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "hello"},
		},
	})
	require.NoError(t, err)

	// Create an assistant message with a tool call but no tool result вЂ”
	// this simulates a cancelled/interrupted agent tool call.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.TextContent{Text: "let me check"},
			message.ToolCall{
				ID:       "call_orphaned_1",
				Name:     "agent",
				Input:    `{"prompt":"do something"}`,
				Finished: true,
			},
		},
	})
	require.NoError(t, err)

	// Create the next user message (the one that interrupted the tool call).
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "Fix #2"},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	history, _ := agent.preparePrompt(msgs, true)

	// The history must contain a synthetic tool result for the orphaned call.
	found := false
	for _, msg := range history {
		if msg.Role != fantasy.MessageRoleTool {
			continue
		}
		for _, part := range msg.Content {
			if tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
				if tr.ToolCallID == "call_orphaned_1" {
					found = true
					_, isError := tr.Output.(fantasy.ToolResultOutputContentError)
					require.True(t, isError, "orphaned tool result should be an error")
				}
			}
		}
	}
	require.True(t, found, "expected synthetic tool result for orphaned tool call")
}

func TestPreparePrompt_OrphanedToolUseMixed(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "hello"},
		},
	})
	require.NoError(t, err)

	// Assistant with 2 tool calls: one has a result, one is orphaned.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "call_ok",
				Name:     "view",
				Input:    `{"path":"/foo"}`,
				Finished: true,
			},
			message.ToolCall{
				ID:       "call_orphaned",
				Name:     "agent",
				Input:    `{"prompt":"search"}`,
				Finished: true,
			},
		},
	})
	require.NoError(t, err)

	// Only one tool result вЂ” for call_ok.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "call_ok",
				Name:       "view",
				Content:    "file contents",
			},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	history, _ := agent.preparePrompt(msgs, true)

	// Should have a synthetic result only for the orphaned call.
	var syntheticCount int
	for _, msg := range history {
		if msg.Role != fantasy.MessageRoleTool {
			continue
		}
		for _, part := range msg.Content {
			if tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
				if tr.ToolCallID == "call_orphaned" {
					syntheticCount++
				}
			}
		}
	}
	require.Equal(t, 1, syntheticCount, "expected exactly one synthetic result for the orphaned call")
}

// requireToolCallAdjacency asserts that every assistant message in history
// with tool calls is immediately followed by a tool message that responds to
// each of those calls, with no other message in between. This is what
// strict-adjacency providers (e.g. Kimi, DeepSeek) require.
func requireToolCallAdjacency(t *testing.T, history []fantasy.Message) {
	t.Helper()
	for i, msg := range history {
		if msg.Role != fantasy.MessageRoleAssistant {
			continue
		}
		var callIDs []string
		for _, part := range msg.Content {
			if tc, ok := fantasy.AsMessagePart[fantasy.ToolCallPart](part); ok {
				callIDs = append(callIDs, tc.ToolCallID)
			}
		}
		if len(callIDs) == 0 {
			continue
		}
		require.Less(t, i+1, len(history), "assistant with tool calls must be followed by a tool message")
		next := history[i+1]
		require.Equal(t, fantasy.MessageRoleTool, next.Role,
			"assistant with tool calls %v must be immediately followed by a tool message, got %q", callIDs, next.Role)
		responded := make(map[string]bool, len(callIDs))
		for _, part := range next.Content {
			if tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
				responded[tr.ToolCallID] = true
			}
		}
		for _, id := range callIDs {
			require.True(t, responded[id],
				"tool result for call %q must immediately follow its assistant message", id)
		}
	}
}

func TestPreparePrompt_NonAdjacentToolResults(t *testing.T) {
	// A user message written between an assistant's tool call and its
	// result (e.g. resuming while a tool is still running) must not end up
	// between the two in the built history.
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "run commands"},
		},
	})
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "call_A",
				Name:     "bash",
				Input:    `{"command":"date"}`,
				Finished: true,
			},
			message.ToolCall{
				ID:       "call_B",
				Name:     "bash",
				Input:    `{"command":"uptime"}`,
				Finished: true,
			},
		},
	})
	require.NoError(t, err)

	// Interleaved user message written while the tools were still running.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "are we done?"},
		},
	})
	require.NoError(t, err)

	// Results arrive late, after the interleaved user message.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "call_A",
				Name:       "bash",
				Content:    "Fri May 2 21:00:00 UTC 2026",
			},
		},
	})
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "call_B",
				Name:       "bash",
				Content:    "21:00  up 3 days",
			},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	require.Equal(t, message.User, msgs[2].Role, "interleaved user should be between assistant and results in DB order")

	history, _ := agent.preparePrompt(msgs, false)

	requireToolCallAdjacency(t, history)

	// The interleaved user message must still be present, after the results.
	var foundInterleaved bool
	for _, msg := range history {
		if msg.Role != fantasy.MessageRoleUser {
			continue
		}
		for _, part := range msg.Content {
			if text, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok && text.Text == "are we done?" {
				foundInterleaved = true
			}
		}
	}
	require.True(t, foundInterleaved, "interleaved user message must not be dropped")
}

func TestPreparePrompt_ResultBeforeAssistant(t *testing.T) {
	// A tool result written before its assistant message (e.g. concurrent
	// writes) must be emitted after the assistant, exactly once, not at its
	// stored position.
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "call_X", Name: "bash", Content: "result"},
		},
	})
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "call_X", Name: "bash", Input: `{}`, Finished: true},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	history, _ := agent.preparePrompt(msgs, false)

	requireToolCallAdjacency(t, history)

	resultCount := 0
	for _, msg := range history {
		if msg.Role != fantasy.MessageRoleTool {
			continue
		}
		for _, part := range msg.Content {
			if tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok && tr.ToolCallID == "call_X" {
				resultCount++
			}
		}
	}
	require.Equal(t, 1, resultCount, "result must be emitted exactly once")
}

func TestPreparePrompt_BundledResultsAcrossAssistants(t *testing.T) {
	// A single tool message can hold results for calls issued by different
	// assistant messages. Each assistant must be followed by its own
	// results, and no result may be emitted twice.
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "call_1", Name: "bash", Input: `{}`, Finished: true},
		},
	})
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "and now?"},
		},
	})
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{ID: "call_2", Name: "view", Input: `{"path":"/foo"}`, Finished: true},
		},
	})
	require.NoError(t, err)

	// Both results land in the same tool message, out of order.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "call_2", Name: "view", Content: "file contents"},
			message.ToolResult{ToolCallID: "call_1", Name: "bash", Content: "output"},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	history, _ := agent.preparePrompt(msgs, false)

	requireToolCallAdjacency(t, history)

	counts := make(map[string]int)
	for _, msg := range history {
		if msg.Role != fantasy.MessageRoleTool {
			continue
		}
		for _, part := range msg.Content {
			if tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part); ok {
				counts[tr.ToolCallID]++
			}
		}
	}
	require.Equal(t, map[string]int{"call_1": 1, "call_2": 1}, counts,
		"each result must be emitted exactly once, next to its assistant")
}

func TestPreparePrompt_DropsOrphanedToolResults(t *testing.T) {
	// A tool result whose call is not in the history (e.g. the assistant
	// message was cut off by a session summary) must be dropped instead of
	// producing an unanswerable tool message.
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	ctx := t.Context()
	sess, err := env.sessions.Create(ctx, "test")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{ToolCallID: "call_gone", Name: "bash", Content: "output"},
		},
	})
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.User,
		Parts: []message.ContentPart{
			message.TextContent{Text: "hello"},
		},
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)

	history, _ := agent.preparePrompt(msgs, false)

	for _, msg := range history {
		require.NotEqual(t, fantasy.MessageRoleTool, msg.Role, "orphaned tool results must be dropped")
	}
}

func TestWorkaroundProviderMediaLimitations_TextOnlyModel(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	pngBase64 := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))

	messages := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call_1",
					Output: fantasy.ToolResultOutputContentMedia{
						Data:      pngBase64,
						MediaType: "image/png",
					},
				},
			},
		},
	}

	// Non-Anthropic provider, no image support вЂ” should replace media with
	// a text placeholder and not create a synthetic user message.
	largeModel := Model{
		ModelCfg: config.SelectedModel{Provider: "openai"},
		CatwalkCfg: catwalk.Model{
			SupportsImages: false,
		},
	}

	result := agent.workaroundProviderMediaLimitations(messages, largeModel)

	// Should produce exactly one message: the tool message with a text
	// placeholder. No synthetic user message with FilePart.
	require.Len(t, result, 1)
	require.Equal(t, fantasy.MessageRoleTool, result[0].Role)

	tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](result[0].Content[0])
	require.True(t, ok)
	_, ok = fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](tr.Output)
	require.True(t, ok)
}

func TestWorkaroundProviderMediaLimitations_VisionModel(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	pngBase64 := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))

	messages := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call_1",
					Output: fantasy.ToolResultOutputContentMedia{
						Data:      pngBase64,
						MediaType: "image/png",
					},
				},
			},
		},
	}

	// Non-Anthropic provider, image support вЂ” should create a synthetic
	// user message with FilePart.
	largeModel := Model{
		ModelCfg: config.SelectedModel{Provider: "openai"},
		CatwalkCfg: catwalk.Model{
			SupportsImages: true,
		},
	}

	result := agent.workaroundProviderMediaLimitations(messages, largeModel)

	// Should produce two messages: tool message with placeholder text,
	// and synthetic user message with FilePart.
	require.Len(t, result, 2)
	require.Equal(t, fantasy.MessageRoleTool, result[0].Role)
	require.Equal(t, fantasy.MessageRoleUser, result[1].Role)

	// The tool message should have text placeholder.
	tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](result[0].Content[0])
	require.True(t, ok)
	textOutput, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentText](tr.Output)
	require.True(t, ok)
	require.Contains(t, textOutput.Text, "see attached file")

	// The synthetic user message should contain a TextPart and a FilePart.
	require.Len(t, result[1].Content, 2)
	file, ok := fantasy.AsMessagePart[fantasy.FilePart](result[1].Content[1])
	require.True(t, ok)
	require.Equal(t, "image/png", file.MediaType)
}

func TestWorkaroundProviderMediaLimitations_AnthropicProvider(t *testing.T) {
	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "test prompt")
	agent := sa.(*sessionAgent)

	pngBase64 := base64.StdEncoding.EncodeToString([]byte("fake-png-data"))

	messages := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "call_1",
					Output: fantasy.ToolResultOutputContentMedia{
						Data:      pngBase64,
						MediaType: "image/png",
					},
				},
			},
		},
	}

	// Anthropic provider вЂ” should return messages unchanged regardless of
	// SupportsImages, since Anthropic handles media in tool results natively.
	largeModel := Model{
		ModelCfg: config.SelectedModel{Provider: string(catwalk.InferenceProviderAnthropic)},
		CatwalkCfg: catwalk.Model{
			SupportsImages: true,
		},
	}

	result := agent.workaroundProviderMediaLimitations(messages, largeModel)
	require.Len(t, result, 1)
	require.Equal(t, fantasy.MessageRoleTool, result[0].Role)

	// The media should still be in the tool result, untouched.
	tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](result[0].Content[0])
	require.True(t, ok)
	media, ok := fantasy.AsToolResultOutputType[fantasy.ToolResultOutputContentMedia](tr.Output)
	require.True(t, ok)
	require.Equal(t, "image/png", media.MediaType)
}

func TestProviderRetryLogFields(t *testing.T) {
	t.Run("nil provider error", func(t *testing.T) {
		fields := providerRetryLogFields(nil, 2*time.Second)
		require.Equal(t, []any{"retry_delay", "2s"}, fields)
	})

	t.Run("provider error with title and message", func(t *testing.T) {
		fields := providerRetryLogFields(&fantasy.ProviderError{
			StatusCode: 429,
			Title:      "rate limit",
			Message:    "too many requests",
		}, 1500*time.Millisecond)
		require.Equal(t, []any{
			"retry_delay", "1.5s",
			"status_code", 429,
			"title", "rate limit",
			"message", "too many requests",
		}, fields)
	})

	t.Run("provider error without optional strings", func(t *testing.T) {
		fields := providerRetryLogFields(&fantasy.ProviderError{
			StatusCode: 503,
		}, time.Second)
		require.Equal(t, []any{
			"retry_delay", "1s",
			"status_code", 503,
		}, fields)
	})
}
