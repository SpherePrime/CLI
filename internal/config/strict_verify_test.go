package config

import (
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestStrictVerifySectionRouting(t *testing.T) {
	t.Parallel()

	require.Equal(t, optionsSectionFile, sectionFileFor("options.strict_verify"))
	require.Equal(t, optionsSectionFile, sectionFileFor("session_strict_verify"))
	require.Equal(t, optionsSectionFile, sectionFileFor("session_strict_verify.chat-1"))
}

func TestStrictVerifyDefaultsToFalseAndRespectsScoping(t *testing.T) {
	t.Parallel()

	var nilCfg *Config
	require.False(t, nilCfg.IsStrictVerifyEnabled(""))
	require.False(t, nilCfg.IsStrictVerifyEnabled("chat-1"))

	var nilStore *ConfigStore
	require.False(t, nilStore.IsStrictVerifyEnabled("chat-1"))

	cfg := &Config{}
	require.False(t, cfg.IsStrictVerifyEnabled(""))
	require.False(t, cfg.IsStrictVerifyEnabled("chat-1"))

	cfg.Options = &Options{StrictVerify: true}
	require.True(t, cfg.IsStrictVerifyEnabled(""))
	require.True(t, cfg.IsStrictVerifyEnabled("chat-other"))

	cfg.SessionStrictVerify = map[string]bool{
		"chat-off": false,
	}
	require.False(t, cfg.IsStrictVerifyEnabled("chat-off"))
	require.True(t, cfg.IsStrictVerifyEnabled("chat-other"))

	cfg.Options.StrictVerify = false
	cfg.SessionStrictVerify["chat-on"] = true
	require.False(t, cfg.IsStrictVerifyEnabled(""))
	require.False(t, cfg.IsStrictVerifyEnabled("chat-other"))
	require.True(t, cfg.IsStrictVerifyEnabled("chat-on"))
}

func TestStrictVerifyCloneForWriteIsolation(t *testing.T) {
	t.Parallel()

	orig := &Config{
		Options: &Options{StrictVerify: true},
		SessionStrictVerify: map[string]bool{
			"s1": true,
		},
	}

	clone := orig.cloneForWrite()
	clone.Options.StrictVerify = false
	clone.SessionStrictVerify["s1"] = false
	clone.SessionStrictVerify["s2"] = true

	require.True(t, orig.Options.StrictVerify)
	require.True(t, orig.SessionStrictVerify["s1"])
	require.NotContains(t, orig.SessionStrictVerify, "s2")
}

func TestStrictVerifyRuntimeOverridesPrecedence(t *testing.T) {
	t.Parallel()

	store := &ConfigStore{
		config: &Config{
			Options: &Options{StrictVerify: false},
			SessionStrictVerify: map[string]bool{
				"chat-off": false,
			},
		},
	}
	require.False(t, store.IsStrictVerifyEnabled(""))
	require.False(t, store.IsStrictVerifyEnabled("chat-off"))

	store.OverrideStrictVerify(true)
	require.True(t, store.IsStrictVerifyEnabled(""))
	require.True(t, store.IsStrictVerifyEnabled("chat-off"))
	require.True(t, store.Config().IsStrictVerifyEnabled("chat-off"))
	require.True(t, store.Config().Options.StrictVerify)

	val := false
	store.Overrides().StrictVerify = &val
	store.config.Options.StrictVerify = true
	require.False(t, store.IsStrictVerifyEnabled(""))
	require.False(t, store.IsStrictVerifyEnabled("chat-off"))
}

func TestUpdateStrictVerifyConfigPersistsAndReloads(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, "global")
	workDir := filepath.Join(tmpDir, "work")
	t.Setenv("PRIME_GLOBAL_CONFIG", globalDir)
	t.Setenv("PRIME_GLOBAL_DATA", globalDir)

	store, err := Init(workDir, "", false)
	require.NoError(t, err)
	require.False(t, store.IsStrictVerifyEnabled(""))

	require.NoError(t, store.UpdateStrictVerifyConfig(ScopeGlobal, "", true))
	require.True(t, store.IsStrictVerifyEnabled(""))
	require.True(t, store.Config().IsStrictVerifyEnabled("chat-2"))

	require.NoError(t, store.UpdateStrictVerifyConfig(ScopeWorkspace, "chat-1", false))
	require.False(t, store.IsStrictVerifyEnabled("chat-1"))
	require.True(t, store.IsStrictVerifyEnabled("chat-2"))

	reloaded, err := Init(workDir, "", false)
	require.NoError(t, err)
	require.True(t, reloaded.IsStrictVerifyEnabled(""))
	require.False(t, reloaded.IsStrictVerifyEnabled("chat-1"))
	require.True(t, reloaded.IsStrictVerifyEnabled("chat-2"))

	reloaded.OverrideStrictVerify(false)
	require.False(t, reloaded.IsStrictVerifyEnabled(""))
	require.False(t, reloaded.IsStrictVerifyEnabled("chat-2"))
}
