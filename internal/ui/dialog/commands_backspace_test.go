package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/textinput"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func newTestCommands(t *testing.T) *Commands {
	t.Helper()
	sty := styles.ColorTonePantera()
	d, err := NewCommands(&common.Common{
		Workspace: &sessionMouseWorkspace{},
		Styles:    &sty,
	}, "", false, false, false, nil, nil)
	require.NoError(t, err)
	return d
}

func TestCommandsBackspaceDoesNotCloseWindowWhenEmpty(t *testing.T) {
	t.Parallel()

	c := newTestCommands(t)
	overlay := NewOverlay(c)
	require.True(t, overlay.HasDialogs())

	overlay.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	require.True(t, overlay.ContainsDialog("commands"),
		"backspace must not close the commands dialog even when the filter is empty")
}

func TestCommandsBackspaceDeletesTextWhenFieldHasText(t *testing.T) {
	t.Parallel()

	c := newTestCommands(t)
	c.input = textinput.New()
	c.input.Focus()
	c.input.SetValue("abc")

	overlay := NewOverlay(c)

	overlay.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})

	require.True(t, overlay.ContainsDialog("commands"), "dialog must stay open")
	require.Equal(t, "ab", c.input.Value(), "backspace must delete one character")
}
