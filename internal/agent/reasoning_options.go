package agent

import (
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/anthropic"
)

const (
	anthropicDefaultMaxTokens  = 32000
	anthropicMinThinkingBudget = 1024
	geminiFlashMaxBudget       = 24576
	geminiMinBudget            = 128
)

var anthropicThinkingBudgets = map[string]int64{
	config.ReasoningModeLow:    4096,
	config.ReasoningModeMedium: 10240,
	config.ReasoningModeHigh:   16384,
	config.ReasoningModeXHigh:  24576,
	config.ReasoningModeMax:    32000,
}

var geminiThinkingBudgets = map[string]int64{
	config.ReasoningModeLow:    1024,
	config.ReasoningModeMedium: 8192,
	config.ReasoningModeHigh:   16384,
	config.ReasoningModeXHigh:  24576,
	config.ReasoningModeMax:    32768,
}

func providerReasoningEffort(model Model, mode string) string {
	if levels := model.CatwalkCfg.ReasoningLevels; len(levels) > 0 {
		return config.ReasoningLevelFor(mode, levels)
	}
	return mapModeToProviderEffort(mode)
}

func openAIReasoningEffort(model Model, mode string) string {
	if levels := model.CatwalkCfg.ReasoningLevels; len(levels) > 0 {
		return config.ReasoningLevelFor(mode, levels)
	}
	return mapModeToOpenAIEffort(mode)
}

func modelMaxOutputTokens(model Model) int64 {
	if model.ModelCfg.MaxTokens > 0 {
		return model.ModelCfg.MaxTokens
	}
	return model.CatwalkCfg.DefaultMaxTokens
}

func anthropicThinkingBudget(model Model, mode string) int64 {
	budget, ok := anthropicThinkingBudgets[mode]
	if !ok {
		budget = anthropicThinkingBudgets[config.ReasoningModeMedium]
	}
	maxTokens := modelMaxOutputTokens(model)
	if maxTokens <= 0 {
		maxTokens = anthropicDefaultMaxTokens
	}
	budget = min(budget, maxTokens-anthropicMinThinkingBudget)
	return max(budget, anthropicMinThinkingBudget)
}

func anthropicSupportsEffort(model Model) bool {
	return len(model.CatwalkCfg.ReasoningLevels) > 0 || anthropic.SupportsAdaptiveThinking(model.CatwalkCfg.ID)
}

func googleThinkingConfig(model Model, mode string) map[string]any {
	modelID := strings.ToLower(model.CatwalkCfg.ID)
	if strings.Contains(modelID, "gemini-2") {
		return map[string]any{
			"thinking_budget":  geminiThinkingBudget(modelID, mode),
			"include_thoughts": thinkingEnabled(mode),
		}
	}

	levels := model.CatwalkCfg.ReasoningLevels
	level := openAIReasoningEffort(model, mode)
	if !thinkingEnabled(mode) {
		level = config.ReasoningOffLevel(levels)
		if level == "" && len(levels) > 0 {
			level = config.ReasoningLevelFor(config.ReasoningModeLow, levels)
		}
	}
	thinkingConfig := map[string]any{"include_thoughts": true}
	if level != "" {
		thinkingConfig["thinking_level"] = level
	}
	return thinkingConfig
}

func geminiThinkingBudget(modelID, mode string) int64 {
	isFlash := strings.Contains(modelID, "flash")
	if !thinkingEnabled(mode) {
		if isFlash {
			return 0
		}
		return geminiMinBudget
	}
	budget, ok := geminiThinkingBudgets[mode]
	if !ok {
		budget = geminiThinkingBudgets[config.ReasoningModeMedium]
	}
	if isFlash {
		budget = min(budget, geminiFlashMaxBudget)
	}
	return budget
}
