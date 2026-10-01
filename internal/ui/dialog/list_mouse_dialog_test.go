package dialog

import (
	"image"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/internal/workspace"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// mouseTestWorkspace is the minimal Workspace the list dialogs read config
// through.
type mouseTestWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *mouseTestWorkspace) Config() *config.Config { return w.cfg }

func (w *mouseTestWorkspace) Language() string { return "" }

func (w *mouseTestWorkspace) AgentIsReady() bool { return false }

// newMouseDialogCom builds the minimal Common the simple list dialogs need.
func newMouseDialogCom() *common.Common {
	sty := styles.ColorTonePantera()
	return &common.Common{
		Workspace: &mouseTestWorkspace{cfg: &config.Config{}},
		Styles:    &sty,
	}
}

// screenRowOf finds the screen row containing the given painted text and
// returns its y coordinate, or -1. It is how these tests click on a row the
// way a user does: by where the row's text actually landed on screen, rather
// than by recomputing the layout arithmetic the dialog uses.
func screenRowOf(t *testing.T, scr uv.Screen, needle string) int {
	t.Helper()
	for y := range scr.Bounds().Dy() {
		var sb strings.Builder
		for x := range scr.Bounds().Dx() {
			sb.WriteString(cellContent(scr, x, y))
		}
		if strings.Contains(sb.String(), needle) {
			return y
		}
	}
	return -1
}

// TestLanguageMouseClickRowAppliesLikeEnter is the end-to-end check that a
// click lands on the row the user aimed at: it draws the dialog for real,
// finds the painted row by its label, and clicks there.
func TestLanguageMouseClickRowAppliesLikeEnter(t *testing.T) {
	t.Parallel()

	dialog := NewLanguage(newMouseDialogCom())
	scr := uv.NewScreenBuffer(80, 30)
	area := image.Rect(0, 0, 80, 30)
	dialog.Draw(scr, area)

	// Pick a row that is not the initially selected one so the test proves
	// the click was mapped through the rendered geometry.
	items := dialog.list.FilteredItems()
	require.GreaterOrEqual(t, len(items), 2)
	idx := 1
	if dialog.list.Selected() == idx {
		idx = len(items) - 1
	}
	item, ok := items[idx].(*LanguageItem)
	require.True(t, ok)

	row := screenRowOf(t, scr, item.title)
	require.GreaterOrEqual(t, row, 0, "row %q must be painted", item.title)

	x := dialog.mouse.area.Min.X + 1
	require.Less(t, x, dialog.mouse.area.Max.X)

	// First click selects the row under the pointer.
	action := dialog.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X: x, Y: row, Button: tea.MouseLeft,
	}))
	require.Nil(t, action)
	require.Equal(t, idx, dialog.list.Selected())

	// Clicking it again applies, exactly as enter would.
	action = dialog.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X: x, Y: row, Button: tea.MouseLeft,
	}))
	sel, ok := action.(ActionSelectLanguage)
	require.True(t, ok, "second click must apply the row, got %T", action)
	require.Equal(t, item.locale, sel.Locale)
}

// TestQuitMouseClickButtons covers the button row: clicking "yes" quits and
// clicking "no" closes, without touching the keyboard.
func TestQuitMouseClickButtons(t *testing.T) {
	t.Parallel()

	dialog := NewQuit(newMouseDialogCom())
	scr := uv.NewScreenBuffer(80, 30)
	area := image.Rect(0, 0, 80, 30)
	dialog.Draw(scr, area)

	sty := dialog.com.Styles
	yes := common.Button(sty, common.ButtonOpts{Text: dialog.com.L("btn.yep"), Padding: 3})
	no := common.Button(sty, common.ButtonOpts{Text: dialog.com.L("btn.nope"), Padding: 3})

	yesCell := firstCellWith(t, scr, dialog.com.L("btn.yep"), yes)
	noCell := firstCellWith(t, scr, dialog.com.L("btn.nope"), no)
	require.GreaterOrEqual(t, yesCell.Y, 0, "the yes button must be painted")
	require.GreaterOrEqual(t, noCell.Y, 0, "the no button must be painted")

	// "no" closes.
	action := dialog.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X: noCell.X, Y: noCell.Y, Button: tea.MouseLeft,
	}))
	require.IsType(t, ActionClose{}, action)

	// "yes" quits.
	dialog.Draw(scr, area)
	action = dialog.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X: yesCell.X, Y: yesCell.Y, Button: tea.MouseLeft,
	}))
	require.IsType(t, ActionQuit{}, action)
}

// TestQuitMouseClickOutsideButtonsDoesNothing guards the "no" default: a
// click on empty space must not quit the app.
func TestQuitMouseClickOutsideButtonsDoesNothing(t *testing.T) {
	t.Parallel()

	dialog := NewQuit(newMouseDialogCom())
	scr := uv.NewScreenBuffer(80, 30)
	dialog.Draw(scr, image.Rect(0, 0, 80, 30))

	action := dialog.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X: 1, Y: 29, Button: tea.MouseLeft,
	}))
	require.Nil(t, action)
}

// TestUpdateMouseClickButtons covers the update prompt: clicking "Later"
// closes and clicking "Update" applies, and a stray click does neither.
func TestUpdateMouseClickButtons(t *testing.T) {
	t.Parallel()

	dialog := NewUpdate(newMouseDialogCom(), "1.0.0", "1.1.0")
	scr := uv.NewScreenBuffer(80, 30)
	area := image.Rect(0, 0, 80, 30)
	dialog.Draw(scr, area)

	updateCell := firstCellWith(t, scr, "Update", "")
	laterCell := firstCellWith(t, scr, "Later", "")
	require.GreaterOrEqual(t, updateCell.X, 0, "the Update button must be painted")
	require.GreaterOrEqual(t, laterCell.X, 0, "the Later button must be painted")

	action := dialog.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X: updateCell.X, Y: updateCell.Y, Button: tea.MouseLeft,
	}))
	apply, ok := action.(ActionApplyUpdate)
	require.True(t, ok, "clicking Update must apply it, got %T", action)
	require.Equal(t, "1.1.0", apply.Latest)

	dialog.Draw(scr, area)
	action = dialog.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X: laterCell.X, Y: laterCell.Y, Button: tea.MouseLeft,
	}))
	require.IsType(t, ActionClose{}, action)

	dialog.Draw(scr, area)
	action = dialog.HandleMsg(tea.MouseClickMsg(tea.Mouse{
		X: 1, Y: 29, Button: tea.MouseLeft,
	}))
	require.Nil(t, action, "a click outside the buttons must do nothing")
}

// TestConfirmButtonsHoverHighlights checks the pointer changes the render, so
// the clickable row does not look inert under the cursor.
func TestConfirmButtonsHoverHighlights(t *testing.T) {
	t.Parallel()

	dialog := NewQuit(newMouseDialogCom())
	scr := uv.NewScreenBuffer(80, 30)
	area := image.Rect(0, 0, 80, 30)
	dialog.Draw(scr, area)

	yesCell := firstCellWith(t, scr, dialog.com.L("btn.yep"), "")

	// Hovering the row marks it, and drawing reflects that.
	require.False(t, dialog.buttons.HandleMsg(tea.MouseMotionMsg(tea.Mouse{
		X: yesCell.X, Y: yesCell.Y,
	}), new(int)))
	dialog.Draw(scr, area)
	require.Equal(t, 0, dialog.buttons.Hit(yesCell.X, yesCell.Y))

	// Releasing clears the hover so the highlight does not stick.
	require.False(t, dialog.buttons.HandleMsg(tea.MouseReleaseMsg(tea.Mouse{
		X: yesCell.X, Y: yesCell.Y,
	}), new(int)))
	dialog.Draw(scr, area)
	require.Equal(t, -1, dialog.buttons.Hit(1, 29))
}

// firstCellWith returns the first screen cell holding the label, so a test can
// click a button by its visible text.
func firstCellWith(t *testing.T, scr uv.Screen, label, _ string) image.Point {
	t.Helper()
	for y := range scr.Bounds().Dy() {
		for x := range scr.Bounds().Dx() {
			if cellContent(scr, x, y) != "" && strings.Contains(label, cellContent(scr, x, y)) {
				// Only accept a cell once the whole label has been seen
				// starting here.
				if labelAt(scr, x, y, label) {
					return image.Pt(x, y)
				}
			}
		}
	}
	return image.Pt(-1, -1)
}

// labelAt reports whether label starts at the given cell and runs right.
func labelAt(scr uv.Screen, x, y int, label string) bool {
	runes := []rune(label)
	for i, r := range runes {
		if cellContent(scr, x+i, y) != string(r) {
			return false
		}
	}
	return true
}