package model

import (
	"errors"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"testing"
)

func TestThemeSelectionThroughCommandPalette(t *testing.T) {
	u, ws := newAppearanceFlowUI(t)
	_ = u.openCommandsDialog()
	_ = u.handleDialogMsg(tea.PasteMsg{Content: "theme"})
	runCmds(u, u.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.True(t, u.dialog.ContainsDialog("themes"))
	_ = u.handleDialogMsg(tea.PasteMsg{Content: "nord"})
	runCmds(u, u.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.Equal(t, "nord", ws.cfg.Options.TUI.Theme)
	require.Contains(t, ws.written, "options.tui.theme")
	require.Equal(t, "nord", u.themeKey)
	require.False(t, u.dialog.ContainsDialog("themes"))
	u.applyThemeForProvider("hyper")
	require.Equal(t, "nord", u.themeKey)
	_ = u.openDialog("themes")
	_ = u.handleDialogMsg(tea.PasteMsg{Content: "automatic"})
	runCmds(u, u.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.Equal(t, "", ws.cfg.Options.TUI.Theme)
	u.applyThemeForProvider("hyper")
	require.Equal(t, "hyper", u.themeKey)
}
func TestDesignSelectionThroughCommandPalette(t *testing.T) {
	u, ws := newAppearanceFlowUI(t)
	_ = u.openCommandsDialog()
	_ = u.handleDialogMsg(tea.PasteMsg{Content: "design"})
	runCmds(u, u.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.True(t, u.dialog.ContainsDialog("designs"))
	_ = u.handleDialogMsg(tea.PasteMsg{Content: "cards"})
	runCmds(u, u.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter}))
	require.Contains(t, ws.written, "options.tui.design")
	require.False(t, u.dialog.ContainsDialog("designs"))
	require.Equal(t, "\u256d", u.com.Styles.Editor.PanelFocused.GetBorderStyle().TopLeft)
}

type appearanceTestWorkspace struct {
	*agentFlowWorkspace
	projectOverride bool
	failWrite       bool
	scopes          []config.Scope
}

func (w *appearanceTestWorkspace) SetConfigField(scope config.Scope, key string, value any) error {
	if w.failWrite {
		return errors.New("write rejected")
	}
	w.written = append(w.written, key)
	w.scopes = append(w.scopes, scope)
	cfg := *w.cfg
	opts := *cfg.Options
	tui := *opts.TUI
	if !(w.projectOverride && scope == config.ScopeGlobal) {
		switch key {
		case "options.tui.theme":
			tui.Theme = value.(string)
		case "options.tui.design":
			tui.Design = value.(string)
		}
	}
	opts.TUI = &tui
	cfg.Options = &opts
	w.cfg = &cfg
	return nil
}
func newAppearanceFlowUI(t *testing.T) (*UI, *appearanceTestWorkspace) {
	u, original := newAgentFlowUI(t)
	ws := &appearanceTestWorkspace{agentFlowWorkspace: original}
	u.com.Workspace = ws
	return u, ws
}
func TestAppearanceSelectionHonorsReloadAndProjectOverride(t *testing.T) {
	u, ws := newAppearanceFlowUI(t)
	ws.cfg.Options.TUI.Theme = "rose"
	ws.projectOverride = true
	require.NoError(t, u.selectAppearance(dialog.ActionSelectAppearance{Kind: "themes", Key: "nord"}))
	require.Equal(t, "nord", u.com.Config().Options.TUI.Theme)
	require.Equal(t, "nord", u.themeKey)
	require.Equal(t, []config.Scope{config.ScopeGlobal, config.ScopeWorkspace}, ws.scopes)
}
func TestAppearanceWriteFailureKeepsActiveStyle(t *testing.T) {
	u, ws := newAppearanceFlowUI(t)
	ws.failWrite = true
	key := u.themeKey
	require.Error(t, u.selectAppearance(dialog.ActionSelectAppearance{Kind: "themes", Key: "nord"}))
	require.Equal(t, key, u.themeKey)
	require.Empty(t, u.com.Config().Options.TUI.Theme)
}
