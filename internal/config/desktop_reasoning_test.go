package config

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
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
	if mode, err := NormalizeModelReasoningMode(catwalk.Model{}, "high"); err != nil || mode != ReasoningModeHigh {
		t.Fatalf("custom model reasoning rejected: %q %v", mode, err)
	}
	if mode, err := NormalizeModelReasoningMode(model, ReasoningModeDefault); err != nil || mode != "" {
		t.Fatalf("default reasoning mode not cleared: %q %v", mode, err)
	}
}
