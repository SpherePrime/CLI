package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/textinput"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// backspaceMsg is a backspace key press.
func backspaceMsg() tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: tea.KeyBackspace}
}

func TestOverlayBackspaceGoesBack(t *testing.T) {
	t.Parallel()

	overlay := NewOverlay(newBackspaceTestDialog("base"), newBackspaceTestDialog("front"))
	require.True(t, overlay.HasDialogs())

	// A dialog with no text field treats backspace as navigation.
	require.Nil(t, overlay.Update(backspaceMsg()))
	require.False(t, overlay.ContainsDialog("front"), "backspace must pop the front dialog")
	require.True(t, overlay.ContainsDialog("base"), "the dialog underneath must stay open")
}

func TestOverlayBackspaceGoesBackWhenFieldEmpty(t *testing.T) {
	t.Parallel()

	front := newBackspaceTestDialog("front")
	front.input = textinput.New()

	overlay := NewOverlay(newBackspaceTestDialog("base"), front)

	// The field is empty, so there is nothing to delete: backspace navigates.
	require.Nil(t, overlay.Update(backspaceMsg()))
	require.False(t, overlay.ContainsDialog("front"))
	require.True(t, overlay.ContainsDialog("base"))
}

func TestOverlayBackspaceReachesDialogWhenFieldHasText(t *testing.T) {
	t.Parallel()

	front := newBackspaceTestDialog("front")
	front.input = textinput.New()
	front.input.SetValue("typed")

	overlay := NewOverlay(newBackspaceTestDialog("base"), front)

	// The field has content, so backspace belongs to the text field and must
	// not pop the dialog.
	overlay.Update(backspaceMsg())
	require.True(t, overlay.ContainsDialog("front"), "backspace must not close a dialog whose field has text")
	require.Equal(t, 1, front.handled, "the backspace must reach the dialog's own handler")
}

func TestOverlayNonBackspaceKeyDoesNotNavigate(t *testing.T) {
	t.Parallel()

	overlay := NewOverlay(newBackspaceTestDialog("base"), newBackspaceTestDialog("front"))

	overlay.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	require.True(t, overlay.ContainsDialog("front"), "only backspace navigates back")
}