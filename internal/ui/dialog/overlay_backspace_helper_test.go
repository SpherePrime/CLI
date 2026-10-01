package dialog

import (
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/textinput"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
)

// backspaceTestDialog is a minimal dialog used to test the overlay's
// backspace routing. Its field is empty unless a test fills it in, so tests
// can exercise both meanings of the key.
type backspaceTestDialog struct {
	id      string
	input   textinput.Model
	handled int
}

var (
	_ Dialog         = (*backspaceTestDialog)(nil)
	_ BackspaceAware = (*backspaceTestDialog)(nil)
)

func newBackspaceTestDialog(id string) *backspaceTestDialog {
	return &backspaceTestDialog{id: id}
}

func (d *backspaceTestDialog) ID() string { return d.id }

func (d *backspaceTestDialog) HandleMsg(tea.Msg) Action {
	d.handled++
	return nil
}

func (*backspaceTestDialog) Draw(uv.Screen, uv.Rectangle) *tea.Cursor { return nil }

// BackspaceDeletesText reports whether the test field consumes backspace.
func (d *backspaceTestDialog) BackspaceDeletesText() bool {
	return BackspaceDeletesText(d.input)
}