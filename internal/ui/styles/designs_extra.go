package styles

import (
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"image/color"
)

func applyAdditionalDesign(s Styles, panel lipgloss.Style, border lipgloss.Border, primary, accent, surface, foreground color.Color) (Styles, lipgloss.Style, lipgloss.Border) {
	switch s.Design {
	case "opencode":
		panel = panel.Border(lipgloss.ThickBorder(), false, false, false, true).Padding(1, 2)
		s.DesignHeader = lipgloss.NewStyle().Foreground(foreground).Background(s.Background).Padding(0, 1)
		s.DesignSidebar = s.DesignSidebar.Border(lipgloss.NormalBorder(), false, false, false, true)
		s.Editor.PromptNormalIconFocused = lipgloss.NewStyle().Foreground(primary).SetString("›   ")
	case "paper":
		panel = panel.Border(lipgloss.NormalBorder(), true, false, true, false).Padding(1, 2)
		s.DesignHeader = lipgloss.NewStyle().Foreground(primary).Background(s.Background).Align(lipgloss.Center)
		s.Editor.PromptNormalIconFocused = lipgloss.NewStyle().Foreground(primary).SetString("✎   ")
	case "blueprint":
		border = lipgloss.NormalBorder()
		panel = panel.Border(border).Padding(1, 1)
		s.DesignHeader = s.DesignHeader.Border(border).BorderForeground(primary)
		s.DesignSidebar = s.DesignSidebar.Border(border).Background(s.Background)
		s.Editor.PromptNormalIconFocused = lipgloss.NewStyle().Foreground(accent).SetString("+   ")
	case "ember":
		panel = panel.Border(lipgloss.ThickBorder(), true, false, false, false).BorderForeground(primary).Padding(1, 3)
		s.DesignHeader = lipgloss.NewStyle().Foreground(primary).Background(s.Background).Align(lipgloss.Center)
		s.DesignSidebar = s.DesignSidebar.Border(lipgloss.NormalBorder(), true, false, false, false).BorderForeground(accent)
		s.Editor.PromptNormalIconFocused = lipgloss.NewStyle().Foreground(primary).SetString("»   ")
	}
	return s, panel, border
}
