package main

// This is an example for testing logo treatments. Do not remove.

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/SpherePrime/CLI/internal/ui/logo"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/term"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

func main() {
	shimmer := flag.Bool("shimmer", false, "animate the iridescent ramp across the logo")
	flag.Parse()

	w, _, err := term.GetSize(os.Stdout.Fd())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not get terminal size: %s", err)
	}

	s := styles.ColorTonePantera()
	opts := logo.Opts{
		FieldColor:   s.Logo.FieldColor,
		TitleColorA:  s.Logo.TitleColorA,
		TitleColorB:  s.Logo.TitleColorB,
		LabelColor:   s.Logo.LabelColor,
		VersionColor: s.Logo.VersionColor,
		Ramp:         s.IridescentRamp,
		FieldRamp:    s.IridescentRampMuted,
		Width:        w,
		Unstable:     !*shimmer,
	}

	renderCompact := func(hyper bool) string {
		opts.Hyper = hyper
		return logo.Render(s.Logo.GradCanvas, "v1.0.0", true, opts)
	}

	renderWide := func(hyper bool) string {
		opts.Hyper = hyper
		return logo.Render(s.Logo.GradCanvas, "v1.0.0", false, opts)
	}

	if !*shimmer {
		lipgloss.Println(
			lipgloss.JoinHorizontal(lipgloss.Top, renderCompact(false), "  ", renderCompact(true)),
		)
		for i := range 6 {
			lipgloss.Println(renderWide(i > 0))
		}
		return
	}

	// One loop of the ramp, then stop, so the animation can be inspected
	// frame by frame without a terminal that can keep up.
	for range styles.ShimmerFrames {
		opts.Phase++
		frame := lipgloss.JoinHorizontal(
			lipgloss.Top,
			renderCompact(false),
			"  ",
			renderCompact(true),
		) + "\n" + renderWide(true) + "\n" + renderWide(false)
		os.Stdout.WriteString(ansi.EraseEntireScreen + ansi.CursorPosition(1, 1) + frame)
		time.Sleep(time.Second / 12)
	}
}
