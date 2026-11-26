package plugins

import (
	"context"

	"github.com/SpherePrime/CLI/internal/voice"
)

const VoiceName = "voice"

func voicePlugin() Plugin {
	return Plugin{
		Name:        VoiceName,
		Title:       "Voice dictation",
		Description: "Dictate into the prompt with alt+v using the built-in local TinyASR model (en-ru-uk).",
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
