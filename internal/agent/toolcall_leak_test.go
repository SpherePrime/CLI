package agent

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// leakTestModel is a [fantasy.LanguageModel] stub whose stream is built from
// the call it receives, so tests can control both the parts and the tool list.
type leakTestModel struct {
	stream func(call fantasy.Call, yield func(fantasy.StreamPart) bool)
}

func (m *leakTestModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{}, nil
}

func (m *leakTestModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	return func(yield func(fantasy.StreamPart) bool) {
		m.stream(call, yield)
	}, nil
}

func (m *leakTestModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return &fantasy.ObjectResponse{}, nil
}

func (m *leakTestModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}

func (m *leakTestModel) Provider() string { return "fake" }
func (m *leakTestModel) Model() string    { return "fake-model" }

// dsmlPrefix is the decoration DeepSeek-family models put in front of every
// tag of a leaked tool call.
const dsmlPrefix = "｜DSML｜ "

func tag(prefix, name string) string    { return "<" + prefix + name + ">" }
func endTag(prefix, name string) string { return "</" + prefix + name + ">" }

// leakedBlock builds the markup a model writes when the endpoint does not turn
// its tool call into a structured tool_calls field. prefix is empty for the
// plain XML shape and dsmlPrefix for the DSML shape.
func leakedBlock(prefix string) string {
	return strings.Join([]string{
		tag(prefix, "tool_calls"),
		tag(prefix, `invoke name="view"`),
		tag(prefix, `parameter name="path" string="true"`) + "internal/agent/agent.go" + endTag(prefix, "parameter"),
		endTag(prefix, "invoke"),
		endTag(prefix, "tool_calls"),
	}, "\n")
}

func chunkParts(text string) []fantasy.StreamPart {
	const chunkSize = 7
	runes := []rune(text)
	var parts []fantasy.StreamPart
	for start := 0; start < len(runes); start += chunkSize {
		end := min(start+chunkSize, len(runes))
		parts = append(parts, fantasy.StreamPart{
			Type:  fantasy.StreamPartTypeTextDelta,
			ID:    "0",
			Delta: string(runes[start:end]),
		})
	}
	return parts
}

// streamLeakText runs text through the wrapper and returns the emitted parts.
func streamLeakText(t *testing.T, text string, tools []fantasy.Tool) []fantasy.StreamPart {
	t.Helper()

	model := &leakTestModel{stream: func(_ fantasy.Call, yield func(fantasy.StreamPart) bool) {
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "0"}) {
			return
		}
		for _, part := range chunkParts(text) {
			if !yield(part) {
				return
			}
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "0"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}}

	stream, err := newToolCallLeakModel(model).Stream(t.Context(), fantasy.Call{Tools: tools})
	require.NoError(t, err)

	var parts []fantasy.StreamPart
	for part := range stream {
		parts = append(parts, part)
	}
	return parts
}

func visibleText(parts []fantasy.StreamPart) string {
	var text strings.Builder
	for _, part := range parts {
		if part.Type == fantasy.StreamPartTypeTextDelta {
			text.WriteString(part.Delta)
		}
	}
	return text.String()
}

func toolCallsOf(parts []fantasy.StreamPart) []fantasy.StreamPart {
	var calls []fantasy.StreamPart
	for _, part := range parts {
		if part.Type == fantasy.StreamPartTypeToolCall {
			calls = append(calls, part)
		}
	}
	return calls
}

func finishReasonOf(parts []fantasy.StreamPart) fantasy.FinishReason {
	reason := fantasy.FinishReasonUnknown
	for _, part := range parts {
		if part.Type == fantasy.StreamPartTypeFinish {
			reason = part.FinishReason
		}
	}
	return reason
}

func viewTool() []fantasy.Tool {
	return []fantasy.Tool{fantasy.FunctionTool{Name: "view"}}
}

func TestToolCallLeakModel_ConvertsLeakedMarkupToToolCalls(t *testing.T) {
	t.Parallel()

	parts := streamLeakText(t, "Готово.\n\n"+leakedBlock(dsmlPrefix), viewTool())

	require.Equal(t, "Готово.\n\n", visibleText(parts), "the leaked markup must not reach the chat")
	calls := toolCallsOf(parts)
	require.Len(t, calls, 1)
	require.Equal(t, "view", calls[0].ToolCallName)
	require.JSONEq(t, `{"path":"internal/agent/agent.go"}`, calls[0].ToolCallInput)
	require.Equal(t, fantasy.FinishReasonToolCalls, finishReasonOf(parts),
		"the turn must continue instead of ending as a finished answer")
}

func TestToolCallLeakModel_ConvertsPlainXMLMarkup(t *testing.T) {
	t.Parallel()

	parts := streamLeakText(t, leakedBlock(""), viewTool())

	require.Empty(t, visibleText(parts))
	require.Len(t, toolCallsOf(parts), 1)
	require.Equal(t, fantasy.FinishReasonToolCalls, finishReasonOf(parts))
}

func TestToolCallLeakModel_KeepsOrdinaryText(t *testing.T) {
	t.Parallel()

	const answer = "Обычный ответ без вызовов инструментов.\n\nВторой абзац."
	parts := streamLeakText(t, answer, viewTool())

	require.Equal(t, answer, visibleText(parts))
	require.Empty(t, toolCallsOf(parts))
	require.Equal(t, fantasy.FinishReasonStop, finishReasonOf(parts))
}

func TestToolCallLeakModel_KeepsMarkupQuotedInCodeFence(t *testing.T) {
	t.Parallel()

	answer := "Пример формата:\n\n```\n" + leakedBlock(dsmlPrefix) + "\n```\n"
	parts := streamLeakText(t, answer, viewTool())

	require.Equal(t, answer, visibleText(parts), "a quoted example must stay visible")
	require.Empty(t, toolCallsOf(parts))
	require.Equal(t, fantasy.FinishReasonStop, finishReasonOf(parts))
}

func TestToolCallLeakModel_KeepsMarkupForUnknownTool(t *testing.T) {
	t.Parallel()

	tools := []fantasy.Tool{fantasy.FunctionTool{Name: "bash"}}
	parts := streamLeakText(t, leakedBlock(dsmlPrefix), tools)

	require.Contains(t, visibleText(parts), "tool_calls")
	require.Empty(t, toolCallsOf(parts))
	require.Equal(t, fantasy.FinishReasonStop, finishReasonOf(parts))
}

func TestToolCallLeakModel_RecoversBlockWithoutClosingTag(t *testing.T) {
	t.Parallel()

	block := leakedBlock(dsmlPrefix)
	truncated := strings.TrimSuffix(block, endTag(dsmlPrefix, "tool_calls"))
	parts := streamLeakText(t, truncated, viewTool())

	calls := toolCallsOf(parts)
	require.Len(t, calls, 1)
	require.Equal(t, "view", calls[0].ToolCallName)
	require.Equal(t, fantasy.FinishReasonToolCalls, finishReasonOf(parts))
}

func TestParseLeakedToolCalls_DSMLParameters(t *testing.T) {
	t.Parallel()

	calls := parseLeakedToolCalls(leakedBlock(dsmlPrefix))
	require.Len(t, calls, 1)
	require.Equal(t, "view", calls[0].name)
	require.JSONEq(t, `{"path":"internal/agent/agent.go"}`, calls[0].input)
}

func TestParseLeakedToolCalls_DecodesRawJSONParameter(t *testing.T) {
	t.Parallel()

	block := strings.Join([]string{
		tag("", "tool_calls"),
		tag("", `invoke name="multiedit"`),
		tag("", `parameter name="edits" string="false"`) + `[{"new_string":"a","old_string":"b"}]` + endTag("", "parameter"),
		tag("", `parameter name="file_path" string="true"`) + "main.go" + endTag("", "parameter"),
		endTag("", "invoke"),
		endTag("", "tool_calls"),
	}, "\n")

	calls := parseLeakedToolCalls(block)
	require.Len(t, calls, 1)
	require.Equal(t, "multiedit", calls[0].name)
	require.JSONEq(t, `{"edits":[{"new_string":"a","old_string":"b"}],"file_path":"main.go"}`, calls[0].input)
}

func TestParseLeakedToolCalls_IgnoresIncompleteInvoke(t *testing.T) {
	t.Parallel()

	block := tag(dsmlPrefix, "tool_calls") + "\n" +
		tag(dsmlPrefix, `invoke name="view"`) + "\n" +
		tag(dsmlPrefix, `parameter name="path" string="true"`) + "main.go"

	require.Empty(t, parseLeakedToolCalls(block))
}

func TestToolCallLeakModel_AgentRunsRecoveredCallAndContinues(t *testing.T) {
	t.Parallel()

	var ran atomic.Bool
	tool := fantasy.NewAgentTool("view", "read a file", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		ran.Store(true)
		return fantasy.NewTextResponse("file contents"), nil
	})

	var steps atomic.Int64
	model := &leakTestModel{stream: func(_ fantasy.Call, yield func(fantasy.StreamPart) bool) {
		if steps.Add(1) > 1 {
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "0"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "0", Delta: "Готово."})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "0"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "0"}) {
			return
		}
		for _, part := range chunkParts("Читаю файл.\n\n" + leakedBlock(dsmlPrefix)) {
			if !yield(part) {
				return
			}
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "0"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}}

	runner := fantasy.NewAgent(newToolCallLeakModel(model), fantasy.WithTools(tool))
	result, err := runner.Stream(t.Context(), fantasy.AgentStreamCall{Prompt: "прочитай файл"})
	require.NoError(t, err)

	require.True(t, ran.Load(), "the recovered tool call must run")
	require.EqualValues(t, 2, steps.Load(), "the turn must continue after the leaked call")

	var firstStep strings.Builder
	for _, content := range result.Steps[0].Content {
		if text, ok := fantasy.AsContentType[fantasy.TextContent](content); ok {
			firstStep.WriteString(text.Text)
		}
	}
	require.Equal(t, "Читаю файл.\n\n", firstStep.String(), "the markup must not be stored as assistant text")
}
