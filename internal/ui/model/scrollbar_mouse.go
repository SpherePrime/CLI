package model

import (
	"github.com/SpherePrime/CLI/internal/ui/common"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
)

// handleScrollbarMouseDown grabs the sidebar or chat scrollbar thumb sitting
// under the absolute cell x,y, scrolling to the grabbed position.
func (m *UI) handleScrollbarMouseDown(x, y int) (bool, tea.Cmd) {
	if handled, cmd := m.handleSidebarScrollbarMouseDown(x, y); handled {
		return true, cmd
	}
	return m.chat.HandleScrollbarMouseDown(x, y)
}

// handleScrollbarMouseDrag moves whichever thumb the pointer is holding to
// the absolute screen row y.
func (m *UI) handleScrollbarMouseDrag(y int) (bool, tea.Cmd) {
	if handled, cmd := m.handleSidebarScrollbarMouseDrag(y); handled {
		return true, cmd
	}
	if m.chat.ScrollbarDragging() {
		// Nothing but the chat scroll position changes, so a memoized frame
		// can serve the new position.
		m.markScrollOnly()
		return m.chat.HandleScrollbarMouseDrag(y)
	}
	return false, nil
}

// handleInlineScrollbar routes a mouse message to the active inline editor's
// scrollbar thumb, reporting whether it consumed the event.
func (m *UI) handleInlineScrollbar(msg tea.Msg) bool {
	if m.activeInline == nil {
		return false
	}
	draggable, ok := m.activeInline.(common.ScrollbarDraggable)
	return ok && draggable.HandleScrollbarMouse(msg)
}

// scrollbarDragging reports whether the pointer is holding a thumb.
func (m *UI) scrollbarDragging() bool {
	return m.chat.ScrollbarDragging() || m.sidebarScrollbarDragging()
}

// handleScrollbarMouseUp releases any held scrollbar thumb.
func (m *UI) handleScrollbarMouseUp() {
	m.chat.HandleScrollbarMouseUp()
	m.handleSidebarScrollbarMouseUp()
}
