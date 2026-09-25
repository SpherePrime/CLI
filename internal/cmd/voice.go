package cmd

import (
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

var voiceCmd = &cobra.Command{
	Use:   "voice",
	Short: "Check voice dictation setup",
	Long: `Report which microphone recorder and which Whisper engine Prime would use
for dictation in the TUI.

Dictation is toggled with the hotkey from options.voice.hotkey, which is
alt+v or ctrl+shift+space unless you changed it. Recording and transcription
are delegated to external tools, so this lists what was found and what is
missing.`,
	Example: `
# Show the resolved setup
prime voice

# Record from the microphone and print what Whisper hears
prime voice test

# Keep the audio for inspection
prime voice test --duration 6 --save dictation.wav`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		settings, err := voiceSettingsFromConfig(cmd)
		if err != nil {
			return err
		}
		printVoiceSetup(cmd, settings)
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

func init() {
	voiceTestCmd.Flags().IntVar(&voiceTestDuration, "duration", 3, "seconds to record")
	voiceTestCmd.Flags().StringVar(&voiceTestSave, "save", "", "write the recorded audio to this WAV file")
	voiceCmd.AddCommand(voiceTestCmd)
	rootCmd.AddCommand(voiceCmd)
}

// voiceSettingsFromConfig loads options.voice from the merged config.
func voiceSettingsFromConfig(cmd *cobra.Command) (voice.Settings, error) {
	cwd, err := ResolveCwd(cmd)
	if err != nil {
		return voice.Settings{}, err
	}
	dataDir, _ := cmd.Flags().GetString("data-dir")
	debug, _ := cmd.Flags().GetBool("debug")

	store, err := config.Init(cwd, dataDir, debug)
	if err != nil {
		return voice.Settings{}, err
	}
	cfg := store.Config()
	var options *config.VoiceOptions
	if cfg != nil && cfg.Options != nil {
		options = cfg.Options.Voice
	}
	var resolver voice.VariableResolver
	if store.Resolver() != nil {
		resolver = store.Resolver()
	}
	return voice.SettingsFrom(options, resolver), nil
}

// printVoiceSetup reports the resolved pipeline and everything else available.
func printVoiceSetup(cmd *cobra.Command, settings voice.Settings) {
	label := lipgloss.NewStyle().Bold(true).Foreground(colortone.Charple)
	value := lipgloss.NewStyle().Foreground(colortone.Steam)
	missing := lipgloss.NewStyle().Foreground(colortone.Cherry)

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	detector := voice.NewDetector(settings)
	record, transcribe := detector.Backends(ctx)
	plan, planErr := detector.Plan(ctx)

	cmd.Println(label.Render("Enabled:") + " " + value.Render(fmt.Sprint(settings.Enabled)))
	cmd.Println(label.Render("Hotkey:") + " " + value.Render(joinKeys(settings.Hotkeys())))
	cmd.Println(label.Render("Language:") + " " + value.Render(orNone(settings.Language, "auto-detect")))
	cmd.Println(label.Render("Model:") + " " + value.Render(orNone(settings.Model, "default")))
	cmd.Println()

	cmd.Println(label.Render("Microphone recorders:"))
	printBackends(cmd, record, missing)
	cmd.Println()

	cmd.Println(label.Render("Whisper engines:"))
	printTranscribers(cmd, transcribe, missing)
	cmd.Println()

	if planErr != nil {
		cmd.Println(missing.Render("Not ready: " + planErr.Error()))
		return
	}
	cmd.Println(label.Render("In use:") + " " +
		value.Render(plan.Recorder.Name()+" -> "+plan.Transcriber.Name()))
}

// printBackends lists recorder names, highlighting the one Prime picked.
func printBackends(cmd *cobra.Command, recorders []voice.Recorder, missing lipgloss.Style) {
	if len(recorders) == 0 {
		cmd.Println("  " + missing.Render("none found"))
		return
	}
	for i, recorder := range recorders {
		cmd.Println("  " + backendLine(i, recorder.Name()))
	}
}

// printTranscribers lists engine names, highlighting the one Prime picked.
func printTranscribers(cmd *cobra.Command, engines []voice.Transcriber, missing lipgloss.Style) {
	if len(engines) == 0 {
		cmd.Println("  " + missing.Render("none found"))
		return
	}
	for i, engine := range engines {
		cmd.Println("  " + backendLine(i, engine.Name()))
	}
}

// backendLine renders one discovered backend, marking the first as chosen.
func backendLine(index int, name string) string {
	if index == 0 {
		return lipgloss.NewStyle().Bold(true).Render(name) + " (used)"
	}
	return name
}

// runVoiceTest captures audio outside the TUI and prints the transcript.
func runVoiceTest(cmd *cobra.Command, args []string) error {
	settings, err := voiceSettingsFromConfig(cmd)
	if err != nil {
		return err
	}
	if !settings.Enabled {
		return errors.New("voice input is off: run \"option voice on\" in your primerc")
	}
	duration := time.Duration(voiceTestDuration) * time.Second
	if duration <= 0 {
		return errors.New("--duration must be at least one second")
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
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
