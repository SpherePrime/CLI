package voice

import (
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
)

func SettingsFrom(options *config.VoiceOptions, resolver VariableResolver) Settings {
	settings := Settings{Enabled: true, Language: DefaultLanguage, Resolver: resolver}
	if options == nil {
		return settings
	}
	settings.Enabled = options.IsEnabled()
	settings.Language = NormalizeLanguage(options.Language)
	settings.RecordCommand = strings.TrimSpace(options.RecordCommand)
	if options.MaxDuration > 0 {
		settings.MaxDuration = time.Duration(options.MaxDuration) * time.Second
	}
	return settings
}
