package model

import (
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
)

// appMargin is the spacing the app area keeps from the screen edges.
const appMargin = 1

// The main pane is never allowed to collapse completely: margins and lower
// panes give way on short windows so chat keeps at least this much room.
const (
	minMainAreaHeight = 1
	minMainAreaWidth  = 1
)

// minEditorContentWidth keeps word wrapping in the prompt sane on windows that
// are narrower than the editor chrome around it.
const minEditorContentWidth = 1

// insetTop moves the top edge of r down by n rows, skipping the margin when r
// is too short to keep at least one visible row.
func insetTop(r uv.Rectangle, n int) uv.Rectangle {
	if r.Dy() > n {
		r.Min.Y += n
	}
	return r
}

// insetBottom moves the bottom edge of r up by n rows, skipping the margin
// when r is too short to keep at least one visible row.
func insetBottom(r uv.Rectangle, n int) uv.Rectangle {
	if r.Dy() > n {
		r.Max.Y -= n
	}
	return r
}

// insetLeft moves the left edge of r right by n columns, skipping the margin
// when r is too narrow to keep at least one visible column.
func insetLeft(r uv.Rectangle, n int) uv.Rectangle {
	if r.Dx() > n {
		r.Min.X += n
	}
	return r
}

// insetRight moves the right edge of r left by n columns, skipping the margin
// when r is too narrow to keep at least one visible column.
func insetRight(r uv.Rectangle, n int) uv.Rectangle {
	if r.Dx() > n {
		r.Max.X -= n
	}
	return r
}

// fitLowerPane caps the height requested by a pane that sits below the main
// area (editor, pills) so the main area keeps minMainAreaHeight rows. It
// returns 0 when there is no room for both, which hands everything to main.
func fitLowerPane(requested, available int) int {
	if requested <= 0 || available <= minMainAreaHeight {
		return 0
	}
	return min(requested, available-minMainAreaHeight)
}

// fitSidebarWidth caps the sidebar width so the main area keeps
// minMainAreaWidth columns. It returns 0 when both cannot fit side by side.
func fitSidebarWidth(requested, available int) int {
	if available <= minMainAreaWidth {
		return 0
	}
	return min(requested, available-minMainAreaWidth)
}

// clampToArea pulls r back inside area and removes inverted edges, so child
// components never receive a negative width or height.
func clampToArea(r, area uv.Rectangle) uv.Rectangle {
	r.Min.X = min(max(r.Min.X, area.Min.X), area.Max.X)
	r.Min.Y = min(max(r.Min.Y, area.Min.Y), area.Max.Y)
	r.Max.X = min(max(r.Max.X, r.Min.X), area.Max.X)
	r.Max.Y = min(max(r.Max.Y, r.Min.Y), area.Max.Y)
	return r
}

// withinArea clamps every region of the layout to the screen. Terminal windows
// can be smaller than the fixed margins and panels the layout reserves; the
// guards above keep those regions from turning into negative rectangles, which
// child components draw as empty holes and use for scroll math.
func (l uiLayout) withinArea() uiLayout {
	for _, rect := range []*uv.Rectangle{
		&l.header,
		&l.main,
		&l.pills,
		&l.editor,
		&l.sidebar,
		&l.status,
		&l.sessionDetails,
	} {
		*rect = clampToArea(*rect, l.area)
	}
	return l
}
