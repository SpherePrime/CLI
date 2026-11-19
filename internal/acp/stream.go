package acp

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/session"
)

type trackedToolCall struct {
	name          string
	title         string
	kind          string
	status        string
	input         string
	resultEmitted bool
}

type StreamTranslator struct {
	sessionID      string
	emit           func(update any)
	textOffsets    map[string]int
	thoughtOffsets map[string]int
	toolStates     map[string]*trackedToolCall
	lastPlanJSON   string
}

func NewStreamTranslator(sessionID string, emit func(update any)) *StreamTranslator {
	return &StreamTranslator{
		sessionID:      sessionID,
		emit:           emit,
		textOffsets:    make(map[string]int),
		thoughtOffsets: make(map[string]int),
		toolStates:     make(map[string]*trackedToolCall),
	}
}

func (t *StreamTranslator) HandleMessage(msg message.Message) {
	switch msg.Role {
	case message.Assistant:
		t.handleAssistantMessage(msg)
	case message.Tool:
		t.handleToolMessage(msg)
	}
}

func (t *StreamTranslator) handleAssistantMessage(msg message.Message) {
	key := msg.ID
	if key == "" {
		key = "_assistant"
	}

	thinking := collectThinkingText(msg)
	if offset := t.thoughtOffsets[key]; len(thinking) > offset {
		delta := thinking[offset:]
		t.thoughtOffsets[key] = len(thinking)
		t.emitUpdate(AgentThoughtChunkUpdate{
			SessionUpdate: "agent_thought_chunk",
			Content: ContentBlock{
				Type: "text",
				Text: delta,
			},
		})
	}

	text := collectVisibleText(msg)
	if offset := t.textOffsets[key]; len(text) > offset {
		delta := text[offset:]
		t.textOffsets[key] = len(text)
		t.emitUpdate(AgentMessageChunkUpdate{
			SessionUpdate: "agent_message_chunk",
			Content: ContentBlock{
				Type: "text",
				Text: delta,
			},
		})
	}

	for _, tc := range msg.ToolCalls() {
		if tc.ID == "" {
			continue
		}
		kind := MapToolKind(tc.Name)
		title := BuildToolTitle(tc.Name, tc.Input)
		locations := ExtractToolLocations(tc.Name, tc.Input)
		rawInput := parseToolRawInput(tc.Input)
		status := ToolStatusPending
		if tc.Finished {
			status = ToolStatusInProgress
		}

		state, exists := t.toolStates[tc.ID]
		if !exists {
			t.toolStates[tc.ID] = &trackedToolCall{
				name:   tc.Name,
				title:  title,
				kind:   kind,
				status: status,
				input:  tc.Input,
			}
			t.emitUpdate(ToolCallUpdate{
				SessionUpdate: "tool_call",
				ToolCallID:    tc.ID,
				Title:         title,
				Kind:          kind,
				Status:        status,
				Locations:     locations,
				RawInput:      rawInput,
			})
			continue
		}

		if state.resultEmitted {
			continue
		}
		if state.status == status && state.input == tc.Input && state.title == title {
			continue
		}

		state.status = status
		state.input = tc.Input
		state.title = title
		t.emitUpdate(ToolCallProgressUpdate{
			SessionUpdate: "tool_call_update",
			ToolCallID:    tc.ID,
			Title:         title,
			Kind:          kind,
			Status:        status,
			Locations:     locations,
			RawInput:      rawInput,
		})
	}
}

func (t *StreamTranslator) handleToolMessage(msg message.Message) {
	for _, tr := range msg.ToolResults() {
		if tr.ToolCallID == "" {
			continue
		}
		state, exists := t.toolStates[tr.ToolCallID]
		if exists && state.resultEmitted {
			continue
		}
		if !exists {
			kind := MapToolKind(tr.Name)
			title := BuildToolTitle(tr.Name, "")
			state = &trackedToolCall{
				name:   tr.Name,
				title:  title,
				kind:   kind,
				status: ToolStatusInProgress,
			}
			t.toolStates[tr.ToolCallID] = state
			t.emitUpdate(ToolCallUpdate{
				SessionUpdate: "tool_call",
				ToolCallID:    tr.ToolCallID,
				Title:         title,
				Kind:          kind,
				Status:        ToolStatusInProgress,
			})
		}

		status := ToolStatusCompleted
		if tr.IsError {
			status = ToolStatusFailed
		}
		content := buildToolResultContent(tr)
		rawOutput := map[string]any{
			"content": tr.Content,
			"isError": tr.IsError,
		}
		if tr.Metadata != "" {
			rawOutput["metadata"] = tr.Metadata
		}

		state.status = status
		state.resultEmitted = true
		t.emitUpdate(ToolCallProgressUpdate{
			SessionUpdate: "tool_call_update",
			ToolCallID:    tr.ToolCallID,
			Status:        status,
			Content:       content,
			RawOutput:     rawOutput,
		})
	}
}

func (t *StreamTranslator) ReconcileFinalText(messageID, text string) {
	if text == "" {
		return
	}
	key := messageID
	if key == "" {
		key = "_assistant"
	}
	offset := t.textOffsets[key]
	if len(text) <= offset {
		return
	}
	delta := text[offset:]
	t.textOffsets[key] = len(text)
	t.emitUpdate(AgentMessageChunkUpdate{
		SessionUpdate: "agent_message_chunk",
		Content: ContentBlock{
			Type: "text",
			Text: delta,
		},
	})
}

func (t *StreamTranslator) HandleTodos(todos []session.Todo) {
	if len(todos) == 0 && t.lastPlanJSON == "" {
		return
	}
	entries := make([]PlanEntry, 0, len(todos))
	for _, todo := range todos {
		entries = append(entries, PlanEntry{
			Content:  todo.Content,
			Priority: PlanPriorityMedium,
			Status:   mapTodoStatus(todo.Status),
		})
	}
	serialized, err := json.Marshal(entries)
	if err != nil {
		return
	}
	snap := string(serialized)
	if snap == t.lastPlanJSON {
		return
	}
	t.lastPlanJSON = snap
	t.emitUpdate(PlanUpdate{
		SessionUpdate: "plan",
		Entries:       entries,
	})
}

func (t *StreamTranslator) emitUpdate(update any) {
	if t.emit != nil {
		t.emit(update)
	}
}

func ReplayMessages(sendUpdate func(any) error, sessionID string, messages []message.Message, todos []session.Todo) error {
	var firstErr error
	emit := func(update any) {
		if firstErr != nil || sendUpdate == nil {
			return
		}
		if err := sendUpdate(update); err != nil {
			firstErr = err
		}
	}

	translator := NewStreamTranslator(sessionID, emit)
	for _, msg := range messages {
		if firstErr != nil {
			return firstErr
		}
		switch msg.Role {
		case message.User:
			replayUserMessage(emit, msg)
		case message.Assistant, message.Tool:
			translator.HandleMessage(msg)
		}
	}

	if firstErr == nil && len(todos) > 0 {
		translator.HandleTodos(todos)
	}
	return firstErr
}

func replayUserMessage(emit func(any), msg message.Message) {
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case message.TextContent:
			if p.Hidden || p.Text == "" {
				continue
			}
			emit(UserMessageChunkUpdate{
				SessionUpdate: "user_message_chunk",
				Content: ContentBlock{
					Type: "text",
					Text: p.Text,
				},
			})
		case message.BinaryContent:
			if len(p.Data) == 0 || !strings.HasPrefix(p.MIMEType, "image/") {
				continue
			}
			emit(UserMessageChunkUpdate{
				SessionUpdate: "user_message_chunk",
				Content: ContentBlock{
					Type:     "image",
					Data:     base64.StdEncoding.EncodeToString(p.Data),
					MimeType: p.MIMEType,
				},
			})
		case message.ShellCommand:
			if p.Command == "" {
				continue
			}
			emit(UserMessageChunkUpdate{
				SessionUpdate: "user_message_chunk",
				Content: ContentBlock{
					Type: "text",
					Text: "!" + p.Command,
				},
			})
		}
	}
}

func MapToolKind(toolName string) string {
	switch toolName {
	case "view", "ls", "list_mcp_resources", "read_mcp_resource", "read_instruction",
		"prime_info", "prime_logs", "lsp_symbols", "lsp_definition",
		"lsp_call_hierarchy", "lsp_diagnostics":
		return ToolKindRead
	case "edit", "multiedit", "write", "lsp_rename", "lsp_replace_symbol":
		return ToolKindEdit
	case "glob", "grep", "sourcegraph", "lsp_references", "web_search",
		"search_instructions", "search_skills", "search_mcp", "search_tools":
		return ToolKindSearch
	case "bash", "sudo", "job_output", "job_kill", "lsp_restart":
		return ToolKindExecute
	case "fetch", "agentic_fetch", "download":
		return ToolKindFetch
	default:
		if strings.HasPrefix(toolName, "lsp_") {
			return ToolKindRead
		}
		return ToolKindOther
	}
}

func BuildToolTitle(toolName, rawInput string) string {
	name := toolName
	if name == "" {
		name = "tool"
	}
	fields := parseToolInputMap(rawInput)
	if len(fields) == 0 {
		return name
	}

	switch name {
	case "bash", "sudo":
		if desc := stringField(fields, "description"); desc != "" {
			return name + ": " + truncateText(desc, 80)
		}
		if cmd := stringField(fields, "command"); cmd != "" {
			return name + ": " + truncateText(cmd, 80)
		}
	case "view", "edit", "multiedit", "write", "ls":
		if p := firstPathField(fields); p != "" {
			return name + " " + p
		}
	case "grep", "glob", "web_search", "sourcegraph":
		if pat := stringField(fields, "pattern"); pat != "" {
			return name + " " + truncateText(pat, 80)
		}
		if q := stringField(fields, "query"); q != "" {
			return name + " " + truncateText(q, 80)
		}
	case "fetch", "agentic_fetch", "download":
		if u := stringField(fields, "url"); u != "" {
			return name + " " + truncateText(u, 80)
		}
	}

	if p := firstPathField(fields); p != "" {
		return name + " " + p
	}
	return name
}

func ExtractToolLocations(_ string, rawInput string) []ToolCallLocation {
	fields := parseToolInputMap(rawInput)
	if len(fields) == 0 {
		return nil
	}
	pathVal := firstPathField(fields)
	if pathVal == "" {
		return nil
	}
	loc := ToolCallLocation{Path: pathVal}
	if line, ok := intField(fields, "line"); ok && line > 0 {
		loc.Line = &line
	} else if offset, ok := intField(fields, "offset"); ok && offset > 0 {
		loc.Line = &offset
	}
	return []ToolCallLocation{loc}
}

func collectVisibleText(msg message.Message) string {
	var b strings.Builder
	for _, part := range msg.Parts {
		if tc, ok := part.(message.TextContent); ok && !tc.Hidden {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

func collectThinkingText(msg message.Message) string {
	var b strings.Builder
	for _, part := range msg.Parts {
		if rc, ok := part.(message.ReasoningContent); ok {
			b.WriteString(rc.Thinking)
		}
	}
	return b.String()
}

func buildToolResultContent(tr message.ToolResult) []ToolCallContent {
	var out []ToolCallContent
	if tr.Content != "" {
		out = append(out, ToolCallContent{
			Type: "content",
			Content: &ContentBlock{
				Type: "text",
				Text: tr.Content,
			},
		})
	}
	if tr.Data != "" && strings.HasPrefix(tr.MIMEType, "image/") {
		out = append(out, ToolCallContent{
			Type: "content",
			Content: &ContentBlock{
				Type:     "image",
				Data:     tr.Data,
				MimeType: tr.MIMEType,
			},
		})
	}
	for _, img := range tr.Images {
		if img.Data == "" {
			continue
		}
		mime := img.MIMEType
		if mime == "" {
			mime = "image/png"
		}
		out = append(out, ToolCallContent{
			Type: "content",
			Content: &ContentBlock{
				Type:     "image",
				Data:     img.Data,
				MimeType: mime,
			},
		})
	}
	return out
}

func mapTodoStatus(status session.TodoStatus) string {
	switch status {
	case session.TodoStatusInProgress:
		return PlanStatusInProgress
	case session.TodoStatusCompleted:
		return PlanStatusCompleted
	default:
		return PlanStatusPending
	}
}

func parseToolRawInput(rawInput string) any {
	trimmed := strings.TrimSpace(rawInput)
	if trimmed == "" {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed
	}
	return rawInput
}

func parseToolInputMap(rawInput string) map[string]any {
	trimmed := strings.TrimSpace(rawInput)
	if trimmed == "" || trimmed[0] != '{' {
		return nil
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return nil
	}
	return parsed
}

func firstPathField(fields map[string]any) string {
	for _, key := range []string{"file_path", "path", "filePath", "directory"} {
		if v := stringField(fields, key); v != "" {
			return v
		}
	}
	return ""
}

func stringField(fields map[string]any, key string) string {
	val, ok := fields[key]
	if !ok {
		return ""
	}
	s, ok := val.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func intField(fields map[string]any, key string) (int, bool) {
	val, ok := fields[key]
	if !ok {
		return 0, false
	}
	switch n := val.(type) {
	case float64:
		return int(n), true
	case int:
		return n, true
	case int64:
		return int(n), true
	default:
		return 0, false
	}
}

func truncateText(s string, maxLen int) string {
	runes := []rune(strings.ReplaceAll(s, "\n", " "))
	if len(runes) <= maxLen {
		return string(runes)
	}
	return string(runes[:maxLen-1]) + "…"
}
