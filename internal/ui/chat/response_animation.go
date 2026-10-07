package chat

import (
	"strings"

	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

func responseAnimationPattern(design string, frame uint64) string {
	step := int(frame / 3)
	rotate := func(pattern string) string {
		cells := []rune(pattern)
		offset := step % len(cells)
		return string(append(cells[offset:], cells[:offset]...))
	}
	moving := func(length int, empty, active rune) string {
		cells := []rune(strings.Repeat(string(empty), length))
		cells[step%length] = active
		return string(cells)
	}
	switch design {
	case "minimal":
		return rotate("▁▂▃▄▅▄▃▂▁▁")
	case "cards":
		return rotate("▰▰▱▱▱▱▱▱▱▱")
	case "focus":
		return strings.Repeat([]string{"░", "▒", "▓", "█", "▓", "▒"}[step%6], 10)
	case "dashboard":
		return "[" + moving(8, '·', []rune("╱─╲│")[step%4]) + "]"
	case "terminal":
		return "[" + moving(8, '-', '>') + "]"
	case "studio":
		return rotate("─▂▄▆█▆▄▂──")
	case "neon":
		return rotate("▁▃▆█▆▃▁▂▅▇")
	case "opencode":
		return rotate("┃┃││││││││")
	case "paper":
		return "✎ " + moving(8, '·', '•')
	case "blueprint":
		return moving(10, '▧', '▣')
	case "ember":
		return rotate("˙·✧✦✧·˙·✧·")
	default:
		return moving(10, '·', '●')
	}
}

func renderResponseAnimation(sty *styles.Styles, state string, frame uint64, width int, elapsed string) string {
	if width <= 0 {
		return ""
	}
	pattern := responseAnimationPattern(sty.Design, frame)
	active := lipgloss.NewStyle().Foreground(sty.Tool.NameNormal.GetForeground())
	muted := lipgloss.NewStyle().Foreground(sty.Messages.AssistantInfoProvider.GetForeground())
	var animation strings.Builder
	for _, cell := range pattern {
		if strings.ContainsRune("●▰█▓┃▣✦•>╱╲│", cell) {
			animation.WriteString(active.Render(string(cell)))
		} else {
			animation.WriteString(muted.Render(string(cell)))
		}
	}
	if elapsed == "" {
		elapsed = "0s"
	}
	label := sty.Messages.AssistantInfoModel.Render(state + strings.Repeat(" ", max(0, 12-ansi.StringWidth(state))))
	timer := ansi.Truncate(elapsed, 7, "")
	timer += strings.Repeat(" ", max(0, 7-ansi.StringWidth(timer)))
	return ansi.Truncate(animation.String()+" "+label+" "+muted.Render(timer), width, "")
}
