package common

// ScrollbarGeometry returns the first row of the scrollbar thumb and its
// height inside a track of the given height. Rows are counted from the top of
// the track. ok is false when the content fits the viewport and no thumb is
// painted, matching the empty result of [Scrollbar].
func ScrollbarGeometry(height, contentSize, viewportSize, offset int) (thumbPos, thumbSize int, ok bool) {
	if height <= 0 || contentSize <= viewportSize {
		return 0, 0, false
	}

	thumbSize = max(1, height*viewportSize/contentSize)
	maxOffset := contentSize - viewportSize
	if maxOffset <= 0 {
		return 0, 0, false
	}

	trackSpace := height - thumbSize
	if trackSpace > 0 {
		thumbPos = min(trackSpace, offset*trackSpace/maxOffset)
	}
	return thumbPos, thumbSize, true
}

// ScrollbarTrack is the screen column a scrollbar was painted in together
// with the scroll state it mirrors, so pointer input on it can be turned back
// into a scroll offset.
type ScrollbarTrack struct {
	// X is the screen column holding the track, MinY its first row, and
	// Height the number of rows the track spans.
	X      int
	MinY   int
	Height int

	// ContentSize, ViewportSize, and Offset describe the scrolled content.
	ContentSize  int
	ViewportSize int
	Offset       int
}

// Visible reports whether the track holds a painted thumb.
func (t ScrollbarTrack) Visible() bool {
	_, _, ok := ScrollbarGeometry(t.Height, t.ContentSize, t.ViewportSize, t.Offset)
	return ok
}

// Contains reports whether the cell at x,y belongs to the track.
func (t ScrollbarTrack) Contains(x, y int) bool {
	if !t.Visible() || x != t.X {
		return false
	}
	return y >= t.MinY && y < t.MinY+t.Height
}

// Row returns the track row holding screen row y.
func (t ScrollbarTrack) Row(y int) int {
	return y - t.MinY
}

// RowOfOffset returns the track row the thumb starts on for the given offset.
func (t ScrollbarTrack) RowOfOffset(offset int) int {
	pos, _, ok := ScrollbarGeometry(t.Height, t.ContentSize, t.ViewportSize, offset)
	if !ok {
		return 0
	}
	return pos
}

// OffsetAtRow returns the scroll offset that puts the top of the thumb on
// track row, clamped to the offset range the content allows.
func (t ScrollbarTrack) OffsetAtRow(row int) int {
	maxOffset := t.ContentSize - t.ViewportSize
	if maxOffset <= 0 || t.Height <= 0 {
		return 0
	}

	_, thumbSize, ok := ScrollbarGeometry(t.Height, t.ContentSize, t.ViewportSize, t.Offset)
	if !ok {
		return 0
	}

	trackSpace := t.Height - thumbSize
	if trackSpace <= 0 {
		// The thumb spans the whole track: spread the few available offsets
		// over the track height so a drag still reaches both ends.
		trackSpace = t.Height - 1
		if trackSpace <= 0 {
			return maxOffset
		}
		row = min(max(row, 0), trackSpace)
		return (row*maxOffset + trackSpace/2) / trackSpace
	}

	row = min(max(row, 0), trackSpace)
	return (row*maxOffset + trackSpace/2) / trackSpace
}

// ScrollbarDrag holds the state of one pointer drag across a scrollbar track.
// The zero value is idle.
type ScrollbarDrag struct {
	track   ScrollbarTrack
	grabRow int
	active  bool
}

// Active reports whether a drag is in progress.
func (d *ScrollbarDrag) Active() bool {
	return d != nil && d.active
}

// Begin starts a drag when the press at x,y lands on the painted track. It
// returns the offset to scroll to: unchanged when the thumb was grabbed where
// it already is, or the row under the pointer when empty track was clicked.
func (d *ScrollbarDrag) Begin(track ScrollbarTrack, x, y int) (offset int, started bool) {
	if !track.Contains(x, y) {
		return 0, false
	}

	row := track.Row(y)
	target := track.Offset
	thumbPos, thumbSize, _ := ScrollbarGeometry(track.Height, track.ContentSize, track.ViewportSize, track.Offset)
	if row < thumbPos || row >= thumbPos+thumbSize {
		// Empty track: put the top of the thumb under the pointer.
		d.grabRow = 0
		target = track.OffsetAtRow(row)
	} else {
		// The thumb was grabbed where it is, so it stays put and follows the
		// pointer from the grabbed row inside it.
		d.grabRow = row - thumbPos
	}

	d.track = track
	d.active = true
	return target, true
}

// Move returns the offset the thumb should have after the pointer reaches
// screen row y, or false when no drag is active.
func (d *ScrollbarDrag) Move(y int) (offset int, ok bool) {
	if !d.active {
		return 0, false
	}
	return d.track.OffsetAtRow(d.track.Row(y) - d.grabRow), true
}

// End stops the drag.
func (d *ScrollbarDrag) End() {
	d.active = false
	d.grabRow = 0
}
