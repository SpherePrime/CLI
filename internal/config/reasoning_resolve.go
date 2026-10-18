package config

import (
	"slices"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

var reasoningLevelRank = map[string]int{
	"none":              0,
	"minimal":           1,
	ReasoningModeLow:    2,
	ReasoningModeMedium: 3,
	ReasoningModeHigh:   4,
	ReasoningModeXHigh:  5,
	ReasoningModeMax:    6,
}

func isKnownReasoningValue(value string) bool {
	switch value {
	case "none", "off", "disabled", "x-high", "extra-high", "maximum":
		return true
	}
	_, ok := reasoningLevelRank[value]
	return ok && value != "minimal"
}

func (s SelectedModel) HasExplicitReasoningMode() bool {
	return isKnownReasoningValue(s.ReasoningMode) || s.Think || isKnownReasoningValue(s.ReasoningEffort)
}

func (s SelectedModel) ResolveReasoningMode(model catwalk.Model) string {
	if isKnownReasoningValue(s.ReasoningMode) {
		return NormalizeReasoningMode(s.ReasoningMode)
	}
	if s.Think {
		return ReasoningModeMedium
	}
	if isKnownReasoningValue(s.ReasoningEffort) {
		return NormalizeReasoningMode(s.ReasoningEffort)
	}
	if !model.CanReason || len(model.ReasoningLevels) == 0 {
		return ReasoningModeOff
	}
	if model.DefaultReasoningEffort != "" && slices.Contains(model.ReasoningLevels, model.DefaultReasoningEffort) {
		return NormalizeReasoningMode(model.DefaultReasoningEffort)
	}
	for _, level := range model.ReasoningLevels {
		if mode := NormalizeReasoningMode(level); mode != ReasoningModeOff {
			return mode
		}
	}
	return ReasoningModeOff
}

func ReasoningLevelFor(mode string, levels []string) string {
	mode = NormalizeReasoningMode(mode)
	if mode == ReasoningModeOff {
		return ""
	}
	if len(levels) == 0 || slices.Contains(levels, mode) {
		return mode
	}
	requested := reasoningLevelRank[mode]
	best, bestRank := "", -1
	lowest, lowestRank := "", len(reasoningLevelRank)
	for _, level := range levels {
		rank, ok := reasoningLevelRank[level]
		if !ok || rank < reasoningLevelRank[ReasoningModeLow] {
			continue
		}
		if rank <= requested && rank > bestRank {
			best, bestRank = level, rank
		}
		if rank < lowestRank {
			lowest, lowestRank = level, rank
		}
	}
	if best != "" {
		return best
	}
	return lowest
}

func ReasoningOffLevel(levels []string) string {
	for _, level := range []string{"none", "minimal"} {
		if slices.Contains(levels, level) {
			return level
		}
	}
	return ""
}
