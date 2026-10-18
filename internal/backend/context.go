package backend

import (
	"context"
	"errors"

	"github.com/SpherePrime/CLI/internal/agent"
	"github.com/SpherePrime/CLI/internal/proto"
)

func (b *Backend) GetContextBreakdown(ctx context.Context, workspaceID, sessionID string) (proto.ContextBreakdown, error) {
	ws, err := b.GetWorkspace(workspaceID)
	if err != nil {
		return proto.ContextBreakdown{}, err
	}
	reporter, ok := ws.AgentCoordinator.(agent.ContextReporter)
	if ws.AgentCoordinator == nil || !ok {
		return proto.ContextBreakdown{}, errors.New("agent is not initialized")
	}
	breakdown, err := reporter.ContextBreakdown(ctx, sessionID)
	if err != nil {
		return proto.ContextBreakdown{}, err
	}
	result := proto.ContextBreakdown{
		Categories:           make([]proto.ContextCategory, 0, len(breakdown.Categories)),
		EstimatedTotal:       breakdown.EstimatedTotal,
		ReportedTotal:        breakdown.ReportedTotal,
		ContextWindow:        breakdown.ContextWindow,
		AutoSummarizePercent: breakdown.AutoSummarizePercent,
		EstimatedUsage:       breakdown.EstimatedUsage,
	}
	for _, category := range breakdown.Categories {
		result.Categories = append(result.Categories, proto.ContextCategory{
			Key:    category.Key,
			Tokens: category.Tokens,
			Items:  category.Items,
		})
	}
	return result, nil
}
