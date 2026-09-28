// Package logo renders a Prime wordmark in a stylized way.
package logo

import (
	"fmt"
	"image/color"
	"math/rand/v2"
	"strings"

	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

// letterform represents a letterform. It can be stretched horizontally by
// a given amount via the boolean argument.
type letterform func(bool) string

const diag = `╱`

// shimmerRowSkew is how many ramp entries each extra row of the wordmark is
// offset by. It tilts the traveling spectrum so the block reads as one
// diagonal wave rather than rows shifting in lockstep.
const shimmerRowSkew = 5

// Opts are the options for rendering the Prime title art.
type Opts struct {
	FieldColor   color.Color // diagonal lines
	TitleColorA  color.Color // left gradient ramp point
	TitleColorB  color.Color // right gradient ramp point
	LabelColor   color.Color // Prime™ text color
	VersionColor color.Color // version text color
	Width        int         // width of the rendered logo, used for truncation
	Hyper        bool        // whether it is Prime or Hyperprime

	// Ramp, when set, replaces the fixed title gradient with a closed
	// iridescent ramp sampled per cell, so the wordmark shimmers. FieldRamp is
	// the same spectrum softened for the large diagonal fields.
	Ramp      []color.Color
	FieldRamp []color.Color

	// Phase is the shimmer step this frame was rendered at.
	Phase int

	// When true, stretch a random letterform on each render. Has no effect in
	// compact mode. Mainly for testing. In production you will want to cache
	// the stretched letterform to keep the logo from jittering on resize.
	Unstable bool
}

// shimmer reports whether the logo walks an iridescent ramp instead of a
// fixed two-point gradient.
func (o Opts) shimmer() bool { return len(o.Ramp) > 0 }

// cellIndex returns the ramp index at the given cell of the given row.
func (o Opts) cellIndex(cell, row int) int {
	return cell*styles.ShimmerCellStep +
		row*shimmerRowSkew +
		o.Phase*styles.ShimmerFrameStep
}

// paintTitle renders a row of the big wordmark.
func (o Opts) paintTitle(base lipgloss.Style, row string, rowIndex int) string {
	if o.shimmer() {
		return styles.ApplyForegroundCycle(base, row, o.Ramp, o.cellIndex(0, rowIndex), styles.ShimmerCellStep, false)
	}
	return styles.ApplyForegroundGrad(base, row, o.TitleColorA, o.TitleColorB)
}

// paintName renders a bold wordmark that is not part of the big title art,
// such as the one-line "PRIME" used in narrow layouts.
func (o Opts) paintName(base lipgloss.Style, text string, row int, from, to color.Color) string {
	if o.shimmer() {
		return styles.ApplyForegroundCycle(base, text, o.Ramp, o.cellIndex(0, row), styles.ShimmerCellStep, true)
	}
	return styles.ApplyBoldForegroundGrad(base, text, from, to)
}

// paintText renders label, version, or field glyphs.
func (o Opts) paintText(ramp []color.Color, fallback color.Color, text string, cell, row int) string {
	if o.shimmer() {
		if len(ramp) == 0 {
			ramp = o.Ramp
		}
		return styles.ApplyForegroundCycle(lipgloss.NewStyle(), text, ramp, o.cellIndex(cell, row), styles.ShimmerCellStep, false)
	}
	return lipgloss.NewStyle().Foreground(fallback).Render(text)
}

// Render renders the Prime logo. Set the argument to true to render the narrow
// version, intended for use in a sidebar.
//
// The compact argument determines whether it renders compact for the sidebar
// or wider for the main pane.
func Render(base lipgloss.Style, version string, compact bool, o Opts) string {
	label := "Prime™"
	if !o.Hyper {
		label = " " + label
	}

	// Title.
	const spacing = 1
	var hyperLetterforms []letterform
	if o.Hyper {
		hyperLetterforms = []letterform{
			LetterH,
			LetterYAlt,
			LetterP,
			LetterE,
			LetterR,
		}
	}
	primeLetterforms := []letterform{
		LetterP,
		LetterR,
		LetterI,
		LetterM,
		LetterE,
	}
	if o.Hyper && !compact {
		primeLetterforms = append(hyperLetterforms, primeLetterforms...)
	}

	stretchIndex := -1 // -1 means no stretching.
	if !compact && !o.Unstable {
		// Always stretch the same letterform, which is picked once at random.
		stretchIndex = cachedRandN(len(primeLetterforms))
	} else if !compact && o.Unstable {
		// Stretch a random letterform on every render.
		stretchIndex = rand.IntN(len(primeLetterforms))
	}
	prime := renderWord(spacing, stretchIndex, primeLetterforms...)
	if o.Hyper && compact {
		prime = renderWord(spacing, stretchIndex, hyperLetterforms...) + "\n" + prime
	}
	primeWidth := lipgloss.Width(prime)
	b := new(strings.Builder)
	for rowIndex, r := range strings.Split(prime, "\n") {
		fmt.Fprintln(b, o.paintTitle(base, r, rowIndex))
	}
	prime = b.String()

	// Prime and version.
	metaRowGap := 1
	maxVersionWidth := primeWidth - lipgloss.Width(label) - metaRowGap
	version = ansi.Truncate(version, maxVersionWidth, "…") // truncate version if too long.
	if o.Hyper && compact {
		version += " "
	}
	gap := max(0, primeWidth-lipgloss.Width(label)-lipgloss.Width(version))
	labelCells := lipgloss.Width(label)
	metaRow := o.paintText(o.Ramp, o.LabelColor, label, 0, 0) +
		strings.Repeat(" ", gap) +
		o.paintText(o.Ramp, o.VersionColor, version, labelCells+gap, 0)

	// Join the meta row and big Prime title.
	prime = strings.TrimSpace(metaRow + "\n" + prime)

	// Narrow version. If this is Hyperprime, this is also a stacked version.
	// One field row above the title and one below keeps the block symmetric,
	// and both match the title width so the right edge lines up with the
	// version, which is padded out to the same width.
	if compact {
		blank := lipgloss.Height(prime)
		field := o.paintText(o.FieldRamp, o.FieldColor, strings.Repeat(diag, primeWidth), 0, 0)
		bottom := o.paintText(o.FieldRamp, o.FieldColor, strings.Repeat(diag, primeWidth), 0, blank+1)
		return strings.Join([]string{field, prime, bottom}, "\n")
	}

	fieldHeight := lipgloss.Height(prime)

	// Left field.
	const leftWidth = 6
	leftField := new(strings.Builder)
	for i := range fieldHeight {
		row := o.paintText(o.FieldRamp, o.FieldColor, strings.Repeat(diag, leftWidth), 0, i)
		fmt.Fprintln(leftField, row)
	}

	// Right field.
	rightWidth := max(15, o.Width-primeWidth-leftWidth-2) // 2 for the gap.
	const stepDownAt = 0
	rightField := new(strings.Builder)
	for i := range fieldHeight {
		width := rightWidth
		if i >= stepDownAt {
			width = rightWidth - (i - stepDownAt)
		}
		fmt.Fprint(rightField, o.paintText(o.FieldRamp, o.FieldColor, strings.Repeat(diag, width), leftWidth+1, i), "\n")
	}

	// Return the wide version.
	const hGap = " "
	logo := lipgloss.JoinHorizontal(lipgloss.Top, leftField.String(), hGap, prime, hGap, rightField.String())
	if o.Width > 0 {
		// Truncate the logo to the specified width.
		lines := strings.Split(logo, "\n")
		for i, line := range lines {
			lines[i] = ansi.Truncate(line, o.Width, "")
		}
		logo = strings.Join(lines, "\n")
	}
	return logo
}

// SmallRender renders a smaller version of the Prime logo, suitable for
// smaller windows or sidebar usage.
func SmallRender(t *styles.Styles, width int, o Opts) string {
	name := "Prime"
	if o.Hyper {
		name = "HYPERPRIME"
	}
	label := "Prime™"
	labelText := o.paintText(o.Ramp, o.LabelColor, label, 0, 0)
	if !o.shimmer() {
		labelText = t.Logo.SmallLabel.Render(label)
	}
	nameText := o.paintName(t.Logo.GradCanvas, name, 1, t.Logo.SmallGradFromColor, t.Logo.SmallGradToColor)
	title := fmt.Sprintf("%s %s", labelText, nameText)

	remainingWidth := width - lipgloss.Width(title) - 1 // 1 for the space after the name
	if remainingWidth > 0 {
		diagonals := strings.Repeat(diag, remainingWidth)
		diagonalText := o.paintText(o.FieldRamp, o.FieldColor, diagonals, lipgloss.Width(title)+1, 2)
		if !o.shimmer() {
			diagonalText = t.Logo.SmallDiagonals.Render(diagonals)
		}
		title = fmt.Sprintf("%s %s", title, diagonalText)
	}
	return title
}
