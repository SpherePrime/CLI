package agent

import "github.com/SpherePrime/CLI/vendordeps/fantasy"

const (
	minContextCalibration = 0.25
	maxContextCalibration = 4.0
)

func contextCalibrationRatio(reportedPromptTokens, estimatedPromptTokens int64) (float64, bool) {
	if reportedPromptTokens <= 0 || estimatedPromptTokens <= 0 {
		return 0, false
	}
	ratio := float64(reportedPromptTokens) / float64(estimatedPromptTokens)
	if ratio < minContextCalibration || ratio > maxContextCalibration {
		return 0, false
	}
	return ratio, true
}

func (a *sessionAgent) recordContextCalibration(sessionID string, usage fantasy.Usage, messages []fantasy.Message, toolTokens int64) {
	if a.contextCalibrations == nil || sessionID == "" {
		return
	}
	ratio, ok := contextCalibrationRatio(promptTokensFromUsage(usage), estimateMessageTokens(messages)+toolTokens)
	if !ok {
		return
	}
	a.contextCalibrations.Set(sessionID, ratio)
}

func (a *sessionAgent) contextCalibration(sessionID string) (float64, bool) {
	if a.contextCalibrations == nil {
		return 0, false
	}
	return a.contextCalibrations.Get(sessionID)
}

func calibrateContextCategories(categories []ContextCategory, ratio float64) ([]ContextCategory, int64) {
	calibrated := make([]ContextCategory, len(categories))
	var total int64
	for i, category := range categories {
		category.Tokens = int64(float64(category.Tokens)*ratio + 0.5)
		calibrated[i] = category
		total += category.Tokens
	}
	return calibrated, total
}
