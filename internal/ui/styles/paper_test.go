package styles

import (
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
	"image/color"
	"math"
	"testing"
)

func colorLuminance(c color.Color) float64 {
	r, g, b, _ := c.RGBA()
	linear := func(value uint32) float64 {
		channel := float64(value) / 65535
		if channel <= 0.04045 {
			return channel / 12.92
		}
		return math.Pow((channel+0.055)/1.055, 2.4)
	}
	return 0.2126*linear(r) + 0.7152*linear(g) + 0.0722*linear(b)
}

func TestPaperColorsRemainReadable(t *testing.T) {
	theme := ThemeForName("paper")
	backgrounds := []color.Color{theme.Background, theme.Dialog.ContentPanel.GetBackground()}
	colors := append([]color.Color{}, theme.ANSI[:]...)
	for _, value := range []*string{theme.Markdown.Link.Color, theme.Markdown.LinkText.Color, theme.Markdown.CodeBlock.Chroma.NameClass.Color, theme.Markdown.CodeBlock.Chroma.NameFunction.Color, theme.Markdown.CodeBlock.Chroma.LiteralString.Color} {
		require.NotNil(t, value)
		colors = append(colors, lipgloss.Color(*value))
	}
	for _, foreground := range colors {
		for _, background := range backgrounds {
			a, b := colorLuminance(foreground), colorLuminance(background)
			ratio := (math.Max(a, b) + 0.05) / (math.Min(a, b) + 0.05)
			require.GreaterOrEqual(t, ratio, 4.5, "foreground %v background %v", foreground, background)
		}
	}
}
