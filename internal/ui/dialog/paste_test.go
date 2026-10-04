package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// A paste is not a key press, so it never reaches a text field through the
// KeyPressMsg branch. Adding a model to a provider had no tea.PasteMsg case at
// all, so the switch fell through and the pasted model ID was dropped: the
// field could only be filled one character at a time.
func TestProviderSettingsAcceptsPasteIntoModelID(t *testing.T) {
	t.Parallel()

	d, _ := newProviderTestDialog(t, []catwalk.Model{{ID: "existing"}})
	require.True(t, d.openProvider("osnova"))
	d.startAddingModel()
	require.Equal(t, providerSettingsStateModelID, d.state)

	d.HandleMsg(tea.PasteMsg{Content: "gpt-5-codex"})
	require.Equal(t, "gpt-5-codex", d.input.Value(),
		"the pasted model ID has to land in the field")
}

func TestProviderSettingsAcceptsPasteIntoModelContext(t *testing.T) {
	t.Parallel()

	d, _ := newProviderTestDialog(t, []catwalk.Model{{ID: "existing"}})
	require.True(t, d.openProvider("osnova"))
	d.startAddingModel()
	d.input.SetValue("gpt-5-codex")
	d.commitModelID()
	require.Equal(t, providerSettingsStateModelContext, d.state)

	d.HandleMsg(tea.PasteMsg{Content: "400000"})
	require.Equal(t, "400000", d.input.Value())
}

func TestProviderSettingsAcceptsPasteIntoFilter(t *testing.T) {
	t.Parallel()

	d, _ := newProviderTestDialog(t, []catwalk.Model{{ID: "existing"}})
	require.Equal(t, providerSettingsStateProviders, d.state)

	d.HandleMsg(tea.PasteMsg{Content: "osnova"})
	require.Equal(t, "osnova", d.input.Value())
	require.Len(t, d.list.FilteredItems(), 1, "the filter has to be applied")
}

// A paste aimed at a state with no text field must not be forwarded anywhere.
func TestProviderSettingsIgnoresPasteWithoutInput(t *testing.T) {
	t.Parallel()

	d, _ := newProviderTestDialog(t, []catwalk.Model{{ID: "existing"}})
	require.True(t, d.openProvider("osnova"))
	require.Equal(t, providerSettingsStateModels, d.state)

	require.Nil(t, d.HandleMsg(tea.PasteMsg{Content: "anything"}))
}

// The other filter dialogs share the same gap. A session rename is real text
// entry, so a lost paste there is the same bug with worse consequences.
func TestSessionsAcceptsPasteIntoRename(t *testing.T) {
	t.Parallel()

	d := newSessionMouseDialog(t, []session.Session{{ID: "s00", Title: "Session 00"}}, "s00")
	// Enter rename mode the way ctrl+r does, so the item is rebuilt with a
	// focused title input.
	d.HandleMsg(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	require.Equal(t, sessionsModeUpdating, d.sessionsMode)

	item, ok := d.list.SelectedItem().(*SessionItem)
	require.True(t, ok)
	require.NotNil(t, item)

	d.HandleMsg(tea.PasteMsg{Content: "renamed session"})
	require.Equal(t, "renamed session", item.InputValue())
}

func TestSessionsAcceptsPasteIntoFilter(t *testing.T) {
	t.Parallel()

	d := newSessionMouseDialog(t, []session.Session{{ID: "s00", Title: "Session 00"}}, "s00")
	d.HandleMsg(tea.PasteMsg{Content: "alpha"})
	require.Equal(t, "alpha", d.input.Value())
}

func TestCommandsAcceptsPasteIntoFilter(t *testing.T) {
	t.Parallel()

	sty := styles.ColorTonePantera()
	d, err := NewCommands(&common.Common{
		Workspace: &sessionMouseWorkspace{},
		Styles:    &sty,
	}, "", false, false, false, nil, nil)
	require.NoError(t, err)

	d.HandleMsg(tea.PasteMsg{Content: "model"})
	require.Equal(t, "model", d.input.Value())
}

// ctrl+v has to reach the clipboard path in the overlay rather than being
// swallowed by the text input, whose own paste message is unexported and
// therefore unroutable from a dialog.
func TestOverlayRoutesCtrlVToClipboard(t *testing.T) {
	t.Parallel()

	input := newAutoSummarizeTestDialog(t, &config.Options{})
	overlay := NewOverlay(input)

	action := overlay.Update(tea.KeyPressMsg{Code: 'v', Mod: tea.ModCtrl})
	require.NotNil(t, action, "ctrl+v must produce a command that reads the clipboard")

	actionCmd, ok := action.(ActionCmd)
	require.True(t, ok, "expected ActionCmd, got %T", action)
	require.NotNil(t, actionCmd.Cmd, "the clipboard read has to be runnable")

	// Nothing was pasted yet, so the field still holds its seeded value: the
	// overlay must not have consumed the keypress as text.
	require.Equal(t, "85", input.rows[0].input.Value())
}
