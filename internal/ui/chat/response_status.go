package chat

import (
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
)

func responseWorkingLabel(design, state string) string {
	switch design {
	case "cards":
		return "IN PROGRESS · " + state
	case "dashboard":
		return "STATUS / " + state
	case "terminal":
		return "RUN · " + state
	case "studio":
		return "ACTIVITY · " + state
	case "neon":
		return "LIVE · " + state
	case "blueprint":
		return "BUILD / " + state
	case "opencode":
		return "TASK · " + state
	case "paper":
		return state
	case "ember":
		return "ACTIVE · " + state
	default:
		return state
	}
}

func renderResponseFooter(sty *styles.Styles, content, prefix string, width int, final bool) string {
	if width <= 0 {
		return ""
	}
	title := "ROUTED"
	if final {
		title = "DONE"
	}
	if messageFramed(sty, width) {
		return renderMessageLines(sty, content, prefix, width, title)
	}
	if sty.Design == "terminal" {
		return ansi.Truncate("$ ["+title+"] "+content, width, "")
	}
	if sty.Design == "minimal" || sty.Design == "focus" {
		return ansi.Truncate("✓ "+content, width, "")
	}
	if sty.Design == "opencode" || sty.Design == "ember" {
		return renderMessageLines(sty, title+" · "+content, prefix, width, title)
	}
	if sty.Design == "paper" {
		return ansi.Truncate("· "+content, width, "")
	}
	return renderMessageLines(sty, content, prefix, width, title)
}
