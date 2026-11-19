package agent

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type recordingScriptModel struct {
	mu       sync.Mutex
	calls    []fantasy.Call
	streamFn func(idx int, call fantasy.Call) ([]fantasy.StreamPart, error)
}

func (m *recordingScriptModel) Provider() string { return "fake" }
func (m *recordingScriptModel) Model() string    { return "fake-model" }

func (m *recordingScriptModel) Generate(context.Context, fantasy.Call) (*fantasy.Response, error) {
	return &fantasy.Response{}, nil
}

func (m *recordingScriptModel) GenerateObject(context.Context, fantasy.ObjectCall) (*fantasy.ObjectResponse, error) {
	return &fantasy.ObjectResponse{}, nil
}

func (m *recordingScriptModel) StreamObject(context.Context, fantasy.ObjectCall) (fantasy.ObjectStreamResponse, error) {
	return nil, nil
}

func (m *recordingScriptModel) Stream(_ context.Context, call fantasy.Call) (fantasy.StreamResponse, error) {
	m.mu.Lock()
	idx := len(m.calls)
	m.calls = append(m.calls, call)
	m.mu.Unlock()

	parts, err := m.streamFn(idx, call)
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

func (m *recordingScriptModel) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *recordingScriptModel) callAt(i int) fantasy.Call {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[i]
}

func newTestAgentWithDataDir(env fakeEnv, dataDir string, large, small fantasy.LanguageModel, tools ...fantasy.AgentTool) *sessionAgent {
	largeModel := Model{
		Model: large,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
		},
		ModelCfg: config.SelectedModel{
			Provider: "fake",
			Model:    "fake-model",
		},
	}
	smallModel := Model{
		Model: small,
		CatwalkCfg: catwalk.Model{
			ContextWindow:    200000,
			DefaultMaxTokens: 10000,
		},
		ModelCfg: config.SelectedModel{
			Provider: "fake",
			Model:    "fake-model",
		},
	}
	return NewSessionAgent(SessionAgentOptions{
		LargeModel:   largeModel,
		SmallModel:   smallModel,
		SystemPrompt: "system prompt",
		IsYolo:       true,
		Sessions:     env.sessions,
		Messages:     env.messages,
		Tools:        tools,
		DataDir:      dataDir,
	}).(*sessionAgent)
}

func TestSummarize_EmptyOrReasoningOnlyDoesNotOverwriteSummaryMessageID(t *testing.T) {
	t.Parallel()

	t.Run("empty text on both attempts preserves existing SummaryMessageID and summary file", func(t *testing.T) {
		t.Parallel()
		env := testEnv(t)
		ctx := t.Context()
		dataDir := t.TempDir()

		model := &recordingScriptModel{
			streamFn: func(_ int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				return []fantasy.StreamPart{
					{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
				}, nil
			},
		}
		sa := newTestAgentWithDataDir(env, dataDir, model, fastModel{})

		sess, err := env.sessions.Create(ctx, "Existing summary")
		require.NoError(t, err)

		prevSummary, err := env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
			Role:             message.Assistant,
			Parts:            []message.ContentPart{message.TextContent{Text: "previous checkpoint summary"}},
			IsSummaryMessage: true,
		})
		require.NoError(t, err)

		_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "new follow-up turn"}},
		})
		require.NoError(t, err)

		sess.SummaryMessageID = prevSummary.ID
		sess.PromptTokens = 1234
		sess.CompletionTokens = 56
		sess.ContextTokens = 1290
		sess, err = env.sessions.Save(ctx, sess)
		require.NoError(t, err)

		require.NoError(t, appendStepToContextSummary(dataDir, sess.ID, []fantasy.Message{
			fantasy.NewUserMessage("new follow-up turn"),
		}))

		err = sa.Summarize(ctx, sess.ID, nil, nil)
		require.Error(t, err)
		require.Equal(t, 2, model.callCount(), "should retry once with nudge prompt before failing")
		require.True(t, promptHasText(model.callAt(0).Prompt, "previous checkpoint summary"), "prior checkpoint summary must be sent even when dataDir summary file exists")
		require.True(t, promptHasText(model.callAt(0).Prompt, "new follow-up turn"), "incremental summary file content must also be sent")
		require.True(t, promptHasText(model.callAt(0).Prompt, "existing summary checkpoint"))
		require.True(t, promptHasText(model.callAt(1).Prompt, emptySummaryNudgePrompt))

		after, err := env.sessions.Get(ctx, sess.ID)
		require.NoError(t, err)
		require.Equal(t, prevSummary.ID, after.SummaryMessageID)
		require.Equal(t, int64(1234), after.PromptTokens)
		require.Equal(t, int64(56), after.CompletionTokens)
		require.Equal(t, int64(1290), after.ContextTokens)

		_, statErr := os.Stat(contextSummaryFilePath(dataDir, sess.ID))
		require.NoError(t, statErr, "summaries/<sessionID>.md must not be deleted on failed summarize")

		msgs, err := env.messages.List(ctx, sess.ID)
		require.NoError(t, err)
		last := msgs[len(msgs)-1]
		require.True(t, last.IsSummaryMessage)
		require.Equal(t, message.FinishReasonError, last.FinishReason())
		require.Equal(t, "Summarization Error", last.FinishPart().Message)
	})

	t.Run("reasoning-only output on both attempts fails without overwriting SummaryMessageID", func(t *testing.T) {
		t.Parallel()
		env := testEnv(t)
		ctx := t.Context()

		model := &recordingScriptModel{
			streamFn: func(_ int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				return []fantasy.StreamPart{
					{Type: fantasy.StreamPartTypeReasoningStart, ID: "r1"},
					{Type: fantasy.StreamPartTypeReasoningDelta, ID: "r1", Delta: "thinking without outputting summary"},
					{Type: fantasy.StreamPartTypeReasoningEnd, ID: "r1"},
					{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
				}, nil
			},
		}
		sa := newTestAgentWithDataDir(env, "", model, fastModel{})

		sess, err := env.sessions.Create(ctx, "Reasoning only summarize")
		require.NoError(t, err)
		_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "hello"}},
		})
		require.NoError(t, err)

		err = sa.Summarize(ctx, sess.ID, nil, nil)
		require.Error(t, err)
		require.Equal(t, 2, model.callCount())

		after, err := env.sessions.Get(ctx, sess.ID)
		require.NoError(t, err)
		require.Empty(t, after.SummaryMessageID)

		msgs, err := env.messages.List(ctx, sess.ID)
		require.NoError(t, err)
		last := msgs[len(msgs)-1]
		require.Equal(t, message.FinishReasonError, last.FinishReason())
	})

	t.Run("empty first attempt recovers when nudge retry produces summary text", func(t *testing.T) {
		t.Parallel()
		env := testEnv(t)
		ctx := t.Context()
		dataDir := t.TempDir()

		model := &recordingScriptModel{
			streamFn: func(idx int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				if idx == 0 {
					return []fantasy.StreamPart{
						{Type: fantasy.StreamPartTypeReasoningStart, ID: "r1"},
						{Type: fantasy.StreamPartTypeReasoningDelta, ID: "r1", Delta: "only thinking"},
						{Type: fantasy.StreamPartTypeReasoningEnd, ID: "r1"},
						{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
					}, nil
				}
				return []fantasy.StreamPart{
					{Type: fantasy.StreamPartTypeTextStart, ID: "t1"},
					{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: "## Current State\nRecovered summary"},
					{Type: fantasy.StreamPartTypeTextEnd, ID: "t1"},
					{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
				}, nil
			},
		}
		sa := newTestAgentWithDataDir(env, dataDir, model, fastModel{})

		sess, err := env.sessions.Create(ctx, "Recover on nudge")
		require.NoError(t, err)
		_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
			Role:  message.User,
			Parts: []message.ContentPart{message.TextContent{Text: "build feature"}},
		})
		require.NoError(t, err)
		require.NoError(t, appendStepToContextSummary(dataDir, sess.ID, []fantasy.Message{
			fantasy.NewUserMessage("build feature"),
		}))

		err = sa.Summarize(ctx, sess.ID, nil, nil)
		require.NoError(t, err)
		require.Equal(t, 2, model.callCount())

		after, err := env.sessions.Get(ctx, sess.ID)
		require.NoError(t, err)
		require.NotEmpty(t, after.SummaryMessageID)

		sumMsg, err := env.messages.Get(ctx, after.SummaryMessageID)
		require.NoError(t, err)
		require.Equal(t, message.FinishReasonEndTurn, sumMsg.FinishReason())
		require.Equal(t, "## Current State\nRecovered summary", sumMsg.Content().Text)

		_, statErr := os.Stat(contextSummaryFilePath(dataDir, sess.ID))
		require.True(t, os.IsNotExist(statErr), "summaries/<sessionID>.md should be deleted after successful summarize")
	})
}

func TestSummarize_NothingNewSinceLastSummaryPreservesCheckpoint(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	ctx := t.Context()
	dataDir := t.TempDir()

	model := &recordingScriptModel{
		streamFn: func(_ int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
			return nil, errors.New("model should not be called when nothing new to summarize")
		},
	}
	sa := newTestAgentWithDataDir(env, dataDir, model, fastModel{})

	sess, err := env.sessions.Create(ctx, "Already summarized")
	require.NoError(t, err)
	prevSummary, err := env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:             message.Assistant,
		Parts:            []message.ContentPart{message.TextContent{Text: "existing summary"}},
		IsSummaryMessage: true,
	})
	require.NoError(t, err)

	sess.SummaryMessageID = prevSummary.ID
	sess, err = env.sessions.Save(ctx, sess)
	require.NoError(t, err)

	require.NoError(t, appendStepToContextSummary(dataDir, sess.ID, []fantasy.Message{
		fantasy.NewUserMessage("checkpoint note"),
	}))

	err = sa.Summarize(ctx, sess.ID, nil, nil)
	require.ErrorIs(t, err, ErrNothingToSummarize)
	require.Equal(t, 0, model.callCount())

	after, err := env.sessions.Get(ctx, sess.ID)
	require.NoError(t, err)
	require.Equal(t, prevSummary.ID, after.SummaryMessageID)

	msgs, err := env.messages.List(ctx, sess.ID)
	require.NoError(t, err)
	require.Len(t, msgs, 1, "no new summary message should be created")

	_, statErr := os.Stat(contextSummaryFilePath(dataDir, sess.ID))
	require.NoError(t, statErr, "summaries/<sessionID>.md must be preserved")
}

func TestSummarize_FailedSummaryMessagesAreFilteredOut(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	ctx := t.Context()

	model := &recordingScriptModel{
		streamFn: func(_ int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
			return []fantasy.StreamPart{
				{Type: fantasy.StreamPartTypeTextStart, ID: "t1"},
				{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: "updated summary"},
				{Type: fantasy.StreamPartTypeTextEnd, ID: "t1"},
				{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
			}, nil
		},
	}
	sa := newTestAgentWithDataDir(env, "", model, fastModel{})

	sess, err := env.sessions.Create(ctx, "Filter failed summaries")
	require.NoError(t, err)

	prevSummary, err := env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:             message.Assistant,
		Parts:            []message.ContentPart{message.TextContent{Text: "valid checkpoint"}},
		IsSummaryMessage: true,
	})
	require.NoError(t, err)
	sess.SummaryMessageID = prevSummary.ID
	sess, err = env.sessions.Save(ctx, sess)
	require.NoError(t, err)

	// Simulate an earlier failed summary attempt that emitted reasoning only.
	failedSum, err := env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role: message.Assistant,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "stale failed summary reasoning"},
		},
		IsSummaryMessage: true,
	})
	require.NoError(t, err)
	failedSum.AddFinish(message.FinishReasonError, "Summarization Error", "Model returned an empty summary")
	require.NoError(t, env.messages.Update(ctx, failedSum))
	require.NoError(t, env.messages.FlushAll(ctx))

	// With only the active checkpoint and the failed summary, nothing new exists.
	err = sa.Summarize(ctx, sess.ID, nil, nil)
	require.ErrorIs(t, err, ErrNothingToSummarize)
	require.Equal(t, 0, model.callCount())

	// Adding a real user turn allows summarization while filtering out the failed summary's reasoning.
	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "real new turn"}},
	})
	require.NoError(t, err)

	require.NoError(t, sa.Summarize(ctx, sess.ID, nil, nil))
	require.Equal(t, 1, model.callCount())
	require.True(t, promptHasText(model.callAt(0).Prompt, "valid checkpoint"))
	require.True(t, promptHasText(model.callAt(0).Prompt, "real new turn"))
	for _, msg := range model.callAt(0).Prompt {
		for _, part := range msg.Content {
			if rp, ok := fantasy.AsMessagePart[fantasy.ReasoningPart](part); ok {
				require.NotContains(t, rp.Text, "stale failed summary reasoning")
			}
		}
	}
}

func TestRun_EmptyEndTurnReturnsErrorAndPersistsFinishReasonError(t *testing.T) {
	t.Parallel()

	t.Run("empty text with FinishReasonStop", func(t *testing.T) {
		t.Parallel()
		env := testEnv(t)
		ctx := t.Context()

		model := &recordingScriptModel{
			streamFn: func(_ int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				return []fantasy.StreamPart{
					{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
				}, nil
			},
		}
		sa := newTestAgentWithDataDir(env, "", model, fastModel{})

		sess, err := env.sessions.Create(ctx, "Empty stop")
		require.NoError(t, err)

		_, err = sa.Run(ctx, SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "hello",
		})
		require.Error(t, err)
		var providerErr *fantasy.ProviderError
		require.ErrorAs(t, err, &providerErr)
		require.True(t, providerErr.IsRetryable())
		require.False(t, sa.IsSessionBusy(sess.ID))

		msgs, err := env.messages.List(ctx, sess.ID)
		require.NoError(t, err)
		var lastAssistant message.Message
		for _, m := range msgs {
			if m.Role == message.Assistant {
				lastAssistant = m
			}
		}
		require.Equal(t, message.FinishReasonError, lastAssistant.FinishReason())
		require.Equal(t, "Empty Response", lastAssistant.FinishPart().Message)
	})

	t.Run("reasoning-only with FinishReasonStop", func(t *testing.T) {
		t.Parallel()
		env := testEnv(t)
		ctx := t.Context()

		model := &recordingScriptModel{
			streamFn: func(_ int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				return []fantasy.StreamPart{
					{Type: fantasy.StreamPartTypeReasoningStart, ID: "r1"},
					{Type: fantasy.StreamPartTypeReasoningDelta, ID: "r1", Delta: "internal thought only"},
					{Type: fantasy.StreamPartTypeReasoningEnd, ID: "r1"},
					{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
				}, nil
			},
		}
		sa := newTestAgentWithDataDir(env, "", model, fastModel{})

		sess, err := env.sessions.Create(ctx, "Reasoning only stop")
		require.NoError(t, err)

		_, err = sa.Run(ctx, SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "hello",
		})
		require.Error(t, err)

		msgs, err := env.messages.List(ctx, sess.ID)
		require.NoError(t, err)
		var lastAssistant message.Message
		for _, m := range msgs {
			if m.Role == message.Assistant {
				lastAssistant = m
			}
		}
		require.Equal(t, message.FinishReasonError, lastAssistant.FinishReason())
		require.Equal(t, "Empty Response", lastAssistant.FinishPart().Message)
	})

	t.Run("tool call without assistant text still succeeds", func(t *testing.T) {
		t.Parallel()
		env := testEnv(t)
		ctx := t.Context()

		pingTool := fantasy.NewAgentTool("ping", "ping tool", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("pong"), nil
		})
		model := &recordingScriptModel{
			streamFn: func(idx int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				if idx == 0 {
					return []fantasy.StreamPart{
						{Type: fantasy.StreamPartTypeToolCall, ID: "tc1", ToolCallName: "ping", ToolCallInput: `{}`},
						{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls},
					}, nil
				}
				return []fantasy.StreamPart{
					{Type: fantasy.StreamPartTypeTextStart, ID: "t1"},
					{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: "pong completed"},
					{Type: fantasy.StreamPartTypeTextEnd, ID: "t1"},
					{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
				}, nil
			},
		}
		sa := newTestAgentWithDataDir(env, "", model, fastModel{}, pingTool)

		sess, err := env.sessions.Create(ctx, "Tool turn")
		require.NoError(t, err)

		res, err := sa.Run(ctx, SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "ping",
		})
		require.NoError(t, err)
		require.Equal(t, "pong completed", res.Response.Content.Text())
	})

	t.Run("multi-step turn where step 0 has text and tool call and step 1 ends empty returns error", func(t *testing.T) {
		t.Parallel()
		env := testEnv(t)
		ctx := t.Context()

		pingTool := fantasy.NewAgentTool("ping", "ping tool", func(context.Context, struct{}, fantasy.ToolCall) (fantasy.ToolResponse, error) {
			return fantasy.NewTextResponse("pong"), nil
		})
		model := &recordingScriptModel{
			streamFn: func(idx int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				if idx == 0 {
					return []fantasy.StreamPart{
						{Type: fantasy.StreamPartTypeTextStart, ID: "t1"},
						{Type: fantasy.StreamPartTypeTextDelta, ID: "t1", Delta: "Let me check that."},
						{Type: fantasy.StreamPartTypeTextEnd, ID: "t1"},
						{Type: fantasy.StreamPartTypeToolCall, ID: "tc1", ToolCallName: "ping", ToolCallInput: `{}`},
						{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonToolCalls},
					}, nil
				}
				return []fantasy.StreamPart{
					{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonStop},
				}, nil
			},
		}
		sa := newTestAgentWithDataDir(env, "", model, fastModel{}, pingTool)

		sess, err := env.sessions.Create(ctx, "Multi-step empty stop")
		require.NoError(t, err)

		_, err = sa.Run(ctx, SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "ping",
		})
		require.Error(t, err)
		var providerErr *fantasy.ProviderError
		require.ErrorAs(t, err, &providerErr)
		require.True(t, providerErr.IsRetryable())

		msgs, err := env.messages.List(ctx, sess.ID)
		require.NoError(t, err)
		var lastAssistant message.Message
		for _, m := range msgs {
			if m.Role == message.Assistant {
				lastAssistant = m
			}
		}
		require.Equal(t, message.FinishReasonError, lastAssistant.FinishReason())
		require.Equal(t, "Empty Response", lastAssistant.FinishPart().Message)
	})

	t.Run("FinishReasonLength with empty text is preserved as FinishReasonMaxTokens", func(t *testing.T) {
		t.Parallel()
		env := testEnv(t)
		ctx := t.Context()

		model := &recordingScriptModel{
			streamFn: func(_ int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				return []fantasy.StreamPart{
					{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonLength},
				}, nil
			},
		}
		sa := newTestAgentWithDataDir(env, "", model, fastModel{})

		sess, err := env.sessions.Create(ctx, "Max tokens empty")
		require.NoError(t, err)

		_, err = sa.Run(ctx, SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "hello",
		})
		require.NoError(t, err)

		msgs, err := env.messages.List(ctx, sess.ID)
		require.NoError(t, err)
		var lastAssistant message.Message
		for _, m := range msgs {
			if m.Role == message.Assistant {
				lastAssistant = m
			}
		}
		require.Equal(t, message.FinishReasonMaxTokens, lastAssistant.FinishReason())
	})

	t.Run("FinishReasonContentFilter with empty text is preserved as FinishReasonContentFilter", func(t *testing.T) {
		t.Parallel()
		env := testEnv(t)
		ctx := t.Context()

		model := &recordingScriptModel{
			streamFn: func(_ int, _ fantasy.Call) ([]fantasy.StreamPart, error) {
				return []fantasy.StreamPart{
					{Type: fantasy.StreamPartTypeFinish, FinishReason: fantasy.FinishReasonContentFilter},
				}, nil
			},
		}
		sa := newTestAgentWithDataDir(env, "", model, fastModel{})

		sess, err := env.sessions.Create(ctx, "Content filter empty")
		require.NoError(t, err)

		_, err = sa.Run(ctx, SessionAgentCall{
			SessionID: sess.ID,
			Prompt:    "hello",
		})
		require.NoError(t, err)

		msgs, err := env.messages.List(ctx, sess.ID)
		require.NoError(t, err)
		var lastAssistant message.Message
		for _, m := range msgs {
			if m.Role == message.Assistant {
				lastAssistant = m
			}
		}
		require.Equal(t, message.FinishReasonContentFilter, lastAssistant.FinishReason())
	})
}
