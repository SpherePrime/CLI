package styles

import (
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/exp/colortone"
)

// ThemeKeyForProvider returns a stable identifier for the theme
// associated with the given provider ID. Providers that share a theme
// yield the same key, so callers can cheaply detect when switching
// providers would not actually change the active theme and skip the
// expensive style rebuild. This is the single source of truth for the
// provider-to-theme mapping; [ThemeForProvider] builds on it.
func ThemeKeyForProvider(providerID string) string {
	switch providerID {
	case "hyper":
		return "hyper"
	default:
		return "default"
	}
}

// ThemeForProvider returns the Styles associated with the given provider
// ID. Unknown or empty provider IDs yield the default ColorTone Pantera
// theme.
func ThemeForProvider(providerID string) Styles {
	switch ThemeKeyForProvider(providerID) {
	case "hyper":
		return HyperprimeObsidiana()
	default:
		return ColorTonePantera()
	}
}

// ColorTonePantera returns the ColorTone dark theme. It's the default style
// for the UI.
func ColorTonePantera() Styles {
	s := quickStyle(quickStyleOpts{
		primary:   colortone.Charple,
		secondary: colortone.Dolly,
		accent:    colortone.Bok,
		keyword:   colortone.Blush,

		fgBase:       colortone.Sash,
		fgMoreSubtle: colortone.Squid,
		fgSubtle:     colortone.Smoke,
		fgMostSubtle: colortone.Oyster,

		onPrimary: colortone.Butter,

		bgBase:         colortone.Pepper,
		bgLeastVisible: colortone.BBQ,
		bgLessVisible:  colortone.Char,
		bgMostVisible:  colortone.Iron,

		separator: colortone.Char,

		destructive:       colortone.Coral,
		error:             colortone.Sriracha,
		warningSubtle:     colortone.Zest,
		warning:           colortone.Mustard,
		attention:         colortone.Tang,
		busy:              colortone.Citron,
		info:              colortone.Malibu,
		infoMoreSubtle:    colortone.Sardine,
		infoMostSubtle:    colortone.Damson,
		success:           colortone.Julep,
		successMoreSubtle: colortone.Bok,
		successMostSubtle: colortone.Guac,
		yolo:              colortone.Zest,
		plan:              colortone.Charple,
		planMoreSubtle:    colortone.Hazy,
		// ANSI 16-color palette for remapping raw terminal output
		// (e.g. bang-mode shell commands) onto legible ColorTone colors.
		ansiBlack:   colortone.BBQ,
		ansiRed:     colortone.Coral,
		ansiGreen:   colortone.Guac,
		ansiYellow:  colortone.Mustard,
		ansiBlue:    colortone.Charple,
		ansiMagenta: colortone.Dolly,
		ansiCyan:    colortone.Malibu,
		ansiWhite:   colortone.Smoke,

		ansiBrightBlack:   colortone.Iron,
		ansiBrightRed:     colortone.Tuna,
		ansiBrightGreen:   colortone.Julep,
		ansiBrightYellow:  colortone.Zest,
		ansiBrightBlue:    colortone.Guppy,
		ansiBrightMagenta: colortone.Blush,
		ansiBrightCyan:    colortone.Sardine,
		ansiBrightWhite:   colortone.Salt,
	})

	// Bang ! prompt overrides - use Salt/Hazy/Larple colors.
	s.Editor.PromptBangIconFocused = s.Editor.PromptBangIconFocused.
		Foreground(colortone.Salt).
		Background(colortone.Hazy)
	s.Editor.PromptBangDotsFocused = s.Editor.PromptBangDotsFocused.
		Foreground(colortone.Hazy)
	s.Editor.PromptBangDotsBlurred = s.Editor.PromptBangDotsBlurred.
		Foreground(colortone.Larple)

	// Shell bar/prompt overrides - use Charple/Iron/Hazy colors.
	s.Messages.ShellBarFocused = s.Messages.ShellBarFocused.
		BorderForeground(colortone.Charple)
	s.Messages.ShellBarBlurred = s.Messages.ShellBarBlurred.
		BorderForeground(colortone.Iron)
	s.Messages.ShellPrompt = s.Messages.ShellPrompt.
		Foreground(colortone.Hazy)
	s.Messages.ShellPromptBlurred = s.Messages.ShellPromptBlurred.
		Foreground(colortone.Hazy)

	// The ◆ hypercredit symbol inside subdued text (e.g. savings
	// suffixes) uses Mochi so it stays visible against its surroundings.
	s.Messages.SubduedHypercreditIcon = s.Messages.SubduedHypercreditIcon.
		Foreground(colortone.Violet)

	return s
}

// HyperprimeObsidiana returns the Hyperprime dark theme.
func HyperprimeObsidiana() Styles {
	return ColorTonePantera()
}
