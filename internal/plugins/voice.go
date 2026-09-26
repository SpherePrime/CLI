package plugins

import (
	"context"

	"github.com/SpherePrime/CLI/internal/voice"
)

// VoiceName is the plugin id for microphone dictation.
const VoiceName = "voice"

// voicePlugin wires microphone dictation into the plugin system: enabling it
// downloads the Whisper engine and model when they are missing (the menu
// shows progress), warms the resident server, and hands the CLI its alt+v
// hotkey.
func voicePlugin() Plugin {
	return Plugin{
		Name:        VoiceName,
		Title:       "Voice dictation",
		Description: "Dictate into the prompt with alt+v; Whisper runs on a resident local server.",
		Install: func(ctx context.Context, settings voice.Settings, report func(Progress)) error {
			return voice.EnsureInstalled(ctx, settings, func(pr voice.SetupProgress) {
				if report != nil {
					report(Progress{Phase: pr.Phase, Text: pr.Text, Done: pr.Done, Total: pr.Total})
				}
			})
		},
		Activate: func(ctx context.Context, settings voice.Settings) {
			go func() {
				_ = voice.WarmServer(ctx, settings)
			}()
		},
		Deactivate: func(ctx context.Context) {
			_, _ = voice.StopServer(ctx)
		},
	}
}
