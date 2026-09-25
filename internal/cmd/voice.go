package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/voice"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/exp/colortone"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/spf13/cobra"
)

var voiceTestDuration int
var voiceTestSave string
var voiceSetupModel string
var voiceSetupYes bool
var voiceSetupPrintOnly bool

var voiceCmd = &cobra.Command{
	Use:   "voice",
	Short: "Check and install voice dictation",
	Long: `Report which microphone recorder and which Whisper engine Prime would use
for dictation in the TUI, and install what is missing.

Dictation is toggled with the hotkey from options.voice.hotkey, which is
alt+v or ctrl+shift+space unless you changed it. Recording and transcription
are delegated to external tools, so this lists what was found and what is
missing.`,
	Example: `
# Show the resolved setup
prime voice

# Install a Whisper engine and model, and a recorder if needed
prime voice setup

# Record from the microphone and print what Whisper hears
prime voice test

# Keep the audio for inspection
prime voice test --duration 6 --save dictation.wav`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		settings, store, err := voiceSettingsFromConfig(cmd)
		if err != nil {
			return err
		}
		printVoiceSetup(cmd, settings, store)
		return nil
	},
}

var voiceTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Record from the microphone and print the transcript",
	Long: `Record a short sample from the default microphone, transcribe it with the
Whisper engine Prime would pick in the TUI, and print the text. Use it to check
that capture and transcription work before dictating into a session.`,
	Args: cobra.NoArgs,
	RunE: runVoiceTest,
}

var voiceSetupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Download a Whisper engine and install what voice input needs",
	Long: `Install voice input so the dictation hotkey works with no further setup.

Prime downloads a prebuilt whisper.cpp binary and a multilingual model into its
own data directory, and installs a microphone recorder when none is present.
Nothing is written to system locations and no administrator rights are needed.
Already working pieces are left alone.`,
	Example: `
# See what would be installed, without doing it
prime voice setup --print

# Install the small model instead of the default one
prime voice setup --model small

# Install without a prompt, for scripts
prime voice setup --yes`,
	Args: cobra.NoArgs,
	RunE: runVoiceSetup,
}

func init() {
	voiceTestCmd.Flags().IntVar(&voiceTestDuration, "duration", 3, "seconds to record")
	voiceTestCmd.Flags().StringVar(&voiceTestSave, "save", "", "write the recorded audio to this WAV file")

	voiceSetupCmd.Flags().StringVar(&voiceSetupModel, "model", "",
		"model to download: large-v3-turbo-q5_0, tiny, base, small, medium (default large-v3-turbo-q5_0)")
	voiceSetupCmd.Flags().BoolVar(&voiceSetupYes, "yes", false,
		"install without asking for confirmation")
	voiceSetupCmd.Flags().BoolVar(&voiceSetupPrintOnly, "print", false,
		"only show what would be installed")

	voiceCmd.AddCommand(voiceTestCmd)
	voiceCmd.AddCommand(voiceSetupCmd)
	rootCmd.AddCommand(voiceCmd)
}

// voiceSetupStore keeps the config store around so a report can resolve an
// API key template the same way the TUI does.
type voiceSetupStore interface {
	Resolver() config.VariableResolver
}

// voiceSettingsFromConfig loads options.voice from the merged config.
func voiceSettingsFromConfig(cmd *cobra.Command) (voice.Settings, voiceSetupStore, error) {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return voice.Settings{}, nil, err
	}
	dataDir, _ := cmd.Flags().GetString("data-dir")
	debug, _ := cmd.Flags().GetBool("debug")

	store, err := config.Init(cwd, dataDir, debug)
	if err != nil {
		return voice.Settings{}, nil, err
	}
	cfg := store.Config()
	var options *config.VoiceOptions
	if cfg != nil && cfg.Options != nil {
		options = cfg.Options.Voice
	}
	settings := voice.SettingsFrom(options, store.Resolver())
	return settings, store, nil
}

// printVoiceSetup reports the resolved pipeline and everything else available.
func printVoiceSetup(cmd *cobra.Command, settings voice.Settings, store voiceSetupStore) {
	label := lipgloss.NewStyle().Bold(true).Foreground(colortone.Charple)
	value := lipgloss.NewStyle().Foreground(colortone.Steam)
	missing := lipgloss.NewStyle().Foreground(colortone.Cherry)

	ctx := commandContext(cmd)
	detector := voice.NewDetector(settings)
	record, transcribe := detector.Backends(ctx)
	plan, planErr := detector.Plan(ctx)

	cmd.Println(label.Render("Enabled:") + " " + value.Render(fmt.Sprint(settings.Enabled)))
	cmd.Println(label.Render("Hotkey:") + " " + value.Render(joinKeys(settings.Hotkeys())))
	cmd.Println(label.Render("Language:") + " " + value.Render(orNone(settings.Language, "auto-detect")))
	cmd.Println(label.Render("Model:") + " " + value.Render(orNone(settings.Model, "default")))
	cmd.Println()

	cmd.Println(label.Render("Microphone recorders:"))
	printBackends(cmd, names(record), missing)
	cmd.Println()

	cmd.Println(label.Render("Whisper engines:"))
	printTranscribers(cmd, names(transcribe), missing)
	cmd.Println()

	cmd.Println(label.Render("Installed by Prime:") + " " + value.Render(detector.Layout().Root))
	cmd.Println()

	if planErr != nil {
		cmd.Println(missing.Render("Not ready: " + planErr.Error()))
		if errors.Is(planErr, voice.ErrNoRecorder) || errors.Is(planErr, voice.ErrNoTranscriber) {
			cmd.Println(missing.Render("Fix it with: prime voice setup"))
		}
		return
	}
	cmd.Println(label.Render("In use:") + " " +
		value.Render(plan.Recorder.Name()+" -> "+plan.Transcriber.Name()))
}

// names turns backend lists into their display names.
func names[T interface{ Name() string }](items []T) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		out = append(out, item.Name())
	}
	return out
}

// printBackends lists recorder names, marking the first as chosen.
func printBackends(cmd *cobra.Command, recorders []string, missing lipgloss.Style) {
	if len(recorders) == 0 {
		cmd.Println("  " + missing.Render("none found"))
		return
	}
	for i, recorder := range recorders {
		cmd.Println("  " + backendLine(i, recorder))
	}
}

// printTranscribers lists engine names, marking the first as chosen.
func printTranscribers(cmd *cobra.Command, engines []string, missing lipgloss.Style) {
	if len(engines) == 0 {
		cmd.Println("  " + missing.Render("none found"))
		return
	}
	for i, engine := range engines {
		cmd.Println("  " + backendLine(i, engine))
	}
}

// backendLine renders one discovered backend, marking the first as chosen.
func backendLine(index int, name string) string {
	if index == 0 {
		return lipgloss.NewStyle().Bold(true).Render(name) + " (used)"
	}
	return name
}

// runVoiceSetup installs what voice input is missing, after showing the plan.
func runVoiceSetup(cmd *cobra.Command, args []string) error {
	settings, _, err := voiceSettingsFromConfig(cmd)
	if err != nil {
		return err
	}

	ctx := commandContext(cmd)
	detector := voice.NewDetector(settings)
	plan, err := detector.SetupPlan(ctx, voice.SetupOptions{Model: voiceSetupModel})
	if err != nil {
		return err
	}

	cmd.Println("Voice input setup:")
	for _, step := range plan.Steps() {
		cmd.Println("  - " + step)
	}
	if size := plan.TotalBytes(); size > 0 {
		cmd.Printf("  downloads: about %.1f MB total\n", float64(size)/(1<<20))
	}
	if !plan.NeedsApproval() {
		cmd.Println("Nothing to install. Try it with: prime voice test")
		return nil
	}
	if voiceSetupPrintOnly {
		return nil
	}
	if !voiceSetupYes {
		approved, err := confirmSetup(cmd, "Download and install this now? [y/N] ")
		if err != nil {
			return err
		}
		if !approved {
			cmd.Println("Canceled. Run prime voice setup --yes to install without asking.")
			return nil
		}
	}

	log := func(line string) { cmd.Println(line) }
	if err := plan.Apply(ctx, log); err != nil {
		return err
	}

	// Re-detect with a fresh detector so the report reflects what just landed.
	fresh := voice.NewDetector(settings)
	freshPlan, planErr := fresh.Plan(ctx)
	if planErr != nil {
		return fmt.Errorf("installed, but voice input is still not ready: %w", planErr)
	}
	cmd.Println("Ready: " + freshPlan.Recorder.Name() + " -> " + freshPlan.Transcriber.Name())
	cmd.Println("Restart any open Prime session, then press " +
		strings.Join(settings.Hotkeys(), " or ") + " to dictate.")
	return nil
}

// confirmSetup reads a yes/no answer from the terminal.
func confirmSetup(cmd *cobra.Command, prompt string) (bool, error) {
	cmd.Print(prompt)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return false, errors.New("no answer read from the terminal")
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

// runVoiceTest captures audio outside the TUI and prints the transcript.
func runVoiceTest(cmd *cobra.Command, args []string) error {
	settings, _, err := voiceSettingsFromConfig(cmd)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return errors.New(`voice input is off: run "option voice on" in your primerc`)
	}
	duration := time.Duration(voiceTestDuration) * time.Second
	if duration <= 0 {
		return errors.New("--duration must be at least one second")
	}

	ctx := commandContext(cmd)
	detector := voice.NewDetector(settings)
	plan, err := detector.Plan(ctx)
	if err != nil {
		return err
	}

	cmd.Printf("Recording %.0fs with %s, speak now...\n", duration.Seconds(), plan.Recorder.Name())
	session, err := plan.Start(ctx)
	if err != nil {
		return err
	}
	time.Sleep(duration)

	audio, err := session.Stop()
	if err != nil {
		return err
	}
	cmd.Printf("Captured %s of audio.\n", audio.Duration.Round(time.Millisecond))
	if voiceTestSave != "" {
		if err := os.WriteFile(voiceTestSave, audio.WAV, 0o600); err != nil {
			return err
		}
		cmd.Printf("Saved audio to %s\n", voiceTestSave)
	}
	if !audio.WorthTranscribing() {
		return errors.New("recording is too short to transcribe")
	}

	cmd.Printf("Transcribing with %s...\n", plan.Transcriber.Name())
	text, err := plan.Transcribe(ctx, audio)
	if err != nil {
		return err
	}
	if text == "" {
		cmd.Println("No speech recognized.")
		return nil
	}
	cmd.Println(text)
	return nil
}

// commandContext returns the command's context, or a background one when the
// command was built without one.
func commandContext(cmd *cobra.Command) context.Context {
	if ctx := cmd.Context(); ctx != nil {
		return ctx
	}
	return context.Background()
}

// joinKeys renders hotkey candidates for the report.
func joinKeys(keys []string) string {
	if len(keys) == 0 {
		return "none"
	}
	return strings.Join(keys, ", ")
}

// orNone returns value, or fallback when it is empty.
func orNone(value string, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
