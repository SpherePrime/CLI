package agent

import (
	"context"
	"errors"
	"strings"

	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/message"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

const (
	ContextSystemPrompt    = "system_prompt"
	ContextMCPInstructions = "mcp_instructions"
	ContextTools           = "tools"
	ContextMCPTools        = "mcp_tools"
	ContextSummary         = "summary"
	ContextUserMessages    = "user_messages"
	ContextAssistant       = "assistant_messages"
	ContextReasoning       = "reasoning"
	ContextToolCalls       = "tool_calls"
	ContextToolResults     = "tool_results"
	ContextAttachments     = "attachments"
)

var contextCategoryOrder = []string{
	ContextSystemPrompt,
	ContextMCPInstructions,
	ContextTools,
	ContextMCPTools,
	ContextSummary,
	ContextUserMessages,
	ContextAssistant,
	ContextReasoning,
	ContextToolCalls,
	ContextToolResults,
	ContextAttachments,
}

type ContextCategory struct {
	Key    string
	Tokens int64
	Items  int
}

type ContextBreakdown struct {
	Categories           []ContextCategory
	EstimatedTotal       int64
	ReportedTotal        int64
	ContextWindow        int64
	AutoSummarizePercent int
	EstimatedUsage       bool
}

type ContextReporter interface {
	ContextBreakdown(ctx context.Context, sessionID string) (ContextBreakdown, error)
}

func (c *coordinator) ContextBreakdown(ctx context.Context, sessionID string) (ContextBreakdown, error) {
	reporter, ok := c.currentAgent().(ContextReporter)
	if !ok {
		return ContextBreakdown{}, errors.New("context breakdown is not available")
	}
	return reporter.ContextBreakdown(ctx, sessionID)
}

func (a *sessionAgent) ContextBreakdown(ctx context.Context, sessionID string) (ContextBreakdown, error) {
	totals := map[string]*ContextCategory{}
	add := func(key string, tokens int64) {
		if tokens <= 0 {
			return
		}
		category, ok := totals[key]
		if !ok {
			category = &ContextCategory{Key: key}
			totals[key] = category
		}
		category.Tokens += tokens
		category.Items++
	}

	add(ContextSystemPrompt, approxTokenCount(a.systemPrompt.Get())+approxTokenCount(a.systemPromptPrefix.Get()))
	for _, server := range mcp.GetStates() {
		if server.State != mcp.StateConnected || server.Client == nil {
			continue
		}
		if result := server.Client.InitializeResult(); result != nil {
			add(ContextMCPInstructions, approxTokenCount(result.Instructions))
		}
	}
	for _, tool := range a.tools.Copy() {
		key := ContextTools
		if strings.HasPrefix(tool.Info().Name, "mcp_") {
			key = ContextMCPTools
		}
		add(key, estimateToolDefinitionTokens([]fantasy.AgentTool{tool}))
	}

	currentSession, err := a.sessions.Get(ctx, sessionID)
	if err != nil {
		return ContextBreakdown{}, err
	}
	msgs, err := a.getSessionMessages(ctx, currentSession)
	if err != nil {
		return ContextBreakdown{}, err
	}
	for index, msg := range msgs {
		if index == 0 && currentSession.SummaryMessageID != "" && msg.ID == currentSession.SummaryMessageID {
			add(ContextSummary, approxTokenCount(msg.Content().Text))
			continue
		}
		addMessageParts(msg, add)
	}

	breakdown := ContextBreakdown{
		ReportedTotal:  currentSession.PromptTokens + currentSession.CompletionTokens,
		EstimatedUsage: currentSession.EstimatedUsage,
	}
	largeModel := a.largeModel.Get()
	breakdown.ContextWindow = int64(largeModel.CatwalkCfg.ContextWindow)
	breakdown.AutoSummarizePercent = a.autoSummarizePercent
	if largeModel.ModelCfg.AutoSummarizePercent > 0 {
		breakdown.AutoSummarizePercent = largeModel.ModelCfg.AutoSummarizePercent
	}
	for _, key := range contextCategoryOrder {
		if category, ok := totals[key]; ok {
			breakdown.Categories = append(breakdown.Categories, *category)
			breakdown.EstimatedTotal += category.Tokens
		}
	}
	return breakdown, nil
}

func addMessageParts(msg message.Message, add func(string, int64)) {
	textKey := ContextAssistant
	if msg.Role == message.User {
		textKey = ContextUserMessages
	}
	for _, part := range msg.Parts {
		switch p := part.(type) {
		case message.TextContent:
			add(textKey, approxTokenCount(p.Text))
		case message.ReasoningContent:
			add(ContextReasoning, approxTokenCount(p.Thinking))
		case message.ToolCall:
			add(ContextToolCalls, estimateToolCallTokens(p.Name, p.Input))
		case message.ToolResult:
			add(ContextToolResults, approxTokenCount(p.Content)+approxTokenCount(p.Metadata))
			if p.Data != "" {
				add(ContextAttachments, estimateMediaTokens(p.MIMEType, "", len(p.Data)))
			}
		case message.BinaryContent:
			add(ContextAttachments, estimateMediaTokens(p.MIMEType, p.Path, len(p.Data)))
		case message.ImageURLContent:
			add(ContextAttachments, approxTokenCount(p.URL))
		}
	}
}
