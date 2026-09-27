package plugins

import (
	"context"

	"github.com/SpherePrime/CLI/internal/voice"
)

// VoiceName is the plugin id for microphone dictation.
const VoiceName = "voice"

// voicePlugin wires microphone dictation into the plugin system: enabling it
// installs the microphone recorder when it is missing (the menu shows
// progress) and hands the CLI its alt+v hotkey. Google Web Speech needs no
// download, so activation is instant.
func voicePlugin() Plugin {
	return Plugin{
		Name:        VoiceName,
		Title:       "Voice dictation",
		Description: "Dictate into the prompt with alt+v; transcription goes through Google Web Speech.",
		Install: func(ctx context.Context, settings voice.Settings, report func(Progress)) error {
			return voice.EnsureInstalled(ctx, settings, func(pr voice.SetupProgress) {
				if report != nil {
					report(Progress{Phase: pr.Phase, Text: pr.Text, Done: pr.Done, Total: pr.Total})
				}
			})
		},
		Activate:   func(ctx context.Context, settings voice.Settings) {},
		Deactivate: func(ctx context.Context) {},
	}
}
