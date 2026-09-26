package voice

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"time"
)

// Dictation runs like a stenographer: it records the microphone, stops when
// the speaker falls silent, and transcribes what it heard. This is the entry
// point Prime's voice MCP server exposes, so the same engine that used to sit
// behind a UI hotkey now serves agents over MCP.

const (
	// vadFrame is one analysis window: 20 ms of 16 kHz mono 16-bit PCM.
	vadFrame = SampleRate * bytesPerSample / 50

	// DefaultTrailingSilence is how long the speaker must stay quiet before
	// Dictate ends the recording.
	DefaultTrailingSilence = 700 * time.Millisecond

	// DefaultStartTimeout bounds the wait for the first spoken word.
	DefaultStartTimeout = 20 * time.Second

	// DefaultDictationMax bounds one dictate call when the caller is silent
	// about it. Long speeches are handled by calling dictate again.
	DefaultDictationMax = 60 * time.Second

	// MaxDictationDuration caps a single recording request.
	MaxDictationDuration = 5 * time.Minute

	// vadPollInterval is how often the growing recording is re-checked.
	vadPollInterval = 80 * time.Millisecond

	// vadQuietRMS is the default loudness floor. Normal speech sits well
	// above it; fan hiss and room tone sit well below.
	vadQuietRMS = 500.0
)

// DictateOptions tunes one recording. Zero values select the defaults.
type DictateOptions struct {
	// MaxDuration bounds the whole recording.
	MaxDuration time.Duration
	// StartTimeout bounds the wait for speech to begin.
	StartTimeout time.Duration
	// TrailingSilence is the quiet period that ends the recording once
	// speech has started.
	TrailingSilence time.Duration
	// Threshold overrides vadQuietRMS for the loudness gate.
	Threshold float64
	// Poll overrides the silence-check cadence.
	Poll time.Duration
}

// Dictation is the outcome of one successful recording.
type Dictation struct {
	Text     string
	Engine   string
	Recorder string
	Duration time.Duration
}

func (o DictateOptions) max() time.Duration {
	if o.MaxDuration > 0 && o.MaxDuration <= MaxDictationDuration {
		return o.MaxDuration
	}
	return DefaultDictationMax
}

func (o DictateOptions) startTimeout() time.Duration {
	if o.StartTimeout > 0 {
		return o.StartTimeout
	}
	return DefaultStartTimeout
}

func (o DictateOptions) silence() time.Duration {
	if o.TrailingSilence > 0 {
		return o.TrailingSilence
	}
	return DefaultTrailingSilence
}

func (o DictateOptions) threshold() float64 {
	if o.Threshold > 0 {
		return o.Threshold
	}
	return vadQuietRMS
}

func (o DictateOptions) poll() time.Duration {
	if o.Poll > 0 {
		return o.Poll
	}
	return vadPollInterval
}

// capturedSource is the live end of a recording: whatever PCM exists right
// now. Session implements it, and tests can fake it.
type capturedSource interface {
	Captured() ([]byte, error)
}

// Captured returns the PCM recorded so far. It is safe to poll while the
// recorder is still running, so silence can be detected before stopping.
func (s *Session) Captured() ([]byte, error) {
	if s == nil {
		return nil, ErrCaptureFailed
	}
	return s.readPCM()
}

// Dictate records until the speaker stops, then transcribes the recording.
// It reports ErrNoRecorder or ErrNoTranscriber untouched so callers can point
// at voice setup, and ErrNoSpeech when nothing was said in time.
func (p *Plan) Dictate(ctx context.Context, options DictateOptions) (Dictation, error) {
	if p == nil || p.Recorder == nil {
		return Dictation{}, ErrNoRecorder
	}
	if p.Transcriber == nil {
		return Dictation{}, ErrNoTranscriber
	}

	session, err := p.Recorder.Start(ctx)
	if err != nil {
		return Dictation{}, err
	}

	spoke, err := waitForSpeechEnd(ctx, session, options)
	if err != nil {
		session.Abort()
		return Dictation{}, err
	}
	if !spoke {
		session.Abort()
		return Dictation{}, ErrNoSpeech
	}

	audio, err := session.Stop()
	if err != nil {
		return Dictation{}, err
	}
	if !audio.WorthTranscribing() {
		return Dictation{}, ErrNoSpeech
	}

	text, err := p.Transcriber.Transcribe(ctx, audio)
	if err != nil {
		return Dictation{}, err
	}
	return Dictation{
		Text:     text,
		Engine:   p.Transcriber.Name(),
		Recorder: audio.Recorder,
		Duration: audio.Duration,
	}, nil
}

// Dictate resolves the pipeline once and records with it.
func (d *Detector) Dictate(ctx context.Context, options DictateOptions) (Dictation, error) {
	plan, err := d.Plan(ctx)
	if err != nil {
		return Dictation{}, err
	}
	return plan.Dictate(ctx, options)
}

// waitForSpeechEnd polls the live recording until the gate sees speech end,
// the recording hits its bounds, or the caller's context dies. The return
// says whether anything was spoken.
func waitForSpeechEnd(ctx context.Context, live capturedSource, options DictateOptions) (bool, error) {
	gate := &vadGate{silenceNeeded: options.silence(), threshold: options.threshold()}
	startDeadline := time.Now().Add(options.startTimeout())
	recordDeadline := time.Now().Add(options.max())
	ticker := time.NewTicker(options.poll())
	defer ticker.Stop()

	offset := 0
	for {
		select {
		case <-ctx.Done():
			return gate.started, ctx.Err()
		case <-ticker.C:
		}

		now := time.Now()
		pcm, err := live.Captured()
		if err != nil {
			// The sink can flicker while the recorder is still starting
			// up; only a deadline gives up on it.
			if now.After(recordDeadline) {
				return false, fmt.Errorf("%w: %v", ErrCaptureFailed, err)
			}
			continue
		}
		if len(pcm) > offset {
			gate.feed(pcm[offset:len(pcm)], now)
			offset = len(pcm)
		}

		switch {
		case gate.silentTooLong(now):
			return true, nil
		case !gate.started && now.After(startDeadline):
			return false, nil
		case gate.started && now.After(recordDeadline):
			return true, nil
		}
	}
}

// vadGate decides when a stream of PCM contains speech and when that speech
// has ended, based on frame loudness against a fixed floor.
type vadGate struct {
	silenceNeeded time.Duration
	threshold     float64
	pending       []byte
	started       bool
	lastVoiced    time.Time
}

// feed consumes freshly captured PCM, timestamped with when it was read.
func (g *vadGate) feed(pcm []byte, now time.Time) {
	data := pcm
	if len(g.pending) > 0 {
		data = append(append([]byte(nil), g.pending...), pcm...)
	}
	usable := len(data) - len(data)%2
	g.pending = append(g.pending[:0], data[usable:]...)

	for i := 0; i+vadFrame <= usable; i += vadFrame {
		frameAge := time.Duration(usable-i-vadFrame) * time.Second /
			time.Duration(SampleRate*bytesPerSample)
		frameTime := now.Add(-frameAge)
		if frameRMS(data[i:i+vadFrame]) >= g.threshold {
			g.started = true
			if frameTime.After(g.lastVoiced) {
				g.lastVoiced = frameTime
			}
		}
	}
}

// silentTooLong reports whether speech started and has been quiet for at
// least the configured trailing-silence window.
func (g *vadGate) silentTooLong(now time.Time) bool {
	return g.started && now.Sub(g.lastVoiced) >= g.silenceNeeded
}

// frameRMS is the root-mean-square level of one PCM frame, in int16 units.
func frameRMS(frame []byte) float64 {
	var sum float64
	var samples float64
	for i := 0; i+1 < len(frame); i += 2 {
		s := int16(binary.LittleEndian.Uint16(frame[i:]))
		sum += float64(s) * float64(s)
		samples++
	}
	if samples == 0 {
		return 0
	}
	return math.Sqrt(sum / samples)
}
