package voice

import (
	"bytes"
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// helperMarker is the argument that turns this test binary into a fake
// microphone: an endless stream of silent PCM on stdout until it is killed.
const helperMarker = "-test.count=987654321"

func TestCaptureHelper(t *testing.T) {
	if !slices.Contains(os.Args, helperMarker) {
		t.Skip("subprocess used by the capture tests")
	}

	// One chunk is 100ms of audio, so the sleep has to be 100ms too. Sleeping
	// less pushed ten times the audio through the pipe for no benefit: the
	// tests only assert that more than 100ms arrived, and a subprocess
	// blasting 320KB/s is enough to look like a leak to a busy CI runner.
	chunk := make([]byte, SampleRate*bytesPerSample/10)
	for {
		if _, err := os.Stdout.Write(chunk); err != nil {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// helperRecorder streams from the test binary subprocess, which needs no
// microphone or external tool to exist.
func helperRecorder() processRecorder {
	return processRecorder{
		name: "helper",
		build: func(context.Context, string) ([]string, error) {
			return []string{os.Args[0], "-test.run=^TestCaptureHelper$", helperMarker}, nil
		},
	}
}

func TestSessionStreamsPCMUntilStopped(t *testing.T) {
	t.Parallel()

	session, err := helperRecorder().Start(context.Background())
	require.NoError(t, err)
	require.Equal(t, "helper", session.Recorder())

	time.Sleep(400 * time.Millisecond)

	audio, err := session.Stop()
	require.NoError(t, err)
	require.Greater(t, audio.Duration, 100*time.Millisecond)
	require.True(t, audio.WorthTranscribing())
	require.Equal(t, "helper", audio.Recorder)
	require.Equal(t, "RIFF", string(audio.WAV[:4]))
}

func TestSessionStopIsClaimedOnce(t *testing.T) {
	t.Parallel()

	session, err := helperRecorder().Start(context.Background())
	require.NoError(t, err)
	time.Sleep(200 * time.Millisecond)

	_, err = session.Stop()
	require.NoError(t, err)

	_, err = session.Stop()
	require.ErrorIs(t, err, ErrCaptureFailed)

	// Aborting a finished recording is a no-op rather than a panic.
	session.Abort()
}

func TestSessionAbortDiscardsCapture(t *testing.T) {
	t.Parallel()

	session, err := helperRecorder().Start(context.Background())
	require.NoError(t, err)

	time.Sleep(200 * time.Millisecond)
	session.Abort()

	_, err = session.Stop()
	require.ErrorIs(t, err, ErrCaptureFailed)
}

func TestSessionReportsUnlaunchableRecorder(t *testing.T) {
	t.Parallel()

	recorder := processRecorder{
		name:  "missing",
		build: staticBuilder("prime-voice-no-such-recorder", "-t", "raw", "-"),
	}
	_, err := recorder.Start(context.Background())
	require.ErrorIs(t, err, ErrCaptureFailed)
}

func TestSessionReportsBadRecorderCommand(t *testing.T) {
	t.Parallel()

	recorder := processRecorder{name: "empty", build: func(context.Context, string) ([]string, error) {
		return nil, nil
	}}
	_, err := recorder.Start(context.Background())
	require.ErrorIs(t, err, ErrCaptureFailed)
	require.Contains(t, err.Error(), "empty recorder command")
}

func TestShellArgsSubstitutesOutputPath(t *testing.T) {
	t.Parallel()

	args, err := shellArgs(`rec -q -r 16000 "%s"`)(context.Background(), "/tmp/capture.wav")
	require.NoError(t, err)
	require.Equal(t, []string{"rec", "-q", "-r", "16000", "/tmp/capture.wav"}, args)
	require.False(t, usesOutputPlaceholder("rec -t raw -"))
	require.True(t, usesOutputPlaceholder(`rec -q "%s"`))
}

// A capture stopped before the recorder wrote anything has no useful exit
// status to report: Prime canceled the process itself, so the message has to
// name the real problem instead of leaking "context canceled".
func TestEmptyCaptureHidesPrimesOwnCancel(t *testing.T) {
	t.Parallel()

	session := &Session{stderr: &tailBuffer{}, processErr: context.Canceled}
	require.EqualError(t, session.emptyCaptureError(),
		"microphone capture failed: no audio captured")

	_, err := session.stderr.Write([]byte("microphone: no input devices\n"))
	require.NoError(t, err)
	require.EqualError(t, session.emptyCaptureError(),
		"microphone capture failed: no audio captured: microphone: no input devices")

	// A recorder that failed on its own still speaks for itself.
	session.processErr = errors.New("exit status 4")
	require.EqualError(t, session.emptyCaptureError(),
		"microphone capture failed: exit status 4: microphone: no input devices")
}

func TestTailBufferKeepsOnlyTheTail(t *testing.T) {
	t.Parallel()

	buffer := &tailBuffer{}
	_, err := buffer.Write([]byte(strings.Repeat("a", tailLimit*2)))
	require.NoError(t, err)
	require.Equal(t, tailLimit, len(buffer.String()))

	// A recorder that prints a short complaint should surface it verbatim.
	short := &tailBuffer{}
	_, err = short.Write([]byte("no input devices"))
	require.NoError(t, err)
	require.Equal(t, "no input devices", short.String())
}

func TestLimitedWriterCapsOutput(t *testing.T) {
	t.Parallel()

	buffer := &bytes.Buffer{}
	writer := &limitedWriter{buf: buffer, limit: 8}
	written, err := writer.Write([]byte("0123456789abcdef"))
	require.NoError(t, err)
	require.Equal(t, 16, written)
	require.Equal(t, "01234567", buffer.String())
}
