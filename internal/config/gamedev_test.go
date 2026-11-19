package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestGameDevSectionRouting(t *testing.T) {
	t.Parallel()

	require.Equal(t, optionsSectionFile, sectionFileFor("options.game_dev"))
	require.Equal(t, optionsSectionFile, sectionFileFor("session_game_dev"))
	require.Equal(t, optionsSectionFile, sectionFileFor("session_game_dev.chat-1"))
}

func TestGameDevDefaultsToFalseAndRespectsScoping(t *testing.T) {
	t.Parallel()

	var nilCfg *Config
	require.False(t, nilCfg.IsGameDevEnabled(""))
	require.False(t, nilCfg.IsGameDevEnabled("chat-1"))

	var nilStore *ConfigStore
	require.False(t, nilStore.IsGameDevEnabled("chat-1"))

	cfg := &Config{}
	require.False(t, cfg.IsGameDevEnabled(""))
	require.False(t, cfg.IsGameDevEnabled("chat-1"))

	cfg.Options = &Options{GameDev: true}
	require.True(t, cfg.IsGameDevEnabled(""))
	require.True(t, cfg.IsGameDevEnabled("chat-other"))

	cfg.SessionGameDev = map[string]bool{
		"chat-off": false,
	}
	require.False(t, cfg.IsGameDevEnabled("chat-off"))
	require.True(t, cfg.IsGameDevEnabled("chat-other"))

	cfg.Options.GameDev = false
	cfg.SessionGameDev["chat-on"] = true
	require.False(t, cfg.IsGameDevEnabled(""))
	require.False(t, cfg.IsGameDevEnabled("chat-other"))
	require.True(t, cfg.IsGameDevEnabled("chat-on"))
}

func TestGameDevCloneForWriteIsolation(t *testing.T) {
	t.Parallel()

	orig := &Config{
		Options: &Options{GameDev: true},
		SessionGameDev: map[string]bool{
			"s1": true,
		},
	}

	clone := orig.cloneForWrite()
	clone.Options.GameDev = false
	clone.SessionGameDev["s1"] = false
	clone.SessionGameDev["s2"] = true

	require.True(t, orig.Options.GameDev)
	require.True(t, orig.SessionGameDev["s1"])
	require.NotContains(t, orig.SessionGameDev, "s2")
}

func TestGameDevRuntimeOverridesPrecedence(t *testing.T) {
	t.Parallel()

	store := &ConfigStore{
		config: &Config{
			Options: &Options{GameDev: false},
			SessionGameDev: map[string]bool{
				"chat-off": false,
			},
		},
	}
	require.False(t, store.IsGameDevEnabled(""))
	require.False(t, store.IsGameDevEnabled("chat-off"))

	store.OverrideGameDev(true)
	require.True(t, store.IsGameDevEnabled(""))
	require.True(t, store.IsGameDevEnabled("chat-off"))
	require.True(t, store.Config().IsGameDevEnabled("chat-off"))
	require.True(t, store.Config().Options.GameDev)

	val := false
	store.Overrides().GameDev = &val
	store.config.Options.GameDev = true
	require.False(t, store.IsGameDevEnabled(""))
	require.False(t, store.IsGameDevEnabled("chat-off"))
}

func TestUpdateGameDevConfigPersistsAndReloads(t *testing.T) {
	globalDir := t.TempDir()
	workDir := t.TempDir()
	t.Setenv("PRIME_GLOBAL_CONFIG", globalDir)
	t.Setenv("PRIME_GLOBAL_DATA", globalDir)
	resetProviderState()
	t.Cleanup(resetProviderState)

	require.NoError(t, os.WriteFile(
		filepath.Join(globalDir, "prime.json"),
		[]byte(twoProviderConfig("openai", "gpt-4")),
		0o600,
	))

	store, err := Load(workDir, "", false)
	require.NoError(t, err)
	require.False(t, store.IsGameDevEnabled(""))

	require.NoError(t, store.UpdateGameDevConfig(ScopeGlobal, "", true))
	require.True(t, store.IsGameDevEnabled(""))
	require.True(t, store.Config().IsGameDevEnabled("chat-2"))

	require.NoError(t, store.UpdateGameDevConfig(ScopeWorkspace, "chat-1", false))
	require.False(t, store.IsGameDevEnabled("chat-1"))
	require.True(t, store.IsGameDevEnabled("chat-2"))

	globalOpts := readConfigJSON(t, filepath.Join(globalDir, "prime.json"), "options")
	optsMap, ok := globalOpts["options"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, optsMap["game_dev"])

	wsData := readConfigJSON(t, filepath.Join(store.Config().Options.DataDirectory, "prime.json"), "session_game_dev")
	sessMap, ok := wsData["session_game_dev"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, false, sessMap["chat-1"])

	reloaded, err := Load(workDir, "", false)
	require.NoError(t, err)
	require.True(t, reloaded.IsGameDevEnabled(""))
	require.False(t, reloaded.IsGameDevEnabled("chat-1"))
	require.True(t, reloaded.IsGameDevEnabled("chat-2"))

	reloaded.OverrideGameDev(false)
	require.NoError(t, reloaded.ReloadFromDisk(t.Context()))
	require.False(t, reloaded.IsGameDevEnabled(""))
	require.False(t, reloaded.IsGameDevEnabled("chat-2"))
}
