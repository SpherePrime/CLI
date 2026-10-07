package model

import (
	"fmt"
	"github.com/SpherePrime/CLI/internal/appearance"
	"github.com/SpherePrime/CLI/internal/ui/common"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"strings"
)

func (m *UI) updateDesignSidebarScrollState() {
	area := m.layout.sidebar
	if area.Dx() <= 0 || area.Dy() <= 0 || m.session == nil {
		m.sidebarScrollable = false
		m.sidebarMaxOffsetVal = 0
		m.sidebarOffset = 0
		m.sidebarScrollbarTrack = common.ScrollbarTrack{}
		return
	}
	if appearance.DesignSpec(m.com.Styles.Design).Sidebar == "bottom" {
		m.sidebarScrollable = false
		m.sidebarScrollbarTrack = common.ScrollbarTrack{}
		return
	}
	style := m.com.Styles.DesignSidebar
	width := max(1, area.Dx()-style.GetHorizontalFrameSize()-1)
	height := max(0, area.Dy()-style.GetVerticalFrameSize()-2)
	cfg := m.com.Config()
	body := []string{
		m.com.Styles.Sidebar.SessionTitle.Width(width).Render(m.designSessionTitle()),
		common.PrettyPath(m.com.Styles, m.com.Workspace.WorkingDir(), width),
		m.modelInfo(width),
		m.filesInfo(m.com.Workspace.WorkingDir(), width, fileChangeCount(m.sessionFiles), true),
		m.lspInfo(width, len(m.lspStates), true),
	}
	if cfg != nil {
		body = append(body, m.mcpInfo(width, mcpCount(cfg.MCP.Sorted(), m.mcpStates), true))
	}
	if m.com.Styles.Design != "studio" {
		body = append(body, m.skillsInfo(width, len(m.skillStatusItems()), true))
	}
	m.sidebarContent = strings.Join(body, "\n\n")
	m.sidebarTotalLines = strings.Count(m.sidebarContent, "\n") + 1
	m.sidebarContentWidth = width
	m.sidebarContentHeight = height
	m.sidebarScrollable = m.sidebarTotalLines > height
	if !m.sidebarScrollable && m.focus == uiFocusSidebar {
		m.focus = uiFocusMain
		m.chat.Focus()
	}
	m.sidebarMaxOffsetVal = max(0, m.sidebarTotalLines-height)
	m.sidebarOffset = min(m.sidebarOffset, m.sidebarMaxOffsetVal)
	m.sidebarContentTop = area.Min.Y + style.GetBorderTopSize() + style.GetPaddingTop() + 2
	m.recordSidebarScrollbarTrack()
	if m.sidebarScrollbarTrack.Height > 0 {
		m.sidebarScrollbarTrack.X = area.Max.X - style.GetBorderRightSize() - style.GetPaddingRight() - 1
	}
}
func (m *UI) drawDesignSidebar(scr uv.Screen, area uv.Rectangle) {
	style := m.com.Styles.DesignSidebar
	if appearance.DesignSpec(m.com.Styles.Design).Sidebar == "bottom" {
		path := ""
		if m.com.Workspace != nil {
			path = m.com.Workspace.WorkingDir()
		}
		content := fmt.Sprintf("WORKSPACE  %s    |    %s    |    %d tasks / %d files\n%s", m.designSessionTitle(), m.designModelLabel(), m.designTaskCount(), fileChangeCount(m.sessionFiles), path)
		uv.NewStyledString(designPanelView(style, content, area)).Draw(scr, area)
		return
	}
	lines := strings.Split(m.sidebarContent, "\n")
	start := min(m.sidebarOffset, len(lines))
	end := min(start+m.sidebarContentHeight, len(lines))
	heading := "WORKSPACE"
	if m.com.Styles.Design == "studio" {
		heading = "NAVIGATOR"
	}
	content := m.com.Styles.Header.Label.Bold(true).Render(heading) + "\n\n" + strings.Join(lines[start:end], "\n")
	uv.NewStyledString(designPanelView(style, content, area)).Draw(scr, area)
	if m.sidebarScrollbarShown() && m.sidebarContentHeight > 0 {
		scrollbar := common.Scrollbar(m.com.Styles, m.sidebarContentHeight, m.sidebarTotalLines, m.sidebarContentHeight, m.sidebarOffset)
		track := m.sidebarScrollbarTrack
		uv.NewStyledString(scrollbar).Draw(scr, uv.Rect(track.X, track.MinY, 1, track.Height))
	}
}
func (m *UI) drawDesignInspector(scr uv.Screen, area uv.Rectangle) {
	style := m.com.Styles.DesignInspector
	width := max(1, area.Dx()-style.GetHorizontalFrameSize())
	heading := m.com.Styles.Header.Label.Bold(true).Render("INSPECTOR")
	sections := []string{heading, "MODEL", m.modelInfo(width), "TASKS", fmt.Sprintf("%d items", m.designTaskCount()), "FILES", m.filesInfo(m.com.Workspace.WorkingDir(), width, fileChangeCount(m.sessionFiles), true), "SHORTCUTS", "ctrl+p  commands\nctrl+l  model\nctrl+s  sessions"}
	content := lipgloss.JoinVertical(lipgloss.Left, sections...)
	uv.NewStyledString(designPanelView(style, content, area)).Draw(scr, area)
}
