package agent

import (
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/anthropic"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/bedrock"
)

func callMaxOutputTokens(model Model, providerType catwalk.Type) int64 {
	maxTokens := modelMaxOutputTokens(model)
	if maxTokens > 0 {
		return maxTokens
	}
	if providerType == anthropic.Name || providerType == bedrock.Name {
		return anthropicDefaultMaxTokens
	}
	return 0
}
