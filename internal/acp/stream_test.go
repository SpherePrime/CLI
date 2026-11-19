package acp

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestStreamTranslator_IncrementalTextAndThoughtDeltas(t *testing.T) {
	t.Parallel()

	var updates []any
	tr := NewStreamTranslator("sess-1", func(u any) {
		updates = append(updates, u)
	})

	tr.HandleMessage(message.Message{
		ID:        "msg-1",
		SessionID: "sess-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "First thought"},
		},
	})
	tr.HandleMessage(message.Message{
		ID:        "msg-1",
		SessionID: "sess-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "First thought, second part"},
			message.TextContent{Text: "Hello"},
		},
	})
	tr.HandleMessage(message.Message{
		ID:        "msg-1",
		SessionID: "sess-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "First thought, second part"},
			message.TextContent{Text: "Hello world"},
		},
	})
	tr.ReconcileFinalText("msg-1", "Hello world!")
	tr.ReconcileFinalText("msg-1", "Hello world!")

	require.Len(t, updates, 5)

	thought1, ok := updates[0].(AgentThoughtChunkUpdate)
	require.True(t, ok)
	require.Equal(t, "agent_thought_chunk", thought1.SessionUpdate)
	require.Equal(t, "First thought", thought1.Content.Text)

	thought2, ok := updates[1].(AgentThoughtChunkUpdate)
	require.True(t, ok)
	require.Equal(t, ", second part", thought2.Content.Text)

	chunk1, ok := updates[2].(AgentMessageChunkUpdate)
	require.True(t, ok)
	require.Equal(t, "agent_message_chunk", chunk1.SessionUpdate)
	require.Equal(t, "Hello", chunk1.Content.Text)

	chunk2, ok := updates[3].(AgentMessageChunkUpdate)
	require.True(t, ok)
	require.Equal(t, " world", chunk2.Content.Text)

	chunk3, ok := updates[4].(AgentMessageChunkUpdate)
	require.True(t, ok)
	require.Equal(t, "!", chunk3.Content.Text)
}

func TestStreamTranslator_ToolCallLifecycleAndLocations(t *testing.T) {
	t.Parallel()

	var updates []any
	tr := NewStreamTranslator("sess-1", func(u any) {
		updates = append(updates, u)
	})

	tr.HandleMessage(message.Message{
		ID:        "msg-a",
		SessionID: "sess-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "tc-1",
				Name:     "edit",
				Input:    `{"file_path":"main.go","line":12}`,
				Finished: false,
			},
		},
	})

	tr.HandleMessage(message.Message{
		ID:        "msg-a",
		SessionID: "sess-1",
		Role:      message.Assistant,
		Parts: []message.ContentPart{
			message.ToolCall{
				ID:       "tc-1",
				Name:     "edit",
				Input:    `{"file_path":"main.go","line":12}`,
				Finished: true,
			},
			message.ToolCall{
				ID:       "tc-2",
				Name:     "bash",
				Input:    `{"command":"go test ./..."}`,
				Finished: true,
			},
		},
	})

	tr.HandleMessage(message.Message{
		ID:        "msg-tool",
		SessionID: "sess-1",
		Role:      message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "tc-1",
				Name:       "edit",
				Content:    "edited main.go",
				IsError:    false,
			},
			message.ToolResult{
				ToolCallID: "tc-2",
				Name:       "bash",
				Content:    "exit status 1",
				IsError:    true,
			},
		},
	})

	tr.HandleMessage(message.Message{
		ID:        "msg-tool",
		SessionID: "sess-1",
		Role:      message.Tool,
		Parts: []message.ContentPart{
			message.ToolResult{
				ToolCallID: "tc-1",
				Name:       "edit",
				Content:    "edited main.go",
				IsError:    false,
			},
		},
	})

	require.Len(t, updates, 5)

	start1, ok := updates[0].(ToolCallUpdate)
	require.True(t, ok)
	require.Equal(t, "tool_call", start1.SessionUpdate)
	require.Equal(t, "tc-1", start1.ToolCallID)
	require.Equal(t, ToolKindEdit, start1.Kind)
	require.Equal(t, ToolStatusPending, start1.Status)
	require.Equal(t, "edit main.go", start1.Title)
	require.Len(t, start1.Locations, 1)
	require.Equal(t, "main.go", start1.Locations[0].Path)
	require.NotNil(t, start1.Locations[0].Line)
	require.Equal(t, 12, *start1.Locations[0].Line)

	prog1, ok := updates[1].(ToolCallProgressUpdate)
	require.True(t, ok)
	require.Equal(t, "tool_call_update", prog1.SessionUpdate)
	require.Equal(t, "tc-1", prog1.ToolCallID)
	require.Equal(t, ToolStatusInProgress, prog1.Status)

	start2, ok := updates[2].(ToolCallUpdate)
	require.True(t, ok)
	require.Equal(t, "tc-2", start2.ToolCallID)
	require.Equal(t, ToolKindExecute, start2.Kind)
	require.Equal(t, ToolStatusInProgress, start2.Status)
	require.Equal(t, "bash: go test ./...", start2.Title)

	done1, ok := updates[3].(ToolCallProgressUpdate)
	require.True(t, ok)
	require.Equal(t, "tc-1", done1.ToolCallID)
	require.Equal(t, ToolStatusCompleted, done1.Status)
	require.Len(t, done1.Content, 1)
	require.Equal(t, "edited main.go", done1.Content[0].Content.Text)

	done2, ok := updates[4].(ToolCallProgressUpdate)
	require.True(t, ok)
	require.Equal(t, "tc-2", done2.ToolCallID)
	require.Equal(t, ToolStatusFailed, done2.Status)
	require.Len(t, done2.Content, 1)
	require.Equal(t, "exit status 1", done2.Content[0].Content.Text)
}

func TestStreamTranslator_PlanUpdatesFromTodos(t *testing.T) {
	t.Parallel()

	var updates []any
	tr := NewStreamTranslator("sess-1", func(u any) {
		updates = append(updates, u)
	})

	todos := []session.Todo{
		{Content: "Step 1", Status: session.TodoStatusCompleted},
		{Content: "Step 2", Status: session.TodoStatusInProgress},
		{Content: "Step 3", Status: session.TodoStatusPending},
	}

	tr.HandleTodos(nil)
	tr.HandleTodos(todos)
	tr.HandleTodos(todos)

	require.Len(t, updates, 1)
	plan, ok := updates[0].(PlanUpdate)
	require.True(t, ok)
	require.Equal(t, "plan", plan.SessionUpdate)
	require.Len(t, plan.Entries, 3)
	require.Equal(t, PlanStatusCompleted, plan.Entries[0].Status)
	require.Equal(t, PlanStatusInProgress, plan.Entries[1].Status)
	require.Equal(t, PlanStatusPending, plan.Entries[2].Status)
}

func TestMapToolKind(t *testing.T) {
	t.Parallel()

	cases := map[string]string{
		"view":          ToolKindRead,
		"ls":            ToolKindRead,
		"lsp_symbols":   ToolKindRead,
		"edit":          ToolKindEdit,
		"multiedit":     ToolKindEdit,
		"write":         ToolKindEdit,
		"glob":          ToolKindSearch,
		"grep":          ToolKindSearch,
		"web_search":    ToolKindSearch,
		"bash":          ToolKindExecute,
		"fetch":         ToolKindFetch,
		"agentic_fetch": ToolKindFetch,
		"todos":         ToolKindOther,
		"custom_tool":   ToolKindOther,
	}

	for name, want := range cases {
		require.Equal(t, want, MapToolKind(name), "tool %s", name)
	}
}

func TestReplayMessages(t *testing.T) {
	t.Parallel()

	var updates []any
	err := ReplayMessages(
		func(u any) error {
			updates = append(updates, u)
			return nil
		},
		"sess-replay",
		[]message.Message{
			{
				ID:        "u1",
				SessionID: "sess-replay",
				Role:      message.User,
				Parts: []message.ContentPart{
					message.TextContent{Text: "Please inspect"},
					message.TextContent{Text: "hidden note", Hidden: true},
				},
			},
			{
				ID:        "a1",
				SessionID: "sess-replay",
				Role:      message.Assistant,
				Parts: []message.ContentPart{
					message.ReasoningContent{Thinking: "Checking file"},
					message.ToolCall{ID: "tc-1", Name: "view", Input: `{"file_path":"README.md"}`, Finished: true},
					message.TextContent{Text: "Looks good."},
				},
			},
			{
				ID:        "t1",
				SessionID: "sess-replay",
				Role:      message.Tool,
				Parts: []message.ContentPart{
					message.ToolResult{ToolCallID: "tc-1", Name: "view", Content: "# Readme"},
				},
			},
		},
		[]session.Todo{
			{Content: "Inspect readme", Status: session.TodoStatusCompleted},
		},
	)
	require.NoError(t, err)
	require.Len(t, updates, 6)

	uChunk, ok := updates[0].(UserMessageChunkUpdate)
	require.True(t, ok)
	require.Equal(t, "Please inspect", uChunk.Content.Text)

	thChunk, ok := updates[1].(AgentThoughtChunkUpdate)
	require.True(t, ok)
	require.Equal(t, "Checking file", thChunk.Content.Text)

	aChunk, ok := updates[2].(AgentMessageChunkUpdate)
	require.True(t, ok)
	require.Equal(t, "Looks good.", aChunk.Content.Text)

	tcStart, ok := updates[3].(ToolCallUpdate)
	require.True(t, ok)
	require.Equal(t, "tc-1", tcStart.ToolCallID)
	require.Equal(t, ToolKindRead, tcStart.Kind)

	tcDone, ok := updates[4].(ToolCallProgressUpdate)
	require.True(t, ok)
	require.Equal(t, ToolStatusCompleted, tcDone.Status)

	plan, ok := updates[5].(PlanUpdate)
	require.True(t, ok)
	require.Len(t, plan.Entries, 1)
}
