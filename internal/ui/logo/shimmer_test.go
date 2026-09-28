package logo

import (
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func shimmerOpts() Opts {
	t := testTheme().Logo
	return Opts{
		FieldColor:   t.FieldColor,
		TitleColorA:  t.TitleColorA,
		TitleColorB:  t.TitleColorB,
		LabelColor:   t.LabelColor,
		VersionColor: t.VersionColor,
		Ramp:         testTheme().IridescentRamp,
		FieldRamp:    testTheme().IridescentRampMuted,
	}
}

func TestShimmerPaintsTheSameText(t *testing.T) {
	t.Parallel()

	const version = "v1.2.3"
	plain := ansi.Strip(Render(testTheme().Logo.GradCanvas, version, true, testOpts()))
	shimmering := ansi.Strip(Render(testTheme().Logo.GradCanvas, version, true, shimmerOpts()))
	require.Equal(t, plain, shimmering, "the ramp may only change colors")
}

func TestShimmerMovesAcrossPhases(t *testing.T) {
	t.Parallel()

	first := shimmerOpts()
	second := shimmerOpts()
	second.Phase = 3

	a := Render(testTheme().Logo.GradCanvas, "v1.2.3", true, first)
	b := Render(testTheme().Logo.GradCanvas, "v1.2.3", true, second)
	require.NotEqual(t, a, b, "the spectrum must travel between phases")

	require.Equal(t,
		ansi.Strip(Render(testTheme().Logo.GradCanvas, "v1.2.3", true, first)),
		ansi.Strip(Render(testTheme().Logo.GradCanvas, "v1.2.3", true, second)),
		"only the colors may change")
}

func TestShimmerLoopReturnsToItsStart(t *testing.T) {
	t.Parallel()

	opts := shimmerOpts()
	start := Render(testTheme().Logo.GradCanvas, "v1.2.3", true, opts)
	opts.Phase = styles.ShimmerFrames
	require.Equal(t, start, Render(testTheme().Logo.GradCanvas, "v1.2.3", true, opts))
}

func TestSmallRenderShimmers(t *testing.T) {
	t.Parallel()

	theme := testTheme()
	plain := ansi.Strip(SmallRender(&theme, 60, testOpts()))
	shimmering := ansi.Strip(SmallRender(&theme, 60, shimmerOpts()))
	require.Equal(t, plain, shimmering, "the narrow wordmark keeps its text when shimmering")
	require.NotEqual(t, plain, "", "the narrow wordmark must still render")
}

func TestFramesRenderEachStepOnce(t *testing.T) {
	t.Parallel()

	var f Frames
	renders := 0
	render := func(phase int) string {
		renders++
		return strings.Repeat("x", phase+1)
	}

	require.Equal(t, "xxx", f.Frame("wide", 2, render))
	require.Equal(t, "xxx", f.Frame("wide", 2, render), "a repeated step must be served from cache")
	require.Equal(t, 1, renders)

	require.Equal(t, "x", f.Frame("wide", 0, render))
	require.Equal(t, 2, renders, "a new step must render once")

	require.Equal(t, "xxx", f.Frame("narrow", 2, render))
	require.Equal(t, 3, renders, "a new layout must drop the cached frames")
}
