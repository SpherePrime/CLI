package agent

import (
	"context"
	"fmt"

	"github.com/SpherePrime/CLI/internal/config"
)

type reviewDepth string

const (
	reviewDepthLight    reviewDepth = "light"
	reviewDepthQuick    reviewDepth = "quick"
	reviewDepthThorough reviewDepth = "thorough"
	reviewDepthStrict   reviewDepth = "strict"
)

func reviewDepthFor(mode string) reviewDepth {
	if mode == "" || mode == config.ReasoningModeDefault {
		return reviewDepthQuick
	}
	switch config.NormalizeReasoningMode(mode) {
	case config.ReasoningModeOff, config.ReasoningModeLow:
		return reviewDepthLight
	case config.ReasoningModeXHigh:
		return reviewDepthThorough
	case config.ReasoningModeMax:
		if desktopHost() {
			return reviewDepthStrict
		}
		return reviewDepthThorough
	default:
		return reviewDepthQuick
	}
}

func (d reviewDepth) rounds() int {
	switch d {
	case reviewDepthLight:
		return 1
	case reviewDepthStrict:
		return 3
	default:
		return 2
	}
}

func (d reviewDepth) summary() string {
	switch d {
	case reviewDepthLight:
		return "A low reasoning level is selected: one fast pass, report only critical defects."
	case reviewDepthThorough:
		return "A high reasoning level is selected: check carefully, report critical and major defects, list minor ones without blocking."
	case reviewDepthStrict:
		return "Maximum reasoning is selected: check as strictly as possible."
	default:
		return "Check fast and focused, report only critical and major defects, never redo a check whose result cannot have changed."
	}
}

func (d reviewDepth) note() string {
	rounds := "rounds"
	if d.rounds() == 1 {
		rounds = "round"
	}
	return fmt.Sprintf("\n\nREVIEW DEPTH: %s. %s A part gets at most %d review %s; after that only critical defects go back to the author.", d, d.summary(), d.rounds(), rounds)
}

func coordinatorReasoningMode(ctx context.Context, model config.SelectedModel) string {
	if mode, exists := runReasoningMode(ctx); exists {
		return mode
	}
	return model.EffectiveReasoningMode()
}

func (c *coordinator) teamReviewDepth(root string) reviewDepth {
	return reviewDepthFor(c.teamReasoningMode(root))
}

func (c *coordinator) reviewRoundLimit(root string) int {
	return c.teamReviewDepth(root).rounds()
}

func (c *coordinator) reviewDepthNote(root string) string {
	return c.teamReviewDepth(root).note()
}
