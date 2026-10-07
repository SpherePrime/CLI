package styles

import (
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"image/color"
	"strings"
)

func WithBackground(text string, background color.Color) string {
	if background == nil || text == "" {
		return text
	}
	lines := strings.Split(text, "\n")
	width := 0
	for _, line := range lines {
		width = max(width, ansi.StringWidth(line))
	}
	if width == 0 {
		return text
	}
	buffer := uv.NewScreenBuffer(width, len(lines))
	uv.NewStyledString(text).Draw(buffer, buffer.Bounds())
	for y := 0; y < len(lines); y++ {
		for x := 0; x < width; x++ {
			cell := buffer.CellAt(x, y)
			if cell.Style.Bg == nil {
				updated := *cell
				updated.Style.Bg = background
				buffer.SetCell(x, y, &updated)
			}
		}
	}
	return buffer.Render()
}
