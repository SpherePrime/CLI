package agent

import (
	"context"

	"github.com/SpherePrime/CLI/internal/config"
)

func (c *coordinator) withTeamReasoning(ctx context.Context, sessionID string) context.Context {
	if mode, exists := runReasoningMode(ctx); exists {
		c.teamReasoning.Store(sessionID, mode)
		return ctx
	}
	if stored, ok := c.teamReasoning.Load(sessionID); ok {
		return WithReasoningMode(ctx, stored.(string))
	}
	return ctx
}

func (c *coordinator) teamReasoningMode(rootSessionID string) string {
	stored, ok := c.teamReasoning.Load(rootSessionID)
	if !ok {
		return ""
	}
	return stored.(string)
}

func (c *coordinator) teamWorkerModel(ctx context.Context, rootSessionID string, model Model, provider config.ProviderConfig) (Model, bool, error) {
	mode := c.teamReasoningMode(rootSessionID)
	if mode == "" || mode == config.ReasoningModeDefault {
		return model, false, nil
	}
	tuned, err := c.applyRunReasoning(WithReasoningMode(ctx, mode), model, provider)
	if err != nil {
		return model, false, err
	}
	return tuned, true, nil
}
