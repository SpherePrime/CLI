package common

import (
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
)

func scrollbarTestStyles(t *testing.T) *styles.Styles {
	t.Helper()
	sty := styles.ColorTonePantera()
	return &sty
}

// renderedRows strips styling so each track row can be compared against the
// plain scrollbar glyphs.
func renderedRows(t *testing.T, rendered string) []string {
	t.Helper()
	return strings.Split(ansi.Strip(rendered), "\n")
}

func testTrack() ScrollbarTrack {
	return ScrollbarTrack{
		X:            79,
		MinY:         2,
		Height:       20,
		ContentSize:  200,
		ViewportSize: 20,
		Offset:       0,
	}
}

func TestScrollbarGeometryMatchesRenderedThumb(t *testing.T) {
	t.Parallel()

	sty := scrollbarTestStyles(t)

	track := testTrack()
	for offset := range track.ContentSize - track.ViewportSize + 1 {
		pos, size, ok := ScrollbarGeometry(track.Height, track.ContentSize, track.ViewportSize, offset)
		require.True(t, ok)
		require.GreaterOrEqual(t, size, 1)
		require.LessOrEqual(t, pos+size, track.Height)

		lines := renderedRows(t, Scrollbar(sty, track.Height, track.ContentSize, track.ViewportSize, offset))
		require.Len(t, lines, track.Height)

		first, last := -1, -1
		for row, line := range lines {
			if line == styles.ScrollbarThumb {
				if first < 0 {
					first = row
				}
				last = row
			}
		}
		require.Equal(t, pos, first, "thumb row must match at offset %d", offset)
		require.Equal(t, pos+size-1, last, "thumb span must match at offset %d", offset)
	}
}

func TestScrollbarGeometryHiddenWhenContentFits(t *testing.T) {
	t.Parallel()

	_, _, ok := ScrollbarGeometry(20, 20, 20, 0)
	require.False(t, ok, "content that fits the viewport paints no thumb")

	fitting := testTrack()
	fitting.ContentSize = fitting.ViewportSize
	require.False(t, fitting.Visible())

	var drag ScrollbarDrag
	_, started := drag.Begin(fitting, fitting.X, fitting.MinY)
	require.False(t, started, "an absent thumb cannot be grabbed")
}

func TestTrackOffsetAtRowCoversBothEnds(t *testing.T) {
	t.Parallel()

	track := testTrack()
	require.Zero(t, track.OffsetAtRow(0))
	require.Equal(t, track.ContentSize-track.ViewportSize, track.OffsetAtRow(track.Height))

	// Rows above and below the track clamp to the ends instead of
	// producing offsets outside the scroll range.
	require.Zero(t, track.OffsetAtRow(-5))
	require.Equal(t, track.ContentSize-track.ViewportSize, track.OffsetAtRow(track.Height+5))
}

func TestTrackOffsetAtRowIsMonotonic(t *testing.T) {
	t.Parallel()

	track := testTrack()
	maxOffset := track.ContentSize - track.ViewportSize
	previous := -1
	for row := 0; row <= track.Height; row++ {
		offset := track.OffsetAtRow(row)
		require.GreaterOrEqual(t, offset, previous, "row %d must not scroll backwards", row)
		require.LessOrEqual(t, offset, maxOffset)
		previous = offset
	}
}

func TestTrackRowRoundTrip(t *testing.T) {
	t.Parallel()

	track := testTrack()
	_, thumbSize, _ := ScrollbarGeometry(track.Height, track.ContentSize, track.ViewportSize, 0)
	// Only rows up to the space left by the thumb can hold the thumb top.
	for row := 0; row <= track.Height-thumbSize; row++ {
		offset := track.OffsetAtRow(row)
		roundTripped := track.RowOfOffset(offset)
		require.InDelta(t, row, roundTripped, 1, "row %d must round-trip", row)
	}
}

func TestTrackContainsOnlyOwnColumn(t *testing.T) {
	t.Parallel()

	track := testTrack()
	require.True(t, track.Contains(79, 2))
	require.True(t, track.Contains(79, 21))
	require.False(t, track.Contains(79, 22), "one row past the track")
	require.False(t, track.Contains(79, 1), "one row before the track")
	require.False(t, track.Contains(78, 10), "a neighbouring column")
}

func TestScrollbarDragGrabsThumbWithoutJumping(t *testing.T) {
	t.Parallel()

	track := testTrack()
	track.Offset = 90

	var drag ScrollbarDrag
	offset, started := drag.Begin(track, track.X, track.MinY+track.RowOfOffset(track.Offset)+1)
	require.True(t, started)
	require.Equal(t, track.Offset, offset, "grabbing the thumb must not move it")
	require.True(t, drag.Active())

	// Dragging two rows down moves the thumb two rows down.
	offset, ok := drag.Move(track.MinY + track.RowOfOffset(track.Offset) + 3)
	require.True(t, ok)
	require.Greater(t, offset, track.Offset)
	require.InDelta(t, track.RowOfOffset(offset), track.RowOfOffset(track.Offset)+2, 1)

	drag.End()
	require.False(t, drag.Active())
	_, ok = drag.Move(0)
	require.False(t, ok, "a released drag must not report offsets")
}

func TestScrollbarDragClickOnEmptyTrackJumps(t *testing.T) {
	t.Parallel()

	track := testTrack()
	var drag ScrollbarDrag
	offset, started := drag.Begin(track, track.X, track.MinY+track.Height-1)
	require.True(t, started)
	require.Equal(t, track.ContentSize-track.ViewportSize, offset, "clicking the track bottom scrolls to the end")

	// After the jump the pointer holds the top of the thumb, so pulling the
	// drag well up the track scrolls back up.
	offset, ok := drag.Move(track.MinY + 5)
	require.True(t, ok)
	require.Less(t, offset, track.ContentSize-track.ViewportSize)
}

func TestScrollbarDragMissesTrack(t *testing.T) {
	t.Parallel()

	track := testTrack()
	var drag ScrollbarDrag
	_, started := drag.Begin(track, track.X-1, track.MinY)
	require.False(t, started)
	require.False(t, drag.Active())
}

func TestTrackOffsetAtRowWhenThumbFillsTrack(t *testing.T) {
	t.Parallel()

	// Content just barely overflows: the thumb spans the whole track, and a
	// drag still has to reach both ends.
	track := ScrollbarTrack{X: 10, MinY: 0, Height: 10, ContentSize: 11, ViewportSize: 10, Offset: 0}
	require.True(t, track.Visible())
	require.Zero(t, track.OffsetAtRow(0))
	require.Equal(t, 1, track.OffsetAtRow(9))
}
