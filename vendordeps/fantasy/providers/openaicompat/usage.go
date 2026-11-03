package openaicompat

import (
	"encoding/json"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openai"
	openaisdk "github.com/SpherePrime/CLI/vendordeps/openai/openai-go/v3"
)

func compatCacheUsage(usage fantasy.Usage, raw string, promptTokens int64) fantasy.Usage {
	var details struct {
		CacheHitTokens *int64 `json:"prompt_cache_hit_tokens"`
		PromptDetails  struct {
			CachedTokens *int64 `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	}
	if json.Unmarshal([]byte(raw), &details) != nil || details.PromptDetails.CachedTokens != nil || details.CacheHitTokens == nil {
		return usage
	}
	cachedTokens := *details.CacheHitTokens
	if cachedTokens < 0 || cachedTokens > promptTokens {
		return usage
	}
	usage.CacheReadTokens = cachedTokens
	usage.InputTokens = promptTokens - cachedTokens
	return usage
}

func compatUsage(response openaisdk.ChatCompletion) (fantasy.Usage, fantasy.ProviderOptionsData) {
	usage, metadata := openai.DefaultUsageFunc(response)
	return compatCacheUsage(usage, response.Usage.RawJSON(), response.Usage.PromptTokens), metadata
}

func compatStreamUsage(chunk openaisdk.ChatCompletionChunk, ctx map[string]any, metadata fantasy.ProviderMetadata) (fantasy.Usage, fantasy.ProviderMetadata) {
	usage, metadata := openai.DefaultStreamUsageFunc(chunk, ctx, metadata)
	if chunk.Usage.TotalTokens == 0 {
		return usage, metadata
	}
	return compatCacheUsage(usage, chunk.Usage.RawJSON(), chunk.Usage.PromptTokens), metadata
}
