package chat

import (
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
)

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
