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

// helperBurstChunks is how many chunks the helper writes before it starts
// throttling.
//
// One chunk is 100ms of audio and the tests wait out minSpeechDuration, so the
// first few chunks go out back to back. Without this the helper's only output
// arrived one chunk per 100ms starting from process spawn, which made the
// tests a race against process startup: on a loaded macOS runner the spawn
// alone could eat most of the wait, and the capture came back shorter than the
// minimum the code under test is specified to keep. A short burst costs
// nothing and removes the dependency on how fast the runner is.
const helperBurstChunks = 4

func TestCaptureHelper(t *testing.T) {
	if !slices.Contains(os.Args, helperMarker) {
		t.Skip("subprocess used by the capture tests")
	}

	chunk := make([]byte, SampleRate*bytesPerSample/10)
	for range helperBurstChunks {
		if _, err := os.Stdout.Write(chunk); err != nil {
			return
		}
	}

	// After the burst, one chunk is 100ms of audio, so the sleep has to be
	// 100ms too. Sleeping less pushed ten times the audio through the pipe for
	// no benefit, and a subprocess blasting 320KB/s is enough to look like a
	// leak to a busy CI runner.
	for {
		time.Sleep(100 * time.Millisecond)
		if _, err := os.Stdout.Write(chunk); err != nil {
			return
		}
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

	// Wait for the audio rather than for a duration. Process start is not
	// bounded, so a sleep here was a race the test did not control.
	waitForAudio(t, session, helperBurstDuration())

	audio, err := session.Stop()
	require.NoError(t, err)

	// The whole burst must survive. This is the guarantee, not a reproduction:
	// end() used to pick waitDone about half the time, so the copy could still
	// be writing when Stop read the sink file. Reproducing that needs the copy
	// to be slowed down on purpose, which would mean a test-only delay inside
	// the capture path, so the race is fixed by reading rather than by failing
	// here first. Comparing against the burst rather than against
	// minSpeechDuration still makes this a truncation check rather than a race
	// with process startup.
	require.GreaterOrEqual(t, audio.Duration, helperBurstDuration(),
		"the opening burst was truncated: got %s, the helper wrote %s before it started throttling",
		audio.Duration, helperBurstDuration())

	require.True(t, audio.WorthTranscribing(),
		"captured %s, need at least %s", audio.Duration, minSpeechDuration)
	require.Equal(t, "helper", audio.Recorder)
	require.Equal(t, "RIFF", string(audio.WAV[:4]))
}

// helperBurstDuration is how much audio the helper emits before it throttles.
func helperBurstDuration() time.Duration {
	return time.Duration(helperBurstChunks) * 100 * time.Millisecond
}

// waitForAudio blocks until the capture has received at least want of PCM.
//
// These tests used to sleep for a fixed time and hope the helper process had
// started by then. Starting a process is not bounded: on a loaded Windows
// runner it regularly took longer than the sleep, so Stop found an empty
// recording and reported "no audio captured" for a recorder that was working
// perfectly. The same happened on macOS, and both read as a capture bug rather
// than as a test that had not waited long enough.
//
// The tests live in the same package as the capture, so they can watch the sink
// file the copy goroutine writes and wait for the audio to actually arrive.
// That makes them independent of how fast the machine starts processes, which is
// the thing that was never under the test's control.
func waitForAudio(t *testing.T, session *Session, want time.Duration) {
	t.Helper()

	// Two bytes per sample, matching bytesPerSample used by the helper.
	wantBytes := int64(want) * int64(SampleRate) * int64(bytesPerSample) / int64(time.Second)

	// Generous, because this only has to cover process start; the wait ends as
	// soon as the audio lands, so a fast machine does not pay for the bound.
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := os.Stat(session.sink.Name()); err == nil && info.Size() >= wantBytes {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}

	t.Fatalf("no audio after %s: wanted %s of PCM", want, want)
}

func TestSessionStopIsClaimedOnce(t *testing.T) {
	t.Parallel()

	session, err := helperRecorder().Start(context.Background())
	require.NoError(t, err)
	waitForAudio(t, session, 100*time.Millisecond)

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
	waitForAudio(t, session, 100*time.Millisecond)

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

// A capture Prime ended itself must not be reported as a recorder failure:
// cancelling the command context makes os/exec kill the recorder, so the exit
// status is "signal: killed" on Unix and a bare "exit status 1" on Windows.
// Both used to surface as a microphone failure, which is why these capture
// tests failed intermittently on CI runners under load.
func TestEmptyCaptureHidesPrimesOwnKill(t *testing.T) {
	t.Parallel()

	session := &Session{stderr: &tailBuffer{}, processErr: errors.New("signal: killed"), endedByPrime: true}
	require.EqualError(t, session.emptyCaptureError(),
		"microphone capture failed: no audio captured")

	// A recorder that complained before it was killed still gets to say so.
	_, err := session.stderr.Write([]byte("microphone: device busy"))
	require.NoError(t, err)
	require.EqualError(t, session.emptyCaptureError(),
		"microphone capture failed: no audio captured: microphone: device busy")

	// The same status from a recorder that died on its own is still a failure.
	session.endedByPrime = false
	require.EqualError(t, session.emptyCaptureError(),
		"microphone capture failed: signal: killed: microphone: device busy")
}

// A recorder that exits non-zero before Prime stops it is a real failure and
// has to keep saying so.
func TestEmptyCaptureKeepsGenuineRecorderFailure(t *testing.T) {
	t.Parallel()

	session := &Session{stderr: &tailBuffer{}, processErr: errors.New("exit status 4")}
	require.EqualError(t, session.emptyCaptureError(),
		"microphone capture failed: exit status 4")
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
