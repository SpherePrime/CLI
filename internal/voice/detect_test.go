package voice

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// boolPtr is a small helper for options that are stored as pointers so the
// unset state stays distinguishable from false.
func boolPtr(value bool) *bool {
	return &value
}

func TestSettingsFromDefaults(t *testing.T) {
	t.Parallel()

	settings := SettingsFrom(nil, nil)
	require.True(t, settings.Enabled, "voice input works out of the box")
	require.Empty(t, settings.Model)
	require.Equal(t, DefaultHotkeys, settings.Hotkeys())
	require.Equal(t, DefaultMaxDuration, settings.MaxDurationOr())
	require.Equal(t, defaultModel, settings.modelOrDefault())
}

func TestSettingsFromOptions(t *testing.T) {
	t.Parallel()

	settings := SettingsFrom(&config.VoiceOptions{
		Enabled:           boolPtr(false),
		Hotkey:            "CTRL+SHIFT+SPACE, f2",
		Engine:            "WHISPERCPP",
		Model:             "/models/ggml-small.bin",
		Language:          "RU",
		BaseURL:           "http://localhost:8000",
		APIKey:            "secret",
		RecordCommand:     "rec -t raw -",
		TranscribeCommand: "whisper %s",
		MaxDuration:       45,
	}, nil)

	require.False(t, settings.Enabled)
	require.Equal(t, []string{"ctrl+shift+space", "f2"}, settings.Hotkeys())
	require.Equal(t, EngineWhisperCPP, settings.Engine)
	require.Equal(t, "ru", settings.Language)
	require.Equal(t, 45*time.Second, settings.MaxDurationOr())
	require.Equal(t, "rec -t raw -", settings.RecordCommand)
	require.Equal(t, "whisper %s", settings.TranscribeCommand)
}

func TestSettingsEngineTyposFallBackToAuto(t *testing.T) {
	t.Parallel()

	settings := SettingsFrom(&config.VoiceOptions{Engine: "not-an-engine"}, nil)
	require.Equal(t, EngineAuto, settings.Engine)
}

func TestSettingsAutoLanguageMeansDetect(t *testing.T) {
	t.Parallel()

	settings := SettingsFrom(&config.VoiceOptions{Language: "auto"}, nil)
	require.Empty(t, settings.Language)
}

func TestSettingsHotkeysIgnoreEmptyEntries(t *testing.T) {
	t.Parallel()

	settings := SettingsFrom(&config.VoiceOptions{Hotkey: " , alt+j , "}, nil)
	require.Equal(t, []string{"alt+j"}, settings.Hotkeys())
}

func TestSettingsResolvesAPIKeyThroughResolver(t *testing.T) {
	t.Parallel()

	// A stored key is often a variable reference, which only becomes a
	// credential once the resolver expands it.
	settings := Settings{APIKey: "$WHISPER_KEY", Resolver: stubResolver{value: "sk-from-env"}}
	require.Equal(t, "sk-from-env", settings.resolveAPIKey())

	// Without a resolver the value is used as written, so a literal key keeps
	// working.
	require.Equal(t, "sk-literal", Settings{APIKey: "sk-literal"}.resolveAPIKey())
}

type stubResolver struct{ value string }

func (s stubResolver) ResolveValue(string) (string, error) { return s.value, nil }

func TestSettingsUsesInjectedResolver(t *testing.T) {
	t.Parallel()

	settings := Settings{APIKey: "$SOMETHING", Resolver: stubResolver{value: "resolved"}}
	require.Equal(t, "resolved", settings.resolveAPIKey())
}

func TestDetectPlanReportsMissingRecorder(t *testing.T) {
	t.Parallel()

	_, err := detectPlan(context.Background(), Settings{}, emptyProber())
	require.ErrorIs(t, err, ErrNoRecorder)
}

func TestDetectPlanReportsMissingEngine(t *testing.T) {
	t.Parallel()

	settings := Settings{RecordCommand: "some-recorder"}
	_, err := detectPlan(context.Background(), settings, emptyProber())
	require.ErrorIs(t, err, ErrNoTranscriber)
}

func TestDetectPlanPairsFirstBackend(t *testing.T) {
	t.Parallel()

	settings := Settings{RecordCommand: "rec -t raw -", TranscribeCommand: "whisper %s"}
	plan, err := detectPlan(context.Background(), settings, emptyProber())
	require.NoError(t, err)
	require.Equal(t, "record-command", plan.Recorder.Name())
	require.Equal(t, "transcribe-command", plan.Transcriber.Name())
}

func TestDetectorCachesPlanUntilInvalidated(t *testing.T) {
	t.Parallel()

	// The Python probe is the part of detection that costs a process spawn,
	// so counting its calls shows whether the plan was rebuilt.
	probes := 0
	p := emptyProber()
	p.pythonSounddevice = func(context.Context) (string, bool) {
		probes++
		return "", false
	}
	detector := newDetectorWithProber(Settings{}, p)

	_, firstErr := detector.Plan(context.Background())
	_, secondErr := detector.Plan(context.Background())
	require.ErrorIs(t, firstErr, ErrNoRecorder)
	require.ErrorIs(t, secondErr, ErrNoRecorder)
	require.Equal(t, 1, probes, "detection runs once per settings change")

	detector.Invalidate()
	_, err := detector.Plan(context.Background())
	require.ErrorIs(t, err, ErrNoRecorder)
	require.Equal(t, 2, probes)
}

func TestDetectorBackendsAreReported(t *testing.T) {
	t.Parallel()

	settings := Settings{RecordCommand: "rec", TranscribeCommand: "whisper %s"}
	detector := newDetectorWithProber(settings, emptyProber())
	recorders, engines := detector.Backends(context.Background())
	require.Len(t, recorders, 1)
	require.Len(t, engines, 1)
}

// emptyProber is a machine with none of the optional tools installed.
func emptyProber() prober {
	return prober{
		lookPath: func(string) (string, bool) { return "", false },
		pythonSounddevice: func(context.Context) (string, bool) {
			return "", false
		},
		pythonInterpreter: func(context.Context) (string, bool) {
			return "", false
		},
		reachable: func(context.Context, string) bool { return false },
	}
}

func TestNewRecordersHonorsCustomCommand(t *testing.T) {
	t.Parallel()

	recorders := newRecorders(context.Background(), Settings{RecordCommand: "myrec %s"}, proberWith{
		paths: map[string]string{"ffmpeg": "/usr/bin/ffmpeg"},
	}.toProber())
	require.Len(t, recorders, 1, "an explicit recorder replaces discovery")
	require.Equal(t, "record-command", recorders[0].Name())
}

func TestNewRecordersPrefersPythonOnWindowsAndFallbacksElsewhere(t *testing.T) {
	t.Parallel()

	p := proberWith{
		paths:  map[string]string{"ffmpeg": "/usr/bin/ffmpeg"},
		python: "/usr/bin/python",
	}
	recorders := newRecorders(context.Background(), Settings{}, p.toProber())
	require.NotEmpty(t, recorders)

	names := make([]string, 0, len(recorders))
	for _, recorder := range recorders {
		names = append(names, recorder.Name())
	}
	require.Contains(t, names, "ffmpeg")
	require.Contains(t, names, "python-sounddevice")
}

func TestNewRecordersWithoutTools(t *testing.T) {
	t.Parallel()

	require.Empty(t, newRecorders(context.Background(), Settings{}, emptyProber()))
}

// The Windows Store alias for python resolves on PATH but runs no code, so
// the launcher probe has to execute the candidate rather than trust LookPath.
func TestPythonInterpreterProbeRejectsStubLaunchers(t *testing.T) {
	t.Parallel()

	stub := filepath.Join(t.TempDir(), "python.exe")
	probe := pythonInterpreterProbe(func(name string) (string, bool) {
		if name == "python" {
			return stub, true
		}
		return "", false
	})
	_, ok := probe(context.Background())
	require.False(t, ok, "a launcher that cannot run code is not usable")
}

// proberWith is a declarative fake: names found on PATH and a reachable
// localhost Whisper server.
type proberWith struct {
	paths    map[string]string
	python   string
	serverOK bool
	layout   InstallLayout
}

func (p proberWith) toProber() prober {
	return prober{
		lookPath: func(name string) (string, bool) {
			path, ok := p.paths[name]
			return path, ok
		},
		pythonSounddevice: func(context.Context) (string, bool) {
			return p.python, p.python != ""
		},
		pythonInterpreter: func(context.Context) (string, bool) {
			return p.python, p.python != ""
		},
		reachable: func(context.Context, string) bool { return p.serverOK },
		layout:    p.layout,
	}
}
