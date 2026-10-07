package model

import (
	"fmt"
	"github.com/SpherePrime/CLI/internal/appearance"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"strings"
)

func (m *UI) designModelLabel() string {
	cfg := m.com.Config()
	if cfg == nil {
		return ""
	}
	selected := cfg.Models[config.SelectedModelTypeLarge]
	if selected.Model == "" {
		return "Prime"
	}
	return selected.Provider + " / " + selected.Model
}
func (m *UI) designSessionTitle() string {
	if m.session != nil && m.session.Title != "" {
		return m.session.Title
	}
	return "Prime"
}
func (m *UI) drawDesignHeader(scr uv.Screen, area uv.Rectangle) {
	if area.Dx() <= 0 || area.Dy() <= 0 {
		return
	}
	spec := appearance.DesignSpec(m.com.Styles.Design)
	style := m.com.Styles.DesignHeader
	inner := max(1, area.Dx()-style.GetHorizontalFrameSize())
	title := m.designSessionTitle()
	path := ""
	if m.com.Workspace != nil {
		path = m.com.Workspace.WorkingDir()
	}
	var lines []string
	switch spec.Key {
	case "opencode":
		lines = []string{"PRIME / " + title + "    ·    " + m.designModelLabel(), path}
	case "paper":
		lines = []string{"P R I M E  /  P A P E R", title + "    ·    " + m.designModelLabel()}
	case "blueprint":
		lines = []string{"PRIME  [ WORKSPACE ]  /  BLUEPRINT", title + "    ·    " + m.designModelLabel()}
	case "ember":
		lines = []string{"P R I M E  /  E M B E R", title + "    ·    " + m.designModelLabel()}
	case "minimal":
		lines = []string{"PRIME  /  " + title + "    ·    " + m.designModelLabel()}
	case "focus":
		lines = []string{"P R I M E", title, m.designModelLabel()}
	case "terminal":
		lines = []string{"PRIME CONSOLE :: " + path, strings.Repeat("-", inner)}
	case "neon":
		lines = []string{"P R I M E  /  N E O N", fmt.Sprintf("%s    ·    %s    ·    %d tasks", title, m.designModelLabel(), m.designTaskCount())}
	case "studio":
		lines = []string{"PRIME STUDIO     WORKSPACE / CONVERSATION / INSPECTOR", title + "    ·    " + path}
	case "dashboard":
		lines = []string{"PRIME   [ WORKSPACE ]   " + title, m.designModelLabel() + "    ·    " + path}
	default:
		lines = []string{"PRIME / " + strings.ToUpper(spec.Title) + "    ·    " + title, m.designModelLabel() + "    ·    " + path}
	}
	for index, line := range lines {
		lines[index] = ansi.Truncate(line, inner, "")
	}
	view := style.Width(area.Dx()).Height(area.Dy()).MaxHeight(area.Dy()).Render(strings.Join(lines, "\n"))
	uv.NewStyledString(styles.WithBackground(view, style.GetBackground())).Draw(scr, area)
}
func fitDesignLines(text string, width, height int) string {
	if width <= 0 || height <= 0 {
		return ""
	}
	lines := strings.Split(text, "\n")
	if len(lines) > height {
		lines = lines[:height]
	}
	for i, line := range lines {
		lines[i] = ansi.Truncate(line, width, "")
	}
	return strings.Join(lines, "\n")
}
func designPanelView(style lipgloss.Style, text string, area uv.Rectangle) string {
	innerWidth := max(0, area.Dx()-style.GetHorizontalFrameSize())
	innerHeight := max(0, area.Dy()-style.GetVerticalFrameSize())
	return styles.WithBackground(style.Width(area.Dx()).Height(area.Dy()).MaxWidth(area.Dx()).MaxHeight(area.Dy()).Render(fitDesignLines(text, innerWidth, innerHeight)), style.GetBackground())
}

func (m *UI) designTaskCount() int {
	if m.session == nil {
		return 0
	}
	return len(m.session.Todos)
}
