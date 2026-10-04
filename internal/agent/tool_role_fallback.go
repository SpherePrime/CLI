package agent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

// Some OpenAI-compatible endpoints validate role strictly and reject the
// standard OpenAI "tool" message role with a 400/422 like
//
//	{"detail":[{"type":"literal_error","loc":["body","messages",4,"role"],
//	  "msg":"Input should be 'assistant', 'user' or 'system'", "input":"tool"}]}
//
// toolRoleCompatModel retries such calls once with every tool message
// rewritten as a user message carrying the tool result as text.
type toolRoleCompatModel struct {
	fantasy.LanguageModel
}

func newToolRoleCompatModel(m fantasy.LanguageModel) fantasy.LanguageModel {
	if m == nil {
		return m
	}
	return &toolRoleCompatModel{LanguageModel: m}
}

// isToolRoleRejectionErr reports whether the error is a 4xx validation
// failure about the "tool" message role. Only those are worth replaying
// with a rewritten prompt; anything else passes through untouched.
//
// The detection is deliberately conservative: it fires on a 400/422 whose
// payload talks about "role" plus the rejected value ("tool"), the list of
// allowed values, or standard validation phrasing. A 4xx that merely
// contains the word "role" without either of those is not a tool-role
// rejection and is left alone.
func isToolRoleRejectionErr(err error) bool {
	var pe *fantasy.ProviderError
	if !errors.As(err, &pe) {
		return false
	}
	if pe.StatusCode != http.StatusBadRequest && pe.StatusCode != http.StatusUnprocessableEntity {
		return false
	}
	body := string(pe.ResponseBody)
	if body == "" {
		body = pe.Message
	}
	lower := strings.ToLower(body)
	if !strings.Contains(lower, "role") {
		return false
	}
	for _, marker := range []string{
		"tool",
		"assistant",
		"one of",
		"literal_error",
		"input should be",
		"validation",
		"invalid",
		"unrecognized",
	} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// hasToolMessages reports whether the prompt carries a "tool" role message.
func hasToolMessages(prompt fantasy.Prompt) bool {
	for _, msg := range prompt {
		if msg.Role == fantasy.MessageRoleTool {
			return true
		}
	}
	return false
}

// sanitizeToolMessages rewrites every "tool" message as a user message with
// the tool results as text. Assistant tool_calls and everything else are
// kept verbatim.
func sanitizeToolMessages(prompt fantasy.Prompt) fantasy.Prompt {
	out := make(fantasy.Prompt, 0, len(prompt))
	for _, msg := range prompt {
		if msg.Role != fantasy.MessageRoleTool {
			out = append(out, msg)
			continue
		}
		out = append(out, fantasy.Message{
			Role:            fantasy.MessageRoleUser,
			Content:         toolResultAsTextPart(msg),
			ProviderOptions: msg.ProviderOptions,
		})
	}
	return out
}

func toolResultAsTextPart(msg fantasy.Message) []fantasy.MessagePart {
	var texts []string
	for _, part := range msg.Content {
		tr, ok := fantasy.AsMessagePart[fantasy.ToolResultPart](part)
		if !ok {
			continue
		}
		switch out := tr.Output.(type) {
		case fantasy.ToolResultOutputContentText:
			texts = append(texts, fmt.Sprintf("Tool call %q result: %s", tr.ToolCallID, out.Text))
		case fantasy.ToolResultOutputContentError:
			texts = append(texts, fmt.Sprintf("Tool call %q failed: %v", tr.ToolCallID, out.Error))
		case fantasy.ToolResultOutputContentMedia:
			texts = append(texts, fmt.Sprintf("Tool call %q result: %s", tr.ToolCallID, strings.TrimSpace(out.Text)))
		}
	}
	if len(texts) == 0 {
		texts = []string{"(empty tool result)"}
	}
	return []fantasy.MessagePart{
		fantasy.TextPart{Text: strings.Join(texts, "\n")},
	}
}

// Generate implements fantasy.LanguageModel.
func (m *toolRoleCompatModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	resp, err := m.LanguageModel.Generate(ctx, call)
	if err == nil || !isToolRoleRejectionErr(err) || !hasToolMessages(call.Prompt) {
		return resp, err
	}
	sanitized := call
	sanitized.Prompt = sanitizeToolMessages(call.Prompt)
	slog.Info("Provider rejected the tool message role, retrying with tool results as user messages",
		"provider", m.Provider(), "model", m.Model())
	return m.LanguageModel.Generate(ctx, sanitized)
}

// Stream implements fantasy.LanguageModel.
//
// The retry happens on the first part: an unprocessable request fails before
// any response part is produced, so nothing has been forwarded to the caller
// by the time we can replay with the rewritten prompt.
func (m *toolRoleCompatModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	inner, err := m.LanguageModel.Stream(ctx, call)
	if err != nil {
		if !isToolRoleRejectionErr(err) || !hasToolMessages(call.Prompt) {
			return nil, err
		}
		sanitized := call
		sanitized.Prompt = sanitizeToolMessages(call.Prompt)
		slog.Info("Provider rejected the tool message role, retrying with tool results as user messages",
			"provider", m.Provider(), "model", m.Model())
		return m.LanguageModel.Stream(ctx, sanitized)
	}
	return func(yield func(fantasy.StreamPart) bool) {
		retried := false
		var advance func(stream fantasy.StreamResponse)
		advance = func(stream fantasy.StreamResponse) {
			stream(func(part fantasy.StreamPart) bool {
				if !retried && part.Type == fantasy.StreamPartTypeError &&
					isToolRoleRejectionErr(part.Error) && hasToolMessages(call.Prompt) {
					retried = true
					sanitized := call
					sanitized.Prompt = sanitizeToolMessages(call.Prompt)
					slog.Info("Provider rejected the tool message role, retrying with tool results as user messages",
						"provider", m.Provider(), "model", m.Model())
					retriedStream, err := m.LanguageModel.Stream(ctx, sanitized)
					if err != nil {
						yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeError, Error: err})
						return false
					}
					advance(retriedStream)
					return true
				}
				return yield(part)
			})
		}
		advance(inner)
	}, nil
}
