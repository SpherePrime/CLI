package anim

import (
	"regexp"
	"testing"

	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

var truecolor = regexp.MustCompile(`38;2;\d+;\d+;\d+`)

func distinctTruecolors(frames []string) map[string]int {
	seen := make(map[string]int)
	for _, frame := range frames {
		for _, c := range truecolor.FindAllString(frame, -1) {
			seen[c]++
		}
	}
	return seen
}

func renderLoop(t *testing.T, settings Settings, frames int) []string {
	t.Helper()
	a := New(settings)
	out := make([]string, 0, frames)
	for range frames {
		out = append(out, a.Render())
		a.Advance()
	}
	return out
}

func TestGradStopsWalkMoreThanTwoHues(t *testing.T) {
	t.Parallel()

	stops := styles.ThemeForProvider("").Iridescence
	require.NotEmpty(t, stops, "the theme must carry a spectrum for spinners to walk")

	twoColor := renderLoop(t, Settings{
		ID:          "two",
		Size:        6,
		GradColorA:  stops[0],
		GradColorB:  stops[2],
		CycleColors: true,
	}, 20)

	spectrum := renderLoop(t, Settings{
		ID:          "spectrum",
		Size:        6,
		GradColorA:  stops[0],
		GradColorB:  stops[2],
		GradStops:   stops,
		CycleColors: true,
	}, 20)

	require.Greater(t, len(distinctTruecolors(spectrum)), len(distinctTruecolors(twoColor)),
		"a busy spinner must shimmer through the spectrum, not sit in one hue")
}

func TestGradStopsRenderIsDeterministic(t *testing.T) {
	t.Parallel()

	stops := styles.ThemeForProvider("").Iridescence
	settings := Settings{ID: "same", Size: 6, GradColorA: stops[0], GradColorB: stops[2], GradStops: stops, CycleColors: true}
	require.Equal(t, renderLoop(t, settings, 12), renderLoop(t, settings, 12))
}

func TestSpinnerLoopReturnsToItsFirstFrame(t *testing.T) {
	t.Parallel()

	stops := styles.ThemeForProvider("").Iridescence
	settings := Settings{ID: "loop", Size: 6, GradColorA: stops[0], GradColorB: stops[2], GradStops: stops, CycleColors: true}

	a := New(settings)
	// Let the staggered entrance finish so every glyph is a cycling char.
	for range maxBirthSteps + 1 {
		a.Advance()
	}
	first := a.Render()

	// The color loop spans three spinner widths, which is exactly one pass of
	// the frame table, so the glyphs must land back on their starting colors.
	for range a.width * 3 {
		a.Advance()
	}
	require.Equal(t, truecolor.FindAllString(first, -1), truecolor.FindAllString(a.Render(), -1),
		"the spectrum must wrap without a jump")
}
