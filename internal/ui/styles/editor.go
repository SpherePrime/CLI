package styles

import (
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

func (s *Styles) setupEditorPanel(o quickStyleOpts) {
	panel := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	s.Editor.PanelFocused = panel.BorderForeground(o.success)
	s.Editor.PanelBlurred = panel.BorderForeground(o.fgMostSubtle)
	s.Editor.Textarea.Focused.Placeholder = lipgloss.NewStyle().Foreground(o.fgSubtle)
	s.Editor.Textarea.Cursor.Color = o.success
	s.Editor.Textarea.Cursor.Shape = tea.CursorBar
	s.Editor.PromptNormalIconFocused = lipgloss.NewStyle().Foreground(o.success).Bold(true).SetString("›   ")
	s.Editor.PromptNormalIconBlurred = s.Editor.PromptNormalIconFocused.Foreground(o.fgMoreSubtle).Bold(false)
	s.Editor.PromptNormalFocused = lipgloss.NewStyle().SetString("    ")
	s.Editor.PromptNormalBlurred = s.Editor.PromptNormalFocused
	s.Editor.PromptPlanDotsFocused = lipgloss.NewStyle().SetString("    ")
	s.Editor.PromptPlanDotsBlurred = s.Editor.PromptPlanDotsFocused
	s.Editor.PromptYoloDotsFocused = lipgloss.NewStyle().SetString("    ")
	s.Editor.PromptYoloDotsBlurred = s.Editor.PromptYoloDotsFocused
	s.Editor.PromptBangDotsFocused = lipgloss.NewStyle().SetString("    ")
	s.Editor.PromptBangDotsBlurred = s.Editor.PromptBangDotsFocused
}
