package styles

import (
	"image/color"

	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/exp/colortone"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/lucasb-eyer/go-colorful"
)

const (
	// ShimmerRampSize is how many samples the closed iridescent ramp is
	// blended into. One full turn of the spectrum spans this many ramp
	// entries, which is the period of every shimmering surface.
	ShimmerRampSize = 72

	// ShimmerCellStep is how far the ramp advances per terminal cell. With
	// ShimmerRampSize it puts a whole spectrum across 24 cells, wide enough
	// that neighboring glyphs stay in the same family of hues.
	ShimmerCellStep = 3

	// ShimmerFrameStep is how far the ramp advances per shimmer tick. A
	// smaller step makes the loop travel slowly.
	ShimmerFrameStep = 2
)

// ShimmerFrames is how many ticks it takes for the shimmer to return to its
// starting position.
const ShimmerFrames = ShimmerRampSize / ShimmerFrameStep

// ShiftShimmer moves a ramp index forward by whole frames.
func ShiftShimmer(index, frame int) int {
	return index + frame*ShimmerFrameStep
}

// Ramp returns the iridescent ramp a surface should paint with: the full
// spectrum, or the muted one used by large fields.
func (s *Styles) Ramp(muted bool) []color.Color {
	if muted && len(s.IridescentRampMuted) > 0 {
		return s.IridescentRampMuted
	}
	return s.IridescentRamp
}

// ApplyShimmer paints text with the theme's ramp at the given shimmer step, so
// the colors travel across it instead of sitting still. cell shifts where in
// the spectrum the first glyph starts, which lets surfaces next to each other
// stay in step. Themes without a ramp fall back to base.
func (s *Styles) ApplyShimmer(base lipgloss.Style, text string, step, cell int, muted, bold bool) string {
	ramp := s.Ramp(muted)
	if len(ramp) == 0 {
		return base.Render(text)
	}
	start := ShiftShimmer(cell*ShimmerCellStep, step)
	return ApplyForegroundCycle(base, text, ramp, start, ShimmerCellStep, bold)
}

// ShimmerCells is ApplyShimmer kept as one string per cell, for callers that
// trim or reorder the gradient after painting it.
func (s *Styles) ShimmerCells(base lipgloss.Style, text string, step, cell int, muted bool) []string {
	ramp := s.Ramp(muted)
	if len(ramp) == 0 {
		return []string{base.Render(text)}
	}
	start := ShiftShimmer(cell*ShimmerCellStep, step)
	return ForegroundCycle(base, text, ramp, start, ShimmerCellStep, false)
}

// ShimmerColor samples the closed ramp at any index, wrapping in both
// directions so callers can pass negative offsets freely.
func ShimmerColor(ramp []color.Color, index int) color.Color {
	if len(ramp) == 0 {
		return nil
	}
	i := index % len(ramp)
	if i < 0 {
		i += len(ramp)
	}
	return ramp[i]
}

// iridescentStops builds the closed color loop the shimmer walks through. The
// theme's brand colors anchor it so a future theme keeps its identity while
// the remaining stops supply the rest of the spectrum.
func iridescentStops(o quickStyleOpts) []color.Color {
	stops := []color.Color{
		brandOr(o.primary, colortone.Charple),
		colortone.Violet,
		brandOr(o.secondary, colortone.Dolly),
		colortone.Pony,
		colortone.Tuna,
		colortone.Coral,
		colortone.Tang,
		colortone.Zest,
		colortone.Guac,
		colortone.Turtle,
		brandOr(o.info, colortone.Malibu),
	}
	for i, c := range stops {
		if colorUnset(c) {
			stops[i] = colortone.Charple
		}
	}
	return stops
}

// brandOr falls back to a fixed spectrum color when a theme leaves a role
// unset.
func brandOr(c color.Color, fallback colortone.Key) color.Color {
	if colorUnset(c) {
		return fallback
	}
	return c
}

// colorUnset reports whether a theme role carries no usable color.
func colorUnset(c color.Color) bool {
	if c == nil {
		return true
	}
	_, _, _, a := c.RGBA()
	return a == 0
}

// CyclicRamp blends stops into a closed loop of size samples: the entry after
// the last one is the first again, so walking the ramp never hits a seam.
func CyclicRamp(size int, stops []color.Color) []color.Color {
	if size < 1 || len(stops) == 0 {
		return nil
	}
	if len(stops) == 1 {
		ramp := make([]color.Color, size)
		for i := range ramp {
			ramp[i] = stops[0]
		}
		return ramp
	}
	looped := make([]color.Color, 0, len(stops)+1)
	looped = append(looped, stops...)
	looped = append(looped, stops[0])
	// Blend1D includes both endpoints, so ask for one extra sample and drop
	// the duplicate at the end to keep the loop seamless.
	ramp := lipgloss.Blend1D(size+1, looped...)
	return ramp[:size]
}

// MutedRamp pulls every entry of a ramp toward a base color, softening the
// spectrum without rotating its hues. Large shimmering surfaces use it so the
// colors read as a slow sheen rather than a light show. The mix happens in
// linear RGB, which keeps every blended color inside the display gamut.
func MutedRamp(ramp []color.Color, toward color.Color, amount float64) []color.Color {
	if len(ramp) == 0 {
		return nil
	}
	target, ok := colorful.MakeColor(toward)
	if !ok {
		return ramp
	}
	muted := make([]color.Color, len(ramp))
	for i, c := range ramp {
		from, ok := colorful.MakeColor(c)
		if !ok {
			muted[i] = c
			continue
		}
		muted[i] = from.BlendLinearRgb(target, amount)
	}
	return muted
}
