package voice

import (
	"slices"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
)

// Engines lists the accepted values of options.voice.engine.
var Engines = []string{
	EngineAuto,
	EngineGoogle,
	EngineOpenAI,
	EngineCommand,
}

// SettingsFrom translates options.voice into pipeline settings. A nil options
// block yields working defaults, so voice input needs no configuration.
func SettingsFrom(options *config.VoiceOptions, resolver VariableResolver) Settings {
	settings := Settings{Enabled: true, Resolver: resolver}
	if options == nil {
		return settings
	}
	settings.Enabled = options.IsEnabled()
	settings.Engine = normalizedEngine(options.Engine)
	settings.Model = strings.TrimSpace(options.Model)
	settings.Language = normalizedLanguage(options.Language)
	settings.BaseURL = strings.TrimSpace(options.BaseURL)
	settings.APIKey = strings.TrimSpace(options.APIKey)
	settings.RecordCommand = strings.TrimSpace(options.RecordCommand)
	settings.TranscribeCommand = strings.TrimSpace(options.TranscribeCommand)
	if options.MaxDuration > 0 {
		settings.MaxDuration = time.Duration(options.MaxDuration) * time.Second
	}
	return settings
}

// normalizedEngine lowercases the configured engine and falls back to
// detection when it names something Prime does not know, so a typo degrades to
// working behavior instead of a dead hotkey.
func normalizedEngine(engine string) string {
	candidate := strings.ToLower(strings.TrimSpace(engine))
	if slices.Contains(Engines, candidate) {
		return candidate
	}
	return EngineAuto
}

// normalizedLanguage trims the language setting and treats "auto" as unset,
// which keeps "let the engine detect" the single way to ask for detection. A
// comma-separated list keeps its order, because that order is the order an
// engine which has to pick a language itself searches in.
func normalizedLanguage(language string) string {
	return strings.Join(languageCandidates(language), ",")
}
