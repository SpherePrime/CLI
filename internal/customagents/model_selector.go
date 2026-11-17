package customagents

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

const (
	TierFast      = "fast"
	TierBalanced  = "balanced"
	TierReasoning = "reasoning"
	TierFlagship  = "flagship"
)

func ClassifyTaskTier(agent CustomAgent, prompt string) string {
	switch strings.ToLower(strings.TrimSpace(agent.AutoModelHint)) {
	case TierFast, TierBalanced, TierReasoning, TierFlagship:
		return strings.ToLower(strings.TrimSpace(agent.AutoModelHint))
	}

	lower := strings.ToLower(prompt + " " + agent.AcceptanceCriteria + " " + agent.Description)

	flagshipKeywords := []string{
		"architect", "архитект", "security", "безопасн", "root cause", "причин",
		"critical", "критич", "complex", "сложн", "multi-step", "многошаг",
		"audit", "аудит", "migrate", "миграц", "system design", "проектирован",
	}
	reasoningKeywords := []string{
		"refactor", "рефактор", "debug", "отлад", "analyze", "анализ",
		"review", "ревью", "implement", "реализ", "fix", "исправ",
		"code", "код", "algorithm", "алгоритм", "sql", "diff",
		"reason", "докаж", "verify", "верифиц", "test", "тест",
	}
	fastKeywords := []string{
		"ping", "пинг", "heartbeat", "status", "статус", "health",
		"summary", "сводк", "кратко", "route", "маршрут", "monitor", "монитор",
		"notify", "уведом", "check", "проверь",
	}

	hasFlagship := containsAnyToken(lower, flagshipKeywords)
	hasReasoning := containsAnyToken(lower, reasoningKeywords)
	hasFast := containsAnyToken(lower, fastKeywords)

	if agent.Role == RoleOrchestrator || agent.Role == RoleVerifier {
		if hasFlagship || len(prompt) > 1500 {
			return TierFlagship
		}
		return TierReasoning
	}

	if strings.TrimSpace(agent.AcceptanceCriteria) != "" || len(agent.DeterministicChecks) > 0 {
		if hasFlagship {
			return TierFlagship
		}
		return TierReasoning
	}

	if hasFlagship {
		return TierFlagship
	}
	if agent.TriggerType == TriggerMonitor && !hasReasoning {
		return TierFast
	}
	if hasFast && !hasReasoning && len(strings.TrimSpace(prompt)) < 400 {
		return TierFast
	}
	if hasReasoning {
		return TierReasoning
	}

	if agent.TriggerType == TriggerMonitor {
		return TierFast
	}
	if len(strings.TrimSpace(prompt)) > 2000 {
		return TierReasoning
	}

	return TierBalanced
}

func SelectModelForTask(models []ModelOption, agent CustomAgent, prompt string) (ResolvedModelChoice, error) {
	if len(models) == 0 {
		return ResolvedModelChoice{}, errors.New("каталог моделей пуст")
	}

	unlocked := make([]ModelOption, 0, len(models))
	for _, m := range models {
		if !m.Locked && strings.TrimSpace(m.ProviderID) != "" && strings.TrimSpace(m.ModelID) != "" {
			unlocked = append(unlocked, m)
		}
	}
	if len(unlocked) == 0 {
		return ResolvedModelChoice{}, errors.New("нет доступных разблокированных моделей")
	}

	fallbackPrefix := ""
	if agent.ModelMode == ModelModeManual && strings.TrimSpace(agent.ProviderID) != "" && strings.TrimSpace(agent.ModelID) != "" {
		for _, m := range unlocked {
			if m.ProviderID == agent.ProviderID && m.ModelID == agent.ModelID {
				reasoning := normalizeManualReasoning(m, agent.ReasoningMode)
				tier := inferModelTier(m)
				return ResolvedModelChoice{
					ProviderID:    m.ProviderID,
					ModelID:       m.ModelID,
					ReasoningMode: reasoning,
					Reason:        buildManualReason(reasoning),
					Tier:          tier,
				}, nil
			}
		}
		fallbackPrefix = "Указанная вручную модель недоступна или заблокирована; "
	}

	targetTier := ClassifyTaskTier(agent, prompt)

	best := unlocked[0]
	bestScore := scoreModelForTier(best, targetTier)
	for _, m := range unlocked[1:] {
		sc := scoreModelForTier(m, targetTier)
		if sc > bestScore || (sc == bestScore && betterTieBreak(m, best, targetTier)) {
			best = m
			bestScore = sc
		}
	}

	reasoning := selectReasoningForTier(best, targetTier)
	reason := fallbackPrefix + buildAutoReason(agent, targetTier, reasoning)

	return ResolvedModelChoice{
		ProviderID:    best.ProviderID,
		ModelID:       best.ModelID,
		ReasoningMode: reasoning,
		Reason:        reason,
		Tier:          targetTier,
	}, nil
}

func inferModelTier(m ModelOption) string {
	switch strings.ToLower(strings.TrimSpace(m.Tier)) {
	case TierFast, TierBalanced, TierReasoning, TierFlagship:
		return strings.ToLower(strings.TrimSpace(m.Tier))
	}

	id := strings.ToLower(m.ModelID + " " + m.Name)
	fastTokens := []string{"mini", "flash", "haiku", "nano", "lite", "small", "fast", "instant", "turbo", "8b", "7b", "3b"}
	flagshipTokens := []string{"opus", "o3", "o4", "gpt-5", "r1", "ultra", "flagship", "max"}
	reasoningTokens := []string{"sonnet", "pro", "reason", "thinking", "deep", "coder"}

	if containsAnyToken(id, fastTokens) {
		return TierFast
	}
	if containsAnyToken(id, flagshipTokens) {
		return TierFlagship
	}
	if containsAnyToken(id, reasoningTokens) {
		return TierReasoning
	}

	if m.CanReason {
		if m.CostPer1MIn >= 10 {
			return TierFlagship
		}
		if m.CostPer1MIn > 0 && m.CostPer1MIn <= 0.6 {
			return TierFast
		}
		return TierReasoning
	}

	if m.CostPer1MIn > 0 && m.CostPer1MIn <= 0.6 {
		return TierFast
	}
	if m.CostPer1MIn >= 10 {
		return TierFlagship
	}
	return TierBalanced
}

func scoreModelForTier(m ModelOption, targetTier string) float64 {
	modelTier := inferModelTier(m)
	score := tierCompatibilityScore(modelTier, targetTier)

	switch targetTier {
	case TierFast:
		if m.CostPer1MIn > 0 {
			score += max(0, 80-m.CostPer1MIn*10)
		} else {
			score += 20
		}
		if !m.CanReason {
			score += 10
		}
	case TierBalanced:
		if m.CanReason {
			score += 30
		}
		if m.CostPer1MIn > 0 && m.CostPer1MIn <= 5 {
			score += 25
		}
		if m.ContextWindow >= 128000 {
			score += 15
		}
	case TierReasoning:
		if m.CanReason {
			score += 150
		}
		if m.ContextWindow > 0 {
			score += min(60, float64(m.ContextWindow)/10000.0)
		}
	case TierFlagship:
		if m.CanReason {
			score += 180
		}
		if m.ContextWindow > 0 {
			score += min(80, float64(m.ContextWindow)/8000.0)
		}
		score += min(40, m.CostPer1MIn*2)
	}

	return score
}

func tierCompatibilityScore(modelTier, targetTier string) float64 {
	if modelTier == targetTier {
		return 1000
	}
	switch targetTier {
	case TierFast:
		switch modelTier {
		case TierBalanced:
			return 650
		case TierReasoning:
			return 250
		case TierFlagship:
			return 100
		}
	case TierBalanced:
		switch modelTier {
		case TierReasoning:
			return 700
		case TierFast:
			return 600
		case TierFlagship:
			return 400
		}
	case TierReasoning:
		switch modelTier {
		case TierFlagship:
			return 800
		case TierBalanced:
			return 500
		case TierFast:
			return 150
		}
	case TierFlagship:
		switch modelTier {
		case TierReasoning:
			return 800
		case TierBalanced:
			return 400
		case TierFast:
			return 100
		}
	}
	return 0
}

func betterTieBreak(a, b ModelOption, targetTier string) bool {
	if targetTier == TierFast || targetTier == TierBalanced {
		if a.CostPer1MIn != b.CostPer1MIn {
			if a.CostPer1MIn == 0 {
				return false
			}
			if b.CostPer1MIn == 0 {
				return true
			}
			return a.CostPer1MIn < b.CostPer1MIn
		}
	}
	if a.ContextWindow != b.ContextWindow {
		return a.ContextWindow > b.ContextWindow
	}
	if a.ProviderID != b.ProviderID {
		return a.ProviderID < b.ProviderID
	}
	return a.ModelID < b.ModelID
}

func supportedReasoningModes(m ModelOption) []string {
	if len(m.ReasoningModes) > 0 {
		return m.ReasoningModes
	}
	return m.ReasoningLevels
}

func normalizeManualReasoning(m ModelOption, requested string) string {
	if !m.CanReason {
		return ""
	}
	req := strings.ToLower(strings.TrimSpace(requested))
	supported := supportedReasoningModes(m)
	if req == "" {
		if m.DefaultReasoningMode != "" {
			return m.DefaultReasoningMode
		}
		if len(supported) > 0 {
			return supported[0]
		}
		return "medium"
	}
	if len(supported) == 0 {
		return req
	}
	for _, s := range supported {
		if strings.EqualFold(s, req) {
			return s
		}
	}
	if req == "off" || req == "none" {
		return "off"
	}
	if m.DefaultReasoningMode != "" {
		return m.DefaultReasoningMode
	}
	return supported[0]
}

func selectReasoningForTier(m ModelOption, targetTier string) string {
	if !m.CanReason {
		return ""
	}
	var desired []string
	switch targetTier {
	case TierFast:
		desired = []string{"low", "off", "medium"}
	case TierBalanced:
		desired = []string{"medium", "low", "high"}
	case TierReasoning:
		desired = []string{"high", "medium", "xhigh", "max"}
	case TierFlagship:
		desired = []string{"high", "xhigh", "max", "medium"}
	default:
		desired = []string{"medium"}
	}

	supported := supportedReasoningModes(m)
	if len(supported) == 0 {
		return desired[0]
	}
	for _, want := range desired {
		for _, s := range supported {
			if strings.EqualFold(s, want) {
				return s
			}
		}
	}
	if m.DefaultReasoningMode != "" && slices.Contains(supported, m.DefaultReasoningMode) {
		return m.DefaultReasoningMode
	}
	return supported[0]
}

func buildManualReason(reasoning string) string {
	if reasoning == "" {
		return "Ручной выбор модели"
	}
	return fmt.Sprintf("Ручной выбор модели (reasoning: %s)", reasoning)
}

func buildAutoReason(agent CustomAgent, targetTier, reasoning string) string {
	var desc string
	switch targetTier {
	case TierFast:
		if agent.TriggerType == TriggerMonitor {
			desc = "выбрана быстрая модель для фонового мониторинга"
		} else {
			desc = "выбрана быстрая модель для лёгкой задачи"
		}
	case TierBalanced:
		desc = "выбрана сбалансированная модель для стандартной задачи"
	case TierReasoning:
		switch agent.Role {
		case RoleVerifier:
			desc = "выбрана модель с глубоким анализом для строгой верификации"
		case RoleOrchestrator:
			desc = "выбрана модель с глубоким анализом для координации цепочки"
		default:
			desc = "выбрана модель с глубоким анализом для сложной задачи"
		}
	case TierFlagship:
		desc = "выбрана флагманская модель для критической и многошаговой задачи"
	default:
		desc = "выбрана оптимальная доступная модель"
	}
	if reasoning != "" {
		return fmt.Sprintf("Авто: %s (reasoning: %s)", desc, reasoning)
	}
	return "Авто: " + desc
}

func containsAnyToken(text string, tokens []string) bool {
	for _, tok := range tokens {
		if strings.Contains(text, tok) {
			return true
		}
	}
	return false
}
