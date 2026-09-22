package main

// This is an example for testing logo treatments. Do not remove.

import (
	"fmt"
	"os"

	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/internal/ui/logo"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/term"
)

func main() {
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
		Width:        w,
		Unstable:     true,
	}

	renderCompact := func(hyper bool) string {
		opts.Hyper = hyper
		return logo.Render(s.Logo.GradCanvas, "v1.0.0", true, opts)
	}

	renderWide := func(hyper bool) string {
		opts.Hyper = hyper
		return logo.Render(s.Logo.GradCanvas, "v1.0.0", false, opts)
	}

	lipgloss.Println(
		lipgloss.JoinHorizontal(lipgloss.Top, renderCompact(false), "  ", renderCompact(true)),
	)

	for i := range 6 {
		lipgloss.Println(renderWide(i > 0))
	}
}
