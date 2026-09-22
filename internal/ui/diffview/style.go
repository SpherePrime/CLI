package diffview

import (
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/exp/colortone"
)

// LineStyle defines the styles for a given line type in the diff view.
type LineStyle struct {
	LineNumber lipgloss.Style
	Symbol     lipgloss.Style
	Code       lipgloss.Style
}

// Style defines the overall style for the diff view, including styles for
// different line types such as divider, missing, equal, insert, and delete
// lines.
type Style struct {
	DividerLine LineStyle
	MissingLine LineStyle
	EqualLine   LineStyle
	InsertLine  LineStyle
	DeleteLine  LineStyle
	Filename    LineStyle
}

// DefaultLightStyle provides a default light theme style for the diff view.
func DefaultLightStyle() Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(colortone.Iron).
				Background(colortone.Thunder),
			Code: lipgloss.NewStyle().
				Foreground(colortone.Oyster).
				Background(colortone.Anchovy),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(colortone.Sash),
			Code: lipgloss.NewStyle().
				Background(colortone.Sash),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(colortone.Char).
				Background(colortone.Sash),
			Code: lipgloss.NewStyle().
				Foreground(colortone.Pepper).
				Background(colortone.Salt),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(colortone.Turtle).
				Background(lipgloss.Color("#c8e6c9")),
			Symbol: lipgloss.NewStyle().
				Foreground(colortone.Turtle).
				Background(lipgloss.Color("#e8f5e9")),
			Code: lipgloss.NewStyle().
				Foreground(colortone.Pepper).
				Background(lipgloss.Color("#e8f5e9")),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(colortone.Cherry).
				Background(lipgloss.Color("#ffcdd2")),
			Symbol: lipgloss.NewStyle().
				Foreground(colortone.Cherry).
				Background(lipgloss.Color("#ffebee")),
			Code: lipgloss.NewStyle().
				Foreground(colortone.Pepper).
				Background(lipgloss.Color("#ffebee")),
		},
		Filename: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(colortone.Iron).
				Background(colortone.Thunder),
			Code: lipgloss.NewStyle().
				Foreground(colortone.Iron).
				Background(colortone.Thunder),
		},
	}
}

// DefaultDarkStyle provides a default dark theme style for the diff view.
func DefaultDarkStyle() Style {
	return Style{
		DividerLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(colortone.Smoke).
				Background(colortone.Sapphire),
			Code: lipgloss.NewStyle().
				Foreground(colortone.Smoke).
				Background(colortone.Ox),
		},
		MissingLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Background(colortone.Char),
			Code: lipgloss.NewStyle().
				Background(colortone.Char),
		},
		EqualLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(colortone.Sash).
				Background(colortone.Char),
			Code: lipgloss.NewStyle().
				Foreground(colortone.Salt).
				Background(colortone.Pepper),
		},
		InsertLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(colortone.Turtle).
				Background(lipgloss.Color("#293229")),
			Symbol: lipgloss.NewStyle().
				Foreground(colortone.Turtle).
				Background(lipgloss.Color("#303a30")),
			Code: lipgloss.NewStyle().
				Foreground(colortone.Salt).
				Background(lipgloss.Color("#303a30")),
		},
		DeleteLine: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(colortone.Cherry).
				Background(lipgloss.Color("#332929")),
			Symbol: lipgloss.NewStyle().
				Foreground(colortone.Cherry).
				Background(lipgloss.Color("#3a3030")),
			Code: lipgloss.NewStyle().
				Foreground(colortone.Salt).
				Background(lipgloss.Color("#3a3030")),
		},
		Filename: LineStyle{
			LineNumber: lipgloss.NewStyle().
				Foreground(colortone.Smoke).
				Background(colortone.Sapphire),
			Code: lipgloss.NewStyle().
				Foreground(colortone.Smoke).
				Background(colortone.Sapphire),
		},
	}
}
