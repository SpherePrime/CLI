package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/skills"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/anthropic"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/google"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openai"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openaicompat"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type recordedStreamCall struct {
	call fantasy.Call
}

type scriptedOrchestratorModel struct {
	provider string
	modelID  string
	mu       sync.Mutex
	calls    []recordedStreamCall
	steps    []func(ctx context.Context, call fantasy.Call) ([]fantasy.StreamPart, error)
}

func (m *scriptedOrchestratorModel) Provider() string { return m.provider }
func (m *scriptedOrchestratorModel) Model() string    { return m.modelID }

func (m *scriptedOrchestratorModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptedOrchestratorModel) Stream(ctx context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	idx := len(m.calls)
	m.calls = append(m.calls, recordedStreamCall{call: call})
	var stepFn func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error)
	if idx < len(m.steps) {
		stepFn = m.steps[idx]
	}
	m.mu.Unlock()

	if stepFn == nil {
		return nil, fmt.Errorf("%s/%s: unexpected stream call #%d", m.provider, m.modelID, idx)
	}
	parts, err := stepFn(ctx, call)
	if err != nil {
		return nil, err
	}
	return func(yield func(fantasy.StreamPart) bool) {
		for _, p := range parts {
			if !yield(p) {
				return
			}
		}
	}, nil
}

func (m *scriptedOrchestratorModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptedOrchestratorModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, errors.New("not implemented")
}

func (m *scriptedOrchestratorModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *scriptedOrchestratorModel) callAt(i int) fantasy.Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[i].call
}

func toolNamesInCall(call fantasy.Call) []string {
	names := make([]string, 0, len(call.Tools))
	for _, t := range call.Tools {
		if ft, ok := t.(fantasy.FunctionTool); ok {
			names = append(names, ft.Name)
		}
	}
	return names
}

func systemPromptTextInCall(call fantasy.Call) string {
	var parts []string
	for _, msg := range call.Prompt {
		if msg.Role != fantasy.MessageRoleSystem {
			continue
		}
		for _, part := range msg.Content {
			if tp, ok := fantasy.AsMessagePart[fantasy.TextPart](part); ok {
				parts = append(parts, tp.Text)
			}
		}
	}
	return fmt.Sprintf("%v", parts)
}

func textStopParts(text string, usage fantasy.Usage) []fantasy.StreamPart {
	return []fantasy.StreamPart{
		{Type: fantasy.StreamPartTypeTextStart, ID: "t1"},
		{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: text},
		{Type: fantasy.StreamPartTypeTextEnd, ID: "t1"},
		{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop, Usage: usage},
	}
}

func toolCallParts(callID, toolName, input string, usage fantasy.Usage) []fantasy.StreamPart {
	return []fantasy.StreamPart{
		{Type: fantasy.StreamPartTypeToolCall, ID: callID, ToolCallName: toolName, ToolCallInput: input},
		{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls, Usage: usage},
	}
}

func makeOrchestratorModelsConfig(planner, executor *scriptedOrchestratorModel, plannerCatwalk, executorCatwalk catwalk.Model, finalReview bool) *OrchestratorCallConfig {
	return &OrchestratorCallConfig{
		Enabled:            true,
		FinalReviewEnabled: finalReview,
		Planner: OrchestratorRoleConfig{
			Model: Model{
				Model:      planner,
				CatwalkCfg: plannerCatwalk,
				ModelCfg: config.SelectedModel{
					Provider: planner.provider,
					Model:    planner.modelID,
				},
			},
		},
		Executor: OrchestratorRoleConfig{
			Model: Model{
				Model:      executor,
				CatwalkCfg: executorCatwalk,
				ModelCfg: config.SelectedModel{
					Provider: executor.provider,
					Model:    executor.modelID,
				},
			},
		},
	}
}

func defaultTestCatwalkModel(id string) catwalk.Model {
	return catwalk.Model{
		ID:               id,
		Name:             id,
		ContextWindow:    200000,
		DefaultMaxTokens: 8192,
	}
}

func TestOrchestrator_ReadOnlyQAShortcut(t *testing.T) {
	env := testEnv(t)
	planner := &scriptedOrchestratorModel{
		provider: "anthropic",
		modelID:  "planner-claude",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				names := toolNamesInCall(call)
				require.Contains(t, names, "view")
				require.Contains(t, names, HandoffToExecutorToolName)
				require.NotContains(t, names, "write")
				require.NotContains(t, names, "edit")
				require.NotContains(t, names, "bash")
				require.NotContains(t, names, EscalateToPlannerToolName)
				return textStopParts("This function adds two integers.", fantasy.Usage{InputTokens: 100, OutputTokens: 20, TotalTokens: 120}), nil
			},
		},
	}
	executor := &scriptedOrchestratorModel{
		provider: "openai",
		modelID:  "executor-gpt",
	}

	viewTool := fantasy.NewAgentTool("view", "read file", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})
	writeTool := fantasy.NewAgentTool("write", "write file", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("written"), nil
	})
	bashTool := fantasy.NewAgentTool("bash", "run bash", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ran"), nil
	})

	sa := testSessionAgent(env, planner, planner, "base system", viewTool, writeTool, bashTool)
	sess, err := env.sessions.Create(t.Context(), "qa-shortcut")
	require.NoError(t, err)

	orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-claude"), defaultTestCatwalkModel("executor-gpt"), true)
	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID:    sess.ID,
		Prompt:       "Explain what Add(a, b) does",
		Orchestrator: orch,
	})
	require.NoError(t, err)
	require.Equal(t, "This function adds two integers.", res.Response.Content.Text())
	require.Equal(t, 1, planner.callCount())
	require.Equal(t, 0, executor.callCount())

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var assistantMsgs []message.Message
	for _, m := range msgs {
		if m.Role == message.Assistant {
			assistantMsgs = append(assistantMsgs, m)
		}
	}
	require.Len(t, assistantMsgs, 1)
	require.Equal(t, "plan", assistantMsgs[0].OrchestratorPhase())
	require.Equal(t, "planner-claude", assistantMsgs[0].Model)
}

func TestOrchestrator_PlanToExecuteHandoffAndToolGating(t *testing.T) {
	env := testEnv(t)
	var writeCalled atomic.Bool

	planner := &scriptedOrchestratorModel{
		provider: "anthropic",
		modelID:  "planner-model",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				names := toolNamesInCall(call)
				require.Contains(t, names, "view")
				require.Contains(t, names, HandoffToExecutorToolName)
				require.NotContains(t, names, "write")
				require.NotContains(t, names, "bash")
				payload, _ := json.Marshal(handoffToExecutorParams{
					Plan:               "Write greeting to hello.txt",
					TargetFiles:        []string{"hello.txt"},
					VerificationChecks: []string{"check file content"},
					EscalationTriggers: []string{"permission denied"},
				})
				return toolCallParts("handoff-1", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 50, OutputTokens: 30, TotalTokens: 80}), nil
			},
		},
	}
	executor := &scriptedOrchestratorModel{
		provider: "openai",
		modelID:  "executor-model",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				names := toolNamesInCall(call)
				require.Contains(t, names, "write")
				require.Contains(t, names, "view")
				require.Contains(t, names, EscalateToPlannerToolName)
				require.NotContains(t, names, HandoffToExecutorToolName)
				sys := systemPromptTextInCall(call)
				require.Contains(t, sys, "<orchestrator_plan>")
				require.Contains(t, sys, "Write greeting to hello.txt")
				return toolCallParts("write-1", "write", `{}`, fantasy.Usage{InputTokens: 60, OutputTokens: 20, TotalTokens: 80}), nil
			},
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return textStopParts("Updated hello.txt.", fantasy.Usage{InputTokens: 70, OutputTokens: 15, TotalTokens: 85}), nil
			},
		},
	}

	viewTool := fantasy.NewAgentTool("view", "read", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("read"), nil
	})
	writeTool := fantasy.NewAgentTool("write", "write", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		writeCalled.Store(true)
		return fantasy.NewTextResponse("wrote hello.txt"), nil
	})
	bashTool := fantasy.NewAgentTool("bash", "bash", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})

	sa := testSessionAgent(env, planner, executor, "base system", viewTool, writeTool, bashTool)
	sess, err := env.sessions.Create(t.Context(), "handoff")
	require.NoError(t, err)

	orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-model"), defaultTestCatwalkModel("executor-model"), false)
	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID:    sess.ID,
		Prompt:       "Create hello.txt",
		Orchestrator: orch,
	})
	require.NoError(t, err)
	require.True(t, writeCalled.Load())
	require.Equal(t, "Updated hello.txt.", res.Response.Content.Text())
	require.Equal(t, 1, planner.callCount())
	require.Equal(t, 2, executor.callCount())

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var phases []string
	for _, m := range msgs {
		if m.Role == message.Assistant {
			phases = append(phases, m.OrchestratorPhase())
		}
	}
	require.Equal(t, []string{"plan", "execute", "execute"}, phases)
}

func TestOrchestrator_TwoConsecutiveToolFailuresEscalateToPlanner(t *testing.T) {
	env := testEnv(t)
	var bashCalls atomic.Int32

	planner := &scriptedOrchestratorModel{
		provider: "anthropic",
		modelID:  "planner-model",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Run build"})
				return toolCallParts("h1", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 20, OutputTokens: 10}), nil
			},
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				names := toolNamesInCall(call)
				require.NotContains(t, names, "bash")
				require.Contains(t, names, HandoffToExecutorToolName)
				sys := systemPromptTextInCall(call)
				require.Contains(t, sys, "<orchestrator_escalation>")
				require.Contains(t, sys, "2 consecutive tool failures")
				return textStopParts("Escalation handled: build needs missing dependency.", fantasy.Usage{InputTokens: 30, OutputTokens: 15}), nil
			},
		},
	}
	executor := &scriptedOrchestratorModel{
		provider: "openai",
		modelID:  "executor-model",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return toolCallParts("b1", "bash", `{}`, fantasy.Usage{InputTokens: 20, OutputTokens: 10}), nil
			},
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return toolCallParts("b2", "bash", `{}`, fantasy.Usage{InputTokens: 20, OutputTokens: 10}), nil
			},
		},
	}

	bashTool := fantasy.NewAgentTool("bash", "run command", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		n := bashCalls.Add(1)
		return fantasy.NewTextErrorResponse(fmt.Sprintf("command failed #%d", n)), nil
	})
	viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})

	sa := testSessionAgent(env, planner, executor, "base system", bashTool, viewTool)
	sess, err := env.sessions.Create(t.Context(), "two-failures")
	require.NoError(t, err)

	orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-model"), defaultTestCatwalkModel("executor-model"), true)
	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID:    sess.ID,
		Prompt:       "Run the build",
		Orchestrator: orch,
	})
	require.NoError(t, err)
	require.Equal(t, "Escalation handled: build needs missing dependency.", res.Response.Content.Text())
	require.Equal(t, 2, planner.callCount())
	require.Equal(t, 2, executor.callCount())
	require.EqualValues(t, 2, bashCalls.Load())
}

func TestOrchestrator_MidTurnSteeringSwitchesBackToPlanner(t *testing.T) {
	env := testEnv(t)
	entered := make(chan struct{})
	release := make(chan struct{})

	planner := &scriptedOrchestratorModel{
		provider: "anthropic",
		modelID:  "planner-model",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Run slow step"})
				return toolCallParts("h1", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				names := toolNamesInCall(call)
				require.NotContains(t, names, "bash")
				require.Contains(t, names, HandoffToExecutorToolName)
				sys := systemPromptTextInCall(call)
				require.Contains(t, sys, "steering")
				rawPrompt, _ := json.Marshal(call.Prompt)
				require.Contains(t, string(rawPrompt), "actually switch to SQLite")
				return textStopParts("Replanned around SQLite.", fantasy.Usage{InputTokens: 20, OutputTokens: 10}), nil
			},
		},
	}
	executor := &scriptedOrchestratorModel{
		provider: "openai",
		modelID:  "executor-model",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return toolCallParts("wait-1", "bash", `{}`, fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
		},
	}

	slowBashTool := fantasy.NewAgentTool("bash", "slow command", func(ctx context.Context, _ struct{}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
		close(entered)
		select {
		case <-release:
			return fantasy.NewTextResponse("step done"), nil
		case <-ctx.Done():
			return fantasy.ToolResponse{}, ctx.Err()
		}
	})
	viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})

	sa := testSessionAgent(env, planner, executor, "base system", slowBashTool, viewTool).(*sessionAgent)
	sess, err := env.sessions.Create(t.Context(), "steering")
	require.NoError(t, err)

	orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-model"), defaultTestCatwalkModel("executor-model"), false)
	doneCh := make(chan *fantasy.AgentResult, 1)
	errCh := make(chan error, 1)
	go func() {
		r, runErr := sa.Run(t.Context(), SessionAgentCall{
			SessionID:    sess.ID,
			Prompt:       "Do the task",
			Orchestrator: orch,
		})
		doneCh <- r
		errCh <- runErr
	}()

	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("executor tool never started")
	}

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "actually switch to SQLite",
	})
	require.NoError(t, err)
	entries := sa.QueueEntries(sess.ID)
	require.Len(t, entries, 1)
	steerDone, err := sa.RunQueuedNow(t.Context(), sess.ID, entries[0].ID)
	require.NoError(t, err)

	close(release)
	require.NoError(t, <-errCh)
	res := <-doneCh
	<-steerDone

	require.Equal(t, "Replanned around SQLite.", res.Response.Content.Text())
	require.Equal(t, 2, planner.callCount())
	require.Equal(t, 1, executor.callCount())
}

func TestOrchestrator_FinalReviewTriggeredAndCapped(t *testing.T) {
	env := testEnv(t)

	planner := &scriptedOrchestratorModel{
		provider: "anthropic",
		modelID:  "planner-model",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			// Step 0: Initial Plan -> handoff
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Edit main.go"})
				return toolCallParts("h1", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
			// Review Round 1: request a fix via handoff_to_executor
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				names := toolNamesInCall(call)
				require.NotContains(t, names, "write")
				require.Contains(t, names, HandoffToExecutorToolName)
				sys := systemPromptTextInCall(call)
				require.Contains(t, sys, "REVIEW phase")
				payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Add missing error check"})
				return toolCallParts("h2", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
			// Review Round 2: still hands off -> cap prevents round 3
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				sys := systemPromptTextInCall(call)
				require.Contains(t, sys, "REVIEW phase")
				payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Polish comment"})
				return toolCallParts("h3", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
		},
	}
	executor := &scriptedOrchestratorModel{
		provider: "openai",
		modelID:  "executor-model",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			// Execute 1: write + stop -> triggers Review Round 1
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return toolCallParts("w1", "write", `{}`, fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return textStopParts("First edit complete.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
			// Execute 2: write + stop -> triggers Review Round 2
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return toolCallParts("w2", "write", `{}`, fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return textStopParts("Second edit complete.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
			// Execute 3: stop -> reviewRounds == 2, so turn stops cleanly without Review Round 3
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return textStopParts("Polished and complete.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
		},
	}

	writeTool := fantasy.NewAgentTool("write", "write", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})
	viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})

	sa := testSessionAgent(env, planner, executor, "base system", writeTool, viewTool)
	sess, err := env.sessions.Create(t.Context(), "review-cap")
	require.NoError(t, err)

	orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-model"), defaultTestCatwalkModel("executor-model"), true)
	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID:    sess.ID,
		Prompt:       "Implement feature",
		Orchestrator: orch,
	})
	require.NoError(t, err)
	require.Equal(t, "Polished and complete.", res.Response.Content.Text())
	require.Equal(t, 3, planner.callCount())
	require.Equal(t, 5, executor.callCount())

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	var reviewCount int
	for _, m := range msgs {
		if m.Role == message.Assistant && m.OrchestratorPhase() == "review" {
			reviewCount++
		}
	}
	require.Equal(t, 2, reviewCount)
}

func TestOrchestrator_CrossModelReasoningAndSignatureStripping(t *testing.T) {
	env := testEnv(t)

	planner := &scriptedOrchestratorModel{
		provider: "google",
		modelID:  "gemini-planner",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Apply edit"})
				return []fantasy.StreamPart{
					{
						Type:  fantasy.StreamPartTypeReasoningStart,
						ID:    "r1",
						Delta: "secret gemini reasoning",
						ProviderMetadata: fantasy.ProviderMetadata{
							google.Name: &google.ReasoningMetadata{Signature: "thought-sig-xyz", ToolID: "h1"},
						},
					},
					{
						Type: fantasy.StreamPartTypeReasoningEnd,
						ID:   "r1",
						ProviderMetadata: fantasy.ProviderMetadata{
							google.Name:    &google.ReasoningMetadata{Signature: "thought-sig-xyz", ToolID: "h1"},
							anthropic.Name: &anthropic.ReasoningOptionMetadata{Signature: "anth-sig"},
							openai.Name:    &openai.ResponsesReasoningMetadata{ItemID: "rs_123"},
						},
					},
					{Type: fantasy.StreamPartTypeTextStart, ID: "t1"},
					{
						Type:             fantasy.StreamPartTypeTextDelta,
						ID:               "t1",
						Delta:            "Handing off.",
						ProviderMetadata: fantasy.ProviderMetadata{google.Name: &google.ReasoningMetadata{Signature: "text-sig"}},
					},
					{
						Type:             fantasy.StreamPartTypeTextEnd,
						ID:               "t1",
						ProviderMetadata: fantasy.ProviderMetadata{google.Name: &google.ReasoningMetadata{Signature: "text-sig"}},
					},
					{
						Type:             fantasy.StreamPartTypeToolCall,
						ID:               "h1",
						ToolCallName:     HandoffToExecutorToolName,
						ToolCallInput:    string(payload),
						ProviderMetadata: fantasy.ProviderMetadata{google.Name: &google.ReasoningMetadata{Signature: "tc-sig"}},
					},
					{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls},
				}, nil
			},
		},
	}

	var inspectedCall fantasy.Call
	executor := &scriptedOrchestratorModel{
		provider: "anthropic",
		modelID:  "claude-executor",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				inspectedCall = call
				return textStopParts("Done.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
		},
	}

	viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})
	sa := testSessionAgent(env, planner, executor, "base system", viewTool)
	sess, err := env.sessions.Create(t.Context(), "cross-model-strip")
	require.NoError(t, err)

	orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("gemini-planner"), defaultTestCatwalkModel("claude-executor"), false)
	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID:    sess.ID,
		Prompt:       "Please edit",
		Orchestrator: orch,
	})
	require.NoError(t, err)

	var foundAssistant bool
	var foundToolResult bool
	for _, msg := range inspectedCall.Prompt {
		if msg.Role == fantasy.MessageRoleAssistant {
			foundAssistant = true
			for _, part := range msg.Content {
				_, isReasoning := part.(fantasy.ReasoningPart)
				require.False(t, isReasoning, "ReasoningPart from different model must be stripped")
				switch p := part.(type) {
				case fantasy.TextPart:
					require.Nil(t, p.ProviderOptions, "TextPart ProviderOptions must be stripped")
					require.Equal(t, "Handing off.", p.Text)
				case fantasy.ToolCallPart:
					require.Nil(t, p.ProviderOptions, "ToolCallPart ProviderOptions (e.g. ThoughtSignature) must be stripped")
					require.Equal(t, HandoffToExecutorToolName, p.ToolName)
				}
			}
		}
		if msg.Role == fantasy.MessageRoleTool {
			foundToolResult = true
		}
	}
	require.True(t, foundAssistant)
	require.True(t, foundToolResult)
}

func TestOrchestrator_PerStepUsageCostAndMetadataAttribution(t *testing.T) {
	env := testEnv(t)

	planner := &scriptedOrchestratorModel{
		provider: "anthropic",
		modelID:  "expensive-planner",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			// Plan step: 1M in ($10) + 100k out ($3) = $13.00
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Write file"})
				return toolCallParts("h1", HandoffToExecutorToolName, string(payload), fantasy.Usage{
					InputTokens:  1_000_000,
					OutputTokens: 100_000,
					TotalTokens:  1_100_000,
				}), nil
			},
			// Review step: 500k in ($5) + 50k out ($1.50) = $6.50
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return textStopParts("Verified.", fantasy.Usage{
					InputTokens:  500_000,
					OutputTokens: 50_000,
					TotalTokens:  550_000,
				}), nil
			},
		},
	}
	executor := &scriptedOrchestratorModel{
		provider: "openai",
		modelID:  "cheap-executor",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			// Execute step 1: 1M in ($1) + 100k out ($0.20) = $1.20
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return toolCallParts("w1", "write", `{}`, fantasy.Usage{
					InputTokens:  1_000_000,
					OutputTokens: 100_000,
					TotalTokens:  1_100_000,
				}), nil
			},
			// Execute step 2: 1M in ($1) + 100k out ($0.20) = $1.20
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return textStopParts("Wrote file.", fantasy.Usage{
					InputTokens:  1_000_000,
					OutputTokens: 100_000,
					TotalTokens:  1_100_000,
				}), nil
			},
		},
	}

	writeTool := fantasy.NewAgentTool("write", "write", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})
	viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("ok"), nil
	})

	plannerCatwalk := catwalk.Model{
		ID:               "expensive-planner",
		Name:             "Expensive Planner",
		ContextWindow:    10_000_000,
		DefaultMaxTokens: 8192,
		CostPer1MIn:      10.0,
		CostPer1MOut:     30.0,
	}
	executorCatwalk := catwalk.Model{
		ID:               "cheap-executor",
		Name:             "Cheap Executor",
		ContextWindow:    10_000_000,
		DefaultMaxTokens: 8192,
		CostPer1MIn:      1.0,
		CostPer1MOut:     2.0,
	}

	sa := testSessionAgent(env, planner, executor, "base system", writeTool, viewTool)
	sess, err := env.sessions.Create(t.Context(), "cost-attribution")
	require.NoError(t, err)

	orch := makeOrchestratorModelsConfig(planner, executor, plannerCatwalk, executorCatwalk, true)
	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID:    sess.ID,
		Prompt:       "Write and verify",
		Orchestrator: orch,
	})
	require.NoError(t, err)
	require.Equal(t, "Verified.", res.Response.Content.Text())

	updatedSess, err := env.sessions.Get(t.Context(), sess.ID)
	require.NoError(t, err)
	// Expected total cost: 13.00 (plan) + 1.20 (exec 1) + 1.20 (exec 2) + 6.50 (review) = 21.90
	require.InDelta(t, 21.90, updatedSess.Cost, 0.001)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)
	type msgMeta struct {
		provider string
		model    string
		phase    string
	}
	var got []msgMeta
	for _, m := range msgs {
		if m.Role == message.Assistant {
			got = append(got, msgMeta{
				provider: m.Provider,
				model:    m.Model,
				phase:    m.OrchestratorPhase(),
			})
			// Non-LLM OrchestratorStepMeta must not emit parts into ToAIMessage.
			aiMsgs := m.ToAIMessage()
			require.Len(t, aiMsgs, 1)
			for _, part := range aiMsgs[0].Content {
				switch part.(type) {
				case fantasy.TextPart, fantasy.ReasoningPart, fantasy.ToolCallPart:
				default:
					t.Fatalf("unexpected part type in ToAIMessage: %T", part)
				}
			}
		}
	}
	require.Equal(t, []msgMeta{
		{provider: "anthropic", model: "expensive-planner", phase: "plan"},
		{provider: "openai", model: "cheap-executor", phase: "execute"},
		{provider: "openai", model: "cheap-executor", phase: "execute"},
		{provider: "anthropic", model: "expensive-planner", phase: "review"},
	}, got)
}

func TestOrchestrator_SubagentToolsExcludedAndBlocked(t *testing.T) {
	env := testEnv(t)
	var writeCalled atomic.Bool
	var bashCalled atomic.Bool

	planner := &scriptedOrchestratorModel{
		provider: "anthropic",
		modelID:  "planner-model",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				names := toolNamesInCall(call)
				require.NotContains(t, names, AgentToolName)
				require.NotContains(t, names, AgentJobsToolName)
				require.NotContains(t, names, AgentMessageToolName)
				require.Contains(t, names, HandoffToExecutorToolName)
				payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Implement directly"})
				return toolCallParts("h1", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				names := toolNamesInCall(call)
				require.NotContains(t, names, AgentToolName)
				require.NotContains(t, names, AgentJobsToolName)
				require.NotContains(t, names, AgentMessageToolName)
				return textStopParts("Verified.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
		},
	}
	executor := &scriptedOrchestratorModel{
		provider: "openai",
		modelID:  "executor-model",
		steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				names := toolNamesInCall(call)
				require.NotContains(t, names, AgentToolName)
				require.NotContains(t, names, AgentJobsToolName)
				require.NotContains(t, names, AgentMessageToolName)
				require.Contains(t, names, "write")
				require.Contains(t, names, "bash")
				sys := systemPromptTextInCall(call)
				require.Contains(t, sys, "overrides any coordinator or subagent-delegation instructions")
				return toolCallParts("w1", "write", `{}`, fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return toolCallParts("b1", "bash", `{}`, fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
			func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
				return textStopParts("Done directly.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
			},
		},
	}

	agentTool := fantasy.NewAgentTool(AgentToolName, "spawn", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("should not run"), nil
	})
	agentJobsTool := fantasy.NewAgentTool(AgentJobsToolName, "jobs", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("should not run"), nil
	})
	agentMessageTool := fantasy.NewAgentTool(AgentMessageToolName, "msg", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("should not run"), nil
	})
	writeTool := fantasy.NewAgentTool("write", "write", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		writeCalled.Store(true)
		return fantasy.NewTextResponse("wrote"), nil
	})
	bashTool := fantasy.NewAgentTool("bash", "bash", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		bashCalled.Store(true)
		return fantasy.NewTextResponse("ran"), nil
	})
	viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
		return fantasy.NewTextResponse("viewed"), nil
	})

	orchCfg := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-model"), defaultTestCatwalkModel("executor-model"), true)
	rt := newOrchestratorRuntime(*orchCfg)
	gate := rt.beforeToolCallGate()
	for _, blocked := range []string{AgentToolName, AgentJobsToolName, AgentMessageToolName} {
		err := gate(t.Context(), fantasy.ToolCall{Name: blocked})
		require.Error(t, err)
		require.Contains(t, err.Error(), "disabled in orchestrator mode; execute directly instead of spawning subagents")
	}
	rt.mu.Lock()
	rt.phase = OrchestratorPhaseExecute
	rt.mu.Unlock()
	for _, blocked := range []string{AgentToolName, AgentJobsToolName, AgentMessageToolName} {
		err := gate(t.Context(), fantasy.ToolCall{Name: blocked})
		require.Error(t, err)
		require.Contains(t, err.Error(), "disabled in orchestrator mode; execute directly instead of spawning subagents")
	}
	require.NoError(t, gate(t.Context(), fantasy.ToolCall{Name: "write"}))
	require.NoError(t, gate(t.Context(), fantasy.ToolCall{Name: "bash"}))

	sa := testSessionAgent(env, planner, executor, "base system", agentTool, agentJobsTool, agentMessageTool, writeTool, bashTool, viewTool)
	sess, err := env.sessions.Create(t.Context(), "subagent-blocked")
	require.NoError(t, err)

	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID:    sess.ID,
		Prompt:       "Implement and test",
		Orchestrator: orchCfg,
	})
	require.NoError(t, err)
	require.Equal(t, "Verified.", res.Response.Content.Text())
	require.True(t, writeCalled.Load())
	require.True(t, bashCalled.Load())
}

func TestOrchestrator_ProgressCheckInTriggersAndCapsAtThree(t *testing.T) {
	t.Run("wraps up directly on progress check-in", func(t *testing.T) {
		env := testEnv(t)
		var reviewNoteSeen atomic.Bool

		planner := &scriptedOrchestratorModel{
			provider: "anthropic",
			modelID:  "planner-model",
			steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Run long batch"})
					return toolCallParts("h1", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					sys := systemPromptTextInCall(call)
					require.Contains(t, sys, "REVIEW phase")
					require.Contains(t, sys, "<orchestrator_review_note>")
					require.Contains(t, sys, "Progress check-in (1/3)")
					reviewNoteSeen.Store(true)
					return textStopParts("Progress looks complete, wrapping up.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
			},
		}

		execSteps := make([]func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error), 0, maxOrchestratorStepsBeforeCheckIn)
		for i := range maxOrchestratorStepsBeforeCheckIn {
			idx := i
			execSteps = append(execSteps, func(_ context.Context, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				return toolCallParts(fmt.Sprintf("w-%d", idx), "write", fmt.Sprintf(`{"step":%d}`, idx), fantasy.Usage{InputTokens: 5, OutputTokens: 5}), nil
			})
		}
		executor := &scriptedOrchestratorModel{
			provider: "openai",
			modelID:  "executor-model",
			steps:    execSteps,
		}

		writeTool := fantasy.NewAgentTool("write", "write", func(context.Context, struct{ Step int }, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})
		viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})

		sa := testSessionAgent(env, planner, executor, "base system", writeTool, viewTool)
		sess, err := env.sessions.Create(t.Context(), "checkin-wrapup")
		require.NoError(t, err)

		orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-model"), defaultTestCatwalkModel("executor-model"), true)
		res, err := sa.Run(t.Context(), SessionAgentCall{
			SessionID:    sess.ID,
			Prompt:       "Do long task",
			Orchestrator: orch,
		})
		require.NoError(t, err)
		require.True(t, reviewNoteSeen.Load())
		require.Equal(t, "Progress looks complete, wrapping up.", res.Response.Content.Text())
		require.Equal(t, 2, planner.callCount())
		require.Equal(t, maxOrchestratorStepsBeforeCheckIn, executor.callCount())
	})

	t.Run("hands back on check-in and caps at three check-ins while preserving final review", func(t *testing.T) {
		env := testEnv(t)

		planner := &scriptedOrchestratorModel{
			provider: "anthropic",
			modelID:  "planner-model",
			steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
				func(_ context.Context, _ fantasy.Call) ([]fantasy.StreamPart, error) {
					payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Phase 1"})
					return toolCallParts("h0", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					require.Contains(t, systemPromptTextInCall(call), "Progress check-in (1/3)")
					payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Phase 2"})
					return toolCallParts("h1", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					require.Contains(t, systemPromptTextInCall(call), "Progress check-in (2/3)")
					payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Phase 3"})
					return toolCallParts("h2", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					require.Contains(t, systemPromptTextInCall(call), "Progress check-in (3/3)")
					payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Phase 4"})
					return toolCallParts("h3", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					sys := systemPromptTextInCall(call)
					require.Contains(t, sys, "REVIEW phase")
					require.NotContains(t, sys, "<orchestrator_review_note>")
					return textStopParts("Final review verified all phases.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
			},
		}

		totalToolSteps := maxOrchestratorStepsBeforeCheckIn*maxOrchestratorProgressCheckIns + 10
		execSteps := make([]func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error), 0, totalToolSteps+1)
		for i := range totalToolSteps {
			idx := i
			execSteps = append(execSteps, func(_ context.Context, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				return toolCallParts(fmt.Sprintf("w-%d", idx), "write", fmt.Sprintf(`{"step":%d}`, idx), fantasy.Usage{InputTokens: 5, OutputTokens: 5}), nil
			})
		}
		execSteps = append(execSteps, func(_ context.Context, _ fantasy.Call) ([]fantasy.StreamPart, error) {
			return textStopParts("All 34 steps complete.", fantasy.Usage{InputTokens: 5, OutputTokens: 5}), nil
		})

		executor := &scriptedOrchestratorModel{
			provider: "openai",
			modelID:  "executor-model",
			steps:    execSteps,
		}

		writeTool := fantasy.NewAgentTool("write", "write", func(context.Context, struct{ Step int }, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})
		viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})

		sa := testSessionAgent(env, planner, executor, "base system", writeTool, viewTool)
		sess, err := env.sessions.Create(t.Context(), "checkin-cap")
		require.NoError(t, err)

		orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-model"), defaultTestCatwalkModel("executor-model"), true)
		res, err := sa.Run(t.Context(), SessionAgentCall{
			SessionID:    sess.ID,
			Prompt:       "Run huge task",
			Orchestrator: orch,
		})
		require.NoError(t, err)
		require.Equal(t, "Final review verified all phases.", res.Response.Content.Text())
		require.Equal(t, 5, planner.callCount())
		require.Equal(t, totalToolSteps+1, executor.callCount())
	})
}

func TestOrchestrator_ReplanIncludesPreviousPlanAfterEscalationAndSteering(t *testing.T) {
	t.Run("after escalation", func(t *testing.T) {
		env := testEnv(t)
		planner := &scriptedOrchestratorModel{
			provider: "anthropic",
			modelID:  "planner-model",
			steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
				func(_ context.Context, _ fantasy.Call) ([]fantasy.StreamPart, error) {
					payload, _ := json.Marshal(handoffToExecutorParams{
						Plan:               "Migrate config parser",
						TargetFiles:        []string{"internal/config/load.go"},
						VerificationChecks: []string{"go test ./internal/config"},
						EscalationTriggers: []string{"schema mismatch"},
					})
					return toolCallParts("h1", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					sys := systemPromptTextInCall(call)
					require.Contains(t, sys, "<previous_orchestrator_plan>")
					require.Contains(t, sys, "Migrate config parser")
					require.Contains(t, sys, "internal/config/load.go")
					require.Contains(t, sys, "go test ./internal/config")
					require.Contains(t, sys, "schema mismatch")
					require.Contains(t, sys, "<orchestrator_escalation>")
					return textStopParts("Replanned around schema mismatch.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
			},
		}
		executor := &scriptedOrchestratorModel{
			provider: "openai",
			modelID:  "executor-model",
			steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
				func(_ context.Context, _ fantasy.Call) ([]fantasy.StreamPart, error) {
					payload, _ := json.Marshal(escalateToPlannerParams{
						Reason:  "schema mismatch in config loader",
						Blocker: "unexpected field type",
					})
					return toolCallParts("e1", EscalateToPlannerToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
			},
		}

		viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})
		sa := testSessionAgent(env, planner, executor, "base system", viewTool)
		sess, err := env.sessions.Create(t.Context(), "replan-escalation")
		require.NoError(t, err)

		orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-model"), defaultTestCatwalkModel("executor-model"), false)
		res, err := sa.Run(t.Context(), SessionAgentCall{
			SessionID:    sess.ID,
			Prompt:       "Migrate config",
			Orchestrator: orch,
		})
		require.NoError(t, err)
		require.Equal(t, "Replanned around schema mismatch.", res.Response.Content.Text())
	})

	t.Run("after mid-turn steering", func(t *testing.T) {
		env := testEnv(t)
		entered := make(chan struct{})
		release := make(chan struct{})

		planner := &scriptedOrchestratorModel{
			provider: "anthropic",
			modelID:  "planner-model",
			steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
				func(_ context.Context, _ fantasy.Call) ([]fantasy.StreamPart, error) {
					payload, _ := json.Marshal(handoffToExecutorParams{
						Plan:               "Use Postgres backend",
						TargetFiles:        []string{"db/store.go"},
						VerificationChecks: []string{"go test ./db"},
					})
					return toolCallParts("h1", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					sys := systemPromptTextInCall(call)
					require.Contains(t, sys, "<previous_orchestrator_plan>")
					require.Contains(t, sys, "Use Postgres backend")
					require.Contains(t, sys, "db/store.go")
					require.Contains(t, sys, "<orchestrator_escalation>")
					require.Contains(t, sys, "steering")
					return textStopParts("Switched plan to SQLite.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
			},
		}
		executor := &scriptedOrchestratorModel{
			provider: "openai",
			modelID:  "executor-model",
			steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
				func(_ context.Context, _ fantasy.Call) ([]fantasy.StreamPart, error) {
					return toolCallParts("wait-1", "bash", `{}`, fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
			},
		}

		slowBashTool := fantasy.NewAgentTool("bash", "slow command", func(ctx context.Context, _ struct{}, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			close(entered)
			select {
			case <-release:
				return fantasy.NewTextResponse("done"), nil
			case <-ctx.Done():
				return fantasy.ToolResponse{}, ctx.Err()
			}
		})
		viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})

		sa := testSessionAgent(env, planner, executor, "base system", slowBashTool, viewTool).(*sessionAgent)
		sess, err := env.sessions.Create(t.Context(), "replan-steering")
		require.NoError(t, err)

		orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-model"), defaultTestCatwalkModel("executor-model"), false)
		doneCh := make(chan *fantasy.AgentResult, 1)
		errCh := make(chan error, 1)
		go func() {
			r, runErr := sa.Run(t.Context(), SessionAgentCall{
				SessionID:    sess.ID,
				Prompt:       "Implement storage",
				Orchestrator: orch,
			})
			doneCh <- r
			errCh <- runErr
		}()

		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			t.Fatal("executor tool never started")
		}

		_, err = sa.Run(t.Context(), SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "use SQLite instead",
		})
		require.NoError(t, err)
		entries := sa.QueueEntries(sess.ID)
		require.Len(t, entries, 1)
		steerDone, err := sa.RunQueuedNow(t.Context(), sess.ID, entries[0].ID)
		require.NoError(t, err)

		close(release)
		require.NoError(t, <-errCh)
		res := <-doneCh
		<-steerDone

		require.Equal(t, "Switched plan to SQLite.", res.Response.Content.Text())
	})
}

func TestOrchestrator_CoordinatorPreservesRoleReasoningAndSkipsTeamNote(t *testing.T) {
	t.Parallel()

	t.Run("configured reasoning modes are not overwritten by WithReasoningMode off", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		cfg.Config().Options.DisableDefaultProviders = true
		cfg.Config().Providers.Reset(map[string]config.ProviderConfig{})
		cfg.Config().Providers.Set("mock-p", config.ProviderConfig{
			ID:      "mock-p",
			Name:    "Mock Provider",
			Type:    openaicompat.Name,
			BaseURL: "http://127.0.0.1:0/v1",
			APIKey:  "test",
			Models: []catwalk.Model{
				{
					ID:               "planner-m",
					Name:             "Planner Model",
					DefaultMaxTokens: 4096,
					CanReason:        true,
					ReasoningLevels:  []string{"none", "low", "medium", "high"},
				},
				{
					ID:               "executor-m",
					Name:             "Executor Model",
					DefaultMaxTokens: 4096,
					CanReason:        true,
					ReasoningLevels:  []string{"none", "low", "medium", "high"},
				},
			},
		})
		cfg.OverridePreferredModel(config.SelectedModelTypeLarge, config.SelectedModel{Provider: "mock-p", Model: "planner-m"})
		cfg.OverridePreferredModel(config.SelectedModelTypeSmall, config.SelectedModel{Provider: "mock-p", Model: "executor-m"})
		cfg.SetupAgents()
		cfg.Config().Orchestrator = config.OrchestratorConfig{
			Enabled: true,
			Planner: &config.SelectedModel{
				Provider:      "mock-p",
				Model:         "planner-m",
				ReasoningMode: config.ReasoningModeHigh,
			},
			Executor: &config.SelectedModel{
				Provider:      "mock-p",
				Model:         "executor-m",
				ReasoningMode: config.ReasoningModeLow,
			},
		}

		coord := &coordinator{
			cfg:          cfg,
			sessions:     env.sessions,
			messages:     env.messages,
			permissions:  env.permissions,
			history:      env.history,
			filetracker:  *env.filetracker,
			skillTracker: skills.NewTracker(nil),
			agents:       make(map[string]SessionAgent),
			subAgents:    make(map[string]SessionAgent),
			agentJobs:    make(map[string]*agentJob),
		}

		var recorded SessionAgentCall
		mockGen := newMockAgent("mock-p", 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			recorded = call
			return agentResultWithText("ok"), nil
		})
		coord.agents[config.AgentGeneral] = mockGen
		coord.mainAgent = mockGen
		coord.mainAgentName = config.AgentGeneral

		sess, err := env.sessions.Create(t.Context(), "orch-reasoning")
		require.NoError(t, err)
		coord.agentJobs["job-1"] = &agentJob{
			id:              "job-1",
			name:            "helper",
			agent:           config.AgentCode,
			parentSessionID: sess.ID,
			rootSessionID:   sess.ID,
			hasRun:          true,
		}

		ctx := WithReasoningMode(t.Context(), config.ReasoningModeOff)
		_, err = coord.Run(ctx, sess.ID, "hello")
		require.NoError(t, err)
		require.NotNil(t, recorded.Orchestrator)
		require.Equal(t, config.ReasoningModeHigh, recorded.Orchestrator.Planner.Model.ModelCfg.ReasoningMode)
		require.Equal(t, config.ReasoningModeLow, recorded.Orchestrator.Executor.Model.ModelCfg.ReasoningMode)
		require.Empty(t, recorded.TeamNote)

		pOpts, ok := recorded.Orchestrator.Planner.ProviderOptions[openaicompat.Name].(*openaicompat.ProviderOptions)
		require.True(t, ok)
		require.NotNil(t, pOpts.ReasoningEffort)
		require.Equal(t, "high", string(*pOpts.ReasoningEffort))

		eOpts, ok := recorded.Orchestrator.Executor.ProviderOptions[openaicompat.Name].(*openaicompat.ProviderOptions)
		require.True(t, ok)
		require.NotNil(t, eOpts.ReasoningEffort)
		require.Equal(t, "low", string(*eOpts.ReasoningEffort))
	})

	t.Run("applies default high to capable planner and medium to capable executor when unset", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		cfg.Config().Options.DisableDefaultProviders = true
		cfg.Config().Providers.Reset(map[string]config.ProviderConfig{})
		cfg.Config().Providers.Set("mock-p", config.ProviderConfig{
			ID:      "mock-p",
			Name:    "Mock Provider",
			Type:    openaicompat.Name,
			BaseURL: "http://127.0.0.1:0/v1",
			APIKey:  "test",
			Models: []catwalk.Model{
				{
					ID:               "planner-m",
					Name:             "Planner Model",
					DefaultMaxTokens: 4096,
					CanReason:        true,
					ReasoningLevels:  []string{"low", "medium", "high"},
				},
				{
					ID:               "executor-m",
					Name:             "Executor Model",
					DefaultMaxTokens: 4096,
					CanReason:        true,
					ReasoningLevels:  []string{"low", "medium", "high"},
				},
			},
		})
		cfg.OverridePreferredModel(config.SelectedModelTypeLarge, config.SelectedModel{Provider: "mock-p", Model: "planner-m"})
		cfg.OverridePreferredModel(config.SelectedModelTypeSmall, config.SelectedModel{Provider: "mock-p", Model: "executor-m"})
		cfg.SetupAgents()
		cfg.Config().Orchestrator = config.OrchestratorConfig{
			Enabled: true,
		}

		coord := &coordinator{
			cfg:          cfg,
			sessions:     env.sessions,
			messages:     env.messages,
			permissions:  env.permissions,
			history:      env.history,
			filetracker:  *env.filetracker,
			skillTracker: skills.NewTracker(nil),
			agents:       make(map[string]SessionAgent),
			subAgents:    make(map[string]SessionAgent),
			agentJobs:    make(map[string]*agentJob),
		}

		var recorded SessionAgentCall
		mockGen := newMockAgent("mock-p", 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			recorded = call
			return agentResultWithText("ok"), nil
		})
		coord.agents[config.AgentGeneral] = mockGen
		coord.mainAgent = mockGen
		coord.mainAgentName = config.AgentGeneral

		sess, err := env.sessions.Create(t.Context(), "orch-default-reasoning")
		require.NoError(t, err)

		ctx := WithReasoningMode(t.Context(), config.ReasoningModeOff)
		_, err = coord.Run(ctx, sess.ID, "hello")
		require.NoError(t, err)
		require.NotNil(t, recorded.Orchestrator)
		require.Equal(t, config.ReasoningModeHigh, recorded.Orchestrator.Planner.Model.ModelCfg.ReasoningMode)
		require.Equal(t, config.ReasoningModeMedium, recorded.Orchestrator.Executor.Model.ModelCfg.ReasoningMode)
	})

	t.Run("resolveOrchestratorRoleModel handles defaults and fallbacks", func(t *testing.T) {
		offDefaultPlanner := resolveOrchestratorRoleModel("planner", Model{
			CatwalkCfg: catwalk.Model{
				CanReason:              true,
				DefaultReasoningEffort: "none",
				ReasoningLevels:        []string{"none", "low", "medium", "high"},
			},
		})
		require.Equal(t, config.ReasoningModeHigh, offDefaultPlanner.ModelCfg.ReasoningMode)

		offDefaultExecutor := resolveOrchestratorRoleModel("executor", Model{
			CatwalkCfg: catwalk.Model{
				CanReason:              true,
				DefaultReasoningEffort: "none",
				ReasoningLevels:        []string{"none", "low", "medium", "high"},
			},
		})
		require.Equal(t, config.ReasoningModeOff, offDefaultExecutor.ModelCfg.ReasoningMode)

		emptyLevelsPlanner := resolveOrchestratorRoleModel("planner", Model{
			CatwalkCfg: catwalk.Model{
				CanReason: true,
			},
		})
		require.Equal(t, config.ReasoningModeHigh, emptyLevelsPlanner.ModelCfg.ReasoningMode)

		explicitOffCapablePlanner := resolveOrchestratorRoleModel("planner", Model{
			CatwalkCfg: catwalk.Model{
				CanReason:       true,
				ReasoningLevels: []string{"none", "low", "medium", "high"},
			},
			ModelCfg: config.SelectedModel{
				ReasoningMode: config.ReasoningModeOff,
			},
		})
		require.Equal(t, config.ReasoningModeHigh, explicitOffCapablePlanner.ModelCfg.ReasoningMode)
	})
}

func TestOrchestrator_PerRoleSamplingIsolation(t *testing.T) {
	f64Ptr := func(v float64) *float64 { return &v }

	t.Run("executor nil temperature clears planner temperature across steps", func(t *testing.T) {
		env := testEnv(t)
		planner := &scriptedOrchestratorModel{
			provider: "anthropic",
			modelID:  "planner-model",
			steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					require.NotNil(t, call.Temperature)
					require.InDelta(t, 0.9, *call.Temperature, 0.0001)
					payload, _ := json.Marshal(handoffToExecutorParams{Plan: "Write file"})
					return toolCallParts("h1", HandoffToExecutorToolName, string(payload), fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					require.NotNil(t, call.Temperature)
					require.InDelta(t, 0.9, *call.Temperature, 0.0001)
					return textStopParts("Verified.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
			},
		}
		executor := &scriptedOrchestratorModel{
			provider: "openai",
			modelID:  "executor-model",
			steps: []func(context.Context, fantasy.Call) ([]fantasy.StreamPart, error){
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					require.Nil(t, call.Temperature)
					return toolCallParts("w1", "write", `{}`, fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
				func(_ context.Context, call fantasy.Call) ([]fantasy.StreamPart, error) {
					require.Nil(t, call.Temperature)
					return textStopParts("Wrote file.", fantasy.Usage{InputTokens: 10, OutputTokens: 10}), nil
				},
			},
		}

		writeTool := fantasy.NewAgentTool("write", "write", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})
		viewTool := fantasy.NewAgentTool("view", "view", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		})

		sa := testSessionAgent(env, planner, executor, "base system", writeTool, viewTool)
		sess, err := env.sessions.Create(t.Context(), "sampling-isolation")
		require.NoError(t, err)

		orch := makeOrchestratorModelsConfig(planner, executor, defaultTestCatwalkModel("planner-model"), defaultTestCatwalkModel("executor-model"), true)
		orch.Planner.Temperature = f64Ptr(0.9)
		orch.Executor.Temperature = nil

		res, err := sa.Run(t.Context(), SessionAgentCall{
			SessionID:    sess.ID,
			Prompt:       "Edit",
			Temperature:  f64Ptr(0.9),
			Orchestrator: orch,
		})
		require.NoError(t, err)
		require.Equal(t, "Verified.", res.Response.Content.Text())
	})

	t.Run("coordinator populates planner and executor sampling independently", func(t *testing.T) {
		env := testEnv(t)
		cfg, err := config.Init(env.workingDir, "", false)
		require.NoError(t, err)
		cfg.Config().Options.DisableDefaultProviders = true
		cfg.Config().Providers.Reset(map[string]config.ProviderConfig{})
		cfg.Config().Providers.Set("mock-p", config.ProviderConfig{
			ID:      "mock-p",
			Name:    "Mock Provider",
			Type:    openaicompat.Name,
			BaseURL: "http://127.0.0.1:0/v1",
			APIKey:  "test",
			Models: []catwalk.Model{
				{
					ID:               "planner-m",
					Name:             "Planner Model",
					DefaultMaxTokens: 4096,
					Options:          catwalk.ModelOptions{Temperature: f64Ptr(0.8)},
				},
				{
					ID:               "executor-m",
					Name:             "Executor Model",
					DefaultMaxTokens: 4096,
					Options:          catwalk.ModelOptions{Temperature: f64Ptr(0.2)},
				},
			},
		})
		cfg.OverridePreferredModel(config.SelectedModelTypeLarge, config.SelectedModel{Provider: "mock-p", Model: "planner-m"})
		cfg.OverridePreferredModel(config.SelectedModelTypeSmall, config.SelectedModel{Provider: "mock-p", Model: "executor-m"})
		cfg.SetupAgents()
		cfg.Config().Orchestrator = config.OrchestratorConfig{
			Enabled:  true,
			Planner:  &config.SelectedModel{Provider: "mock-p", Model: "planner-m"},
			Executor: &config.SelectedModel{Provider: "mock-p", Model: "executor-m"},
		}

		coord := &coordinator{
			cfg:          cfg,
			sessions:     env.sessions,
			messages:     env.messages,
			permissions:  env.permissions,
			history:      env.history,
			filetracker:  *env.filetracker,
			skillTracker: skills.NewTracker(nil),
			agents:       make(map[string]SessionAgent),
			subAgents:    make(map[string]SessionAgent),
			agentJobs:    make(map[string]*agentJob),
		}

		var recorded SessionAgentCall
		mockGen := newMockAgent("mock-p", 4096, func(_ context.Context, call SessionAgentCall) (*fantasy.AgentResult, error) {
			recorded = call
			return agentResultWithText("ok"), nil
		})
		coord.agents[config.AgentGeneral] = mockGen
		coord.mainAgent = mockGen
		coord.mainAgentName = config.AgentGeneral

		sess, err := env.sessions.Create(t.Context(), "coord-sampling")
		require.NoError(t, err)

		_, err = coord.Run(t.Context(), sess.ID, "hello")
		require.NoError(t, err)
		require.NotNil(t, recorded.Orchestrator)
		require.NotNil(t, recorded.Orchestrator.Planner.Temperature)
		require.InDelta(t, 0.8, *recorded.Orchestrator.Planner.Temperature, 0.0001)
		require.NotNil(t, recorded.Orchestrator.Executor.Temperature)
		require.InDelta(t, 0.2, *recorded.Orchestrator.Executor.Temperature, 0.0001)
	})
}
