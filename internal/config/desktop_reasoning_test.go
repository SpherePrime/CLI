package config

import (
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"testing"
)

func TestDesktopReasoningValidation(t *testing.T) {
	model := catwalk.Model{CanReason: true, ReasoningLevels: []string{"low", "medium", "high"}}
	for _, mode := range []string{"off", "low", "medium", "high", "xhigh", "max"} {
		if _, err := NormalizeModelReasoningMode(model, mode); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := NormalizeModelReasoningMode(model, "typo"); err == nil {
		t.Fatal("invalid reasoning silently accepted")
	}
	if _, err := NormalizeModelReasoningMode(catwalk.Model{}, "high"); err == nil {
		t.Fatal("unsupported reasoning accepted")
	}
}
