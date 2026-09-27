package voice

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/sh/v3/shell"
)

// commandOutputLimit bounds how much engine output is kept in memory.
const commandOutputLimit = 1 << 20

// shellFields parses a command line into argv without running a shell, so
// quoted arguments behave the way they look.
func shellFields(command string) ([]string, error) {
	return shell.Fields(command, nil)
}

// runCommand runs argv and returns the trimmed output streams.
func runCommand(ctx context.Context, name string, args []string) (string, string, error) {
	return runCommandWithEnv(ctx, name, args, nil)
}

// mergeEnv returns an environment with extra entries appended, or nil to keep
// inheriting the current environment untouched.
func mergeEnv(extra []string) []string {
	if len(extra) == 0 {
		return nil
	}
	return append(os.Environ(), extra...)
}

// runCommandWithEnv runs argv with an explicit environment, or the current one
// when env is nil.
func runCommandWithEnv(ctx context.Context, name string, args []string, env []string) (stdout string, stderr string, err error) {
	ctx, cancel := context.WithTimeout(ctx, transcriptionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.WaitDelay = 5 * time.Second
	if env != nil {
		cmd.Env = env
	}
	// The terminal belongs to the TUI, so a tool that reads stdin must never
	// see it.
	cmd.Stdin = nil

	var out, errOut bytes.Buffer
	cmd.Stdout = &limitedWriter{buf: &out}
	cmd.Stderr = &limitedWriter{buf: &errOut}

	runErr := cmd.Run()
	stdout = out.String()
	stderr = strings.TrimSpace(errOut.String())
	if runErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return stdout, stderr, fmt.Errorf("timed out after %s", transcriptionTimeout)
		}
		return stdout, stderr, runErr
	}
	return stdout, stderr, nil
}

// limitedWriter caps captured output so a verbose engine cannot grow the
// buffer without bound.
type limitedWriter struct {
	buf   *bytes.Buffer
	limit int
}

func (w *limitedWriter) Write(p []byte) (int, error) {
	limit := w.limit
	if limit == 0 {
		limit = commandOutputLimit
	}
	if room := limit - w.buf.Len(); room > 0 {
		if len(p) > room {
			w.buf.Write(p[:room])
			return len(p), nil
		}
	}
	return w.buf.Write(p)
}

// transcriptMarker matches the timing prefixes command line engines
// put in front of spoken text.
var transcriptMarker = regexp.MustCompile(`(?m)^\s*\[[0-9:.]{4,}\s*-*>\s*[0-9:.]{4,}\]\s*`)

// spokenTag matches engine annotations such as "<noise>" or "[BLANK_AUDIO]".
var spokenTag = regexp.MustCompile(`(?i)[\[(?:<]*(?:noise|blank_audio|silence|backchannel)[\])>]*`)

// tidyTranscript flattens engine output into a single line of dictated text.
func tidyTranscript(raw string) string {
	text := transcriptMarker.ReplaceAllString(raw, "")
	text = spokenTag.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}

// commandTranscriber runs options.voice.transcribe-command, whose stdout is
// taken as the transcript.
type commandTranscriber struct {
	command string
	lang    string
}

func (t commandTranscriber) Name() string { return "transcribe-command" }

func (t commandTranscriber) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	if audio == nil || len(audio.WAV) == 0 {
		return "", ErrNoSpeech
	}
	file, err := os.CreateTemp("", "prime-voice-dictation-*.wav")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	path := file.Name()
	defer func() { _ = os.Remove(path) }()

	if _, err := file.Write(audio.WAV); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if err := file.Close(); err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}

	args, err := commandTranscribeArgs(t.command, path)
	if err != nil {
		return "", err
	}

	stdout, stderr, err := runCommandWithEnv(ctx, args[0], args[1:], commandTranscribeEnv(t.lang))
	if err != nil {
		return "", engineError(err, stderr)
	}
	return tidyTranscript(stdout), nil
}

// commandTranscribeArgs turns options.voice.transcribe-command into argv.
func commandTranscribeArgs(command string, wavPath string) ([]string, error) {
	fields, err := shellFields(command)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if len(fields) == 0 {
		return nil, fmt.Errorf("%w: empty transcription command", ErrTranscribeFailed)
	}
	args := make([]string, 0, len(fields))
	for _, field := range fields {
		args = append(args, strings.ReplaceAll(field, "%s", wavPath))
	}
	if !strings.Contains(command, "%s") {
		args = append(args, wavPath)
	}
	return args, nil
}

// commandTranscribeEnv hands a custom engine the recording format.
func commandTranscribeEnv(language string) []string {
	env := os.Environ()
	if language != "" {
		env = append(env, "PRIME_VOICE_LANGUAGE="+language)
	}
	return append(env, "PRIME_VOICE_SAMPLE_RATE=16000")
}

// engineError explains a failed engine run, keeping the tool's own complaint.
func engineError(err error, stderr string) error {
	if stderr != "" {
		return fmt.Errorf("%w: %v: %s", ErrTranscribeFailed, err, stderr)
	}
	return fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
}
