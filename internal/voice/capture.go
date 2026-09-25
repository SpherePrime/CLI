package voice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/sh/v3/shell"
)

// stopGracePeriod is how long a recorder gets to finish writing after it was
// asked to stop before it is killed outright.
const stopGracePeriod = 2 * time.Second

// tailLimit bounds how much recorder or engine output is kept for error
// messages, so a chatty tool cannot grow the status bar without end.
const tailLimit = 600

// Session is a running microphone capture.
type Session struct {
	recorder string
	cmd      *exec.Cmd
	pipe     *os.File
	sink     *os.File
	outPath  string
	fileMode bool

	stdin    io.WriteCloser
	graceful bool
	copyDone chan struct{}
	waitDone chan struct{}
	cancel   context.CancelFunc
	stderr   *tailBuffer
	stop     stopHow

	// processErr is written before waitDone is closed, so reading it after
	// receiving from that channel is safe.
	processErr error

	mu      sync.Mutex
	stopped bool
}

// Recorder reports which backend is capturing the microphone.
func (s *Session) Recorder() string {
	if s == nil {
		return ""
	}
	return s.recorder
}

// Stop ends capture and returns the recorded audio.
func (s *Session) Stop() (*Audio, error) {
	if !s.markDone() {
		return nil, ErrCaptureFailed
	}
	s.end()
	pcm, err := s.readPCM()
	if cleanupErr := s.cleanup(); cleanupErr != nil {
		slog.Debug("Voice temp file cleanup failed", "error", cleanupErr)
	}
	if err != nil {
		return nil, err
	}
	if len(pcm) == 0 {
		return nil, s.emptyCaptureError()
	}
	return &Audio{
		WAV:      encodeWAV(pcm),
		Duration: pcmDuration(pcm),
		Recorder: s.recorder,
	}, nil
}

// Abort ends capture and discards whatever was recorded.
func (s *Session) Abort() {
	if !s.markDone() {
		return
	}
	s.end()
	_ = s.cleanup()
}

// markDone claims the session for exactly one Stop or Abort.
func (s *Session) markDone() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stopped {
		return false
	}
	s.stopped = true
	return true
}

// end stops the capture process and waits for the pipe and the process to
// wind down, so the caller can read everything the recorder wrote.
func (s *Session) end() {
	s.cancel()
	drained := false
	for !drained {
		select {
		case <-s.copyDone:
			drained = true
		case <-s.waitDone:
			drained = true
		case <-time.After(stopGracePeriod):
			// Closing the read end unblocks a copy that a lingering
			// grandchild is holding open.
			if s.pipe != nil {
				_ = s.pipe.Close()
				s.pipe = nil
			}
			_ = s.cmd.Process.Kill()
		}
	}
	<-s.waitDone
}

// readPCM returns the captured PCM, whatever shape the recorder produced it
// in.
func (s *Session) readPCM() ([]byte, error) {
	if s.fileMode {
		raw, err := os.ReadFile(s.outPath)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCaptureFailed, err)
		}
		return decodePCM(raw)
	}
	if err := s.sink.Sync(); err != nil {
		slog.Debug("Voice capture sync failed", "error", err)
	}
	name := s.sink.Name()
	raw, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrCaptureFailed, err)
	}
	return decodePCM(raw)
}

// cleanup removes the temporary files backing the capture.
func (s *Session) cleanup() error {
	if s.pipe != nil {
		_ = s.pipe.Close()
	}
	if s.sink != nil {
		name := s.sink.Name()
		if err := s.sink.Close(); err != nil {
			return err
		}
		return os.Remove(name)
	}
	return os.Remove(s.outPath)
}

// emptyCaptureError explains a capture that produced no audio. A non-zero
// exit means the recorder itself failed, usually because the machine has no
// usable input device.
func (s *Session) emptyCaptureError() error {
	tail := s.stderr.String()
	switch {
	case s.processErr != nil && tail != "":
		return fmt.Errorf("%w: %v: %s", ErrCaptureFailed, s.processErr, tail)
	case s.processErr != nil:
		return fmt.Errorf("%w: %v", ErrCaptureFailed, s.processErr)
	case tail != "":
		return fmt.Errorf("%w: no audio captured: %s", ErrCaptureFailed, tail)
	default:
		return fmt.Errorf("%w: no audio captured", ErrCaptureFailed)
	}
}

// tailBuffer keeps the last bytes written to it, for error messages.
type tailBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

var _ io.Writer = (*tailBuffer)(nil)

func (b *tailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, err := b.buf.Write(p); err != nil {
		return 0, err
	}
	if b.buf.Len() > tailLimit {
		trimmed := b.buf.Bytes()[b.buf.Len()-tailLimit:]
		b.buf.Reset()
		b.buf.Write(trimmed)
	}
	return len(p), nil
}

func (b *tailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(bytes.TrimSpace(b.buf.Bytes()))
}

// captureBuilder returns the argv that launches one recorder backend. The out
// path is a temporary file: recorders in file mode write a complete audio file
// to it, everything else streams raw PCM to stdout instead.
type captureBuilder func(ctx context.Context, outPath string) ([]string, error)

// processRecorder captures microphone audio with an external command.
type processRecorder struct {
	name string
	// fileMode reports that the command writes its own audio file to the
	// given path rather than streaming PCM on stdout.
	fileMode bool
	// stop is how the command prefers to be told capture is over.
	stop  stopHow
	build captureBuilder
}

func (r processRecorder) Name() string { return r.name }

// Start launches the recorder. Capture streams into a temporary file until the
// session is stopped, so nothing is held in memory for long recordings.
func (r processRecorder) Start(ctx context.Context) (*Session, error) {
	outPath := ""
	if r.fileMode {
		f, err := os.CreateTemp("", "prime-voice-*.wav")
		if err != nil {
			return nil, err
		}
		outPath = f.Name()
		_ = f.Close()
	}

	argv, err := r.build(ctx, outPath)
	if err != nil {
		removePath(outPath)
		return nil, err
	}
	if len(argv) == 0 {
		removePath(outPath)
		return nil, fmt.Errorf("%w: empty recorder command", ErrCaptureFailed)
	}

	sink, err := os.CreateTemp("", "prime-voice-*.pcm")
	if err != nil {
		removePath(outPath)
		return nil, err
	}

	cmdCtx, cancel := context.WithCancel(ctx)
	cmd := exec.CommandContext(cmdCtx, argv[0], argv[1:]...)
	cmd.WaitDelay = stopGracePeriod
	cmd.Stderr = &tailBuffer{}
	if r.fileMode {
		cmd.Stdout = nil
		sink = nil
	}

	session := &Session{
		recorder: r.name,
		cmd:      cmd,
		sink:     sink,
		outPath:  outPath,
		fileMode: r.fileMode,
		stop:     r.stop,
		copyDone: make(chan struct{}),
		waitDone: make(chan struct{}),
		cancel:   cancel,
		stderr:   cmd.Stderr.(*tailBuffer),
	}

	var writeEnd *os.File
	if !r.fileMode {
		read, write, err := os.Pipe()
		if err != nil {
			cancel()
			close(session.copyDone)
			_ = sink.Close()
			_ = os.Remove(sink.Name())
			removePath(outPath)
			return nil, err
		}
		writeEnd = write
		cmd.Stdout = write
		session.pipe = read
		go func() {
			defer close(session.copyDone)
			_, _ = io.Copy(sink, read)
		}()
	} else {
		close(session.copyDone)
	}

	session.attachStop(cmd)

	if err := cmd.Start(); err != nil {
		cancel()
		if writeEnd != nil {
			_ = writeEnd.Close()
		}
		if session.pipe != nil {
			_ = session.pipe.Close()
		}
		if sink != nil {
			_ = sink.Close()
			_ = os.Remove(sink.Name())
		}
		removePath(outPath)
		return nil, fmt.Errorf("%w: %v", ErrCaptureFailed, err)
	}
	// The parent's copy of the write end has to go, otherwise the capture
	// never sees EOF after the recorder exits.
	if writeEnd != nil {
		_ = writeEnd.Close()
	}

	go func() {
		session.processErr = cmd.Wait()
		close(session.waitDone)
	}()
	return session, nil
}

// attachStop wires up the way this recorder prefers to be told that capture
// is over. os/exec still kills the process when the grace period runs out, so
// a recorder that ignores the polite request cannot hang the stop key.
func (s *Session) attachStop(cmd *exec.Cmd) {
	switch s.stop {
	case stopStdinQ:
		stdin, err := cmd.StdinPipe()
		if err != nil {
			slog.Debug("Voice recorder stdin unavailable", "recorder", s.recorder, "error", err)
			return
		}
		s.stdin = stdin
		cmd.Cancel = func() error {
			if _, err := io.WriteString(stdin, "q\n"); err != nil {
				return cmd.Process.Kill()
			}
			return nil
		}
	case stopInterrupt:
		cmd.Cancel = func() error {
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				return cmd.Process.Kill()
			}
			return nil
		}
	}
}

// usesOutputPlaceholder reports whether a custom command takes over the output
// file itself, which it does when it references the %s placeholder.
func usesOutputPlaceholder(command string) bool {
	return strings.Contains(command, "%s")
}

// shellArgs parses a shell command line into argv, substituting the output
// path for every %s.
func shellArgs(command string) captureBuilder {
	return func(_ context.Context, outPath string) ([]string, error) {
		fields, err := shell.Fields(command, nil)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrCaptureFailed, err)
		}
		for i, field := range fields {
			if strings.Contains(field, "%s") {
				fields[i] = strings.ReplaceAll(field, "%s", outPath)
			}
		}
		return fields, nil
	}
}

func removePath(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
}
