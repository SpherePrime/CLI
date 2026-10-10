package styles

import (
	"github.com/SpherePrime/CLI/internal/appearance"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

func ApplyDesign(s Styles, name string) Styles {
	s.Design = appearance.DesignKey(name)
	primary := s.Tool.NameNormal.GetForeground()
	accent := s.Header.Label.GetForeground()
	surface := s.Dialog.ContentPanel.GetBackground()
	foreground := s.Header.Wrapper.GetForeground()
	s.Canvas = lipgloss.NewStyle().Background(s.Background).Foreground(foreground)
	s.DesignHeader = lipgloss.NewStyle().Foreground(primary).Background(surface).Padding(0, 1)
	s.DesignSidebar = lipgloss.NewStyle().Foreground(foreground).Background(surface).Border(lipgloss.RoundedBorder()).BorderForeground(primary).Padding(0, 1)
	s.DesignInspector = s.DesignSidebar.BorderForeground(accent)
	border := lipgloss.RoundedBorder()
	panel := lipgloss.NewStyle().Background(surface).Foreground(foreground).Border(border).BorderForeground(primary).Padding(0, 1)
	switch s.Design {
	case "classic":
		s.Editor.PanelFocused = s.Editor.PanelFocused.Background(s.Background)
		s.Editor.PanelBlurred = s.Editor.PanelBlurred.Background(s.Background)
		s.DesignHeader = s.DesignHeader.Border(border).BorderForeground(primary)
		return s
	case "minimal":
		panel = lipgloss.NewStyle().Foreground(foreground).Background(s.Background).Padding(0, 0)
		s.DesignHeader = lipgloss.NewStyle().Foreground(primary).Background(s.Background).BorderBottom(true).BorderStyle(lipgloss.NormalBorder()).BorderForeground(primary)
		s.Messages.ThinkingBox = s.Messages.ThinkingBox.UnsetBackground().Padding(0)
		s.Editor.PromptNormalIconFocused = s.Editor.PromptNormalIconFocused.SetString(">   ")
	case "focus":
		panel = panel.Border(lipgloss.ThickBorder(), false, false, false, true).Padding(1, 2)
		s.DesignHeader = lipgloss.NewStyle().Foreground(primary).Background(s.Background).Align(lipgloss.Center)
		s.Editor.PromptNormalIconFocused = s.Editor.PromptNormalIconFocused.SetString("›   ")
	case "cards":
		panel = panel.Padding(1, 2)
	case "dashboard":
		border = lipgloss.DoubleBorder()
		panel = panel.Border(border).Padding(0, 2)
		s.DesignSidebar = s.DesignSidebar.Border(border)
	case "terminal":
		border = lipgloss.ASCIIBorder()
		panel = lipgloss.NewStyle().Foreground(foreground).Background(s.Background).BorderBottom(true).BorderStyle(border).BorderForeground(primary).Padding(0)
		s.DesignHeader = lipgloss.NewStyle().Foreground(primary).Background(s.Background)
		s.DesignSidebar = s.DesignSidebar.Border(border)
		s.Editor.PromptNormalIconFocused = lipgloss.NewStyle().Foreground(primary).Bold(true).SetString("$   ")
		s.Editor.PromptNormalIconBlurred = s.Editor.PromptNormalIconFocused.Bold(false)
		s.Messages.UserFocused = lipgloss.NewStyle().Foreground(primary).SetString("> ")
		s.Messages.UserBlurred = s.Messages.UserFocused
		s.Messages.AssistantFocused = lipgloss.NewStyle().Foreground(foreground).SetString("| ")
		s.Messages.AssistantBlurred = s.Messages.AssistantFocused
	case "studio":
		panel = panel.Border(lipgloss.NormalBorder(), true, false, false, false).Padding(0, 2)
		s.DesignSidebar = s.DesignSidebar.Border(lipgloss.NormalBorder(), false, true, false, false)
	case "opencode", "paper", "blueprint", "ember":
		s, panel, border = applyAdditionalDesign(s, panel, border, primary, accent, surface, foreground)
	case "neon":
		border = lipgloss.ThickBorder()
		panel = panel.Border(border).BorderForeground(accent).Padding(1, 2)
		s.DesignHeader = s.DesignHeader.Bold(true).Border(lipgloss.DoubleBorder()).BorderForeground(primary)
		s.DesignSidebar = s.DesignSidebar.Border(lipgloss.DoubleBorder()).BorderForeground(accent)
		s.Editor.Textarea.Cursor.Shape = tea.CursorBlock
		s.Editor.PromptNormalIconFocused = lipgloss.NewStyle().Foreground(accent).Bold(true).SetString("»   ")
	}
	s.Editor.PanelFocused = panel
	s.Editor.PanelBlurred = panel.BorderForeground(s.Header.WorkingDir.GetForeground())
	if s.Design == "cards" || s.Design == "dashboard" || s.Design == "studio" || s.Design == "neon" || s.Design == "blueprint" {
		s.MessageFrame = lipgloss.NewStyle().Border(border).BorderForeground(primary).Background(surface).Foreground(foreground).Padding(0, 1)
		s.Messages.PlanBox = s.Messages.PlanBox.Border(border).Padding(1, 2)
		s.Messages.ThinkingBox = s.Messages.ThinkingBox.Border(border).BorderForeground(accent).Padding(0, 1)
	}
	s.DesignHeader = s.DesignHeader.Border(border).BorderForeground(primary)
	return s
}
