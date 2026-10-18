package config

import (
	"fmt"
	"slices"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

const ReasoningModeDefault = "default"

func NormalizeModelReasoningMode(_ catwalk.Model, mode string) (string, error) {
	if mode == ReasoningModeDefault {
		return "", nil
	}
	valid := []string{"", "none", "off", "disabled", "low", "medium", "high", "xhigh", "x-high", "extra-high", "max", "maximum"}
	if !slices.Contains(valid, mode) {
		return "", fmt.Errorf("invalid reasoning mode %q", mode)
	}
	return NormalizeReasoningMode(mode), nil
}
