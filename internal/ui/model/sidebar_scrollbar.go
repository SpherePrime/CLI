package model

import (
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
)

// handleSidebarScrollbarMouseDown grabs the sidebar thumb when the press at
// the absolute cell x,y lands on the painted sidebar scrollbar column.
func (m *UI) handleSidebarScrollbarMouseDown(x, y int) (bool, tea.Cmd) {
	// The press itself can reveal the scrollbar (by focusing the sidebar),
	// so refresh the geometry before testing for a hit.
	m.recordSidebarScrollbarTrack()
	offset, started := m.sidebarScrollbarDrag.Begin(m.sidebarScrollbarTrack, x, y)
	if !started {
		return false, nil
	}
	return true, m.setSidebarOffset(offset)
}

// handleSidebarScrollbarMouseDrag scrolls the sidebar so the thumb follows the
// pointer at the absolute screen row y.
func (m *UI) handleSidebarScrollbarMouseDrag(y int) (bool, tea.Cmd) {
	offset, ok := m.sidebarScrollbarDrag.Move(y)
	if !ok {
		return false, nil
	}
	return true, m.setSidebarOffset(offset)
}

// handleSidebarScrollbarMouseUp releases the sidebar thumb.
func (m *UI) handleSidebarScrollbarMouseUp() {
	m.sidebarScrollbarDrag.End()
}

// sidebarScrollbarDragging reports whether the pointer is holding the sidebar
// scrollbar.
func (m *UI) sidebarScrollbarDragging() bool {
	return m.sidebarScrollbarDrag.Active()
}

// setSidebarOffset scrolls the sidebar to the given line offset and re-arms
// the scrollbar auto-hide timer.
func (m *UI) setSidebarOffset(offset int) tea.Cmd {
	m.sidebarOffset = max(0, min(offset, m.sidebarMaxOffsetVal))
	m.sidebarScrollbarSeq++
	m.sidebarScrollbarVisible = true
	return sidebarScrollbarHideCmd(m.sidebarScrollbarSeq)
}
