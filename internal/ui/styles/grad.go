package styles

import (
	"fmt"
	"image/color"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/rivo/uniseg"
)

// ForegroundGrad returns a slice of strings representing the input string
// rendered with a horizontal gradient foreground from color1 to color2. Each
// string in the returned slice corresponds to a grapheme cluster in the input
// string. If bold is true, the rendered strings will be bolded.
func ForegroundGrad(base lipgloss.Style, input string, bold bool, color1, color2 color.Color) []string {
	if input == "" {
		return []string{""}
	}
	if len(input) == 1 {
		style := base.Foreground(color1)
		if bold {
			style.Bold(true)
		}
		return []string{style.Render(input)}
	}
	clusters := Graphemes(input)

	ramp := lipgloss.Blend1D(len(clusters), color1, color2)
	for i, c := range ramp {
		style := base.Foreground(c)
		if bold {
			style.Bold(true)
		}
		clusters[i] = style.Render(clusters[i])
	}
	return clusters
}

// ApplyForegroundGrad renders a given string with a horizontal gradient
// foreground.
func ApplyForegroundGrad(base lipgloss.Style, input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}
	var o strings.Builder
	clusters := ForegroundGrad(base, input, false, color1, color2)
	for _, c := range clusters {
		fmt.Fprint(&o, c)
	}
	return o.String()
}

// ApplyBoldForegroundGrad renders a given string with a horizontal gradient
// foreground.
func ApplyBoldForegroundGrad(base lipgloss.Style, input string, color1, color2 color.Color) string {
	if input == "" {
		return ""
	}
	var o strings.Builder
	clusters := ForegroundGrad(base, input, true, color1, color2)
	for _, c := range clusters {
		fmt.Fprint(&o, c)
	}
	return o.String()
}

// Graphemes splits input into grapheme clusters, one entry per rendered cell.
func Graphemes(input string) []string {
	if input == "" {
		return nil
	}
	clusters := make([]string, 0, len(input))
	gr := uniseg.NewGraphemes(input)
	for gr.Next() {
		clusters = append(clusters, gr.Str())
	}
	return clusters
}

// ForegroundCycle returns one rendered string per grapheme cluster of input,
// each colored from a closed ramp starting at rampIndex and advancing step per
// cell. Callers that need to trim or reorder cells (a gradient of icons that
// may be truncated) use this; ApplyForegroundCycle joins it into one string.
func ForegroundCycle(base lipgloss.Style, input string, ramp []color.Color, rampIndex, step int, bold bool) []string {
	clusters := Graphemes(input)
	if len(ramp) == 0 {
		return clusters
	}
	for i, cluster := range clusters {
		style := base.Foreground(ShimmerColor(ramp, rampIndex+i*step))
		if bold {
			style.Bold(true)
		}
		clusters[i] = style.Render(cluster)
	}
	return clusters
}

// ApplyForegroundCycle renders a string whose cells walk a closed color ramp,
// starting at startIndex and advancing step per cell. Because the ramp loops,
// moving startIndex makes the colors travel across the text instead of
// repainting it in place.
func ApplyForegroundCycle(base lipgloss.Style, input string, ramp []color.Color, startIndex, step int, bold bool) string {
	if input == "" || len(ramp) == 0 {
		return input
	}
	var o strings.Builder
	for _, cluster := range ForegroundCycle(base, input, ramp, startIndex, step, bold) {
		fmt.Fprint(&o, cluster)
	}
	return o.String()
}
