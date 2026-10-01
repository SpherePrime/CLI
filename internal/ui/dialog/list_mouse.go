package dialog

import (
	"image"
	"time"

	"github.com/SpherePrime/CLI/internal/ui/common"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
)

// listMouseItem is the subset of list behaviour [ListMouse] drives. Both
// *list.List and *list.FilterableList satisfy it, so a dialog can hand over
// whichever flavour it holds.
type listMouseItem interface {
	ItemIndexAtPosition(x, y int) (idx, itemY int)
	SetSelected(idx int)
	Selected() int
	ScrollToSelected()
	ScrollBy(lines int)
	Offset() int
	Width() int
}

// ListMouse gives a dialog's list the same pointer affordances the keyboard
// already has, so a row can be picked and run without touching the arrow
// keys:
//
//   - click a row to select it (what up/down do),
//   - click the already-selected row, or double-click any row, to activate it
//     (what enter does),
//   - drag the scrollbar thumb,
//   - wheel to scroll.
//
// Dialogs record the on-screen rectangle of their list from Draw with
// [ListMouse.Painted] and forward their mouse messages to
// [ListMouse.HandleMsg]. Clicking the filter input focuses it and places the
// text cursor, matching a click anywhere else in a terminal program.
type ListMouse struct {
	zone ScrollbarZone

	// area is the on-screen rectangle the list body occupies, as recorded
	// by the last Painted call. A zero rectangle means no geometry is known
	// yet (before the first Draw) and every hit test misses.
	area image.Rectangle

	// scrolled reports whether the pointer has scrolled the list. Dialogs
	// consult it from Draw to stop snapping the viewport back to the
	// selected row after the user scrolled away from it.
	scrolled bool

	// lastClickIdx / lastClickTime implement double-click detection.
	lastClickIdx  int
	lastClickTime time.Time

	// singleClick makes a first click activate a row instead of only
	// selecting it. Use it for lists where a row is a command with no
	// separate confirm step, so one click does exactly what enter does.
	singleClick bool
}

// ActivateOnClick makes a first click on a row activate it right away
// rather than only moving the selection. Use it for lists where a row is a
// command with no separate confirm step, so one click does what enter does.
func (m *ListMouse) ActivateOnClick() {
	m.singleClick = true
}

// clickActivateWindow is how long two clicks may be apart and still count as
// a double click.
const clickActivateWindow = 400 * time.Millisecond

// Painted records the on-screen rectangle the list occupies. Call it from
// Draw with the same rectangle passed to the scrollbar's Painted, so row
// clicks and thumb drags agree on where the list is.
func (m *ListMouse) Painted(area image.Rectangle) {
	m.area = area
}

// PaintedScrollbar records a scrollbar drawn into its own screen column. It
// keeps row clicks and thumb drags on the same geometry.
func (m *ListMouse) PaintedScrollbar(column image.Point, height, contentSize, viewportSize, offset int) {
	m.zone.PaintedColumn(column, height, contentSize, viewportSize, offset)
}

// PaintedJoin records the scrollbar [joinScrollbar] appended to content.
func (m *ListMouse) PaintedJoin(origin image.Point, content string, height, contentSize, viewportSize, offset int) {
	m.zone.Painted(origin, content, height, contentSize, viewportSize, offset)
}

// Clear forgets the list geometry and ends any drag, so a frame that paints
// no list cannot leave a stale hit area behind.
func (m *ListMouse) Clear() {
	m.area = image.Rectangle{}
	m.zone.Clear()
}

// Scrolled reports whether the pointer has scrolled this list.
func (m *ListMouse) Scrolled() bool { return m.scrolled }

// ResetScroll clears the scrolled flag, so the next Draw snaps the viewport
// back to the selected row.
func (m *ListMouse) ResetScroll() { m.scrolled = false }

// Dragging reports whether the pointer holds the scrollbar thumb.
func (m *ListMouse) Dragging() bool { return m.zone.Dragging() }

// scrollListTo scrolls the list so its scrollbar thumb lines up with the
// pointer.
func (m *ListMouse) scrollListTo(l listMouseItem, offset int) {
	m.scrolled = true
	l.ScrollBy(offset - l.Offset())
}

// HandleWheel scrolls the list when the wheel event landed over it and
// reports whether it consumed the event.
func (m *ListMouse) HandleWheel(msg common.CoalescedWheelMsg, l listMouseItem) bool {
	lines := int(msg.DeltaY)
	if lines == 0 {
		return false
	}
	if m.area.Empty() || !image.Pt(msg.Mouse.X, msg.Mouse.Y).In(m.area) {
		return false
	}
	m.scrolled = true
	l.ScrollBy(lines)
	return true
}

// HandleMsg routes a pointer message. activate is called with the index of
// the row the user activated and returns the dialog's resulting [Action];
// pass nil for dialogs that have nothing to do on activation.
//
// It returns the action to emit, or nil when nothing happened. A returned
// nil action still means the event was consumed — a click that selected a row
// must not fall through to whatever sits behind the dialog.
func (m *ListMouse) HandleMsg(msg tea.Msg, l listMouseItem, activate func(idx int) Action) Action {
	switch msg := msg.(type) {
	case common.CoalescedWheelMsg:
		m.HandleWheel(msg, l)
		return nil

	case tea.MouseClickMsg:
		if m.zone.HandleMsg(msg, func(offset int) { m.scrollListTo(l, offset) }) {
			return nil
		}
		return m.click(msg, l, activate)

	case tea.MouseMotionMsg:
		m.zone.HandleMsg(msg, func(offset int) { m.scrollListTo(l, offset) })
		return nil

	case tea.MouseReleaseMsg:
		m.zone.HandleMsg(msg, func(offset int) { m.scrollListTo(l, offset) })
		return nil
	}
	return nil
}

// click selects the row under the pointer and activates it on a second click
// or a double click.
func (m *ListMouse) click(msg tea.MouseClickMsg, l listMouseItem, activate func(idx int) Action) Action {
	// Anything other than a plain left press is not a row activation: a
	// right click opens context menus the dialog does not own, and a
	// middle click pastes. Let those through untouched.
	if msg.Button != tea.MouseLeft {
		m.resetClick()
		return nil
	}
	if m.area.Empty() {
		return nil
	}

	// Clip the hit area to the list width so a click past the last column
	// (on the scrollbar gutter or the dialog padding) does not select the
	// row that happens to be under the pointer.
	area := m.area
	if w := l.Width(); w > 0 && area.Max.X > area.Min.X+w {
		area.Max.X = area.Min.X + w
	}

	point := image.Pt(msg.X, msg.Y)
	if !point.In(area) {
		m.resetClick()
		return nil
	}

	idx, _ := l.ItemIndexAtPosition(point.X-area.Min.X, point.Y-area.Min.Y)
	if idx < 0 {
		m.resetClick()
		return nil
	}

	now := time.Now()
	wasSelected := l.Selected() == idx
	doubleClick := idx == m.lastClickIdx && now.Sub(m.lastClickTime) <= clickActivateWindow

	// First press of a double click already selected the row, so only
	// remember the click and let the second one activate.
	if doubleClick {
		m.resetClick()
		return m.activate(activate, idx)
	}

	m.lastClickIdx = idx
	m.lastClickTime = now
	m.scrolled = true
	l.SetSelected(idx)
	l.ScrollToSelected()

	// Clicking the row that is already selected means "run this", which is
	// what the user would do with enter. A first click on a different row
	// only moves the selection, so a stray click can never launch a
	// command the user did not aim at.
	if wasSelected || m.singleClick {
		m.resetClick()
		return m.activate(activate, idx)
	}
	return nil
}

func (m *ListMouse) activate(activate func(idx int) Action, idx int) Action {
	if activate == nil {
		return nil
	}
	return activate(idx)
}

func (m *ListMouse) resetClick() {
	m.lastClickIdx = -1
	m.lastClickTime = time.Time{}
}
