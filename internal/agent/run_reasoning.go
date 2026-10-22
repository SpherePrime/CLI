package agent

import (
	"context"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/anthropic"
)

type runReasoningKey struct{}

func WithReasoningMode(ctx context.Context, mode string) context.Context {
	if mode == "" {
		return ctx
	}
	return context.WithValue(ctx, runReasoningKey{}, mode)
}

func applyRunReasoningMode(ctx context.Context, model config.SelectedModel) config.SelectedModel {
	mode, exists := ctx.Value(runReasoningKey{}).(string)
	if !exists {
		return model
	}
	model.Think = false
	model.ReasoningEffort = ""
	model.ReasoningMode = ""
	if mode != config.ReasoningModeDefault {
		model.ReasoningMode = config.NormalizeReasoningMode(mode)
	}
	return model
}

func (c *coordinator) applyRunReasoning(ctx context.Context, model Model, provider config.ProviderConfig) (Model, error) {
	if _, exists := ctx.Value(runReasoningKey{}).(string); !exists {
		return model, nil
	}
	model.ModelCfg = applyRunReasoningMode(ctx, model.ModelCfg)
	if provider.Type == anthropic.Name {
		localProvider, err := c.buildProvider(provider, model.ModelCfg, false)
		if err != nil {
			return Model{}, err
		}
		localModel, err := localProvider.LanguageModel(ctx, model.Model.Model())
		if err != nil {
			return Model{}, err
		}
		model.Model = newReasoningCompatModel(newToolRoleCompatModel(newRequestTimeoutModel(localModel, c.cfg.Config().Options.GetRequestTimeout())))
	}
	return model, nil
}
