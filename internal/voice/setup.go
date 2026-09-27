package voice

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// RecorderKind says how a missing microphone recorder is best obtained.
type RecorderKind int

const (
	// RecorderNone means a recorder is already available.
	RecorderNone RecorderKind = iota
	// RecorderPip installs Python's sounddevice, a couple of megabytes and no
	// admin rights.
	RecorderPip
	// RecorderWinget installs ffmpeg through the Windows package manager.
	RecorderWinget
	// RecorderManual needs a package manager Prime must not call on its own,
	// for example anything that would require sudo.
	RecorderManual
)

// RecorderSetup is the microphone half of an install plan.
type RecorderSetup struct {
	Kind    RecorderKind
	Command []string
	Text    string
}

// SetupProgress is one structured update from an install run. Text carries the
// human line; Done/Total carry byte counts when a step downloads data.
type SetupProgress struct {
	Phase string
	Text  string
	Done  int64
	Total int64
}

// EnsureInstalled makes sure a microphone recorder is present on this machine.
// With Google Web Speech as the default engine, nothing needs to be downloaded
// for transcription, so install is only about the capture half. It finishes in
// one step when the machine is already ready.
func EnsureInstalled(ctx context.Context, settings Settings, emit func(SetupProgress)) error {
	if emit == nil {
		emit = func(SetupProgress) {}
	}
	ready := func() {
		emit(SetupProgress{Phase: "ready", Text: "Voice is ready", Done: 1, Total: 1})
	}

	detector := NewDetector(settings)
	if _, err := detector.Plan(ctx); err == nil {
		ready()
		return nil
	}

	recorder := recorderSetup(ctx, detector.prober)
	switch recorder.Kind {
	case RecorderNone:
		ready()
		return nil
	case RecorderManual:
		return fmt.Errorf("%w: %s", ErrNoRecorder, recorder.Text)
	case RecorderPip, RecorderWinget:
		emit(SetupProgress{Phase: "recorder", Text: recorder.Text + "..."})
		if err := runInstaller(ctx, recorder.Command, func(line string) {
			emit(SetupProgress{Phase: "recorder", Text: line})
		}); err != nil {
			return fmt.Errorf("%s failed: %w", recorder.Text, err)
		}
	}
	detector.Invalidate()
	ready()
	return nil
}

// recorderSetup picks the least invasive way to get a working recorder on this
// platform.
func recorderSetup(ctx context.Context, p prober) RecorderSetup {
	if python, ok := p.python(ctx); ok {
		args := []string{python, "-m", "pip", "install", "sounddevice"}
		if runtime.GOOS != "windows" {
			args = []string{python, "-m", "pip", "install", "--user", "sounddevice"}
		}
		return RecorderSetup{
			Kind:    RecorderPip,
			Command: args,
			Text:    "installing the sounddevice module for Python",
		}
	}
	if winget, ok := p.lookPathTool("winget"); ok {
		return RecorderSetup{
			Kind: RecorderWinget,
			Command: []string{winget, "install", "-e", "--id", "Gyan.FFmpeg",
				"--accept-source-agreements", "--accept-package-agreements"},
			Text: "installing ffmpeg with winget",
		}
	}
	return RecorderSetup{
		Kind: RecorderManual,
		Text: "install a recorder Prime can use: ffmpeg, sox, or Python with " +
			"'pip install sounddevice' (Linux: apt install ffmpeg or sox)",
	}
}

// runInstaller runs an installer command and forwards its output lines.
func runInstaller(ctx context.Context, command []string, log func(string)) error {
	if len(command) == 0 {
		return fmt.Errorf("empty install command")
	}
	execCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(execCtx, command[0], command[1:]...)
	// Installers must never read the terminal, which Prime owns.
	cmd.Stdin = nil
	pipe, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout

	if err := cmd.Start(); err != nil {
		return err
	}
	scanner := bufio.NewScanner(pipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			log("  " + line)
		}
	}
	waitErr := cmd.Wait()
	if execCtx.Err() == context.DeadlineExceeded {
		return fmt.Errorf("installer timed out after 15 minutes")
	}
	return waitErr
}
