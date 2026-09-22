package dialog

import (
	"fmt"
	"image"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/question"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// cellContent returns the glyph painted at a screen cell.
func cellContent(scr uv.Screen, x, y int) string {
	cell := scr.CellAt(x, y)
	if cell == nil {
		return ""
	}
	return cell.Content
}

// requireTrackMatchesPaintedScrollbar checks that a recorded scrollbar track
// sits exactly on the column of thumb and track glyphs a dialog painted.
func requireTrackMatchesPaintedScrollbar(t *testing.T, scr uv.Screen, track common.ScrollbarTrack) {
	t.Helper()

	require.True(t, track.Visible(), "the dialog must paint a scrollbar")
	thumbRow, thumbSize, _ := common.ScrollbarGeometry(track.Height, track.ContentSize, track.ViewportSize, track.Offset)
	require.Equal(t, styles.ScrollbarThumb, cellContent(scr, track.X, track.MinY+thumbRow),
		"the recorded column must hold the painted thumb")
	if thumbRow+thumbSize < track.Height {
		require.Equal(t, styles.ScrollbarTrack, cellContent(scr, track.X, track.MinY+thumbRow+thumbSize),
			"the recorded column must hold the painted track")
	}
}

func manySessions(count int) []session.Session {
	sessions := make([]session.Session, count)
	for i := range sessions {
		sessions[i] = session.Session{
			ID:    fmt.Sprintf("s%02d", i),
			Title: fmt.Sprintf("Session %02d", i),
		}
	}
	return sessions
}

func TestSessionScrollbarDragScrollsList(t *testing.T) {
	t.Parallel()

	sess := newSessionMouseDialog(t, manySessions(40), "s00")
	scr := uv.NewScreenBuffer(100, 40)
	sess.Draw(scr, image.Rect(0, 0, 100, 40))

	track := sess.scrollbarZone.track
	requireTrackMatchesPaintedScrollbar(t, scr, track)
	require.Zero(t, sess.list.Offset())

	// Press the thumb where it is painted, then drag to the bottom of the
	// track: the list follows the pointer all the way down.
	sess.HandleMsg(tea.MouseClickMsg(tea.Mouse{X: track.X, Y: track.MinY, Button: tea.MouseLeft}))
	require.True(t, sess.scrollbarZone.Dragging())

	sess.HandleMsg(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height/2, Button: tea.MouseLeft}))
	middle := sess.list.Offset()
	require.Greater(t, middle, 0, "dragging down must scroll the list")

	sess.HandleMsg(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height - 1, Button: tea.MouseLeft}))
	require.True(t, sess.list.AtBottom(), "dragging to the track bottom must reach the list end")

	// Dragging back to the top returns to the start.
	sess.HandleMsg(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY, Button: tea.MouseLeft}))
	require.Zero(t, sess.list.Offset())

	sess.HandleMsg(tea.MouseReleaseMsg(tea.Mouse{X: track.X, Y: track.MinY, Button: tea.MouseLeft}))
	require.False(t, sess.scrollbarZone.Dragging())

	// Motion after release must not scroll, and the Draw that follows must
	// keep the position the drag left behind.
	sess.HandleMsg(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height - 1, Button: tea.MouseLeft}))
	require.Zero(t, sess.list.Offset())
	sess.Draw(scr, image.Rect(0, 0, 100, 40))
	require.Zero(t, sess.list.Offset())
}

func TestSessionScrollbarClickJumpsToClickedRow(t *testing.T) {
	t.Parallel()

	sess := newSessionMouseDialog(t, manySessions(40), "s00")
	scr := uv.NewScreenBuffer(100, 40)
	sess.Draw(scr, image.Rect(0, 0, 100, 40))
	track := sess.scrollbarZone.track

	// Clicking the empty track at the bottom scrolls there in one press,
	// without needing a drag.
	sess.HandleMsg(tea.MouseClickMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height - 1, Button: tea.MouseLeft}))
	require.True(t, sess.list.AtBottom())
}

func TestSessionListPressOutsideScrollbarStillSelects(t *testing.T) {
	t.Parallel()

	sess := newSessionMouseDialog(t, manySessions(40), "s00")
	scr := uv.NewScreenBuffer(100, 40)
	sess.Draw(scr, image.Rect(0, 0, 100, 40))
	track := sess.scrollbarZone.track

	// A press a column to the left of the track is a list click, not a grab.
	sess.HandleMsg(tea.MouseClickMsg(tea.Mouse{X: track.X - 1, Y: track.MinY + 1, Button: tea.MouseLeft}))
	require.False(t, sess.scrollbarZone.Dragging())
	require.Zero(t, sess.list.Offset(), "a list click must not scroll")
}

func TestPermissionsScrollbarDragScrollsContent(t *testing.T) {
	t.Parallel()

	sty := styles.ColorTonePantera()
	com := &common.Common{Styles: &sty}
	long := strings.Repeat("detail line\n", 200)
	p := NewPermissions(com, permission.PermissionRequest{
		ID:          "perm-scroll",
		ToolCallID:  "tool-scroll",
		ToolName:    "custom_tool",
		Description: long,
	})

	scr := uv.NewScreenBuffer(90, 40)
	p.Draw(scr, image.Rect(0, 0, 90, 40))

	track := p.scrollbarZone.track
	requireTrackMatchesPaintedScrollbar(t, scr, track)
	require.Zero(t, p.viewport.YOffset())

	p.HandleMsg(tea.MouseClickMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height - 1, Button: tea.MouseLeft}))
	require.Greater(t, p.viewport.YOffset(), 0, "clicking the track must scroll the content down")

	p.HandleMsg(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY, Button: tea.MouseLeft}))
	require.Zero(t, p.viewport.YOffset(), "dragging to the top must return to the start")

	p.HandleMsg(tea.MouseReleaseMsg(tea.Mouse{X: track.X, Y: track.MinY, Button: tea.MouseLeft}))
	require.False(t, p.scrollbarZone.Dragging())
}

func TestQuestionChoiceScrollbarDragScrollsChoices(t *testing.T) {
	t.Parallel()

	sty := styles.ColorTonePantera()
	choices := make([]question.Choice, 30)
	for i := range choices {
		choices[i] = question.Choice{ID: fmt.Sprintf("c%02d", i), Label: fmt.Sprintf("Choice %02d", i)}
	}
	choice := NewSingleChoice(&sty, question.Question{
		ID:      "q-scroll",
		Type:    question.TypeSingleChoice,
		Text:    "Pick one",
		Choices: choices,
	})

	scr := uv.NewScreenBuffer(40, 12)
	area := image.Rect(0, 0, 40, 12)
	choice.Draw(scr, area)

	track := choice.scrollbarZone.track
	requireTrackMatchesPaintedScrollbar(t, scr, track)
	require.Zero(t, choice.scrollOffset)

	require.True(t, choice.HandleScrollbarMouse(tea.MouseClickMsg(tea.Mouse{X: track.X, Y: track.MinY, Button: tea.MouseLeft})))
	// Redraw keeps the drag anchored: the thumb must stay where it was painted.
	choice.Draw(scr, area)

	require.True(t, choice.HandleScrollbarMouse(tea.MouseMotionMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height - 1, Button: tea.MouseLeft})))
	require.Greater(t, choice.scrollOffset, 0, "dragging the thumb down must scroll the choices")

	require.True(t, choice.HandleScrollbarMouse(tea.MouseReleaseMsg(tea.Mouse{X: track.X, Y: track.MinY + track.Height - 1, Button: tea.MouseLeft})))
	require.False(t, choice.scrollbarZone.Dragging())
}

func TestScrollbarZoneIgnoresPressesWhenNothingOverflows(t *testing.T) {
	t.Parallel()

	var zone ScrollbarZone
	zone.Painted(image.Pt(10, 2), "content", 10, 8, 10, 0)

	require.False(t, zone.track.Visible(), "content that fits paints no thumb")

	var scrolled int
	handle := func(int) { scrolled++ }
	require.False(t, zone.HandleMsg(tea.MouseClickMsg(tea.Mouse{X: 17, Y: 5}), handle))
	require.Zero(t, scrolled)
}
