package dialog

import (
	"image"

	"github.com/SpherePrime/CLI/internal/ui/common"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

// ScrollbarZone connects a scrollbar column a dialog painted to pointer input,
// so pressing and dragging the thumb scrolls the dialog content. A dialog
// calls [ScrollbarZone.Painted] from Draw and routes its mouse messages
// through [ScrollbarZone.HandleMsg].
type ScrollbarZone struct {
	track common.ScrollbarTrack
	drag  common.ScrollbarDrag
}

// Painted records the scrollbar that joinScrollbar appended to content, where
// the joined block was drawn on screen, and the scroll state it mirrors.
func (z *ScrollbarZone) Painted(bodyOrigin image.Point, content string, height, contentSize, viewportSize, offset int) {
	z.PaintedColumn(image.Pt(bodyOrigin.X+lipgloss.Width(content), bodyOrigin.Y), height, contentSize, viewportSize, offset)
}

// PaintedColumn records a scrollbar drawn straight into its own screen column,
// as inline components do, and the scroll state it mirrors.
func (z *ScrollbarZone) PaintedColumn(column image.Point, height, contentSize, viewportSize, offset int) {
	z.track = common.ScrollbarTrack{
		X:            column.X,
		MinY:         column.Y,
		Height:       height,
		ContentSize:  contentSize,
		ViewportSize: viewportSize,
		Offset:       offset,
	}
	if !z.track.Visible() {
		z.track = common.ScrollbarTrack{}
	}
}

// Dragging reports whether the pointer is holding the thumb.
func (z *ScrollbarZone) Dragging() bool {
	return z.drag.Active()
}

// Clear forgets the scrollbar geometry, used when a frame paints no
// scrollbar so stale presses cannot grab a thumb that is gone.
func (z *ScrollbarZone) Clear() {
	z.track = common.ScrollbarTrack{}
	z.drag.End()
}

// HandleMsg feeds a dialog mouse message to the zone. setOffset scrolls the
// content so the thumb lines up with the pointer. It reports whether the
// message was consumed, letting the dialog skip handling a press that missed
// the scrollbar.
func (z *ScrollbarZone) HandleMsg(msg tea.Msg, setOffset func(offset int)) bool {
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		if msg.Button != tea.MouseLeft {
			return false
		}
		offset, started := z.drag.Begin(z.track, msg.X, msg.Y)
		if !started {
			return false
		}
		setOffset(offset)
		return true
	case tea.MouseMotionMsg:
		offset, ok := z.drag.Move(msg.Y)
		if !ok {
			return false
		}
		setOffset(offset)
		return true
	case tea.MouseReleaseMsg:
		if !z.drag.Active() {
			return false
		}
		z.drag.End()
		return true
	}
	return false
}
