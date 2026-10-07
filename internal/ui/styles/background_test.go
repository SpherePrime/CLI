package styles

import (
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"image/color"
	"testing"
)

func TestBackgroundKeepsTextAndExplicitColors(t *testing.T) {
	background := color.RGBA{R: 30, G: 30, B: 46, A: 255}
	input := "label \x1b[0mplain \x1b[48;2;1;2;3mselected\x1b[m\nnext"
	output := WithBackground(input, background)
	require.Contains(t, ansi.Strip(output), "label plain selected")
	require.Contains(t, ansi.Strip(output), "next")
	buffer := uv.NewScreenBuffer(20, 2)
	uv.NewStyledString(output).Draw(buffer, buffer.Bounds())
	require.Equal(t, background, buffer.CellAt(6, 0).Style.Bg)
	r, g, b, _ := buffer.CellAt(12, 0).Style.Bg.RGBA()
	require.Equal(t, uint32(257), r)
	require.Equal(t, uint32(514), g)
	require.Equal(t, uint32(771), b)
	require.Equal(t, 2, len(buffer.Lines))
}
