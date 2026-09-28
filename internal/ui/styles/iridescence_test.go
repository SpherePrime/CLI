package styles

import (
	"image/color"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func sameColor(a, b color.Color) bool {
	return a == b
}

func distinctCount(ramp []color.Color) int {
	seen := make(map[color.Color]struct{}, len(ramp))
	for _, c := range ramp {
		seen[c] = struct{}{}
	}
	return len(seen)
}

func TestThemeCarriesAnIridescentLoop(t *testing.T) {
	t.Parallel()

	s := ColorTonePantera()
	require.NotEmpty(t, s.Iridescence, "a theme must have a spectrum to shimmer through")
	require.Greater(t, len(s.Iridescence), 3, "three stops is not a spectrum")
	require.Len(t, s.IridescentRamp, ShimmerRampSize)
	require.Len(t, s.IridescentRampMuted, ShimmerRampSize)

	require.Greater(t, distinctCount(s.IridescentRamp), ShimmerRampSize/2,
		"the ramp must stay smooth instead of collapsing into a few hues")
	require.False(t, sameColor(s.IridescentRamp[0], s.IridescentRamp[ShimmerRampSize-1]),
		"the loop must not paint its starting color twice")
}

func TestShimmerColorWrapsBothWays(t *testing.T) {
	t.Parallel()

	ramp := []color.Color{
		color.RGBA{R: 1, A: 0xff},
		color.RGBA{R: 2, A: 0xff},
		color.RGBA{R: 3, A: 0xff},
	}
	require.Equal(t, ramp[0], ShimmerColor(ramp, 0))
	require.Equal(t, ramp[1], ShimmerColor(ramp, 4))
	require.Equal(t, ramp[2], ShimmerColor(ramp, -1), "walking backwards must stay in the loop")
	require.Nil(t, ShimmerColor(nil, 5))
}

func TestMutedRampKeepsTheLoopIntact(t *testing.T) {
	t.Parallel()

	s := ColorTonePantera()
	muted := MutedRamp(s.IridescentRamp, s.Iridescence[0], 0.5)
	require.Len(t, muted, len(s.IridescentRamp))
	require.Greater(t, distinctCount(muted), 1, "muting must not flatten the spectrum")
	require.NotEqual(t, s.IridescentRamp, muted)
}

func TestApplyForegroundCycleOnlyChangesColors(t *testing.T) {
	t.Parallel()

	s := ColorTonePantera()
	const text = "Prime™ 1.2.3"
	painted := ApplyForegroundCycle(lipgloss.NewStyle(), text, s.IridescentRamp, 0, ShimmerCellStep, false)
	require.Equal(t, text, ansi.Strip(painted))

	shifted := ApplyForegroundCycle(lipgloss.NewStyle(), text, s.IridescentRamp, ShimmerFrameStep, ShimmerCellStep, false)
	require.NotEqual(t, painted, shifted, "advancing the step must move the spectrum")
	require.Equal(t, text, ansi.Strip(shifted))

	// A theme without a ramp leaves the caller's own styling in charge.
	require.Equal(t, "abc", ApplyForegroundCycle(lipgloss.NewStyle(), "abc", nil, 0, ShimmerCellStep, false))
}

func TestShimmerCellsMatchApplyShimmer(t *testing.T) {
	t.Parallel()

	s := ColorTonePantera()
	cells := s.ShimmerCells(lipgloss.NewStyle(), "▶▶▶", 3, 0, false)
	require.Len(t, cells, 3, "one entry per cell so callers can trim the gradient")
	require.Equal(t,
		ApplyForegroundCycle(lipgloss.NewStyle(), "▶▶▶", s.IridescentRamp, ShiftShimmer(0, 3), ShimmerCellStep, false),
		strings.Join(cells, ""),
	)
}
