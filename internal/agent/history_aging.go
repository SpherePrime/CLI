package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	agingKeepTurns          = 3
	agingTurnBatch          = 4
	agingSuccessMinBytes    = 1024
	agingFailureMinBytes    = 4096
	agingFailureKeepBytes   = 2048
	agingCallStringMinBytes = 1024
	agingPreviewRunes       = 160
	agingKeepMedia          = 2
	agingMediaBatch         = 5
)

type historyAgingPolicy struct {
	KeepTurns int
	TurnBatch int
	KeepMedia  int
	MediaBatch int
}

func defaultHistoryAgingPolicy() historyAgingPolicy {
	return historyAgingPolicy{KeepTurns: agingKeepTurns, TurnBatch: agingTurnBatch, KeepMedia: agingKeepMedia, MediaBatch: agingMediaBatch}
}

type agedCall struct {
	Name  string
	Input string
	Path  string
}

// agingBoundary returns the index of the first message that stays intact.
// The boundary advances in whole batches of user turns so earlier messages
// keep identical bytes across many requests and prompt caches stay warm.
func agingBoundary(messages []fantasy.Message, policy historyAgingPolicy) int {
	var turnStarts []int
	for i, message := range messages {
		if isUserTurn(message) {
			turnStarts = append(turnStarts, i)
		}
	}
	batch := max(policy.TurnBatch, 1)
	aged := (len(turnStarts) - policy.KeepTurns) / batch * batch
	if aged <= 0 {
		return 0
	}
	return turnStarts[aged]
}

func isUserTurn(message fantasy.Message) bool {
	if message.Role != fantasy.MessageRoleUser {
		return false
	}
	for _, part := range message.Content {
		if _, ok := part.(fantasy.TextPart); ok {
			return true
		}
	}
	return false
}

func ageToolHistory(ctx context.Context, scope string, store toolArtifactStore, messages []fantasy.Message, policy historyAgingPolicy) []fantasy.Message {
	boundary := agingBoundary(messages, policy)
	removedMedia := mediaToRemove(messages, policy)
	if boundary == 0 && len(removedMedia) == 0 {
		return messages
	}
	calls := agedCalls(messages)
	modifiedAfter := laterModifications(messages)
	aged := cloneFantasyMessages(messages)
	for i := range aged {
		old := i < boundary
		for j, part := range aged[i].Content {
			switch p := part.(type) {
			case fantasy.ToolCallPart:
				if old {
					p.Input = shrinkCallInput(p.Input)
					aged[i].Content[j] = p
				}
			case fantasy.ToolResultPart:
				call := calls[p.ToolCallID]
				if inlineDocumentedTool(call.Name) {
					continue
				}
				if media, ok := p.Output.(fantasy.ToolResultOutputContentMedia); ok && removedMedia[p.ToolCallID] {
					p.Output = fantasy.ToolResultOutputContentText{Text: mediaRemovedText(call, media)}
					aged[i].Content[j] = p
					continue
				}
				if !old {
					continue
				}
				stale := call.Path != "" && call.Name == "view" && modifiedAfter[call.Path] > i
				if rewritten, ok := ageToolResult(ctx, scope, store, p, call, stale); ok {
					aged[i].Content[j] = rewritten
				}
			}
		}
	}
	return aged
}

func ageToolResult(ctx context.Context, scope string, store toolArtifactStore, result fantasy.ToolResultPart, call agedCall, stale bool) (fantasy.ToolResultPart, bool) {
	if store == nil {
		return result, false
	}
	switch output := result.Output.(type) {
	case fantasy.ToolResultOutputContentText:
		failed := result.Outcome == fantasy.ToolOutcomeFailure
		if failed && len(output.Text) < agingFailureMinBytes || !failed && len(output.Text) < agingSuccessMinBytes {
			return result, false
		}
		id, err := store.Put(ctx, scope, result.ToolCallID, result.Outcome, []byte(output.Text))
		if err != nil {
			return result, false
		}
		if failed {
			result.Output = fantasy.ToolResultOutputContentText{Text: agedFailureText(call, output.Text, id)}
		} else {
			result.Output = fantasy.ToolResultOutputContentText{Text: agedSuccessText(call, output.Text, id, stale)}
		}
		return result, true
	case fantasy.ToolResultOutputContentError:
		if output.Error == nil || len(output.Error.Error()) < agingFailureMinBytes {
			return result, false
		}
		text := output.Error.Error()
		id, err := store.Put(ctx, scope, result.ToolCallID, fantasy.ToolOutcomeFailure, []byte(text))
		if err != nil {
			return result, false
		}
		result.Output = fantasy.ToolResultOutputContentError{Error: errors.New(agedFailureText(call, text, id))}
		return result, true
	}
	return result, false
}

func agedSuccessText(call agedCall, text, id string, stale bool) string {
	state := "Earlier result"
	if stale {
		state = "Outdated result: the file was modified later"
	}
	return fmt.Sprintf("[%s of %s%s, %d bytes, stored as %s. Preview: %s. Call %s with artifact_id=%q to read it.]",
		state, callLabel(call.Name), callTarget(call), len(text), id, previewLine(text), readToolResultName, id)
}

func agedFailureText(call agedCall, text, id string) string {
	head := utf8SafePrefix(text, agingFailureKeepBytes)
	return fmt.Sprintf("Earlier failure of %s%s, %d bytes, stored as %s. First %d bytes:\n%s\n\nCall %s with artifact_id=%q to read the complete output.",
		callLabel(call.Name), callTarget(call), len(text), id, len(head), head, readToolResultName, id)
}

func mediaRemovedText(call agedCall, media fantasy.ToolResultOutputContentMedia) string {
	text := fmt.Sprintf("[Earlier %s from %s%s removed from context. Run the tool again to see it.]",
		mediaKind(media.MediaType), callLabel(call.Name), callTarget(call))
	if media.Text != "" {
		text = media.Text + "\n" + text
	}
	return text
}

func callLabel(name string) string {
	if name == "" {
		return "a tool"
	}
	return name
}

func callTarget(call agedCall) string {
	if call.Path != "" {
		return " " + call.Path
	}
	return ""
}

func mediaKind(mediaType string) string {
	if strings.HasPrefix(mediaType, "image/") {
		return "image"
	}
	return "attachment"
}

func previewLine(text string) string {
	line := strings.TrimSpace(text)
	if index := strings.IndexByte(line, '\n'); index >= 0 {
		line = strings.TrimSpace(line[:index])
	}
	if utf8.RuneCountInString(line) > agingPreviewRunes {
		line = string([]rune(line)[:agingPreviewRunes]) + "…"
	}
	return fmt.Sprintf("%q", line)
}

func agedCalls(messages []fantasy.Message) map[string]agedCall {
	calls := map[string]agedCall{}
	for _, message := range messages {
		for _, part := range message.Content {
			call, ok := part.(fantasy.ToolCallPart)
			if !ok {
				continue
			}
			calls[call.ToolCallID] = agedCall{Name: call.ToolName, Input: call.Input, Path: callFilePath(call.Input)}
		}
	}
	return calls
}

func callFilePath(input string) string {
	var arguments struct {
		FilePath string `json:"file_path"`
		Path     string `json:"path"`
	}
	if json.Unmarshal([]byte(input), &arguments) != nil {
		return ""
	}
	if arguments.FilePath != "" {
		return arguments.FilePath
	}
	return arguments.Path
}

func modifiesFile(name string) bool {
	switch name {
	case "edit", "multiedit", "write", "lsp_rename", "lsp_replace_symbol":
		return true
	default:
		return false
	}
}

// laterModifications maps a file path to the index of the last message that
// changed it, so reads made before that point can be marked outdated.
func laterModifications(messages []fantasy.Message) map[string]int {
	modified := map[string]int{}
	for i, message := range messages {
		for _, part := range message.Content {
			call, ok := part.(fantasy.ToolCallPart)
			if !ok || !modifiesFile(call.ToolName) {
				continue
			}
			if path := callFilePath(call.Input); path != "" {
				modified[path] = i
			}
		}
	}
	return modified
}

// mediaToRemove picks the oldest media results to drop. Media leaves in
// batches so the request prefix changes once per batch, not per screenshot.
func mediaToRemove(messages []fantasy.Message, policy historyAgingPolicy) map[string]bool {
	var media []string
	for _, message := range messages {
		for _, part := range message.Content {
			if result, ok := part.(fantasy.ToolResultPart); ok {
				if _, isMedia := result.Output.(fantasy.ToolResultOutputContentMedia); isMedia {
					media = append(media, result.ToolCallID)
				}
			}
		}
	}
	batch := max(policy.MediaBatch, 1)
	count := (len(media) - policy.KeepMedia) / batch * batch
	removed := map[string]bool{}
	for _, id := range media[:max(count, 0)] {
		removed[id] = true
	}
	return removed
}

// shrinkCallInput replaces long string arguments of an old tool call, such
// as the full text of a written file, with their size. The result stays a
// valid JSON object with the same keys.
func shrinkCallInput(input string) string {
	if len(input) < agingCallStringMinBytes {
		return input
	}
	var value any
	if json.Unmarshal([]byte(input), &value) != nil {
		return input
	}
	shrunk, changed := shrinkJSONStrings(value)
	if !changed {
		return input
	}
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	if encoder.Encode(shrunk) != nil {
		return input
	}
	return strings.TrimSuffix(buffer.String(), "\n")
}

func shrinkJSONStrings(value any) (any, bool) {
	switch v := value.(type) {
	case string:
		if len(v) < agingCallStringMinBytes {
			return v, false
		}
		return fmt.Sprintf("[%d bytes omitted from an earlier call]", len(v)), true
	case map[string]any:
		changed := false
		for key, item := range v {
			if shrunk, itemChanged := shrinkJSONStrings(item); itemChanged {
				v[key] = shrunk
				changed = true
			}
		}
		return v, changed
	case []any:
		changed := false
		for i, item := range v {
			if shrunk, itemChanged := shrinkJSONStrings(item); itemChanged {
				v[i] = shrunk
				changed = true
			}
		}
		return v, changed
	default:
		return value, false
	}
}
