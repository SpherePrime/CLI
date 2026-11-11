package agent

import (
	"context"

	"github.com/SpherePrime/CLI/internal/config"
)

// lastReasoningKey holds the reasoning mode of the latest run that carried
// one, so runs started without the user's choice (continuations, goals,
// workers of a session that has none stored yet) keep the user's level.
const lastReasoningKey = "last-reasoning-mode"

func (c *coordinator) withTeamReasoning(ctx context.Context, sessionID string) context.Context {
	if mode, exists := runReasoningMode(ctx); exists {
		c.teamReasoning.Store(sessionID, mode)
		c.teamReasoning.Store(lastReasoningKey, mode)
		return ctx
	}
	if mode := c.teamReasoningMode(sessionID); mode != "" {
		return WithReasoningMode(ctx, mode)
	}
	return ctx
}

func (c *coordinator) teamReasoningMode(rootSessionID string) string {
	if stored, ok := c.teamReasoning.Load(rootSessionID); ok {
		return stored.(string)
	}
	if stored, ok := c.teamReasoning.Load(lastReasoningKey); ok {
		return stored.(string)
	}
	return ""
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
