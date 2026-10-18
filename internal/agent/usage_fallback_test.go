package agent

import (
	"errors"
	"fmt"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestUsageIsZero(t *testing.T) {
	t.Parallel()

	require.True(t, usageIsZero(fantasy.Usage{}))
	require.False(t, usageIsZero(fantasy.Usage{InputTokens: 1}))
	require.False(t, usageIsZero(fantasy.Usage{OutputTokens: 1}))
	require.False(t, usageIsZero(fantasy.Usage{TotalTokens: 1}))
	require.False(t, usageIsZero(fantasy.Usage{ReasoningTokens: 1}))
	require.False(t, usageIsZero(fantasy.Usage{CacheCreationTokens: 1}))
	require.False(t, usageIsZero(fantasy.Usage{CacheReadTokens: 1}))
}

func TestFallbackStepUsageKeepsProviderUsage(t *testing.T) {
	t.Parallel()

	usage := fantasy.Usage{
		InputTokens:  10,
		OutputTokens: 5,
		TotalTokens:  15,
	}
	step := fantasy.StepResult{
		Response: fantasy.Response{Usage: usage},
	}

	fallbackUsage, estimated := fallbackStepUsage(nil, step)
	require.False(t, estimated)
	require.Equal(t, usage, fallbackUsage)
}

func TestFallbackStepUsageEstimatesPromptAndAssistantText(t *testing.T) {
	t.Parallel()

	messages := []fantasy.Message{
		fantasy.NewUserMessage("please explain the implementation details"),
	}
	step := fantasy.StepResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.TextContent{Text: "the implementation stores state safely"},
			},
		},
	}

	usage, estimated := fallbackStepUsage(messages, step)
	require.True(t, estimated)
	require.Positive(t, usage.InputTokens)
	require.Positive(t, usage.OutputTokens)
	require.Equal(t, usage.InputTokens+usage.OutputTokens, usage.TotalTokens)
}

func TestFallbackStepUsageEstimatesReasoning(t *testing.T) {
	t.Parallel()

	messages := []fantasy.Message{
		{
			Role: fantasy.MessageRoleAssistant,
			Content: []fantasy.MessagePart{
				fantasy.ReasoningPart{Text: "first reason about the request"},
			},
		},
	}
	step := fantasy.StepResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.ReasoningContent{Text: "second reason about the answer"},
			},
		},
	}

	usage, estimated := fallbackStepUsage(messages, step)
	require.True(t, estimated)
	require.Positive(t, usage.InputTokens)
	require.Positive(t, usage.OutputTokens)
}

func TestFallbackStepUsageEstimatesToolCalls(t *testing.T) {
	t.Parallel()

	step := fantasy.StepResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.ToolCallContent{
					ToolCallID: "tool-call-1",
					ToolName:   "view",
					Input:      `{"file_path":"/tmp/example.go"}`,
				},
			},
		},
	}

	usage, estimated := fallbackStepUsage(nil, step)
	require.True(t, estimated)
	require.Zero(t, usage.InputTokens)
	require.Positive(t, usage.OutputTokens)
	require.Equal(t, usage.OutputTokens, usage.TotalTokens)
}

func TestFallbackStepUsageEstimatesToolResults(t *testing.T) {
	t.Parallel()

	messages := []fantasy.Message{
		{
			Role: fantasy.MessageRoleTool,
			Content: []fantasy.MessagePart{
				fantasy.ToolResultPart{
					ToolCallID: "tool-call-1",
					Output: fantasy.ToolResultOutputContentText{
						Text: "file contents returned by the tool",
					},
				},
				fantasy.ToolResultPart{
					ToolCallID: "tool-call-2",
					Output: fantasy.ToolResultOutputContentError{
						Error: errors.New("permission denied"),
					},
				},
				fantasy.ToolResultPart{
					ToolCallID: "tool-call-3",
					Output: fantasy.ToolResultOutputContentMedia{
						MediaType: "image/png",
						Text:      "screenshot",
						Data:      "abc123",
					},
				},
			},
		},
	}

	usage, estimated := fallbackStepUsage(messages, fantasy.StepResult{})
	require.True(t, estimated)
	require.Positive(t, usage.InputTokens)
	require.Zero(t, usage.OutputTokens)
	require.Equal(t, usage.InputTokens, usage.TotalTokens)
}

func TestFallbackStepUsageSkipsClientToolResultsAsOutput(t *testing.T) {
	t.Parallel()

	step := fantasy.StepResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.ToolResultContent{
					ToolCallID: "tool-call-1",
					ToolName:   "bash",
					Result: fantasy.ToolResultOutputContentText{
						Text: "large client-executed payload that should not count as model output tokens",
					},
				},
			},
		},
	}

	usage, estimated := fallbackStepUsage(nil, step)
	require.False(t, estimated)
	require.Zero(t, usage.OutputTokens)
}

func TestFallbackStepUsageCountsProviderToolResultsAsOutput(t *testing.T) {
	t.Parallel()

	step := fantasy.StepResult{
		Response: fantasy.Response{
			Content: fantasy.ResponseContent{
				fantasy.ToolResultContent{
					ToolCallID:       "tool-call-1",
					ToolName:         "web_search",
					ProviderExecuted: true,
					ClientMetadata:   "provider metadata",
					Result:           fantasy.ToolResultOutputContentText{Text: "provider-executed result"},
				},
			},
		},
	}

	usage, estimated := fallbackStepUsage(nil, step)
	require.True(t, estimated)
	require.Positive(t, usage.OutputTokens)
	require.Equal(t, usage.OutputTokens, usage.TotalTokens)
}

func TestFallbackStepUsageReturnsZeroWithoutContent(t *testing.T) {
	t.Parallel()

	usage, estimated := fallbackStepUsage(nil, fantasy.StepResult{})
	require.False(t, estimated)
	require.True(t, usageIsZero(usage))
}

func TestShouldAutoSummarizeUsesConfiguredThreshold(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		window        int64
		percent       int
		contextTokens int64
		reserved      int64
		want          bool
	}{
		{name: "below threshold", window: 1000, percent: 85, contextTokens: 849},
		{name: "at threshold", window: 1000, percent: 85, contextTokens: 850, want: true},
		{name: "over threshold", window: 1000, percent: 70, contextTokens: 701, want: true},
		{name: "unknown window", window: 0, percent: 85, contextTokens: 900},
		{name: "unknown context", window: 1000, percent: 85, contextTokens: 0},
		{name: "output reserve lowers limit", window: 1000, percent: 90, contextTokens: 750, reserved: 250, want: true},
		{name: "output reserve below limit", window: 1000, percent: 90, contextTokens: 749, reserved: 250},
		{name: "output reserve capped at half window", window: 1000, percent: 90, contextTokens: 499, reserved: 900},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, shouldAutoSummarize(tc.window, tc.percent, tc.contextTokens, tc.reserved))
		})
	}
}

func TestUpdateSessionUsageSkipsEstimatedCost(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	currentSession := &session.Session{ID: "session-id", Cost: 1.25}
	model := Model{CatwalkCfg: catwalk.Model{CostPer1MIn: 10, CostPer1MOut: 20}}
	usage := fantasy.Usage{InputTokens: 1000, OutputTokens: 2000}

	agent.updateSessionUsage(model, currentSession, usage, nil, true)

	require.Equal(t, 1.25, currentSession.Cost)
	require.Equal(t, int64(1000), currentSession.PromptTokens)
	require.Equal(t, int64(2000), currentSession.CompletionTokens)
	require.True(t, currentSession.EstimatedUsage)
}

func TestUpdateSessionUsageKeepsCountersForZeroUsage(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	currentSession := &session.Session{
		ID:               "session-id",
		PromptTokens:     123,
		CompletionTokens: 456,
		Cost:             1.25,
	}
	model := Model{CatwalkCfg: catwalk.Model{CostPer1MIn: 10, CostPer1MOut: 20}}

	agent.updateSessionUsage(model, currentSession, fantasy.Usage{}, nil, false)

	require.Equal(t, 1.25, currentSession.Cost)
	require.Equal(t, int64(123), currentSession.PromptTokens)
	require.Equal(t, int64(456), currentSession.CompletionTokens)
}

func TestUpdateSessionUsagePreservesOmittedCountersForPartialUsage(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	currentSession := &session.Session{
		ID:               "session-id",
		PromptTokens:     123,
		CompletionTokens: 456,
	}
	model := Model{CatwalkCfg: catwalk.Model{CostPer1MIn: 10, CostPer1MOut: 20}}
	usage := fantasy.Usage{InputTokens: 789}

	agent.updateSessionUsage(model, currentSession, usage, nil, false)

	require.Equal(t, int64(789), currentSession.PromptTokens)
	require.Equal(t, int64(456), currentSession.CompletionTokens)
}

func TestUpdateSessionUsagePreservesCountersForTotalOnlyUsage(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	currentSession := &session.Session{
		ID:               "session-id",
		PromptTokens:     123,
		CompletionTokens: 456,
	}
	model := Model{CatwalkCfg: catwalk.Model{CostPer1MIn: 10, CostPer1MOut: 20}}
	usage := fantasy.Usage{TotalTokens: 100}

	agent.updateSessionUsage(model, currentSession, usage, nil, false)

	require.Equal(t, int64(123), currentSession.PromptTokens)
	require.Equal(t, int64(456), currentSession.CompletionTokens)
}

func TestUpdateSessionUsagePreservesPromptForOutputOnlyUsage(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	currentSession := &session.Session{
		ID:               "session-id",
		PromptTokens:     123,
		CompletionTokens: 456,
	}
	model := Model{CatwalkCfg: catwalk.Model{CostPer1MIn: 10, CostPer1MOut: 20}}
	usage := fantasy.Usage{OutputTokens: 50}

	agent.updateSessionUsage(model, currentSession, usage, nil, false)

	require.Equal(t, int64(123), currentSession.PromptTokens)
	require.Equal(t, int64(50), currentSession.CompletionTokens)
}

func TestUpdateSessionUsageKeepsCountersForEstimatedZeroUsage(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	currentSession := &session.Session{
		ID:               "session-id",
		PromptTokens:     123,
		CompletionTokens: 456,
		Cost:             1.25,
	}
	model := Model{CatwalkCfg: catwalk.Model{CostPer1MIn: 10, CostPer1MOut: 20}}

	agent.updateSessionUsage(model, currentSession, fantasy.Usage{}, nil, true)

	require.Equal(t, 1.25, currentSession.Cost)
	require.Equal(t, int64(123), currentSession.PromptTokens)
	require.Equal(t, int64(456), currentSession.CompletionTokens)
}

func TestSummaryCompletionTokens(t *testing.T) {
	t.Parallel()

	summaryMessage := message.Message{
		Parts: []message.ContentPart{
			message.TextContent{Text: "summary text"},
			message.ReasoningContent{Thinking: "reasoning text"},
		},
	}

	require.Equal(t, int64(42), summaryCompletionTokens(fantasy.Usage{OutputTokens: 42}, summaryMessage))
	require.Equal(t, approxTokenCount("summary text")+approxTokenCount("reasoning text"), summaryCompletionTokens(fantasy.Usage{}, summaryMessage))
	require.Zero(t, summaryCompletionTokens(fantasy.Usage{}, message.Message{}))
}

func TestUpdateSessionUsageAddsProviderCost(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	currentSession := &session.Session{ID: "session-id", Cost: 1.25}
	model := Model{CatwalkCfg: catwalk.Model{CostPer1MIn: 10, CostPer1MOut: 20}}
	usage := fantasy.Usage{InputTokens: 1000, OutputTokens: 2000}

	agent.updateSessionUsage(model, currentSession, usage, nil, false)

	require.Equal(t, 1.3, currentSession.Cost)
	require.Equal(t, int64(1000), currentSession.PromptTokens)
	require.Equal(t, int64(2000), currentSession.CompletionTokens)
	require.False(t, currentSession.EstimatedUsage)
}

func TestContextTokensFromUsageIncludesCacheAndOutput(t *testing.T) {
	t.Parallel()

	usage := fantasy.Usage{InputTokens: 100, CacheReadTokens: 2000, CacheCreationTokens: 300, OutputTokens: 50}

	require.Equal(t, int64(2400), promptTokensFromUsage(usage))
	require.Equal(t, int64(2450), contextTokensFromUsage(usage))
}

func TestUpdateSessionUsageCountsCacheCreation(t *testing.T) {
	t.Parallel()

	agent := &sessionAgent{}
	currentSession := &session.Session{ID: "session-id"}
	usage := fantasy.Usage{InputTokens: 10, CacheReadTokens: 1000, CacheCreationTokens: 500, OutputTokens: 40}

	agent.updateSessionUsage(Model{}, currentSession, usage, nil, false)

	require.Equal(t, int64(1510), currentSession.PromptTokens)
	require.Equal(t, int64(40), currentSession.CompletionTokens)
}

func TestIsContextOverflowError(t *testing.T) {
	t.Parallel()

	require.False(t, isContextOverflowError(nil))
	require.False(t, isContextOverflowError(&fantasy.ProviderError{StatusCode: 400}))
	require.True(t, isContextOverflowError(fmt.Errorf("wrapped: %w", &fantasy.ProviderError{ContextTooLargeErr: true})))
}
