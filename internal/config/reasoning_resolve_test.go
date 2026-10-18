package config

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestResolveReasoningMode(t *testing.T) {
	t.Parallel()

	catalogModel := catwalk.Model{
		CanReason:              true,
		ReasoningLevels:        []string{"low", "medium", "high"},
		DefaultReasoningEffort: "high",
	}

	tests := []struct {
		name     string
		selected SelectedModel
		model    catwalk.Model
		want     string
	}{
		{name: "unset uses catalog default", model: catalogModel, want: ReasoningModeHigh},
		{name: "explicit off is honored", selected: SelectedModel{ReasoningMode: ReasoningModeOff}, model: catalogModel, want: ReasoningModeOff},
		{name: "explicit level is honored", selected: SelectedModel{ReasoningMode: ReasoningModeLow}, model: catalogModel, want: ReasoningModeLow},
		{name: "legacy effort is honored", selected: SelectedModel{ReasoningEffort: "medium"}, model: catalogModel, want: ReasoningModeMedium},
		{name: "model without reasoning", model: catwalk.Model{}, want: ReasoningModeOff},
		{name: "invalid value uses catalog default", selected: SelectedModel{ReasoningEffort: "invalid"}, model: catalogModel, want: ReasoningModeHigh},
		{name: "custom model honors explicit level", selected: SelectedModel{ReasoningMode: ReasoningModeMax}, model: catwalk.Model{ID: "custom"}, want: ReasoningModeMax},
		{name: "default outside levels uses first usable level", model: catwalk.Model{CanReason: true, ReasoningLevels: []string{"minimal", "low"}, DefaultReasoningEffort: "max"}, want: ReasoningModeLow},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tc.selected.ResolveReasoningMode(tc.model))
		})
	}
}

func TestReasoningLevelFor(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mode   string
		levels []string
		want   string
	}{
		{name: "off", mode: ReasoningModeOff, levels: []string{"low"}, want: ""},
		{name: "no catalog levels", mode: ReasoningModeMax, want: ReasoningModeMax},
		{name: "supported level", mode: ReasoningModeXHigh, levels: []string{"low", "high", "xhigh"}, want: ReasoningModeXHigh},
		{name: "clamps down to highest supported", mode: ReasoningModeMax, levels: []string{"low", "medium", "high"}, want: ReasoningModeHigh},
		{name: "skips unsupported middle level", mode: ReasoningModeXHigh, levels: []string{"low", "high", "max"}, want: ReasoningModeHigh},
		{name: "raises to lowest usable level", mode: ReasoningModeLow, levels: []string{"minimal", "high", "max"}, want: ReasoningModeHigh},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, ReasoningLevelFor(tc.mode, tc.levels))
		})
	}
}

func TestReasoningOffLevel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "none", ReasoningOffLevel([]string{"none", "minimal", "low"}))
	require.Equal(t, "minimal", ReasoningOffLevel([]string{"minimal", "low"}))
	require.Equal(t, "", ReasoningOffLevel([]string{"low", "high"}))
}
