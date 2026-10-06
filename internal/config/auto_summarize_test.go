package config

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func ptrInt(v int) *int { return &v }

func TestGetAutoSummarizePercentDefaults(t *testing.T) {
	t.Parallel()

	for name, opts := range map[string]*Options{
		"nil options": nil,
		"empty":       {},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, DefaultAutoSummarizePercent, opts.GetAutoSummarizePercent())
		})
	}
}

func TestGetAutoSummarizePercentUsesConfiguredValue(t *testing.T) {
	t.Parallel()

	opts := &Options{AutoSummarizePercent: ptrInt(70)}
	require.Equal(t, 70, opts.GetAutoSummarizePercent())
}

// Zero means "unset" in the config file, so it must resolve to the default
// rather than clamping to the minimum.
func TestGetAutoSummarizePercentTreatsZeroAsUnset(t *testing.T) {
	t.Parallel()

	opts := &Options{AutoSummarizePercent: ptrInt(0)}
	require.Equal(t, DefaultAutoSummarizePercent, opts.GetAutoSummarizePercent())
}

func TestGetAutoSummarizePercentClampsPercentages(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		in   int
		want int
	}{
		{"unset takes the default", 0, DefaultAutoSummarizePercent},
		{"at or above the ceiling", 100, 100},
		{"in range", 88, 88},
		{"negative takes the default", -5, DefaultAutoSummarizePercent},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			require.Equal(t, tc.want, (&Options{AutoSummarizePercent: ptrInt(tc.in)}).GetAutoSummarizePercent())
		})
	}
}

// A per-model override recorded by the model settings dialog beats the global
// default; absent or zero entries fall back to it.
func TestAutoSummarizePercentFor(t *testing.T) {
	t.Parallel()

	global := ptrInt(60)
	cfg := &Config{Options: &Options{AutoSummarizePercent: global}}

	// No entry at all: the global option applies.
	require.Equal(t, 60, cfg.AutoSummarizePercentFor("openai", "gpt-4o"))

	// A stored override wins.
	cfg.ModelSettings = map[string]ModelSettings{
		modelSettingsKey("openai", "gpt-4o"): {AutoSummarizePercent: 90},
	}
	require.Equal(t, 90, cfg.AutoSummarizePercentFor("openai", "gpt-4o"))

	// An entry without a percent does not shadow the global option.
	cfg.ModelSettings[modelSettingsKey("openai", "gpt-4o")] = ModelSettings{}
	require.Equal(t, 60, cfg.AutoSummarizePercentFor("openai", "gpt-4o"))
}

func TestAutoSummarizePercentForClampsOutOfRange(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Options: &Options{},
		ModelSettings: map[string]ModelSettings{
			modelSettingsKey("openai", "gpt-4o"): {AutoSummarizePercent: 150},
		},
	}
	require.Equal(t, 100, cfg.AutoSummarizePercentFor("openai", "gpt-4o"))
}
