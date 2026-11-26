package voice

import (
	"context"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func boolPtr(value bool) *bool {
	return &value
}

func emptyProber() prober {
	return prober{
		lookPath:          func(string) (string, bool) { return "", false },
		pythonSounddevice: func(context.Context) (string, bool) { return "", false },
		pythonInterpreter: func(context.Context) (string, bool) { return "", false },
	}
}

func TestSettingsFromDefaults(t *testing.T) {
	t.Parallel()

	settings := SettingsFrom(nil, nil)
	require.True(t, settings.Enabled)
	require.Empty(t, settings.Language)
	require.Equal(t, DefaultMaxDuration, settings.MaxDurationOr())
}

func TestSettingsFromOptions(t *testing.T) {
	t.Parallel()

	settings := SettingsFrom(&config.VoiceOptions{
		Enabled:       boolPtr(false),
		Language:      "RU",
		RecordCommand: "rec -t raw -",
		MaxDuration:   45,
	}, nil)

	require.False(t, settings.Enabled)
	require.Equal(t, "ru", settings.Language)
	require.Equal(t, 45*time.Second, settings.MaxDurationOr())
	require.Equal(t, "rec -t raw -", settings.RecordCommand)
}

func TestSettingsAutoLanguageMeansDetect(t *testing.T) {
	t.Parallel()

	settings := SettingsFrom(&config.VoiceOptions{Language: "auto"}, nil)
	require.Empty(t, settings.Language)
}

func TestDetectPlanReportsMissingRecorder(t *testing.T) {
	t.Parallel()

	_, err := detectPlan(context.Background(), Settings{}, emptyProber())
	require.ErrorIs(t, err, ErrNoRecorder)
}

func TestDetectPlanSelectsTinyAsrEngine(t *testing.T) {
	t.Parallel()

	settings := Settings{RecordCommand: "some-recorder"}
	plan, err := detectPlan(context.Background(), settings, emptyProber())
	require.NoError(t, err)
	require.Equal(t, "record-command", plan.Recorder.Name())
	require.Equal(t, EngineTinyASR, plan.Transcriber.Name())
}
