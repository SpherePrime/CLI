package model

import (
	"image"

	tea "github.com/dwertyfa288/CLI/vendordeps/bubbletea/v2"
	"github.com/dwertyfa288/CLI/internal/ui/common"
	uv "github.com/dwertyfa288/CLI/vendordeps/dwertyfa288/ultraviolet"
)

// drawScrollbar paints the chat scrollbar in the reserved column of area and
// records where it landed so the pointer can grab the thumb.
func (m *Chat) drawScrollbar(scr uv.Screen, area image.Rectangle, height, width int) {
	contentSize := m.list.TotalHeight() - 1
	offset := m.list.Offset()

	scrollbar := common.Scrollbar(m.com.Styles, height, contentSize, height, offset)
	if scrollbar == "" {
		m.scrollbarTrack = common.ScrollbarTrack{}
		return
	}

	scrollbarArea := image.Rectangle{
		Min: image.Point{X: area.Max.X - width, Y: area.Min.Y},
		Max: image.Point{X: area.Max.X, Y: area.Max.Y},
	}
	uv.NewStyledString(scrollbar).Draw(scr, scrollbarArea)

	m.scrollbarTrack = common.ScrollbarTrack{
		X:            scrollbarArea.Min.X,
		MinY:         scrollbarArea.Min.Y,
		Height:       height,
		ContentSize:  contentSize,
		ViewportSize: height,
		Offset:       offset,
	}
}

// ScrollbarDragging reports whether the pointer is holding the chat scrollbar.
func (m *Chat) ScrollbarDragging() bool {
	return m.scrollbarDrag.Active()
}

// HandleScrollbarMouseDown grabs the chat scrollbar thumb when the press at
// the absolute cell x,y lands on the painted track.
func (m *Chat) HandleScrollbarMouseDown(x, y int) (bool, tea.Cmd) {
	offset, started := m.scrollbarDrag.Begin(m.scrollbarTrack, x, y)
	if !started {
		return false, nil
	}
	return true, m.ScrollBy(offset - m.list.Offset())
}

// HandleScrollbarMouseDrag scrolls the chat so the thumb follows the pointer
// at the absolute screen row y.
func (m *Chat) HandleScrollbarMouseDrag(y int) (bool, tea.Cmd) {
	offset, ok := m.scrollbarDrag.Move(y)
	if !ok {
		return false, nil
	}
	return true, m.ScrollBy(offset - m.list.Offset())
}

// HandleScrollbarMouseUp releases the chat scrollbar thumb.
func (m *Chat) HandleScrollbarMouseUp() {
	m.scrollbarDrag.End()
}
