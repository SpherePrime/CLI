package model

import (
	"fmt"
	"image"
	"strings"

	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	mcp "github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/common"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet/layout"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
)

// modelInfo renders the current model information including reasoning
// settings and context usage/cost for the sidebar. It also returns whether
// the reasoning mode is clickable (i.e. the model supports reasoning).
func (m *UI) modelInfo(width int) (string, bool) {
	model := m.selectedLargeModel()
	reasoningInfo := ""
	providerName := ""
	hasReasoning := false

	if model != nil {
		// Get provider name first
		providerConfig, ok := m.com.Config().Providers.Get(model.ModelCfg.Provider)
		if ok {
			providerName = providerConfig.Name

			// Only check reasoning if model can reason
			if model.CatwalkCfg.CanReason {
				hasReasoning = true
				// Use the unified reasoning mode for display
				mode := model.ModelCfg.ResolveReasoningMode(model.CatwalkCfg)
				reasoningInfo = fmt.Sprintf("Reasoning %s", config.FormatReasoningMode(mode))
			}
		}
	}

	var modelContext *common.ModelContextInfo
	if model != nil && m.session != nil {
		modelContext = &common.ModelContextInfo{
			ContextUsed:    m.session.CompletionTokens + m.session.PromptTokens,
			Cost:           m.session.Cost,
			ModelContext:   model.CatwalkCfg.ContextWindow,
			EstimatedUsage: m.session.EstimatedUsage,
		}
	}
	var modelName string
	if model != nil {
		modelName = model.CatwalkCfg.Name
	}
	rendered := common.ModelInfo(m.com.Styles, modelName, providerName, reasoningInfo, modelContext, width, m.hyperCredits)
	return rendered, hasReasoning
}

// modelInfoStr is a convenience wrapper around modelInfo for callers that
// only need the rendered string (the design sidebar and inspector views).
func (m *UI) modelInfoStr(width int) string {
	s, _ := m.modelInfo(width)
	return s
}

// findModelInfoRow locates the row index (in sidebarContent lines) where the
// model info block begins. The model info string always starts with the model
// icon, so we search for the first line beginning with it.
func (m *UI) findModelInfoRow(modelInfoStr string) int {
	if modelInfoStr == "" {
		return -1
	}
	firstLine := strings.Split(modelInfoStr, "\n")[0]
	lines := strings.Split(m.sidebarContent, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, firstLine) || strings.HasPrefix(firstLine, line) {
			return i
		}
	}
	return -1
}

// updateSidebarScrollState renders the sidebar content and computes scroll
// state (scrollability, max offset, clamp) before drawing. This keeps all
// state mutation in the update path rather than in the draw function.
func (m *UI) updateSidebarScrollState() {
	if m.customDesign() {
		m.updateDesignSidebarScrollState()
		return
	}
	if m.session == nil || m.isCompact {
		m.sidebarScrollbarTrack = common.ScrollbarTrack{}
		return
	}

	const logoHeightBreakpoint = 30

	t := m.com.Styles
	width := m.layout.sidebar.Dx()
	height := m.layout.sidebar.Dy()

	contentWidth := max(width-2, 1)

	title := t.Sidebar.SessionTitle.Width(contentWidth).MaxHeight(2).Render(m.session.Title)
	cwd := common.PrettyPath(t, m.com.Workspace.WorkingDir(), contentWidth)
	phase := m.shimmerStep()
	sidebarLogo := m.compactSidebarLogo(phase)
	if height < logoHeightBreakpoint {
		sidebarLogo = lipgloss.JoinVertical(lipgloss.Left, m.smallSidebarLogo(contentWidth, phase), "")
	}

	var logoRect, contentRect image.Rectangle
	layout.Vertical(
		layout.Len(lipgloss.Height(sidebarLogo)),
		layout.Fill(1),
	).Split(m.layout.sidebar).Assign(&logoRect, &contentRect)

	contentHeight := contentRect.Dy()

	// Render all items without truncation; virtual scrolling handles overflow.
	lspSection := m.lspInfo(contentWidth, len(m.lspStates), true)
	mcpSection := m.mcpInfo(contentWidth, mcpCount(m.com.Config().MCP.Sorted(), m.mcpStates), true)
	skillsSection := m.skillsInfo(contentWidth, len(m.skillStatusItems()), true)
	filesSection := m.filesInfo(m.com.Workspace.WorkingDir(), contentWidth, fileChangeCount(m.sessionFiles), true)

	// Build the scrollable content.
	modelInfoStr, hasReasoning := m.modelInfo(contentWidth)
	m.sidebarReasoningClickable = hasReasoning

	// Build the scrollable content.
	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		cwd,
		"",
		modelInfoStr,
		"",
		filesSection,
		"",
		lspSection,
		"",
		mcpSection,
		"",
		skillsSection,
	)

	totalLines := strings.Count(content, "\n") + 1
	m.sidebarContent = content
	m.sidebarTotalLines = totalLines
	m.sidebarContentWidth = contentWidth
	m.sidebarContentHeight = contentHeight
	m.sidebarDrawLogo = sidebarLogo
	m.sidebarScrollable = totalLines > contentHeight
	m.sidebarMaxOffsetVal = max(0, totalLines-contentHeight)
	m.sidebarContentTop = contentRect.Min.Y

	// Record the row index (in sidebarContent) that renders the model info
	// block, so we can hit-test clicks on it for the reasoning-effort dialog.
	if m.sidebarReasoningClickable {
		m.sidebarModelInfoRow = m.findModelInfoRow(modelInfoStr)
	} else {
		m.sidebarModelInfoRow = -1
	}

	// If the sidebar is focused but no longer scrollable (e.g. after a
	// resize), return focus to the chat.
	if m.focus == uiFocusSidebar && !m.sidebarScrollable {
		m.focus = uiFocusMain
		m.chat.Focus()
	}

	// Clamp sidebarOffset.
	if m.sidebarOffset > m.sidebarMaxOffsetVal {
		m.sidebarOffset = m.sidebarMaxOffsetVal
	}

	// Record where the thumb is painted so a press on that column grabs it.
	m.recordSidebarScrollbarTrack()
}

// recordSidebarScrollbarTrack stores the sidebar scrollbar column geometry for
// pointer hit-testing, or clears it when no thumb is painted.
func (m *UI) recordSidebarScrollbarTrack() {
	if !m.sidebarScrollbarShown() {
		m.sidebarScrollbarTrack = common.ScrollbarTrack{}
		return
	}
	m.sidebarScrollbarTrack = common.ScrollbarTrack{
		X:            m.layout.sidebar.Max.X - 1,
		MinY:         m.sidebarContentTop,
		Height:       m.sidebarContentHeight,
		ContentSize:  m.sidebarTotalLines,
		ViewportSize: m.sidebarContentHeight,
		Offset:       m.sidebarOffset,
	}
}

// sidebarScrollbarShown reports whether the sidebar scrollbar is painted this
// frame: it needs scrollable content and either focus, recent scroll activity,
// or a pointer holding the thumb.
func (m *UI) sidebarScrollbarShown() bool {
	if !m.sidebarScrollable {
		return false
	}
	return m.sidebarScrollbarVisible || m.focus == uiFocusSidebar || m.sidebarScrollbarDrag.Active()
}

// handleSidebarModelInfoClick checks whether a mouse click landed on the
// model info row in the sidebar (which displays reasoning/thinking state).
// If so, it opens the reasoning effort dialog (for OpenAI-style models with
// reasoning levels) or toggles thinking mode (for Anthropic-style models),
// and returns the command to run.
// Returns nil if the click was not on the model info row.
func (m *UI) handleSidebarModelInfoClick(x, y int) tea.Cmd {
	if m.sidebarModelInfoRow < 0 || !m.sidebarReasoningClickable {
		return nil
	}
	if !image.Pt(x, y).In(m.layout.sidebar) {
		return nil
	}
	// Translate screen Y into a row index within sidebarContent.
	rowInContent := y - m.sidebarContentTop + m.sidebarOffset
	if rowInContent < 0 || rowInContent >= m.sidebarTotalLines {
		return nil
	}
	// The model info block may span multiple lines (name, provider,
	// reasoning, context/cost). Accept a click on any of those lines.
	modelInfoLines := m.countModelInfoLines()
	if rowInContent < m.sidebarModelInfoRow || rowInContent >= m.sidebarModelInfoRow+modelInfoLines {
		return nil
	}

	// Always open the reasoning mode dialog — it shows all modes for any model.
	return m.openReasoningDialog()
}

// countModelInfoLines counts how many lines the modelInfo block spans in
// the sidebar content (used for click hit-testing). The block starts at
// sidebarModelInfoRow and ends at the next blank line or section boundary.
func (m *UI) countModelInfoLines() int {
	if m.sidebarModelInfoRow < 0 || m.sidebarContent == "" {
		return 0
	}
	lines := strings.Split(m.sidebarContent, "\n")
	if m.sidebarModelInfoRow >= len(lines) {
		return 0
	}
	count := 0
	for i := m.sidebarModelInfoRow; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			break
		}
		count++
	}
	if count == 0 {
		count = 1
	}
	return count
}

// drawSidebar renders the chat sidebar with a fixed logo and a
// virtual-scrolling content area with an auto-hiding scrollbar. While the
// sidebar is focused, the scrollbar stays visible.
func (m *UI) drawSidebar(scr uv.Screen, area uv.Rectangle) {
	if m.customDesign() {
		m.drawDesignSidebar(scr, area)
		return
	}
	if m.session == nil {
		return
	}

	sidebarLogo := m.sidebarDrawLogo
	contentWidth := m.sidebarContentWidth
	contentHeight := m.sidebarContentHeight
	totalLines := m.sidebarTotalLines

	var logoRect, contentRect image.Rectangle
	layout.Vertical(
		layout.Len(lipgloss.Height(sidebarLogo)),
		layout.Fill(1),
	).Split(area).Assign(&logoRect, &contentRect)

	// Slice visible lines.
	end := min(m.sidebarOffset+contentHeight, totalLines)
	lines := strings.Split(m.sidebarContent, "\n")
	visibleLines := lines[m.sidebarOffset:end]
	visibleStr := strings.Join(visibleLines, "\n")

	// Determine scrollbar visibility: always visible when focused, otherwise
	// auto-hide.
	scrollbarVisible := m.sidebarScrollbarShown()

	// Draw the fixed logo.
	uv.NewStyledString(
		lipgloss.NewStyle().
			MaxWidth(contentWidth).
			MaxHeight(lipgloss.Height(sidebarLogo)).
			Render(sidebarLogo),
	).Draw(scr, logoRect)

	// Draw the visible content in the scrollable area.
	uv.NewStyledString(
		lipgloss.NewStyle().
			MaxWidth(contentWidth).
			MaxHeight(contentHeight).
			Render(visibleStr),
	).Draw(scr, contentRect)

	// Draw scrollbar in the reserved column.
	if scrollbarVisible {
		scrollbar := common.Scrollbar(m.com.Styles, contentHeight, totalLines, contentHeight, m.sidebarOffset)
		if scrollbar != "" {
			scrollbarArea := image.Rectangle{
				Min: image.Point{X: area.Max.X - 1, Y: contentRect.Min.Y},
				Max: image.Point{X: area.Max.X, Y: area.Max.Y},
			}
			uv.NewStyledString(scrollbar).Draw(scr, scrollbarArea)
		}
	}
}

// fileChangeCount returns the number of session files with non-zero additions
// or deletions.
func fileChangeCount(files []SessionFile) int {
	count := 0
	for _, f := range files {
		if f.Additions == 0 && f.Deletions == 0 {
			continue
		}
		count++
	}
	return count
}

func mcpCount(mcpCfgs []config.MCP, states map[string]mcp.ClientInfo) int {
	return len(mcpCfgs)
}
