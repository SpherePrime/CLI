package model

import (
	"image"
	"strings"
	"testing"

	"github.com/dwertyfa288/CLI/internal/config"
	"github.com/dwertyfa288/CLI/internal/ui/common"
	"github.com/dwertyfa288/CLI/internal/ui/styles"
	tea "github.com/dwertyfa288/CLI/vendordeps/bubbletea/v2"
	"github.com/dwertyfa288/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/dwertyfa288/CLI/vendordeps/stretchr/testify/require"
)

// paintFrame renders one frame and returns the chat scrollbar geometry it
// recorded, so tests grab the same column the user sees.
func paintFrame(t *testing.T, u *UI) common.ScrollbarTrack {
	t.Helper()

	u.View()
	track := u.chat.scrollbarTrack
	require.True(t, track.Visible(), "the chat must paint a scrollbar when content overflows")
	return track
}

// cellAt returns the plain rune painted at a screen cell.
func cellAt(t *testing.T, view tea.View, x, y int) string {
	t.Helper()

	lines := strings.Split(ansi.Strip(view.Content), "\n")
	require.Greater(t, len(lines), y, "frame must reach row %d", y)
	cells := []rune(lines[y])
	require.Greater(t, len(cells), x, "frame must reach column %d", x)
	return string(cells[x])
}

func TestChatScrollbarTrackMatchesPaintedColumn(t *testing.T) {
	t.Parallel()

	u := newFrameTestUI(t)
	u.chat.ScrollToTop()
	view := u.View()
	track := u.chat.scrollbarTrack

	require.Equal(t, u.layout.main.Max.X-1, track.X)
	require.Equal(t, u.layout.main.Min.Y, track.MinY)
	require.Equal(t, styles.ScrollbarThumb, cellAt(t, view, track.X, track.MinY),
		"the recorded track must start on the painted thumb")
}

func TestChatScrollbarDragScrollsWithPointer(t *testing.T) {
	t.Parallel()

	u := newFrameTestUI(t)
	u.chat.ScrollToTop()
	track := paintFrame(t, u)
	_, thumbSize, _ := common.ScrollbarGeometry(track.Height, track.ContentSize, track.ViewportSize, track.Offset)

	// Grab the thumb where it is painted: nothing moves until the pointer does.
	u.Update(tea.MouseClickMsg(tea.Mouse{X: track.X, Y: track.MinY + thumbSize/2, Button: tea.MouseLeft}))
	require.True(t, u.chat.ScrollbarDragging())

	u.Update(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY + thumbSize/2 + 5, Button: tea.MouseLeft}))
	require.Greater(t, u.chat.Offset(), 0, "dragging down must scroll the chat down")

	// Dragging back up follows the pointer both ways.
	u.Update(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY, Button: tea.MouseLeft}))
	require.Less(t, u.chat.Offset(), track.ContentSize-track.ViewportSize)

	// Dragging past the bottom of the track clamps at the end of the content.
	u.Update(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height + 10, Button: tea.MouseLeft}))
	require.True(t, u.chat.AtBottom())

	u.Update(tea.MouseReleaseMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height, Button: tea.MouseLeft}))
	require.False(t, u.chat.ScrollbarDragging())

	// After releasing, motion must not scroll anything.
	u.chat.ScrollToTop()
	stopped := u.chat.Offset()
	u.Update(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height, Button: tea.MouseLeft}))
	require.Equal(t, stopped, u.chat.Offset())
}

func TestChatScrollbarClickJumpsToClickedRow(t *testing.T) {
	t.Parallel()

	u := newFrameTestUI(t)
	u.chat.ScrollToTop()
	track := paintFrame(t, u)

	// Pressing the empty track at the bottom scrolls straight to the end.
	u.Update(tea.MouseClickMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height - 1, Button: tea.MouseLeft}))
	require.True(t, u.chat.AtBottom())
}

func TestChatScrollbarDragKeepsThumbVisibleInDefaultMode(t *testing.T) {
	t.Parallel()

	u := newFrameTestUI(t)
	u.chat.scrollbarMode = config.ScrollbarDefault
	u.chat.ScrollToTop()
	track := paintFrame(t, u)

	u.Update(tea.MouseClickMsg(tea.Mouse{X: track.X, Y: track.MinY, Button: tea.MouseLeft}))

	// The auto-hide timer ran, but the thumb is still held, so the column
	// keeps painting while the drag is on.
	u.chat.scrollbarVisible = false
	u.Update(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY + 4, Button: tea.MouseLeft}))
	view := u.View()
	require.Greater(t, thumbRowsInView(t, view, track), 0, "a held thumb must stay painted")
}

// thumbRowsInView counts the painted thumb cells in a scrollbar column.
func thumbRowsInView(t *testing.T, view tea.View, track common.ScrollbarTrack) int {
	t.Helper()

	rows := 0
	for row := range track.Height {
		if cellAt(t, view, track.X, track.MinY+row) == styles.ScrollbarThumb {
			rows++
		}
	}
	return rows
}

func TestChatScrollbarPressOutsideColumnStartsTextSelection(t *testing.T) {
	t.Parallel()

	u := newFrameTestUI(t)
	u.chat.ScrollToTop()
	paintFrame(t, u)

	u.Update(tea.MouseClickMsg(tea.Mouse{
		X:      u.layout.main.Min.X + 1,
		Y:      u.layout.main.Min.Y + 1,
		Button: tea.MouseLeft,
	}))
	require.False(t, u.chat.ScrollbarDragging(), "a press away from the track must not grab it")
}

func TestSidebarScrollbarDragScrollsSidebar(t *testing.T) {
	t.Parallel()

	u := newFrameTestUI(t)
	u.layout.sidebar = image.Rect(0, 0, 32, 40)
	u.sidebarScrollable = true
	u.sidebarTotalLines = 100
	u.sidebarContentHeight = 20
	u.sidebarContentTop = 10
	u.sidebarMaxOffsetVal = 80
	u.recordSidebarScrollbarTrack()

	u.Update(tea.MouseClickMsg(tea.Mouse{X: 31, Y: 10, Button: tea.MouseLeft}))
	require.True(t, u.sidebarScrollbarDrag.Active())
	require.Zero(t, u.sidebarOffset)

	u.Update(tea.MouseMotionMsg(tea.Mouse{X: 31, Y: 29, Button: tea.MouseLeft}))
	require.Equal(t, 80, u.sidebarOffset)
	require.True(t, u.sidebarScrollbarVisible, "scrolling must reveal the scrollbar")

	u.Update(tea.MouseMotionMsg(tea.Mouse{X: 31, Y: 10, Button: tea.MouseLeft}))
	require.Zero(t, u.sidebarOffset)

	u.Update(tea.MouseReleaseMsg(tea.Mouse{X: 31, Y: 10, Button: tea.MouseLeft}))
	require.False(t, u.sidebarScrollbarDrag.Active())
}
