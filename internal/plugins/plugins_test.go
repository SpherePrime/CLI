package plugins

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestRegistryHasVoice(t *testing.T) {
	plugin, ok := Get(VoiceName)
	require.True(t, ok)
	require.NotNil(t, plugin.Install)
	require.NotNil(t, plugin.Activate)
	require.NotNil(t, plugin.Deactivate)
	require.NotEmpty(t, plugin.Title)

	_, ok = Get("nope")
	require.False(t, ok)
}

func TestEnabledReflectsConfig(t *testing.T) {
	cfg := &config.Config{}
	require.False(t, Enabled(cfg, VoiceName), "no entry means not installed")

	cfg.Plugins = map[string]config.PluginConfig{
		VoiceName: {Enabled: true},
		"other":   {Enabled: false},
	}
	require.True(t, Enabled(cfg, VoiceName))
	require.False(t, Enabled(cfg, "other"))
}

func TestSettingsForNilSafe(t *testing.T) {
	settings := SettingsFor(nil, nil)
	require.True(t, settings.Enabled, "defaults keep dictation usable")
}

func TestRunningReportsNothingWithoutTasks(t *testing.T) {
	require.Nil(t, Running(VoiceName))
}
