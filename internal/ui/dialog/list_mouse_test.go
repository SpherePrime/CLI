package dialog

import (
	"image"
	"testing"

	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/list"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// mouseTestItem is a minimal one-line list item for hit-testing the mouse
// helper.
type mouseTestItem struct {
	*list.Versioned
	id string
}

func (m *mouseTestItem) Render(int) string { return m.id }

func (m *mouseTestItem) Finished() bool { return true }

// newMouseTestList returns a list of count single-line items in a viewport
// viewportHeight rows tall, so a caller can choose whether the content
// overflows (and therefore whether scrolling is possible).
func newMouseTestList(count, viewportHeight int) *list.List {
	items := make([]list.Item, count)
	for i := range count {
		items[i] = &mouseTestItem{Versioned: list.NewVersioned(), id: "row"}
	}
	l := list.NewList(items...)
	l.SetSize(20, viewportHeight)
	return l
}

// clickAt builds a left-button press at the given screen cell.
func clickAt(x, y int) tea.MouseClickMsg {
	return tea.MouseClickMsg(tea.Mouse{X: x, Y: y, Button: tea.MouseLeft})
}

// The list area the helper is told to expect, matching newMouseTestList's
// geometry: 20xN at the origin.
func testArea(height int) image.Rectangle {
	return image.Rect(0, 0, 20, height)
}

func TestListMouseClickSelectsRow(t *testing.T) {
	t.Parallel()

	l := newMouseTestList(5, 5)
	var m ListMouse
	m.Painted(testArea(5))
	l.SetSelected(0)

	// Clicking row 3 selects it and does not activate anything yet, so a
	// stray click can never launch the wrong item.
	action := m.HandleMsg(clickAt(2, 3), l, func(int) Action { return ActionClose{} })
	require.Nil(t, action)
	require.Equal(t, 3, l.Selected())
}

func TestListMouseClickSelectedRowActivates(t *testing.T) {
	t.Parallel()

	l := newMouseTestList(5, 5)
	var m ListMouse
	m.Painted(testArea(5))
	l.SetSelected(3)

	action := m.HandleMsg(clickAt(2, 3), l, func(idx int) Action { return ActionSelectLanguage{Locale: "row"} })
	sel, ok := action.(ActionSelectLanguage)
	require.True(t, ok, "clicking the selected row must activate it, got %T", action)
	require.Equal(t, "row", sel.Locale)
}

func TestListMouseDoubleClickActivates(t *testing.T) {
	t.Parallel()

	l := newMouseTestList(5, 5)
	var m ListMouse
	m.Painted(testArea(5))
	l.SetSelected(0)

	// First click on a different row only selects.
	require.Nil(t, m.HandleMsg(clickAt(2, 4), l, func(int) Action { return ActionClose{} }))
	require.Equal(t, 4, l.Selected())

	// The second click on the same row is a double click and activates.
	action := m.HandleMsg(clickAt(2, 4), l, func(int) Action { return ActionClose{} })
	require.IsType(t, ActionClose{}, action)
}

func TestListMouseActivateOnClick(t *testing.T) {
	t.Parallel()

	l := newMouseTestList(5, 5)
	var m ListMouse
	m.Painted(testArea(5))
	m.ActivateOnClick()
	l.SetSelected(0)

	// One click is enough when a row's action is self-evident.
	action := m.HandleMsg(clickAt(2, 2), l, func(int) Action { return ActionClose{} })
	require.IsType(t, ActionClose{}, action)
	require.Equal(t, 2, l.Selected())
}

func TestListMouseClickOutsideAreaIsIgnored(t *testing.T) {
	t.Parallel()

	l := newMouseTestList(5, 5)
	var m ListMouse
	m.Painted(testArea(5))
	l.SetSelected(1)

	// Below the list, left of it, and right of the list content width.
	for _, pt := range [][2]int{{2, 9}, {-1, 2}, {25, 2}} {
		action := m.HandleMsg(clickAt(pt[0], pt[1]), l, func(int) Action { return ActionClose{} })
		require.Nil(t, action, "click at %v must be ignored", pt)
		require.Equal(t, 1, l.Selected())
	}
}

func TestListMouseWithoutGeometryIsIgnored(t *testing.T) {
	t.Parallel()

	l := newMouseTestList(5, 5)
	var m ListMouse
	l.SetSelected(1)

	// No Painted call yet: a click before the first Draw must not panic or
	// select anything.
	require.Nil(t, m.HandleMsg(clickAt(2, 2), l, func(int) Action { return ActionClose{} }))
	require.Equal(t, 1, l.Selected())
}

func TestListMouseNonLeftButtonIsIgnored(t *testing.T) {
	t.Parallel()

	l := newMouseTestList(5, 5)
	var m ListMouse
	m.Painted(testArea(5))
	l.SetSelected(1)

	right := tea.MouseClickMsg(tea.Mouse{X: 2, Y: 3, Button: tea.MouseRight})
	require.Nil(t, m.HandleMsg(right, l, func(int) Action { return ActionClose{} }))
	require.Equal(t, 1, l.Selected())
}

func TestListMouseClearForgetsGeometry(t *testing.T) {
	t.Parallel()

	l := newMouseTestList(5, 5)
	var m ListMouse
	m.Painted(testArea(5))
	l.SetSelected(1)
	m.Clear()

	require.Nil(t, m.HandleMsg(clickAt(2, 3), l, func(int) Action { return ActionClose{} }))
	require.Equal(t, 1, l.Selected())
}

func TestListMouseWheelScrollsOverList(t *testing.T) {
	t.Parallel()

	l := newMouseTestList(20, 5)
	var m ListMouse
	m.Painted(image.Rect(0, 0, 20, 5))
	l.SetSelected(0)

	before := l.Offset()
	wheel := common.CoalescedWheelMsg{Mouse: tea.Mouse{X: 2, Y: 2}, DeltaY: 3}
	m.HandleMsg(wheel, l, nil)
	require.Equal(t, before+3, l.Offset())
	require.True(t, m.Scrolled())

	// A wheel outside the list leaves it alone.
	outside := common.CoalescedWheelMsg{Mouse: tea.Mouse{X: 2, Y: 40}, DeltaY: 3}
	m.HandleMsg(outside, l, nil)
	require.Equal(t, before+3, l.Offset())
}

func TestListMouseScrollbarDragTakesPriorityOverRows(t *testing.T) {
	t.Parallel()

	l := newMouseTestList(40, 5)
	var m ListMouse
	m.Painted(image.Rect(0, 0, 20, 5))
	// A scrollbar painted just right of the list content.
	m.PaintedScrollbar(image.Pt(20, 0), 5, 40, 5, 0)
	l.SetSelected(0)

	before := l.Selected()
	// Press on the track below the thumb jumps the list down.
	require.Nil(t, m.HandleMsg(clickAt(20, 4), l, func(int) Action { return ActionClose{} }))
	require.Equal(t, before, l.Selected(), "a scrollbar press must not select a row")
	require.NotEqual(t, 0, l.Offset(), "a scrollbar press must scroll")
}