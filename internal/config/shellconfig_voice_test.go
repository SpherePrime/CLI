package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/voice"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestShellConfigOptionVoice(t *testing.T) {
	store := loadPrimeSh(t, `option voice on
option voice language ru
option voice max-duration 45
option voice record-command "rec -q -t raw -"`)

	options := store.Config().Options
	require.NotNil(t, options)

	blocks := options.Voice
	require.NotNil(t, blocks)
	require.True(t, blocks.IsEnabled())
	require.Equal(t, "ru", blocks.Language)
	require.Equal(t, 45, blocks.MaxDuration)
	require.Equal(t, "rec -q -t raw -", blocks.RecordCommand)
}

func TestShellConfigOptionVoiceOff(t *testing.T) {
	store := loadPrimeSh(t, `option voice off`)
	require.False(t, store.Config().Options.Voice.IsEnabled())
}

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

func TestLegacyVoiceOptionsJsonIgnoredSafely(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "config", "prime")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("PRIME_GLOBAL_CONFIG", configDir)
	t.Setenv("PRIME_GLOBAL_DATA", filepath.Join(root, "data", "prime"))

	section := filepath.Join(configDir, "options.json")
	require.NoError(t, os.WriteFile(section,
		[]byte(`{"options":{"voice":{"engine":"google","model":"ggml-tiny","accelerator":"cuda","language":"uk-UA"}}}`), 0o600))

	store, err := config.Load(t.TempDir(), t.TempDir(), false)
	require.NoError(t, err)
	settings := voice.SettingsFrom(store.Config().Options.Voice, store.Resolver())
	require.Equal(t, "uk", settings.Language)
}
