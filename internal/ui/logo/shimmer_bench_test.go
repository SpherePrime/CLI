package logo

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/ui/styles"
)

func benchmarkOpts(shimmer bool) Opts {
	t := testTheme().Logo
	o := Opts{
		FieldColor:   t.FieldColor,
		TitleColorA:  t.TitleColorA,
		TitleColorB:  t.TitleColorB,
		LabelColor:   t.LabelColor,
		VersionColor: t.VersionColor,
		Width:        140,
	}
	if shimmer {
		theme := testTheme()
		o.Ramp = theme.IridescentRamp
		o.FieldRamp = theme.IridescentRampMuted
	}
	return o
}

func BenchmarkRenderStatic(b *testing.B) {
	theme := testTheme()
	opts := benchmarkOpts(false)
	for b.Loop() {
		Render(theme.Logo.GradCanvas, "v0.4.30", false, opts)
	}
}

func BenchmarkRenderShimmer(b *testing.B) {
	theme := testTheme()
	opts := benchmarkOpts(true)
	opts.Phase = 1
	for b.Loop() {
		Render(theme.Logo.GradCanvas, "v0.4.30", false, opts)
	}
}

func BenchmarkShimmerRampBuild(b *testing.B) {
	theme := testTheme()
	for b.Loop() {
		styles.CyclicRamp(styles.ShimmerRampSize, theme.Iridescence)
	}
}
