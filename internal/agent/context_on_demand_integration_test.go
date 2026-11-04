package agent

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type scriptedStep struct {
	toolCalls []fantasy.StreamPart
	text      string
}

type scriptedModel struct {
	mu       sync.Mutex
	steps    []scriptedStep
	index    int
	requests []fantasy.Call
}

func (m *scriptedModel) Provider() string { return "fake" }
func (m *scriptedModel) Model() string    { return "fake-model" }

func (m *scriptedModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptedModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptedModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptedModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	m.requests = append(m.requests, call)
	step := scriptedStep{text: "done"}
	if m.index < len(m.steps) {
		step = m.steps[m.index]
	}
	m.index++
	m.mu.Unlock()
	return func(yield func(fantasy.StreamPart) bool) {
		if len(step.toolCalls) > 0 {
			for _, part := range step.toolCalls {
				if !yield(part) {
					return
				}
			}
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextStart, ID: "1"}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextDelta, ID: "1", Delta: step.text}) {
			return
		}
		if !yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeTextEnd, ID: "1"}) {
			return
		}
		yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop})
	}, nil
}

func (m *scriptedModel) reset(steps ...scriptedStep) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.steps = steps
	m.index = 0
	m.requests = nil
}

func (m *scriptedModel) captured() []fantasy.Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]fantasy.Call(nil), m.requests...)
}

func toolCallPart(id, name, input string) fantasy.StreamPart {
	return fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: id, ToolCallName: name, ToolCallInput: input}
}

func documentedFixtureTool(t *testing.T, name string, runs *int) fantasy.AgentTool {
	t.Helper()
	return fantasy.NewAgentTool(name, strings.Repeat("Long documentation for the tool. ", 20), func(context.Context, struct {
		Path string `json:"path" description:"File to change"`
	}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		*runs++
		return fantasy.NewTextResponse("changed"), nil
	})
}

func requestToolNames(call fantasy.Call) []string {
	names := make([]string, 0, len(call.Tools))
	for _, tool := range call.Tools {
		names = append(names, tool.GetName())
	}
	return names
}

func requestToolDescription(call fantasy.Call, name string) string {
	for _, tool := range call.Tools {
		if function, ok := tool.(fantasy.FunctionTool); ok && function.Name == name {
			return function.Description
		}
	}
	return ""
}

func requestToolResultTexts(call fantasy.Call) []string {
	var texts []string
	for _, message := range call.Prompt {
		for _, part := range message.Content {
			result, ok := part.(fantasy.ToolResultPart)
			if !ok {
				continue
			}
			if text, ok := result.Output.(fantasy.ToolResultOutputContentText); ok {
				texts = append(texts, text.Text)
			}
			if failure, ok := result.Output.(fantasy.ToolResultOutputContentError); ok && failure.Error != nil {
				texts = append(texts, failure.Error.Error())
			}
		}
	}
	return texts
}

func requestSystemText(call fantasy.Call) string {
	for _, message := range call.Prompt {
		if message.Role != fantasy.MessageRoleSystem {
			continue
		}
		for _, part := range message.Content {
			if text, ok := part.(fantasy.TextPart); ok {
				return text.Text
			}
		}
	}
	return ""
}

func TestToolDocumentationIsOptionalAndRemainsReadable(t *testing.T) {
	env := testEnv(t)
	model := &scriptedModel{}
	runs := 0
	tool := documentedFixtureTool(t, "big_tool", &runs)
	agent := testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system", tool, newReadToolDocumentationTool(), newReadToolResultTool()).(*sessionAgent)
	session, err := env.sessions.Create(t.Context(), "documentation gate")
	require.NoError(t, err)

	model.reset(scriptedStep{toolCalls: []fantasy.StreamPart{toolCallPart("c1", "big_tool", `{"path":"a"}`)}})
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "change a"})
	require.NoError(t, err)
	require.Equal(t, 1, runs, "a valid call must run without reading documentation")

	requests := model.captured()
	require.NotEmpty(t, requests)
	names := requestToolNames(requests[0])
	require.Contains(t, names, "read_tool_documentation")
	require.Contains(t, names, "read_tool_result")
	require.Contains(t, names, "big_tool")
	description := requestToolDescription(requests[0], "big_tool")
	require.NotContains(t, description, "Long documentation for the tool. Long documentation", "the card must be compact")
	require.Contains(t, description, "read_tool_documentation", "the card must point at the full documentation")
	require.Equal(t, "system", requestSystemText(requests[0]), "the system prompt must not change")

	model.reset(
		scriptedStep{toolCalls: []fantasy.StreamPart{toolCallPart("c2", "read_tool_documentation", `{"tool_name":"big_tool"}`)}},
		scriptedStep{toolCalls: []fantasy.StreamPart{toolCallPart("c3", "big_tool", `{"path":"a"}`)}},
	)
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "change a again"})
	require.NoError(t, err)
	require.Equal(t, 2, runs, "reading optional documentation must not prevent execution")

	requests = model.captured()
	require.GreaterOrEqual(t, len(requests), 2)
	documentation := strings.Join(requestToolResultTexts(requests[1]), "\n")
	require.Contains(t, documentation, "Input schema (JSON Schema)")
	require.Contains(t, documentation, "File to change", "the full documentation must include property descriptions")
}

func TestToolDocumentationInSameRequestDoesNotBlockExecution(t *testing.T) {
	env := testEnv(t)
	model := &scriptedModel{}
	runs := 0
	tool := documentedFixtureTool(t, "big_tool", &runs)
	agent := testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system", tool, newReadToolDocumentationTool(), newReadToolResultTool()).(*sessionAgent)
	session, err := env.sessions.Create(t.Context(), "same request")
	require.NoError(t, err)

	model.reset(scriptedStep{toolCalls: []fantasy.StreamPart{
		toolCallPart("c1", "read_tool_documentation", `{"tool_name":"big_tool"}`),
		toolCallPart("c2", "big_tool", `{"path":"a"}`),
	}})
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "read and change"})
	require.NoError(t, err)
	require.Equal(t, 1, runs, "optional documentation must not block a call in the same request")
}

func TestOptionalDocumentationStillRejectsInvalidArguments(t *testing.T) {
	env := testEnv(t)
	model := &scriptedModel{}
	runs := 0
	tool := documentedFixtureTool(t, "big_tool", &runs)
	agent := testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system", tool, newReadToolDocumentationTool()).(*sessionAgent)
	session, err := env.sessions.Create(t.Context(), "invalid arguments")
	require.NoError(t, err)
	model.reset(scriptedStep{toolCalls: []fantasy.StreamPart{toolCallPart("c1", "big_tool", `{"path":42}`)}})
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "change a"})
	require.NoError(t, err)
	require.Zero(t, runs)
	requests := model.captured()
	require.GreaterOrEqual(t, len(requests), 2)
	diagnostics := strings.Join(requestToolResultTexts(requests[1]), "\n")
	require.NotEmpty(t, diagnostics)
	require.NotContains(t, diagnostics, "tool_documentation_required")
}

func TestToolResultProjectionShortensSuccessAndKeepsFailureRecoverable(t *testing.T) {
	env := testEnv(t)
	model := &scriptedModel{}
	large := strings.Repeat("row of output\n", 2000)
	agent := testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system",
		fantasy.NewAgentTool("bulk_success", "Return a very large successful result.", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			response := fantasy.NewTextResponse(large)
			response.Outcome = fantasy.ToolOutcomeSuccess
			return response, nil
		}),
		newReadToolDocumentationTool(), newReadToolResultTool(),
	).(*sessionAgent)
	agent.dataDir = t.TempDir()
	session, err := env.sessions.Create(t.Context(), "projection")
	require.NoError(t, err)

	model.reset(
		scriptedStep{toolCalls: []fantasy.StreamPart{toolCallPart("c1", "bulk_success", `{}`)}},
		scriptedStep{text: "done"},
	)
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "run bulk"})
	require.NoError(t, err)

	requests := model.captured()
	require.Len(t, requests, 2)
	texts := requestToolResultTexts(requests[1])
	require.NotEmpty(t, texts)
	shortened := strings.Join(texts, "\n")
	require.Contains(t, shortened, "shortened")
	require.Contains(t, shortened, "read_tool_result")
	require.Less(t, len(shortened), len(large), "the outgoing result must be smaller than the raw output")

	stored, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	var storedText string
	for _, msg := range stored {
		for _, part := range msg.Parts {
			if result, ok := part.(message.ToolResult); ok {
				storedText = result.Content
			}
		}
	}
	require.Equal(t, len(large), len(storedText), "the complete result must stay stored")
}

func TestToolResultProjectionKeepsFailureDiagnosticsAndStoresThem(t *testing.T) {
	env := testEnv(t)
	model := &scriptedModel{}
	diagnostics := strings.Repeat("fatal: something broke in the build\n", 4000)
	agent := testSessionAgent(env, model, &finishStreamModel{text: "title"}, "system",
		fantasy.NewAgentTool("bulk_failure", "Fail with a very large diagnostic.", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			response := fantasy.NewTextErrorResponse(diagnostics)
			response.Outcome = fantasy.ToolOutcomeFailure
			return response, nil
		}),
		newReadToolDocumentationTool(), newReadToolResultTool(),
	).(*sessionAgent)
	agent.dataDir = t.TempDir()
	session, err := env.sessions.Create(t.Context(), "failure projection")
	require.NoError(t, err)

	model.reset(
		scriptedStep{toolCalls: []fantasy.StreamPart{toolCallPart("c1", "bulk_failure", `{}`)}},
		scriptedStep{text: "done"},
	)
	_, err = agent.Run(t.Context(), SessionAgentCall{SessionID: session.ID, Prompt: "run failure"})
	require.NoError(t, err)

	requests := model.captured()
	require.GreaterOrEqual(t, len(requests), 2)
	shortened := strings.Join(requestToolResultTexts(requests[1]), "\n")
	require.Contains(t, shortened, "failure result shortened")
	require.Contains(t, shortened, "read_tool_result")
	require.Contains(t, shortened, "fatal: something broke in the build", "the first bytes must stay visible")
	require.Less(t, len(shortened), len(diagnostics), "the outgoing failure must be smaller than the raw diagnostics")

	stored, err := env.messages.List(t.Context(), session.ID)
	require.NoError(t, err)
	var storedText string
	var storedIsError bool
	for _, msg := range stored {
		for _, part := range msg.Parts {
			if result, ok := part.(message.ToolResult); ok {
				storedText = result.Content
				storedIsError = result.IsError
			}
		}
	}
	require.Equal(t, len(diagnostics), len(storedText), "the complete diagnostics must stay stored")
	require.True(t, storedIsError, "the stored result must stay a failure")
}
