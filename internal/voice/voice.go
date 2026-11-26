package voice

import (
	"context"
	"errors"
	"time"
)

const (
	SampleRate           = 16000
	Channels             = 1
	BitDepth             = 16
	bytesPerSample       = BitDepth / 8
	DefaultMaxDuration   = 3 * time.Minute
	MinAudioDuration     = 400 * time.Millisecond
	minSpeechDuration    = MinAudioDuration
	EngineTinyASR        = "tinyasr"
	EngineAuto           = "auto"
	transcriptionTimeout = 2 * time.Minute
)

var (
	ErrDisabled         = errors.New("voice input is disabled")
	ErrNoRecorder       = errors.New("no microphone recorder available")
	ErrNoTranscriber    = errors.New("no speech-to-text engine available")
	ErrNoSpeech         = errors.New("no speech detected")
	ErrCaptureFailed    = errors.New("microphone capture failed")
	ErrTranscribeFailed = errors.New("speech transcription failed")
)

type VariableResolver interface {
	ResolveValue(value string) (string, error)
}

type SetupProgress struct {
	Phase string
	Text  string
	Done  int64
	Total int64
}

type Settings struct {
	Enabled           bool
	Language          string
	InterfaceLanguage string
	DataDir           string
	RecordCommand     string
	MaxDuration       time.Duration
	Resolver          VariableResolver
}

func (s Settings) MaxDurationOr() time.Duration {
	if s.MaxDuration > 0 {
		return s.MaxDuration
	}
	return DefaultMaxDuration
}

type Audio struct {
	WAV      []byte
	Duration time.Duration
	Recorder string
}

func (a *Audio) WorthTranscribing() bool {
	return a != nil && len(a.WAV) > headerSize && a.Duration >= MinAudioDuration
}

type Recorder interface {
	Name() string
	Start(ctx context.Context) (*Session, error)
}

type Transcriber interface {
	Name() string
	Transcribe(ctx context.Context, audio *Audio) (string, error)
}

type Plan struct {
	Recorder    Recorder
	Transcriber Transcriber
}

func (p *Plan) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	if p == nil || p.Transcriber == nil {
		return "", ErrNoTranscriber
	}
	return p.Transcriber.Transcribe(ctx, audio)
}
