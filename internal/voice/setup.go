package voice

import (
	"bufio"
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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

// RecorderSetup is the microphone half of a setup plan.
type RecorderSetup struct {
	Kind    RecorderKind
	Command []string
	Text    string
}

// SetupPlan is what the voice MCP server's setup tool would install, and what
// it does when the user agrees. Nothing is downloaded or run until Apply is
// called.
type SetupPlan struct {
	// Layout is where downloaded tools are placed.
	Layout InstallLayout
	// Engine carries the whisper.cpp archive to download, nil when a working
	// engine already exists.
	Engine *releaseAsset
	// EngineTag is the release the engine archive comes from.
	EngineTag string
	// Model is the ggml model to download, empty when one is already found or
	// no engine will exist after setup.
	Model string
	// ModelURL overrides where the model is downloaded from, which setup fills
	// in from the public model repository and tests point at a local server.
	ModelURL string
	// Recorder is the microphone step, with Kind RecorderNone when capture
	// already works.
	Recorder RecorderSetup
	// Notes explain decisions the user should know before approving.
	Notes []string
}

// SetupOptions tunes a setup plan. Zero values mean the platform defaults.
type SetupOptions struct {
	// Model overrides the model name to download.
	Model string
	// ReleasesURL points at the GitHub release listing; tests serve their own.
	ReleasesURL string
	// Client is the HTTP client used for the release list and downloads.
	Client *http.Client
}

// ReleasesURL is the endpoint holding the prebuilt Whisper binaries.
func (o SetupOptions) releasesURL() string {
	if o.ReleasesURL != "" {
		return o.ReleasesURL
	}
	return whisperCPPAPIURL
}

// client returns the HTTP client to use for setup traffic.
func (o SetupOptions) client() *http.Client {
	if o.Client != nil {
		return o.Client
	}
	return &http.Client{Timeout: downloadTimeout}
}

// model returns the model name to install.
func (o SetupOptions) model() string {
	if strings.TrimSpace(o.Model) != "" {
		return strings.TrimSpace(o.Model)
	}
	return DefaultSetupModel
}

// SetupPlan works out what this machine still needs for voice input.
func (d *Detector) SetupPlan(ctx context.Context, options SetupOptions) (*SetupPlan, error) {
	return planSetup(ctx, d.Settings(), d.prober, options)
}

// planSetup is the testable core of SetupPlan.
func planSetup(ctx context.Context, settings Settings, p prober, options SetupOptions) (*SetupPlan, error) {
	plan := &SetupPlan{Layout: p.layout}

	recorders := newRecorders(ctx, settings, p)
	if len(recorders) == 0 {
		plan.Recorder = recorderSetup(ctx, p)
	}

	engines := newTranscribers(ctx, settings, p)
	if len(engines) > 0 {
		plan.Notes = append(plan.Notes, "a Whisper engine is already available: "+engines[0].Name())
		return plan, nil
	}

	assetName, ok := platformAssetName()
	if !ok {
		plan.Notes = append(plan.Notes,
			"no prebuilt Whisper binaries exist for "+runtime.GOOS+"/"+runtime.GOARCH+
				"; install whisper.cpp or openai-whisper with your package manager, "+
				"or point option voice base-url and api-key at a transcription endpoint")
		return plan, nil
	}

	releases, err := releasesClient(ctx, options.client(), options.releasesURL())
	if err != nil {
		return nil, err
	}
	asset, tag, found := pickAsset(releases, assetName)
	if !found {
		return nil, fmt.Errorf("the newest whisper.cpp releases carry no %s archive", assetName)
	}
	plan.Engine = &asset
	plan.EngineTag = tag

	if _, ok := whisperModelFile(settings, p.layout); !ok {
		name := options.model()
		if !setupModelKnown(name) {
			return nil, fmt.Errorf("unknown model %q, choose one of: %s",
				name, strings.Join(modelNames(), ", "))
		}
		plan.Model = name
		plan.ModelURL = modelURLFor(name)
	}
	if plan.Engine == nil && plan.Model == "" && plan.Recorder.Kind == RecorderNone {
		plan.Notes = append(plan.Notes, "nothing to install: the voice MCP server is ready to dictate")
	}
	return plan, nil
}

// modelNames lists the downloadable model names.
func modelNames() []string {
	names := make([]string, 0, len(SetupModels))
	for _, model := range SetupModels {
		names = append(names, model.Name)
	}
	return names
}

// whisperModelFile looks for the model configured by name or path, including
// the directory where setup downloads models.
func whisperModelFile(settings Settings, layout InstallLayout) (string, bool) {
	return ggmlModelFile(settings.Model, layout.modelSearchDirs())
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

// Empty reports whether setup has nothing left to do.
func (p *SetupPlan) Empty() bool {
	return p == nil || (p.Engine == nil && p.Model == "" && p.Recorder.Kind == RecorderNone)
}

// NeedsApproval reports whether setup would download anything or run an
// installer, which is what the confirmation prompt is about.
func (p *SetupPlan) NeedsApproval() bool {
	return p.Engine != nil || p.Model != "" || p.Recorder.Kind != RecorderNone
}

// Steps renders the plan as lines a person can read before approving.
func (p *SetupPlan) Steps() []string {
	var steps []string
	if p.Recorder.Kind != RecorderNone {
		steps = append(steps, "Microphone: "+p.Recorder.Text)
	}
	if p.Engine != nil {
		steps = append(steps, fmt.Sprintf("Whisper engine: whisper.cpp %s (%s, %.1f MB) into %s",
			p.EngineTag, p.Engine.Name, float64(p.Engine.Size)/(1<<20), p.Layout.Bin()))
	}
	if p.Model != "" {
		steps = append(steps, fmt.Sprintf("Whisper model: %s (about %d MB) into %s",
			p.Model, modelSizeMB(p.Model), p.Layout.Models()))
	}
	steps = append(steps, p.Notes...)
	if len(steps) == 0 {
		steps = append(steps, "nothing to do, voice input is already set up")
	}
	return steps
}

// TotalBytes is the download size the plan asks the user to accept.
func (p *SetupPlan) TotalBytes() int64 {
	var total int64
	if p.Engine != nil {
		total += p.Engine.Size
	}
	if p.Model != "" {
		total += int64(modelSizeMB(p.Model)) << 20
	}
	return total
}

// Apply carries out the plan: engine binaries, then the model, then the
// recorder. Progress goes to log, which keeps the CLI and any future UI caller
// in control of output.
func (p *SetupPlan) Apply(ctx context.Context, log func(string)) error {
	if log == nil {
		log = func(string) {}
	}
	if err := p.installEngine(ctx, log); err != nil {
		return err
	}
	if err := p.installModel(ctx, log); err != nil {
		return err
	}
	return p.installRecorder(ctx, log)
}

// installEngine downloads and unpacks the whisper.cpp archive.
func (p *SetupPlan) installEngine(ctx context.Context, log func(string)) error {
	if p.Engine == nil {
		return nil
	}
	if err := os.MkdirAll(p.Layout.Bin(), 0o755); err != nil {
		return err
	}

	temp, err := os.CreateTemp("", "prime-voice-*"+filepath.Ext(p.Engine.Name))
	if err != nil {
		return err
	}
	archive := temp.Name()
	_ = temp.Close()
	defer func() { _ = os.Remove(archive) }()

	log(fmt.Sprintf("Downloading %s (%.1f MB)...", p.Engine.Name, float64(p.Engine.Size)/(1<<20)))
	client := &http.Client{Timeout: downloadTimeout}
	if err := downloadFile(ctx, client, p.Engine.URL, archive, assetSHA256(*p.Engine),
		func(done, total int64) { log(fmt.Sprintf("  %s of %s", humanBytes(done), humanMB(total))) }); err != nil {
		return err
	}

	log("Unpacking into " + p.Layout.Bin())
	if _, err := extractArchive(archive, p.Layout.Bin()); err != nil {
		return err
	}
	if _, ok := p.Layout.Binary(whisperCPPBinary); !ok {
		return fmt.Errorf("whisper.cpp was unpacked but %s is missing from %s", whisperCPPBinary, p.Layout.Bin())
	}
	return nil
}

// installModel downloads the ggml model file next to the engine.
func (p *SetupPlan) installModel(ctx context.Context, log func(string)) error {
	if p.Model == "" {
		return nil
	}
	if err := os.MkdirAll(p.Layout.Models(), 0o755); err != nil {
		return err
	}
	dest := filepath.Join(p.Layout.Models(), modelFileName(p.Model))
	if info, err := os.Stat(dest); err == nil && info.Size() > 0 {
		log("Model " + p.Model + " is already downloaded")
		return nil
	}

	log(fmt.Sprintf("Downloading the %s model (about %d MB)... this is the slow part",
		p.Model, modelSizeMB(p.Model)))
	url := p.ModelURL
	if url == "" {
		url = modelURLFor(p.Model)
	}
	client := &http.Client{Timeout: downloadTimeout}
	if err := downloadFile(ctx, client, url, dest, "",
		func(done, total int64) { log(fmt.Sprintf("  %s downloaded", humanBytes(done))) }); err != nil {
		return err
	}
	log("Model ready: " + dest)
	return nil
}

// installRecorder runs the installer chosen by the plan, or explains what has
// to be done by hand.
func (p *SetupPlan) installRecorder(ctx context.Context, log func(string)) error {
	switch p.Recorder.Kind {
	case RecorderNone:
		return nil
	case RecorderManual:
		log(p.Recorder.Text)
		return fmt.Errorf("voice input still needs a microphone recorder: %s", p.Recorder.Text)
	case RecorderPip, RecorderWinget:
		log(p.Recorder.Text + "...")
		if err := runInstaller(ctx, p.Recorder.Command, log); err != nil {
			return fmt.Errorf("%s failed: %w", p.Recorder.Text, err)
		}
	}
	return nil
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

// humanBytes renders a byte count for progress lines.
func humanBytes(bytes int64) string {
	switch {
	case bytes >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(bytes)/(1<<30))
	case bytes >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(bytes)/(1<<20))
	case bytes >= 1<<10:
		return fmt.Sprintf("%.0f KB", float64(bytes)/(1<<10))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// humanMB renders a total that may be unknown, as reported by the server.
func humanMB(bytes int64) string {
	if bytes < 0 {
		return "unknown size"
	}
	return humanBytes(bytes)
}
