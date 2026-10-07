package model

import "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"

func (m *UI) editorChromeParts(width int) (caption, footer string) {
	if width < 30 {
		return "", ""
	}
	switch m.com.Styles.Design {
	case "opencode", "paper", "blueprint", "ember":
		caption = m.com.L("editor.design.compose")
		footer = m.com.L("editor.design.actions")
	case "cards":
		caption = m.com.L("editor.design.new_message")
		footer = m.com.L("editor.design.actions")
	case "focus":
		caption = m.com.L("editor.design.message")
	case "dashboard", "studio":
		caption = m.com.L("editor.design.compose")
		footer = m.com.L("editor.design.actions")
	case "neon":
		caption = m.com.L("editor.design.transmit")
		footer = m.com.L("editor.design.actions")
	}
	if caption != "" && (m.com.Styles.Design == "dashboard" || m.com.Styles.Design == "studio" || m.com.Styles.Design == "neon" || m.com.Styles.Design == "opencode" || m.com.Styles.Design == "blueprint" || m.com.Styles.Design == "ember") {
		mode := "CODE"
		if m.mode == uiInputModePlan {
			mode = "PLAN"
		}
		if m.bangMode {
			mode = "SHELL"
		}
		caption += "  /  " + mode
	}
	inner := max(1, width-m.editorPanelStyleForWidth(width).GetHorizontalFrameSize())
	caption = ansi.Truncate(caption, inner, "")
	footer = ansi.Truncate(footer, inner, "")
	return caption, footer
}
func (m *UI) editorChromeHeight(width int) int {
	caption, footer := m.editorChromeParts(width)
	rows := 0
	if caption != "" {
		rows++
	}
	if footer != "" {
		rows++
	}
	return rows
}
func (m *UI) editorCaptionHeight(width int) int {
	caption, _ := m.editorChromeParts(width)
	if caption != "" {
		return 1
	}
	return 0
}
