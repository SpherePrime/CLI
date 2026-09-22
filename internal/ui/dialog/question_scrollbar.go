package dialog

import (
	"github.com/SpherePrime/CLI/internal/ui/common"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
)

// HandleScrollbarMouse drags the choice list scrollbar thumb. Satisfies
// common.ScrollbarDraggable so the UI can route pointer events here.
func (c *choiceList) HandleScrollbarMouse(msg tea.Msg) bool {
	return c.scrollbarZone.HandleMsg(msg, c.scrollThumbTo)
}

// scrollThumbTo scrolls the choice list to an absolute line offset. Pointer
// scrolling behaves like the wheel: bounds are enforced, but the cursor is not
// snapped back into view until the keyboard is used again.
func (c *choiceList) scrollThumbTo(offset int) {
	c.scrollOffset = offset
	c.wheelActive = true
	c.clampToBounds(c.lastLines, c.lastViewport)
}

// HandleScrollbarMouse drags the confirm tab scrollbar thumb.
func (c *ConfirmComponent) HandleScrollbarMouse(msg tea.Msg) bool {
	return c.scrollbarZone.HandleMsg(msg, func(offset int) {
		c.scrollOffset = offset
	})
}

// HandleScrollbarMouse drags the free-text scrollbar thumb.
func (d *FreeText) HandleScrollbarMouse(msg tea.Msg) bool {
	return d.scrollbarZone.HandleMsg(msg, func(offset int) {
		d.scrollOffset = offset
		d.wheelActive = true
	})
}

// HandleScrollbarMouse forwards a pointer event on the scrollbar to the active
// question, which painted it. Satisfies common.ScrollbarDraggable.
func (f *QuestionForm) HandleScrollbarMouse(msg tea.Msg) bool {
	if f.isConfirmTab() && f.confirmComp != nil {
		return f.confirmComp.HandleScrollbarMouse(msg)
	}
	if f.activeIdx < len(f.questions) {
		if draggable, ok := f.questions[f.activeIdx].(common.ScrollbarDraggable); ok {
			return draggable.HandleScrollbarMouse(msg)
		}
	}
	return false
}
