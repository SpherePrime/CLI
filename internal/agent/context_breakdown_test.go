package agent

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type stubContextTool struct {
	info fantasy.ToolInfo
}

func (s *stubContextTool) Info() fantasy.ToolInfo {
	return s.info
}

func (s *stubContextTool) Run(context.Context, fantasy.ToolCall) (fantasy.ToolResponse, error) {
	return fantasy.NewTextResponse("ok"), nil
}

func (s *stubContextTool) ProviderOptions() fantasy.ProviderOptions {
	return nil
}

func (s *stubContextTool) SetProviderOptions(fantasy.ProviderOptions) {}

func TestAddMessagePartsCategorizesHistory(t *testing.T) {
	t.Parallel()

	totals := map[string]int64{}
	add := func(key string, tokens int64) { totals[key] += tokens }

	addMessageParts(message.Message{Role: message.User, Parts: []message.ContentPart{
		message.TextContent{Text: "12345678"},
		message.BinaryContent{MIMEType: "image/png", Data: make([]byte, 40)},
	}}, add)
	addMessageParts(message.Message{Role: message.Assistant, Parts: []message.ContentPart{
		message.ReasoningContent{Thinking: "1234"},
		message.TextContent{Text: "1234"},
		message.ToolCall{Name: "view", Input: "{}"},
	}}, add)
	addMessageParts(message.Message{Role: message.Tool, Parts: []message.ContentPart{
		message.ToolResult{Content: "12345678"},
	}}, add)

	require.Equal(t, int64(2), totals[ContextUserMessages])
	require.Equal(t, int64(1), totals[ContextAssistant])
	require.Equal(t, int64(1), totals[ContextReasoning])
	require.Positive(t, totals[ContextToolCalls])
	require.Equal(t, int64(2), totals[ContextToolResults])
	require.Positive(t, totals[ContextAttachments])
}

func TestContextBreakdown_ReportedTotal(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	sa := testSessionAgent(env, nil, nil, "system prompt instructions")
	agent := sa.(*sessionAgent)
	ctx := t.Context()

	t.Run("zero when estimated usage is true", func(t *testing.T) {
		sess, err := env.sessions.Create(ctx, "estimated")
		require.NoError(t, err)
		sess.PromptTokens = 500
		sess.CompletionTokens = 200
		sess.EstimatedUsage = true
		_, err = env.sessions.Save(ctx, sess)
		require.NoError(t, err)

		breakdown, err := agent.ContextBreakdown(ctx, sess.ID)
		require.NoError(t, err)
		require.Zero(t, breakdown.ReportedTotal)
		require.True(t, breakdown.EstimatedUsage)
		require.Positive(t, breakdown.EstimatedTotal)
	})

	t.Run("zero when prompt tokens is zero", func(t *testing.T) {
		sess, err := env.sessions.Create(ctx, "zero prompt")
		require.NoError(t, err)
		sess.PromptTokens = 0
		sess.CompletionTokens = 5000
		sess.EstimatedUsage = false
		_, err = env.sessions.Save(ctx, sess)
		require.NoError(t, err)

		breakdown, err := agent.ContextBreakdown(ctx, sess.ID)
		require.NoError(t, err)
		require.Zero(t, breakdown.ReportedTotal)
		require.False(t, breakdown.EstimatedUsage)
	})

	t.Run("sum when reported and prompt tokens positive", func(t *testing.T) {
		sess, err := env.sessions.Create(ctx, "reported")
		require.NoError(t, err)
		sess.PromptTokens = 1200
		sess.CompletionTokens = 300
		sess.EstimatedUsage = false
		_, err = env.sessions.Save(ctx, sess)
		require.NoError(t, err)

		breakdown, err := agent.ContextBreakdown(ctx, sess.ID)
		require.NoError(t, err)
		require.Equal(t, int64(1500), breakdown.ReportedTotal)
		require.False(t, breakdown.EstimatedUsage)
	})
}

func TestContextBreakdown_PostCompactionMatchesBaseAndSummary(t *testing.T) {
	t.Parallel()

	env := testEnv(t)
	builtInTool := &stubContextTool{
		info: fantasy.ToolInfo{
			Name:        "view",
			Description: "read file contents",
			Parameters:  map[string]any{"file_path": map[string]any{"type": "string"}},
		},
	}
	mcpTool := &stubContextTool{
		info: fantasy.ToolInfo{
			Name:        "mcp_docs_lookup",
			Description: "search docs",
			Parameters:  map[string]any{"query": map[string]any{"type": "string"}},
		},
	}
	sa := testSessionAgent(env, nil, nil, "system prompt instructions for post compaction", builtInTool, mcpTool)
	agent := sa.(*sessionAgent)
	ctx := t.Context()

	sess, err := env.sessions.Create(ctx, "compacted")
	require.NoError(t, err)

	_, err = env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:  message.User,
		Parts: []message.ContentPart{message.TextContent{Text: "old message before summary"}},
	})
	require.NoError(t, err)

	summaryText := "compacted summary of prior work and active tasks"
	summaryMsg, err := env.messages.Create(ctx, sess.ID, message.CreateMessageParams{
		Role:             message.Assistant,
		IsSummaryMessage: true,
		Parts: []message.ContentPart{
			message.ReasoningContent{Thinking: "internal reasoning should not be counted"},
			message.TextContent{Text: summaryText},
		},
	})
	require.NoError(t, err)

	baseTokens := agent.estimateBaseContextTokens()
	summaryTokens := approxTokenCount(summaryText)
	sess.SummaryMessageID = summaryMsg.ID
	sess.PromptTokens = baseTokens
	sess.CompletionTokens = summaryTokens
	sess.ContextTokens = baseTokens + summaryTokens
	sess.EstimatedUsage = true
	_, err = env.sessions.Save(ctx, sess)
	require.NoError(t, err)

	breakdown, err := agent.ContextBreakdown(ctx, sess.ID)
	require.NoError(t, err)
	require.Zero(t, breakdown.ReportedTotal)
	require.True(t, breakdown.EstimatedUsage)
	require.False(t, breakdown.Calibrated)
	require.Equal(t, baseTokens+summaryTokens, breakdown.EstimatedTotal)

	byKey := map[string]ContextCategory{}
	for _, cat := range breakdown.Categories {
		byKey[cat.Key] = cat
	}
	require.Contains(t, byKey, ContextSystemPrompt)
	require.Contains(t, byKey, ContextTools)
	require.Contains(t, byKey, ContextMCPTools)
	require.Equal(t, summaryTokens, byKey[ContextSummary].Tokens)
	require.NotContains(t, byKey, ContextReasoning)
}
