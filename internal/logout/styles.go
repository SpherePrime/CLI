package logout

import (
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/exp/colortone"
)

// buttonPadding is the horizontal padding inside a button.
const buttonPadding = 5

var (
	questionStyle = lipgloss.NewStyle().
			Bold(true)

	buttonFocusedStyle = lipgloss.NewStyle().
				Foreground(colortone.Sash).
				Background(colortone.Blush)
	buttonBlurredStyle = lipgloss.NewStyle().
				Foreground(colortone.Soda).
				Background(colortone.BBQ)

	choiceSelectedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(colortone.Blush)
)

// renderButton renders a selectable button with an underlined accelerator
// key (its first letter).
func renderButton(text string, selected bool) string {
	base := buttonBlurredStyle
	if selected {
		base = buttonFocusedStyle
	}
	rendered := base.Padding(0, buttonPadding).Render(text)
	if text != "" {
		// Underline the accelerator key (the first letter). The range style
		// must not carry padding, as StyleRanges re-renders the matched text.
		rendered = lipgloss.StyleRanges(rendered,
			lipgloss.NewRange(buttonPadding, buttonPadding+1, base.Underline(true)))
	}
	return rendered
}
