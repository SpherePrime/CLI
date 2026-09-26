// Package voice turns microphone audio into text with Whisper.
//
// Both halves of the pipeline are delegated to external tools so Prime stays
// a pure Go binary without a cgo audio dependency. Capture uses ffmpeg,
// PipeWire/ALSA helpers, sox, Python's sounddevice module, or a user-supplied
// command. Transcription uses whisper.cpp, the Python Whisper CLI,
// whisper-ctranslate2, a whisper.cpp HTTP server, or any OpenAI-compatible
// audio/transcriptions endpoint.
package voice

import (
	"context"
	"errors"
	"time"
)

// Captured audio format. Whisper expects 16 kHz mono PCM, so recorders are
// asked for exactly that and the RIFF header is written by Prime itself.
const (
	SampleRate = 16000
	Channels   = 1

	// bytesPerSample is the width of the little-endian signed PCM we capture.
	bytesPerSample = 2

	// DefaultMaxDuration bounds a single dictation so a forgotten session
	// cannot fill the disk with audio.
	DefaultMaxDuration = 5 * time.Minute

	// minSpeechDuration is the shortest recording worth transcribing. Taps
	// shorter than this are treated as mistakes and dropped.
	minSpeechDuration = 300 * time.Millisecond

	// defaultModel is the Whisper model used when options.voice.model is
	// unset. It is multilingual, which matters because dictation is offered
	// in languages Prime's UI supports.
	defaultModel = "base"
)

// Engine names accepted by options.voice.engine. EngineAuto lets Prime pick.
const (
	EngineAuto        = "auto"
	EngineWhisperCPP  = "whispercpp"
	EngineWhisperPy   = "openai-whisper"
	EngineCTranslate2 = "whisper-ctranslate2"
	EngineServer      = "server"
	EngineOpenAI      = "openai"
	EngineCommand     = "command"
)

var (
	// ErrNoRecorder means no microphone capture tool was found.
	ErrNoRecorder = errors.New(
		"no microphone recorder found. Enable the voice plugin from the /plugins menu, " +
			"or install ffmpeg " +
			"(winget install Gyan.FFmpeg, brew install ffmpeg) or sox, " +
			"or set option voice record-command",
	)
	// ErrNoTranscriber means no Whisper engine was found.
	ErrNoTranscriber = errors.New(
		"no Whisper engine found. Enable the voice plugin from the /plugins menu to download " +
			"whisper.cpp and a model, " +
			"or set option voice base-url and api-key at an OpenAI-compatible transcription endpoint",
	)
	// ErrNoSpeech means the recording is too short to transcribe.
	ErrNoSpeech = errors.New("recording too short, nothing to transcribe")
	// ErrCaptureFailed means the recorder stopped before capturing audio.
	ErrCaptureFailed = errors.New("microphone capture failed")
	// ErrTranscribeFailed means the Whisper engine returned an error.
	ErrTranscribeFailed = errors.New("whisper transcription failed")
)

// Settings configures the voice pipeline. It is derived from
// options.voice, so every field is optional and zero values mean "let Prime
// decide".
type Settings struct {
	// Enabled gates the whole feature.
	Enabled bool
	// Engine pins a specific Whisper backend, or EngineAuto to probe.
	Engine string
	// Model is a Whisper model name ("base", "small") or a whisper.cpp
	// ggml model file.
	Model string
	// Language is an ISO 639-1 code ("ru", "en"). Empty means auto-detect.
	Language string
	// BaseURL points at an OpenAI-compatible API root or a whisper.cpp
	// server /inference endpoint.
	BaseURL string
	// APIKey authorizes BaseURL. It may still be an unresolved "$ENV_VAR"
	// reference when the settings are built.
	APIKey string
	// RecordCommand overrides microphone capture with a custom command.
	RecordCommand string
	// TranscribeCommand overrides transcription with a custom command.
	TranscribeCommand string
	// MaxDuration bounds one recording. Zero means DefaultMaxDuration.
	MaxDuration time.Duration
	// Resolver expands "$VAR" style references in APIKey.
	Resolver VariableResolver
}

// VariableResolver expands a stored config value, which is often a template
// such as "$OPENAI_API_KEY", into the secret itself.
type VariableResolver interface {
	ResolveValue(value string) (string, error)
}

// MaxDurationOr returns the configured cap, falling back to
// DefaultMaxDuration.
func (s Settings) MaxDurationOr() time.Duration {
	if s.MaxDuration > 0 {
		return s.MaxDuration
	}
	return DefaultMaxDuration
}

// modelOrDefault returns the configured model name, or the multilingual
// default when unset.
func (s Settings) modelOrDefault() string {
	if s.Model == "" {
		return defaultModel
	}
	return s.Model
}

// resolveAPIKey expands an environment-variable reference in the configured
// API key and falls back to well-known provider variables.
func (s Settings) resolveAPIKey() string {
	key := s.APIKey
	if key != "" && s.Resolver != nil {
		if resolved, err := s.Resolver.ResolveValue(key); err == nil {
			key = resolved
		}
	}
	return key
}

// Recorder captures 16 kHz mono PCM from the default input device.
type Recorder interface {
	// Name identifies the backend, for the recording indicator and logs.
	Name() string
	// Start launches capture. The returned session must be stopped.
	Start(ctx context.Context) (*Session, error)
}

// Transcriber turns captured audio into text with Whisper.
type Transcriber interface {
	// Name identifies the backend, for status messages.
	Name() string
	// Transcribe returns the dictated text for the recording.
	Transcribe(ctx context.Context, audio *Audio) (string, error)
}

// Audio is a finished recording with a well-formed WAV header.
type Audio struct {
	WAV      []byte
	Duration time.Duration
	Recorder string
}

// WorthTranscribing reports whether the recording is long enough to be more
// than an accidental keypress.
func (a *Audio) WorthTranscribing() bool {
	return a != nil && len(a.WAV) > 0 && a.Duration >= minSpeechDuration
}

// Plan is the resolved voice pipeline: which tool records and which one
// transcribes.
type Plan struct {
	Recorder    Recorder
	Transcriber Transcriber
}

// Start begins microphone capture with the plan's recorder.
func (p *Plan) Start(ctx context.Context) (*Session, error) {
	if p == nil || p.Recorder == nil {
		return nil, ErrNoRecorder
	}
	return p.Recorder.Start(ctx)
}

// Transcribe runs the plan's Whisper engine over a recording.
func (p *Plan) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	if p == nil || p.Transcriber == nil {
		return "", ErrNoTranscriber
	}
	return p.Transcriber.Transcribe(ctx, audio)
}
