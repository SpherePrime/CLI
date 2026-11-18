package agent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/agent/tools"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSessionAgentRun_MaxStepsStopsLoop(t *testing.T) {
	env := testEnv(t)
	var stepCalls atomic.Int32
	model := &leakTestModel{
		stream: func(_ fantasy.Call, yield func(fantasy.StreamPart) bool) {
			n := stepCalls.Add(1)
			callID := "call_" + string(rune('a'+n-1))
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputStart, ID: callID, ToolCallName: "echo"})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolInputEnd, ID: callID})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeToolCall, ID: callID, ToolCallName: "echo", ToolCallInput: `{}`})
			yield(fantasy.StreamPart{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls})
		},
	}

	echoTool := fantasy.NewAgentTool(
		"echo",
		"echo",
		func(_ context.Context, _ map[string]any, _ fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("ok"), nil
		},
	)
	sa := testSessionAgent(env, model, model, "system", echoTool)

	sess, err := env.sessions.Create(t.Context(), "max-steps")
	require.NoError(t, err)

	res, err := sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "keep looping",
		MaxSteps:  3,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Len(t, res.Steps, 3)
	require.Equal(t, int32(3), stepCalls.Load())
}

func TestInvokeSubAgent_DisableAutoContinue(t *testing.T) {
	env := testEnv(t)
	coord := newTestCoordinator(t, env, "p", config.ProviderConfig{ID: "p"})
	attemptsLimit := 3
	coord.cfg.Config().Options.AutoContinue = true
	coord.cfg.Config().Options.AutoContinueAttempts = &attemptsLimit

	sess, err := env.sessions.Create(t.Context(), "sub")
	require.NoError(t, err)

	var attempts atomic.Int32
	agent := newMockAgent("p", 1024, func(_ context.Context, _ SessionAgentCall) (*fantasy.AgentResult, error) {
		attempts.Add(1)
		return nil, errors.New("provider failure")
	})

	_, err = coord.invokeSubAgent(t.Context(), subAgentParams{
		Agent:               agent,
		DisableAutoContinue: true,
	}, sess.ID, agent.Model(), 1024, config.ProviderConfig{ID: "p"})
	require.Error(t, err)
	require.Equal(t, int32(1), attempts.Load())

	attempts.Store(0)
	_, err = coord.invokeSubAgent(t.Context(), subAgentParams{
		Agent:               agent,
		DisableAutoContinue: false,
	}, sess.ID, agent.Model(), 1024, config.ProviderConfig{ID: "p"})
	require.Error(t, err)
	require.Equal(t, int32(1+attemptsLimit), attempts.Load())
}

func TestAgenticFetchTool_TimeoutReturnsCleanError(t *testing.T) {
	env := testEnv(t)
	coord := newTestCoordinator(t, env, "p", config.ProviderConfig{ID: "p"})
	coord.cfg.Config().Options.DataDirectory = t.TempDir()
	coord.permissions = env.permissions
	coord.filetracker = *env.filetracker

	parent, err := env.sessions.Create(t.Context(), "parent")
	require.NoError(t, err)

	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	t.Cleanup(func() {
		close(done)
		srv.CloseClientConnections()
		srv.Close()
	})

	fetchTool, err := coord.agenticFetchToolWithTimeout(t.Context(), srv.Client(), 50*time.Millisecond)
	require.NoError(t, err)

	ctx := context.WithValue(t.Context(), tools.SessionIDContextKey, parent.ID)
	ctx = context.WithValue(ctx, tools.MessageIDContextKey, "msg-1")

	resp, err := fetchTool.Run(ctx, fantasy.ToolCall{
		ID:    "af-1",
		Name:  tools.AgenticFetchToolName,
		Input: `{"url":"` + srv.URL + `","prompt":"summarize"}`,
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "timed out")
}

func TestCustomAgenticFetchTool_TimeoutReturnsCleanError(t *testing.T) {
	exec := NewCustomExecutor(config.NewTestStore(&config.Config{}))
	done := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-done:
		}
	}))
	t.Cleanup(func() {
		close(done)
		srv.CloseClientConnections()
		srv.Close()
	})

	perms := permission.NewPermissionService(t.TempDir(), true, nil)
	tool := exec.newCustomAgenticFetchToolWithTimeout(
		Model{
			Model:      &fakeLanguageModel{},
			CatwalkCfg: catwalk.Model{ID: "m"},
		},
		t.TempDir(),
		perms,
		newCustomFileTracker(),
		config.ToolGlob{},
		config.ToolGrep{},
		50*time.Millisecond,
	)

	resp, err := tool.Run(t.Context(), fantasy.ToolCall{
		ID:    "caf-1",
		Name:  tools.AgenticFetchToolName,
		Input: `{"url":"` + srv.URL + `","prompt":"summarize"}`,
	})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "timed out")
}

func TestSessionAgentRun_ReconcilesUnfinishedToolCallOnAbnormalFinish(t *testing.T) {
	env := testEnv(t)
	model := &leakTestModel{
		stream: func(_ fantasy.Call, yield func(fantasy.StreamPart) bool) {
			yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeToolInputStart,
				ID:           "tc_unfinished",
				ToolCallName: "web_search",
			})
			yield(fantasy.StreamPart{
				Type:         fantasy.StreamPartTypeFinish,
				FinishReason: fantasy.FinishReasonLength,
			})
		},
	}

	sa := testSessionAgent(env, model, model, "system")
	sess, err := env.sessions.Create(t.Context(), "abnormal-finish")
	require.NoError(t, err)

	_, err = sa.Run(t.Context(), SessionAgentCall{
		SessionID: sess.ID,
		Prompt:    "search something",
	})
	require.NoError(t, err)

	msgs, err := env.messages.List(t.Context(), sess.ID)
	require.NoError(t, err)

	var foundToolCall *message.ToolCall
	var foundToolResult *message.ToolResult
	for _, msg := range msgs {
		for _, tc := range msg.ToolCalls() {
			if tc.ID == "tc_unfinished" {
				tcCopy := tc
				foundToolCall = &tcCopy
			}
		}
		if msg.Role == message.Tool {
			for _, tr := range msg.ToolResults() {
				if tr.ToolCallID == "tc_unfinished" {
					trCopy := tr
					foundToolResult = &trCopy
				}
			}
		}
	}

	require.NotNil(t, foundToolCall, "assistant message should contain the tool call")
	require.True(t, foundToolCall.Finished, "unfinished tool call must be marked finished")
	require.Equal(t, "{}", foundToolCall.Input, "empty tool call input must default to {}")

	require.NotNil(t, foundToolResult, "orphan tool call must receive a persisted error tool result")
	require.True(t, foundToolResult.IsError)
	require.Equal(t, "web_search", foundToolResult.Name)
}
