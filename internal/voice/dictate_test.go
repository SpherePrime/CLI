package voice

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func pcmLevel(level int16, samples int) []byte {
	buf := make([]byte, samples*bytesPerSample)
	for i := 0; i < samples; i++ {
		binary.LittleEndian.PutUint16(buf[i*2:], uint16(level))
	}
	return buf
}

func TestFrameRMS(t *testing.T) {
	require.Zero(t, frameRMS(pcmLevel(0, vadFrame/2)))
	require.InDelta(t, 4000, frameRMS(pcmLevel(4000, vadFrame/2)), 1)
	require.InDelta(t, 4000, frameRMS(pcmLevel(-4000, vadFrame/2)), 1)
}

func TestVADGateDetectsSpeechThenSilence(t *testing.T) {
	gate := &vadGate{silenceNeeded: 100 * time.Millisecond, threshold: vadQuietRMS}
	loud := pcmLevel(8000, vadFrame/2)
	quiet := pcmLevel(0, vadFrame/2)

	now := time.Now()
	gate.feed(loud, now)
	require.True(t, gate.started)
	require.False(t, gate.silentTooLong(now))

	for range 3 {
		now = now.Add(20 * time.Millisecond)
		gate.feed(quiet, now)
	}
	require.False(t, gate.silentTooLong(now))

	now = now.Add(120 * time.Millisecond)
	require.True(t, gate.silentTooLong(now))
}

func TestVADGateKeepsOddByteUntilNextFrame(t *testing.T) {
	gate := &vadGate{silenceNeeded: time.Second, threshold: vadQuietRMS}
	data := append(pcmLevel(8000, vadFrame/2), 0x11)
	gate.feed(data, time.Now())
	require.True(t, gate.started)
}

type staticSource struct {
	samples int
}

func (s *staticSource) Captured() ([]byte, error) {
	level := int16(0)
	if s.samples > 0 {
		level = 8000
	}
	return pcmLevel(level, vadFrame/2*s.samples), nil
}

func TestWaitForSpeechEndStopsAfterSilence(t *testing.T) {
	spoke, err := waitForSpeechEnd(context.Background(), &staticSource{samples: 40}, DictateOptions{
		Poll:            5 * time.Millisecond,
		TrailingSilence: 30 * time.Millisecond,
		MaxDuration:     time.Second,
		StartTimeout:    time.Second,
	}, nil)
	require.NoError(t, err)
	require.True(t, spoke)
}

func TestWaitForSpeechEndGivesUpOnSilenceOnly(t *testing.T) {
	spoke, err := waitForSpeechEnd(context.Background(), &staticSource{samples: 0}, DictateOptions{
		Poll:            5 * time.Millisecond,
		TrailingSilence: 30 * time.Millisecond,
		MaxDuration:     time.Second,
		StartTimeout:    80 * time.Millisecond,
	}, nil)
	require.NoError(t, err)
	require.False(t, spoke)
}

func TestWaitForSpeechEndRespectsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	_, err := waitForSpeechEnd(ctx, &staticSource{samples: 0}, DictateOptions{
		Poll:         5 * time.Millisecond,
		StartTimeout: time.Minute,
		MaxDuration:  time.Minute,
	}, nil)
	require.ErrorIs(t, err, context.Canceled)
}

func TestWaitForSpeechEndStopSignalTranscribesImmediately(t *testing.T) {
	stop := make(chan struct{})
	close(stop)
	spoke, err := waitForSpeechEnd(context.Background(), &staticSource{samples: 0}, DictateOptions{
		Poll:            5 * time.Millisecond,
		TrailingSilence: time.Second,
		MaxDuration:     time.Minute,
		StartTimeout:    time.Minute,
	}, stop)
	require.NoError(t, err)
	require.True(t, spoke, "a manual end always tries to transcribe what was captured")
}

// staticRecorder hands Dictate a ready-made Session whose audio file is
// already complete, so the silence gate sees speech and trailing quiet on the
// very first poll.
type staticRecorder struct {
	pcm []byte
}

func (r staticRecorder) Name() string { return "static" }

func (r staticRecorder) Start(context.Context) (*Session, error) {
	f, err := os.CreateTemp("", "prime-voice-test-*.pcm")
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(r.pcm); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}

	copyDone := make(chan struct{})
	waitDone := make(chan struct{})
	close(copyDone)
	close(waitDone)
	return &Session{
		recorder: r.Name(),
		outPath:  f.Name(),
		fileMode: true,
		copyDone: copyDone,
		waitDone: waitDone,
		cancel:   func() {},
		stderr:   &tailBuffer{},
	}, nil
}

type echoTranscriber struct{ text string }

func (echoTranscriber) Name() string { return "echo" }

func (e echoTranscriber) Transcribe(_ context.Context, audio *Audio) (string, error) {
	if !audio.WorthTranscribing() {
		return "", ErrNoSpeech
	}
	return e.text, nil
}

func TestPlanDictateHappyPath(t *testing.T) {
	plan := &Plan{
		Recorder:    staticRecorder{pcm: pcmLevel(8000, SampleRate/2)},
		Transcriber: echoTranscriber{text: "привет мир"},
	}
	dictation, err := plan.Dictate(context.Background(), DictateOptions{
		Poll:            5 * time.Millisecond,
		TrailingSilence: 30 * time.Millisecond,
		MaxDuration:     time.Second,
		StartTimeout:    time.Second,
	})
	require.NoError(t, err)
	require.Equal(t, "привет мир", dictation.Text)
	require.Equal(t, "echo", dictation.Engine)
	require.Equal(t, "static", dictation.Recorder)
	require.InDelta(t, 0.5, dictation.Duration.Seconds(), 0.05)
}

func TestPlanDictateNoSpeech(t *testing.T) {
	plan := &Plan{
		Recorder:    staticRecorder{pcm: pcmLevel(0, SampleRate/4)},
		Transcriber: echoTranscriber{text: "не должно вызываться"},
	}
	_, err := plan.Dictate(context.Background(), DictateOptions{
		Poll:            5 * time.Millisecond,
		StartTimeout:    60 * time.Millisecond,
		TrailingSilence: 20 * time.Millisecond,
		MaxDuration:     time.Second,
	})
	require.ErrorIs(t, err, ErrNoSpeech)
}

func TestPlanDictateMissingParts(t *testing.T) {
	_, err := (&Plan{}).Dictate(context.Background(), DictateOptions{})
	require.ErrorIs(t, err, ErrNoRecorder)

	_, err = (&Plan{Recorder: staticRecorder{}}).Dictate(context.Background(), DictateOptions{})
	require.True(t, errors.Is(err, ErrNoTranscriber))
}
