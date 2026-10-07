package config_test

import (
	"github.com/SpherePrime/CLI/internal/appearance"
	"github.com/SpherePrime/CLI/internal/config"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestThemeShellOption(t *testing.T) {
	store := loadPrimeSh(t, `option ui theme nord`)
	require.Equal(t, "nord", store.Config().Options.TUI.Theme)
	_, err := loadPrimeShErr(t, `option ui theme unknown`)
	require.Error(t, err)
}

func TestDesignShellOption(t *testing.T) {
	store := loadPrimeSh(t, `option ui design cards`)
	require.Equal(t, "cards", store.Config().Options.TUI.Design)
	_, err := loadPrimeShErr(t, `option ui design unknown`)
	require.Error(t, err)
}
func TestAppearancePersistsAcrossReload(t *testing.T) {
	store := loadPrimeSh(t, "")
	require.NoError(t, store.SetConfigField(config.ScopeGlobal, "options.tui.theme", "nord"))
	require.NoError(t, store.SetConfigField(config.ScopeGlobal, "options.tui.design", "dashboard"))
	reloaded, err := config.Load(t.TempDir(), t.TempDir(), false)
	require.NoError(t, err)
	require.Equal(t, "nord", reloaded.Config().Options.TUI.Theme)
	require.Equal(t, "dashboard", reloaded.Config().Options.TUI.Design)
}

func TestAppearanceWorkspaceOverridePersists(t *testing.T) {
	store := loadPrimeSh(t, `option ui theme rose`)
	require.NoError(t, store.SetConfigField(config.ScopeGlobal, "options.tui.theme", "nord"))
	require.Equal(t, "rose", store.Config().Options.TUI.Theme)
	require.NoError(t, store.SetConfigField(config.ScopeWorkspace, "options.tui.theme", "nord"))
	require.Equal(t, "nord", store.Config().Options.TUI.Theme)
}

func TestAdditionalAppearanceOptionsPersist(t *testing.T) {
	for _, name := range []string{"opencode", "paper", "blueprint", "ember"} {
		t.Run(name, func(t *testing.T) {
			preset := appearance.DesignSpec(name)
			store := loadPrimeSh(t, "option ui design "+name+"\noption ui theme "+preset.Theme)
			require.Equal(t, name, store.Config().Options.TUI.Design)
			require.Equal(t, preset.Theme, store.Config().Options.TUI.Theme)
			require.NoError(t, store.SetConfigField(config.ScopeWorkspace, "options.tui.design", name))
			require.Equal(t, name, store.Config().Options.TUI.Design)
		})
	}
}
