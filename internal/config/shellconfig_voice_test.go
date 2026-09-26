package config_test

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/voice"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// options.voice is a nested block, so the keys written by the builtin have to
// match the JSON tags on the struct exactly.
func TestShellConfigOptionVoice(t *testing.T) {
	store := loadPrimeSh(t, `option voice on
option voice language ru
option voice engine openai
option voice model small
option voice base-url http://localhost:8000
option voice api-key sk-test
option voice max-duration 45
option voice record-command "rec -q -t raw -"
option voice transcribe-command "my-whisper %s"`)

	options := store.Config().Options
	require.NotNil(t, options)

	blocks := options.Voice
	require.NotNil(t, blocks)
	require.True(t, blocks.IsEnabled())
	require.Equal(t, "ru", blocks.Language)
	require.Equal(t, "openai", blocks.Engine)
	require.Equal(t, "small", blocks.Model)
	require.Equal(t, "http://localhost:8000", blocks.BaseURL)
	require.Equal(t, "sk-test", blocks.APIKey)
	require.Equal(t, 45, blocks.MaxDuration)
	require.Equal(t, "rec -q -t raw -", blocks.RecordCommand)
	require.Equal(t, "my-whisper %s", blocks.TranscribeCommand)
}

func TestShellConfigOptionVoiceOff(t *testing.T) {
	store := loadPrimeSh(t, `option voice off`)
	require.False(t, store.Config().Options.Voice.IsEnabled())
}

// The default has to stay "enabled", because the option block is absent for
// most users and dictation should work wherever the tools are installed.
func TestVoiceEnabledByDefault(t *testing.T) {
	var blocks *config.VoiceOptions
	require.True(t, blocks.IsEnabled())
}

func TestVoiceSettingsFollowConfig(t *testing.T) {
	store := loadPrimeSh(t, `option voice language ru`)

	settings := voice.SettingsFrom(store.Config().Options.Voice, store.Resolver())
	require.True(t, settings.Enabled)
	require.Equal(t, "ru", settings.Language)
}
