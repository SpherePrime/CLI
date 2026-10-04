package agent

import (
	"context"
	"io"
	"net/http"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// toolRoleTestModel records every prompt it receives and can fail the first
// attempt with a configurable provider error.
type toolRoleTestModel struct {
	prompts     []fantasy.Prompt
	failOnce    *fantasy.ProviderError
	failedOnce  bool
}

func (m *toolRoleTestModel) takeFailure() bool {
	if m.failOnce == nil || m.failedOnce {
		return false
	}
	m.failedOnce = true
	return true
}

func (m *toolRoleTestModel) Provider() string { return "fake" }
func (m *toolRoleTestModel) Model() string    { return "fake-model" }

func (m *toolRoleTestModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return &fantasy.ObjectResponse{}, nil
}

func (m *toolRoleTestModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}

func (m *toolRoleTestModel) Generate(ctx context.Context, call fantasy.Call) (*fantasy.Response, error) {
	m.prompts = append(m.prompts, call.Prompt)
	if m.takeFailure() {
		return nil, m.failOnce
	}
	return &fantasy.Response{}, nil
}

func (m *toolRoleTestModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.prompts = append(m.prompts, call.Prompt)
	if m.takeFailure() {
		return func(yield func(fantasy.StreamPart) bool) {
			yield(fantasy.StreamPart{
				Type:  fantasy.StreamPartTypeError,
				Error: m.failOnce,
			})
		}, nil
	}
	return func(yield func(fantasy.StreamPart) bool) {
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish})
	}, nil
}

func toolRoleErrorBody() *fantasy.ProviderError {
	return &fantasy.ProviderError{
		StatusCode:   http.StatusUnprocessableEntity,
		Message:      `{"detail":[{"type":"literal_error","loc":["body","messages",4,"role"],"msg":"Input should be 'assistant', 'user' or 'system'","input":"tool"}]}`,
		ResponseBody: []byte(`{"detail":[{"type":"literal_error","loc":["body","messages",4,"role"],"msg":"Input should be 'assistant', 'user' or 'system'","input":"tool"}]}`),
	}
}

func toolPrompt() fantasy.Call {
	return fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewSystemMessage("system"),
			fantasy.NewUserMessage("user"),
			{
				Role:    fantasy.MessageRoleAssistant,
				Content: []fantasy.MessagePart{fantasy.ToolCallPart{ToolCallID: "c1", ToolName: "ls"}},
			},
			{
				Role:    fantasy.MessageRoleTool,
				Content: []fantasy.MessagePart{fantasy.ToolResultPart{ToolCallID: "c1", Output: fantasy.ToolResultOutputContentText{Text: "ok"}}},
			},
		},
	}
}

func TestIsToolRoleRejectionErr(t *testing.T) {
	t.Parallel()

	require.True(t, isToolRoleRejectionErr(toolRoleErrorBody()))
	require.True(t, isToolRoleRejectionErr(&fantasy.ProviderError{
		StatusCode: http.StatusBadRequest,
		Message:    "value of role must be one of: system, user, assistant",
	}))

	require.False(t, isToolRoleRejectionErr(&fantasy.ProviderError{
		StatusCode: http.StatusTooManyRequests,
		Message:    "role",
	}))
	require.False(t, isToolRoleRejectionErr(&fantasy.ProviderError{
		StatusCode: http.StatusUnauthorized,
		ResponseBody: []byte(`{"detail":[{"msg":"Input should be 'assistant', 'user' or 'system'","input":"tool"}]}`),
	}))
	require.False(t, isToolRoleRejectionErr(io.ErrUnexpectedEOF))
	require.False(t, isToolRoleRejectionErr(&fantasy.ProviderError{
		StatusCode:   http.StatusUnprocessableEntity,
		ResponseBody: []byte(`{"detail":[{"msg":"field missing"}]}`),
	}))
}

func TestSanitizeToolMessages(t *testing.T) {
	t.Parallel()

	call := toolPrompt()
	sanitized := sanitizeToolMessages(call.Prompt)

	require.Len(t, sanitized, len(call.Prompt))
	require.Equal(t, fantasy.MessageRoleSystem, sanitized[0].Role)
	require.Equal(t, fantasy.MessageRoleUser, sanitized[1].Role)
	require.Equal(t, fantasy.MessageRoleAssistant, sanitized[2].Role)
	require.Equal(t, fantasy.MessageRoleUser, sanitized[3].Role, "tool message must become a user message")

	textPart, ok := fantasy.AsMessagePart[fantasy.TextPart](sanitized[3].Content[0])
	require.True(t, ok, "sanitized tool message must carry text")
	require.Contains(t, textPart.Text, "c1")
	require.Contains(t, textPart.Text, "ok")
}

func TestToolRoleCompatModel_RetriesOnGenerateRejection(t *testing.T) {
	t.Parallel()

	inner := &toolRoleTestModel{failOnce: toolRoleErrorBody()}
	m := newToolRoleCompatModel(inner)

	_, err := m.Generate(t.Context(), toolPrompt())
	require.NoError(t, err)
	require.Len(t, inner.prompts, 2, "the sanitized prompt must be retried")
	require.False(t, hasToolMessages(inner.prompts[1]), "the retry must not carry tool messages")
}

func TestToolRoleCompatModel_RetriesOnStreamRejection(t *testing.T) {
	t.Parallel()

	inner := &toolRoleTestModel{failOnce: toolRoleErrorBody()}
	m := newToolRoleCompatModel(inner)

	stream, err := m.Stream(t.Context(), toolPrompt())
	require.NoError(t, err)

	var parts []fantasy.StreamPart
	stream(func(part fantasy.StreamPart) bool {
		parts = append(parts, part)
		return true
	})

	require.Len(t, inner.prompts, 2, "the sanitized prompt must be retried")
	require.False(t, hasToolMessages(inner.prompts[1]))
	require.Equal(t, fantasy.StreamPartTypeFinish, parts[len(parts)-1].Type, "the retried stream must finish cleanly")
}

func TestToolRoleCompatModel_PassesThroughOtherErrors(t *testing.T) {
	t.Parallel()

	inner := &toolRoleTestModel{failOnce: &fantasy.ProviderError{
		StatusCode: http.StatusTooManyRequests,
		Message:    "rate limited",
	}}
	m := newToolRoleCompatModel(inner)

	_, err := m.Generate(t.Context(), toolPrompt())
	require.Error(t, err)
	require.Len(t, inner.prompts, 1, "non-role errors must not trigger the rewrite")
}

func TestToolRoleCompatModel_SkipsRetryWithoutToolMessages(t *testing.T) {
	t.Parallel()

	inner := &toolRoleTestModel{failOnce: toolRoleErrorBody()}
	m := newToolRoleCompatModel(inner)

	call := fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewUserMessage("hello"),
		},
	}
	_, err := m.Generate(t.Context(), call)
	require.Error(t, err)
	require.Len(t, inner.prompts, 1, "a prompt without tool messages must pass through")
}

func TestToolRoleCompatModel_SkipsNil(t *testing.T) {
	t.Parallel()

	var m fantasy.LanguageModel
	var result fantasy.LanguageModel = newToolRoleCompatModel(m)
	require.Nil(t, result, "wrapping a nil model must stay nil")
}

func TestToolRoleCompatModel_StreamErrorIsNotSwallowed(t *testing.T) {
	t.Parallel()

	rejection := toolRoleErrorBody()
	inner := &toolRoleTestModel{failOnce: rejection}
	m := newToolRoleCompatModel(inner)

	// No tool messages in the prompt: the rejection must surface as-is.
	stream, err := m.Stream(t.Context(), fantasy.Call{
		Prompt: fantasy.Prompt{fantasy.NewUserMessage("hello")},
	})
	require.NoError(t, err)

	var sawError bool
	stream(func(part fantasy.StreamPart) bool {
		if part.Type == fantasy.StreamPartTypeError && part.Error == any(rejection) {
			sawError = true
		}
		return true
	})
	require.True(t, sawError, "a rejection without tool messages must not be replayed")
}
