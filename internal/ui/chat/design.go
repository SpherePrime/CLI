package chat

import (
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"hash/fnv"
	"strings"
)

func messageFramed(sty *styles.Styles, width int) bool {
	return width >= 8 && (sty.Design == "cards" || sty.Design == "dashboard" || sty.Design == "studio" || sty.Design == "neon" || sty.Design == "blueprint")
}

func rawMessageLines(sty *styles.Styles, content string, width int) string {
	if messageFramed(sty, width) {
		return " \n" + content + "\n "
	}
	return content
}

func messageFrameHeight(sty *styles.Styles, width int) int {
	if messageFramed(sty, width) {
		return 2
	}
	return 0
}

func designCacheKey(sty *styles.Styles, key uint64) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(sty.Design))
	return key ^ h.Sum64()
}

func designMessageWidth(sty *styles.Styles, width int) int {
	if messageFramed(sty, width) {
		width--
	}
	return max(1, cappedMessageWidth(width))
}
func renderMessageLines(sty *styles.Styles, content, prefix string, width int, title string) string {
	if width <= 0 {
		return ""
	}
	lines := strings.Split(content, "\n")
	framed := messageFramed(sty, width)
	border := sty.MessageFrame.GetBorderStyle()
	if border.Top == "" {
		border = lipgloss.RoundedBorder()
		if sty.Design == "blueprint" {
			border = lipgloss.NormalBorder()
		}
	}
	frameColor := lipgloss.NewStyle().Foreground(sty.MessageFrame.GetBorderLeftForeground()).Background(sty.MessageFrame.GetBackground())
	background := lipgloss.NewStyle().Background(sty.MessageFrame.GetBackground())
	if framed {
		if len(lines) >= 3 {
			lines = lines[1 : len(lines)-1]
		}
	} else if sty.Design == "terminal" {
		switch title {
		case "YOU":
			prefix = "> "
		case "TOOL":
			prefix = "$ "
		default:
			prefix = "| "
		}
	} else if sty.Design == "minimal" || sty.Design == "focus" {
		prefix = "  "
	} else if sty.Design == "opencode" {
		prefix = "│ "
		if title == "YOU" {
			prefix = "┃ "
		} else if title == "TOOL" {
			prefix = "┆ "
		}
		prefix = lipgloss.NewStyle().Foreground(sty.Header.Label.GetForeground()).Render(prefix)
	} else if sty.Design == "paper" {
		prefix = "  "
		if title == "YOU" {
			prefix = "› "
		} else if title == "TOOL" {
			prefix = "· "
		}
		prefix = lipgloss.NewStyle().Foreground(sty.Header.Label.GetForeground()).Render(prefix)
	} else if sty.Design == "ember" {
		prefix = lipgloss.NewStyle().Foreground(sty.Header.Label.GetForeground()).Render("▌ ")
	}
	for i, line := range lines {
		if framed {
			left, right := border.Left, border.Right
			inner := max(1, width-3)
			line = ansi.Truncate(line, inner, "")
			lines[i] = frameColor.Render(left) + background.Render(" "+line+strings.Repeat(" ", max(0, inner-ansi.StringWidth(line)))) + frameColor.Render(right)
		} else {
			lines[i] = ansi.Truncate(prefix+line, width, "")
		}
	}
	if framed {
		label := ansi.Truncate(" "+title+" ", width-2, "")
		top := border.TopLeft + label + strings.Repeat(border.Top, max(0, width-2-ansi.StringWidth(label))) + border.TopRight
		bottom := border.BottomLeft + strings.Repeat(border.Bottom, width-2) + border.BottomRight
		lines = append([]string{frameColor.Render(top)}, lines...)
		lines = append(lines, frameColor.Render(bottom))
	}
	rendered := strings.Join(lines, "\n")
	if framed {
		return styles.WithBackground(rendered, sty.MessageFrame.GetBackground())
	}
	return rendered
}
