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
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/anthropic"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/google"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openai"
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
