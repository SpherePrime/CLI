package styles

import "github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"

type themePalette struct{ background, surface, elevated, foreground, muted, primary, secondary, accent string }

var namedPalettes = map[string]themePalette{
	"graphite":   {"#151515", "#222222", "#383838", "#E8E3DC", "#AAA39A", "#E5A45E", "#D2CCC3", "#7AB8A1"},
	"paper":      {"#F4F1EA", "#E8E3D8", "#D4CCBD", "#282E38", "#555B65", "#365E8D", "#8B4C5D", "#8D612C"},
	"cobalt":     {"#0B1525", "#13233A", "#254666", "#D9ECFF", "#91AEC7", "#63B3FF", "#7DE0D5", "#E9B75D"},
	"hyper":      {"#100D16", "#211829", "#3B2A46", "#F5ECFA", "#B9A5C4", "#E879F9", "#A78BFA", "#F472B6"},
	"dracula":    {"#282A36", "#303241", "#44475A", "#F8F8F2", "#A5A7C4", "#BD93F9", "#FF79C6", "#50FA7B"},
	"nord":       {"#2E3440", "#343C49", "#434C5E", "#ECEFF4", "#A7B4C8", "#88C0D0", "#81A1C1", "#A3BE8C"},
	"tokyo":      {"#1A1B26", "#24283B", "#3B4261", "#C0CAF5", "#929CC5", "#7AA2F7", "#BB9AF7", "#7DCFFF"},
	"catppuccin": {"#1E1E2E", "#313244", "#45475A", "#CDD6F4", "#A6ADC8", "#CBA6F7", "#F5C2E7", "#A6E3A1"},
	"gruvbox":    {"#282828", "#32302F", "#504945", "#EBDBB2", "#BDAE93", "#FABD2F", "#FE8019", "#B8BB26"},
	"solarized":  {"#002B36", "#073642", "#15505B", "#EEE8D5", "#93A1A1", "#2AA198", "#268BD2", "#B58900"},
	"ocean":      {"#071C2C", "#0D2C40", "#19485D", "#D6F4FA", "#8EB8C7", "#38BDF8", "#818CF8", "#2DD4BF"},
	"cyberpunk":  {"#190C24", "#291638", "#4A275A", "#F8E8FF", "#BDA0CB", "#FF4FD8", "#22D3EE", "#FDE047"},
	"sunset":     {"#241923", "#352536", "#563C50", "#FCE7DF", "#C5A8B6", "#FB923C", "#F472B6", "#FCD34D"},
}

func namedPaletteTheme(name string) Styles {
	p := namedPalettes[name]
	c := lipgloss.Color
	errorColor, warningColor, successColor := "#FF8095", "#FCD34D", "#4ADE80"
	if name == "paper" {
		errorColor, warningColor, successColor = "#AE2942", "#805E17", "#20643C"
	}
	infoMuted, successMuted := p.elevated, successColor
	if name == "paper" {
		infoMuted, successMuted = p.muted, successColor
	}
	theme := pantheraBase(quickStyleOpts{
		primary: c(p.primary), secondary: c(p.secondary), accent: c(p.accent), keyword: c(p.secondary),
		fgBase: c(p.foreground), fgMoreSubtle: c(p.muted), fgSubtle: c(p.muted), fgMostSubtle: c(p.muted), onPrimary: c(p.background),
		bgBase: c(p.background), bgLeastVisible: c(p.surface), bgLessVisible: c(p.surface), bgMostVisible: c(p.elevated), separator: c(p.elevated),
		destructive: c(errorColor), error: c(errorColor), warningSubtle: c(warningColor), warning: c(warningColor), attention: c(p.accent), busy: c(p.secondary),
		info: c(p.primary), infoMoreSubtle: c(p.secondary), infoMostSubtle: c(infoMuted), success: c(successColor), successMoreSubtle: c(successColor), successMostSubtle: c(successMuted),
		yolo: c(p.accent), plan: c(p.primary), planMoreSubtle: c(p.secondary),
		ansiBlack: c(p.background), ansiRed: c("#F87171"), ansiGreen: c("#86EFAC"), ansiYellow: c("#FCD34D"), ansiBlue: c(p.primary), ansiMagenta: c(p.secondary), ansiCyan: c(p.accent), ansiWhite: c(p.foreground),
		ansiBrightBlack: c(p.muted), ansiBrightRed: c("#FDA4AF"), ansiBrightGreen: c("#BBF7D0"), ansiBrightYellow: c("#FEF08A"), ansiBrightBlue: c(p.primary), ansiBrightMagenta: c(p.secondary), ansiBrightCyan: c(p.accent), ansiBrightWhite: c("#FFFFFF"),
	})
	if name == "paper" {
		return applyPaperTheme(theme)
	}
	return theme
}
