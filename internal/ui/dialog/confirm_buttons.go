package dialog

import (
	"image"

	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

// confirmButtons gives a two-button confirmation dialog the same pointer
// affordances it already has on the keyboard: left/right picks a button,
// enter or space confirms, and now a click on a button selects and confirms
// it in one press, with the button under the pointer highlighted.
//
// Dialogs render their content as a single centered lipgloss view, so
// [confirmButtons.Painted] is told where the button row actually landed
// rather than recomputing the layout. Clicking outside the buttons changes
// nothing, which keeps the dialog's default choice safe.
type confirmButtons struct {
	sty *styles.Styles

	// compositor maps a click back to the button it hit.
	compositor *lipgloss.Compositor
	// hovered is the button under the pointer, or -1.
	hovered int
}

// newConfirmButtons returns button rows for the given theme.
func newConfirmButtons(sty *styles.Styles) confirmButtons {
	return confirmButtons{sty: sty, hovered: -1}
}

// Painted records where the button row landed on screen. body is the dialog's
// content block origin, content the rendered block the buttons are centered
// inside, buttons the rendered button row, and line the button row's index
// within content.
func (c *confirmButtons) Painted(
	body image.Point,
	content, buttons string,
	line int,
	opts []common.ButtonOpts,
	spacing string,
) {
	c.compositor = common.ButtonHitCompositor(
		c.sty, c.decorated(opts), spacing,
		body.X+centeredLineX(lipgloss.Width(content), lipgloss.Width(buttons)),
		body.Y+line,
	)
}

// decorated returns opts with the hovered button flagged, for rendering.
func (c *confirmButtons) decorated(opts []common.ButtonOpts) []common.ButtonOpts {
	out := make([]common.ButtonOpts, len(opts))
	copy(out, opts)
	for i := range out {
		out[i].Hovered = i == c.hovered
	}
	return out
}

// View renders the button row with hover applied.
func (c *confirmButtons) View(opts []common.ButtonOpts, spacing string) string {
	return common.ButtonGroup(c.sty, c.decorated(opts), spacing)
}

// HandleMsg tracks hover for the button row and, for a left click, reports
// the index of the clicked button through clicked (-1 when the click missed).
// It returns true when the event landed on a button.
func (c *confirmButtons) HandleMsg(msg tea.Msg, clicked *int) bool {
	switch msg := msg.(type) {
	case tea.MouseMotionMsg:
		c.hovered = c.Hit(msg.X, msg.Y)
	case tea.MouseReleaseMsg:
		c.hovered = -1
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return false
		}
		*clicked = c.Hit(msg.X, msg.Y)
		return *clicked >= 0
	}
	return false
}

// Hit returns the index of the button at x,y, or -1.
func (c *confirmButtons) Hit(x, y int) int {
	return common.HitButtonIndex(c.compositor, x, y)
}

// dialogFrameOriginOf returns the content origin of a view that will be drawn
// centered in area, so a confirmation dialog can go from the screen rectangle
// to its content origin in one step.
func dialogFrameOriginOf(area uv.Rectangle, view string, frame lipgloss.Style) image.Point {
	return dialogFrameOrigin(dialogRectCentered(area, view), frame)
}