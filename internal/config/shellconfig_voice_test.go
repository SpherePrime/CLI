package config_test

import (
	"os"
	"path/filepath"
	"strconv"
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

// The accelerator normalizes to the GPU accelerators or empty: a typo must
// degrade to the CPU build rather than break the engine, and "cpu" is the
// default, which is stored as empty.
func TestVoiceSettingsAccelerator(t *testing.T) {
	store := loadPrimeSh(t, "option voice accelerator cuda\noption voice gpu-device 1")

	settings := voice.SettingsFrom(store.Config().Options.Voice, store.Resolver())
	require.Equal(t, voice.AcceleratorCUDA, settings.Accelerator)
	require.Equal(t, "1", settings.GPUDevice)

	store = loadPrimeSh(t, "option voice accelerator vulkan")
	settings = voice.SettingsFrom(store.Config().Options.Voice, store.Resolver())
	require.Equal(t, voice.AcceleratorVulkan, settings.Accelerator)

	store = loadPrimeSh(t, "option voice accelerator cpu")
	settings = voice.SettingsFrom(store.Config().Options.Voice, store.Resolver())
	require.Empty(t, settings.Accelerator, "cpu is the default, so it normalizes to empty")
	require.Empty(t, settings.GPUDevice)

	// The shell builtin refuses an unknown accelerator outright, so the
	// typo case only reaches the settings through a hand-edited config; the
	// settings layer is the last line of defense.
	store = storeWithVoiceAccelerator(t, "rocm")
	settings = voice.SettingsFrom(store.Config().Options.Voice, store.Resolver())
	require.Empty(t, settings.Accelerator, "an unknown accelerator means the CPU build")
}

// storeWithVoiceAccelerator loads a store whose global options.json pins the
// voice accelerator directly, bypassing the shell builtin's validation: a
// hand-edited config is the one way an unknown value reaches the settings.
func storeWithVoiceAccelerator(t *testing.T, accelerator string) *config.ConfigStore {
	t.Helper()
	root := t.TempDir()
	configDir := filepath.Join(root, "config", "prime")
	require.NoError(t, os.MkdirAll(configDir, 0o755))
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("PRIME_GLOBAL_CONFIG", configDir)
	t.Setenv("PRIME_GLOBAL_DATA", filepath.Join(root, "data", "prime"))

	section := filepath.Join(configDir, "options.json")
	require.NoError(t, os.WriteFile(section,
		[]byte(`{"options":{"voice":{"accelerator":`+strconv.Quote(accelerator)+`}}}`), 0o600))

	store, err := config.Load(t.TempDir(), t.TempDir(), false)
	require.NoError(t, err)
	return store
}
