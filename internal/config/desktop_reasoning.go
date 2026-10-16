package config

import (
	"fmt"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"slices"
)

func NormalizeModelReasoningMode(model catwalk.Model, mode string) (string, error) {
	valid := []string{"", "none", "off", "disabled", "low", "medium", "high", "xhigh", "x-high", "extra-high", "max", "maximum"}
	if !slices.Contains(valid, mode) {
		return "", fmt.Errorf("invalid reasoning mode %q", mode)
	}
	mode = NormalizeReasoningMode(mode)
	if mode != ReasoningModeOff && !model.CanReason {
		return "", fmt.Errorf("model %q does not support reasoning", model.ID)
	}
	return mode, nil
}
