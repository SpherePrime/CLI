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
	EngineWhisperCPP,
	EngineWhisperPy,
	EngineCTranslate2,
	EngineServer,
	EngineOpenAI,
	EngineCommand,
}

// SettingsFrom translates options.voice into pipeline settings. A nil options
// block yields working defaults, so voice input needs no configuration on a
// machine that already has a Whisper engine.
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

// normalizedLanguage trims the language code and treats "auto" as unset, which
// keeps "let Whisper detect" the single way to ask for detection.
func normalizedLanguage(language string) string {
	candidate := strings.ToLower(strings.TrimSpace(language))
	if candidate == "auto" {
		return ""
	}
	return candidate
}
