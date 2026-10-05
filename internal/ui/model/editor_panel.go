package model

import (
	"image"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

func (m *UI) editorPanelStyle() lipgloss.Style {
	if width := m.layout.editor.Dx(); width > 0 && width < 9 {
		return lipgloss.NewStyle()
	}
	sty := m.com.Styles.Editor
	if !m.textarea.Focused() {
		return sty.PanelBlurred
	}
	panel := sty.PanelFocused
	switch {
	case m.bangMode:
		panel = panel.BorderForeground(sty.PromptBangIconFocused.GetBackground())
	case m.mode == uiInputModePlan:
		panel = panel.BorderForeground(sty.PromptPlanIconFocused.GetBackground())
	case m.yoloModeCached():
		panel = panel.BorderForeground(sty.PromptYoloIconFocused.GetBackground())
	}
	return panel
}

func (m *UI) editorAttachmentsView(width int) string {
	if m.attachments == nil || len(m.attachments.List()) == 0 {
		return ""
	}
	return m.attachments.Render(width)
}

func (m *UI) editorAttachmentsHeight(width int) int {
	view := m.editorAttachmentsView(width)
	if view == "" {
		return 0
	}
	return lipgloss.Height(view)
}

func (m *UI) textareaOrigin() image.Point {
	panel := m.editorPanelStyle()
	return m.layout.editor.Min.Add(image.Pt(
		panel.GetBorderLeftSize()+panel.GetPaddingLeft(),
		panel.GetBorderTopSize()+m.editorAttachmentsHeight(m.layout.editor.Dx()),
	))
}

func (m *UI) renderEditorView(width int) string {
	if width <= 0 {
		return ""
	}
	panel := m.editorPanelStyle()
	view := panel.Width(width).Render(m.textarea.View())
	if attachmentsView := m.editorAttachmentsView(width); attachmentsView != "" {
		view = attachmentsView + "\n" + view
	}
	lines := strings.Split(view, "\n")
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "")
	}
	return strings.Join(lines, "\n")
}
