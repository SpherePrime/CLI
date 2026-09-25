package voice

import (
	"bytes"
	"context"
	"fmt"
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

// transcriptMarker matches the timing prefixes command line Whisper engines
// put in front of spoken text, for example "[00:00.000 --> 00:04.000]".
var transcriptMarker = regexp.MustCompile(`(?m)^\s*\[[0-9:.]{4,}\s*-*>\s*[0-9:.]{4,}\]\s*`)

// spokenTag matches engine annotations such as "<noise>" or "[BLANK_AUDIO]".
var spokenTag = regexp.MustCompile(`(?i)[\[(?:<]*(?:noise|blank_audio|silence|backchannel)[\])>]*`)

// tidyTranscript flattens engine output into a single line of dictated text,
// which is what gets inserted at the editor cursor.
func tidyTranscript(raw string) string {
	text := transcriptMarker.ReplaceAllString(raw, "")
	text = spokenTag.ReplaceAllString(text, "")
	text = strings.ReplaceAll(text, "\r", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.Join(strings.Fields(text), " ")
	return strings.TrimSpace(text)
}
