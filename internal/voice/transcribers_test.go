package voice

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestParseDShowAudioDevicesNumberedListing(t *testing.T) {
	t.Parallel()

	out := strings.Join([]string{
		`[dshow @ 000001f0] Direct audio devices:`,
		`[dshow @ 000001f0] 	3.  Microphone (Realtek(R) Audio)`,
		`[dshow @ 000001f0] 	4.  Stereo Mix (Realtek(R) Audio)`,
		`[dshow @ 000001f0] Direct video devices:`,
		`[dshow @ 000001f0] 	1.  HD Webcam`,
	}, "\n")

	devices := parseDShowAudioDevices(out)
	require.Equal(t, []string{"Microphone (Realtek(R) Audio)", "Stereo Mix (Realtek(R) Audio)"}, devices)
}

func TestParseDShowAudioDevicesQuotedListing(t *testing.T) {
	t.Parallel()

	out := strings.Join([]string{
		`[dshow @ 000002a] Direct audio devices`,
		`[dshow @ 000002a] "Microphone (USB)" (audio)`,
		`[dshow @ 000002a] Direct video devices`,
		`[dshow @ 000002a] "Camera" (video)`,
	}, "\n")

	require.Equal(t, []string{"Microphone (USB)"}, parseDShowAudioDevices(out))
}

func TestParseDShowAudioDevicesWithoutMicrophone(t *testing.T) {
	t.Parallel()

	require.Empty(t, parseDShowAudioDevices(`[dshow @ 0x1] no devices at all`))
}

func TestTranscriptionEndpointShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		base     string
		endpoint string
		style    httpStyle
	}{
		{
			name:     "openai root",
			base:     "https://api.openai.com/v1",
			endpoint: "https://api.openai.com/v1/audio/transcriptions",
			style:    styleOpenAI,
		},
		{
			name:     "explicit inference route",
			base:     "http://127.0.0.1:8000/inference",
			endpoint: "http://127.0.0.1:8000/inference",
			style:    styleWhisperServer,
		},
		{
			name:     "localhost defaults to whisper.cpp server",
			base:     "http://localhost:8000/",
			endpoint: "http://localhost:8000/inference",
			style:    styleWhisperServer,
		},
		{
			name:     "transcriptions route kept",
			base:     "https://groq.example/openai/v1/audio/transcriptions",
			endpoint: "https://groq.example/openai/v1/audio/transcriptions",
			style:    styleOpenAI,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			endpoint, style := transcriptionEndpoint(tc.base)
			require.Equal(t, tc.endpoint, endpoint)
			require.Equal(t, tc.style, style)
		})
	}
}

func TestInferenceEndpointFromHealthURL(t *testing.T) {
	t.Parallel()

	require.Equal(t, "http://127.0.0.1:8000/inference", inferenceEndpoint(localWhisperServer))
}

func TestNewTranscribersFindsLocalCLI(t *testing.T) {
	t.Parallel()

	settings := Settings{Model: writeTestModel(t)}
	p := proberWith{paths: map[string]string{"whisper-cli": "/usr/bin/whisper-cli"}}.toProber()

	engines := newTranscribers(context.Background(), settings, p)
	require.NotEmpty(t, engines)
	require.Equal(t, "whisper.cpp", engines[0].Name())
}

func TestNewTranscribersPrefersRunningServer(t *testing.T) {
	t.Parallel()

	settings := Settings{Model: writeTestModel(t)}
	p := proberWith{
		paths:    map[string]string{"whisper-cli": "/usr/bin/whisper-cli"},
		serverOK: true,
	}.toProber()

	engines := newTranscribers(context.Background(), settings, p)
	require.Equal(t, "whisper.cpp server", engines[0].Name())
}

func TestNewTranscribersUsesConfiguredEndpoint(t *testing.T) {
	t.Parallel()

	settings := Settings{BaseURL: "https://api.example.com/v1", APIKey: "key"}
	engines := newTranscribers(context.Background(), settings, emptyProber())
	require.Equal(t, "whisper api", engines[0].Name())
}

func TestNewTranscribersPinnedEngineOnly(t *testing.T) {
	t.Parallel()

	settings := Settings{
		Engine:  EngineOpenAI,
		BaseURL: "https://api.example.com/v1",
		APIKey:  "key",
		Model:   "custom-model",
	}
	engines := newTranscribers(context.Background(), settings, proberWith{
		paths: map[string]string{"whisper-cli": "/usr/bin/whisper-cli"},
	}.toProber())
	require.Len(t, engines, 1)

	transcriber, ok := engines[0].(httpTranscriber)
	require.True(t, ok)
	require.Equal(t, "custom-model", transcriber.model)
}

func TestNewTranscribersPinnedEngineMissingYieldsNothing(t *testing.T) {
	t.Parallel()

	settings := Settings{Engine: EngineWhisperCPP}
	require.Empty(t, newTranscribers(context.Background(), settings, emptyProber()))
}

func TestHostedTranscriberFromEnvironment(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-test")
	t.Setenv("GROQ_API_KEY", "")

	engines := newTranscribers(context.Background(), Settings{}, emptyProber())
	require.Len(t, engines, 1)

	transcriber, ok := engines[0].(httpTranscriber)
	require.True(t, ok)
	require.Equal(t, "sk-test", transcriber.apiKey)
	require.Equal(t, openAIAPIModel, transcriber.model)
}

func TestWhisperCPPNeedsModelFile(t *testing.T) {
	t.Parallel()

	p := proberWith{paths: map[string]string{"whisper-cli": "/usr/bin/whisper-cli"}}.toProber()
	_, ok := whisperCPPFromPath(Settings{Model: "does-not-exist-anywhere"}, p)
	require.False(t, ok)
}

func TestGGMLModelFileResolution(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	model := filepath.Join(dir, "ggml-base.bin")
	require.NoError(t, os.WriteFile(model, []byte("weights"), 0o600))

	found, ok := ggmlModelFile(model, nil)
	require.True(t, ok)
	require.Equal(t, model, found)

	found, ok = ggmlModelFile("base", []string{dir})
	require.True(t, ok)
	require.Equal(t, model, found)

	require.NoError(t, os.WriteFile(filepath.Join(dir, "ggml-small.bin"), []byte("weights"), 0o600))

	found, ok = ggmlModelFile("small", []string{dir})
	require.True(t, ok)
	require.Equal(t, filepath.Join(dir, "ggml-small.bin"), found)

	// A model that exists in the layout is found even when nothing is
	// configured, which is how a setup download becomes the working engine.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "ggml-large.bin"), []byte("weights"), 0o600))
	found, ok = ggmlModelFile("", []string{dir})
	require.True(t, ok)

	// A name with no matching file still finds the model another install
	// placed in the same directory, which is the case setup relies on.
	_, ok = ggmlModelFile("huge", []string{dir})
	require.True(t, ok)
}

func TestTidyTranscript(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "python timestamps",
			input: "[00:00.000 --> 00:04.000] Привет, мир\n",
			want:  "Привет, мир",
		},
		{
			name:  "whisper.cpp timestamps",
			input: "[00:00:00.000 --> 00:00:02.500] hello there\n",
			want:  "hello there",
		},
		{
			name:  "multiline joins",
			input: "первая строка\nвторая строка\n",
			want:  "первая строка вторая строка",
		},
		{
			name:  "noise markers",
			input: "сделай коммит <noise> и пуш",
			want:  "сделай коммит и пуш",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tc.want, tidyTranscript(tc.input))
		})
	}
}

func TestWhisperCPPArguments(t *testing.T) {
	t.Parallel()

	engine := whisperCPPTranscriber("/bin/whisper-cli", "/models/ggml-base.bin", Settings{}, nil).(cliTranscriber)
	args := engine.argv(engine, "/tmp/dictation.wav", "/tmp/out")

	require.True(t, slices.Contains(args, "-nt"), "timestamps are not wanted in dictation")
	require.Equal(t, "/models/ggml-base.bin", valueAfter(args, "-m"))
	require.Equal(t, "auto", valueAfter(args, "-l"), "whisper.cpp assumes English unless told to detect")
	require.Contains(t, args, "-otxt", "text output is -otxt; -ot is now --offset-t and crashes the run")
	require.NotContains(t, args, "-ot", "-ot followed by another flag is parsed as a broken number")

	withLanguage := whisperCPPTranscriber("/bin/whisper-cli", "/models/ggml-base.bin", Settings{Language: "ru"}, nil).(cliTranscriber)
	require.Equal(t, "ru", valueAfter(withLanguage.argv(withLanguage, "/tmp/a.wav", "/tmp/out"), "-l"))
}

func TestPythonWhisperArguments(t *testing.T) {
	t.Parallel()

	engine := pythonWhisperTranscriber("/bin/whisper", Settings{Language: "ru", Model: "small"}).(cliTranscriber)
	args := engine.argv(engine, "/tmp/dictation.wav", "/tmp/out")

	require.Equal(t, "/tmp/dictation.wav", args[0], "the CLI takes the audio file first")
	require.Equal(t, "small", valueAfter(args, "--model"))
	require.Equal(t, "ru", valueAfter(args, "--language"))
	require.Equal(t, "/tmp/out", valueAfter(args, "--output_dir"))
	require.Equal(t, filepath.Join("/tmp/out", "dictation.txt"), engine.result("/tmp/dictation.wav", "/tmp/out"))
}

func TestCTranslate2Arguments(t *testing.T) {
	t.Parallel()

	engine := ctranslate2Transcriber("/bin/whisper-ctranslate2", Settings{}).(cliTranscriber)
	args := engine.argv(engine, "/tmp/dictation.wav", "/tmp/out")

	require.Equal(t, defaultModel, valueAfter(args, "--model"))
	require.NotContains(t, args, "--language", "detection is left to the engine when unset")
}

func TestCommandTranscribeArgsAppendsAudio(t *testing.T) {
	t.Parallel()

	args, err := commandTranscribeArgs("my-whisper --model base", "/tmp/dictation.wav")
	require.NoError(t, err)
	require.Equal(t, []string{"my-whisper", "--model", "base", "/tmp/dictation.wav"}, args)
}

func TestCommandTranscribeArgsReplacesPlaceholder(t *testing.T) {
	t.Parallel()

	args, err := commandTranscribeArgs("script.sh --audio=%s", "/tmp/dictation.wav")
	require.NoError(t, err)
	require.Equal(t, []string{"script.sh", "--audio=/tmp/dictation.wav"}, args)
}

func TestCommandTranscribeArgsRejectsEmptyCommand(t *testing.T) {
	t.Parallel()

	_, err := commandTranscribeArgs("   ", "/tmp/a.wav")
	require.ErrorIs(t, err, ErrTranscribeFailed)
}

func TestCommandTranscribeEnvCarriesFormat(t *testing.T) {
	t.Parallel()

	env := commandTranscribeEnv("ru")
	require.Contains(t, env, "PRIME_VOICE_LANGUAGE=ru")
	require.Contains(t, env, "PRIME_VOICE_SAMPLE_RATE=16000")

	env = commandTranscribeEnv("")
	require.NotContains(t, env, "PRIME_VOICE_LANGUAGE=")
}

func TestTranscriberRejectsEmptyAudio(t *testing.T) {
	t.Parallel()

	for _, engine := range []Transcriber{
		commandTranscriber{command: "cat"},
		httpTranscriber{label: "x", style: styleWhisperServer, endpoint: "http://127.0.0.1:1/inference"},
		pythonWhisperTranscriber("/bin/whisper", Settings{}),
	} {
		_, err := engine.Transcribe(context.Background(), &Audio{})
		require.ErrorIs(t, err, ErrNoSpeech, engine.Name())
	}
}

// testAudio builds a two second recording for engine tests.
func testAudio(t *testing.T) *Audio {
	t.Helper()

	pcm := make([]byte, SampleRate*bytesPerSample*2)
	return &Audio{
		WAV:      encodeWAV(pcm),
		Duration: pcmDuration(pcm),
		Recorder: "test",
	}
}

// valueAfter returns the flag value following the named flag.
func valueAfter(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// writeTestModel creates a fake ggml model file so engine detection has
// something to find.
func writeTestModel(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "ggml-base.bin")
	require.NoError(t, os.WriteFile(path, []byte("model"), 0o600))
	return path
}
