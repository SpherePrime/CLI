package dialog

import (
	"fmt"
	"image"
	"slices"

	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/help"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/internal/agent/primeai"
	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/lsp"
	"github.com/SpherePrime/CLI/internal/session"
	"github.com/SpherePrime/CLI/internal/skills"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
)

// StatusID is the identifier for the status dialog.
const StatusID = "status"

const statusDialogMaxWidth = 56

// StatusData holds the information shown in the status dialog.
type StatusData struct {
	AccountEmail  string
	PrimeAILimits *primeai.Limits
	Session       *session.Session
	MCPStates     map[string]mcp.ClientInfo
	LSPStates     map[string]workspace.LSPClientInfo
	LSPDiags      map[string]lsp.DiagnosticCounts
	Skills        []skills.CatalogEntry
}

// Status is a read-only dialog showing session usage and server states.
type Status struct {
	com  *common.Common
	data StatusData
	keyMap struct {
		Refresh key.Binding
		Close   key.Binding
	}
	help help.Model

	scrollOffset int
	bodyRect     image.Rectangle
	scrollbarZone ScrollbarZone
}

var _ Dialog = (*Status)(nil)

// NewStatus creates a new status dialog with the given data.
func NewStatus(com *common.Common, data StatusData) *Status {
	s := &Status{com: com, data: data}
	help := help.New()
	help.Styles = com.Styles.DialogHelpStyles()
	s.help = help
	s.keyMap.Refresh = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh"),
	)
	closeKey := CloseKey
	closeKey.SetHelp("esc", com.L("key.cancel"))
	s.keyMap.Close = closeKey
	return s
}

// SetData replaces the displayed data so a refresh can re-render it.
func (s *Status) SetData(data StatusData) {
	s.data = data
}

// ID implements Dialog.
func (s *Status) ID() string {
	return StatusID
}

// RefreshLocale retranslates key hints after the UI language changes.
func (s *Status) RefreshLocale() {
	closeKey := CloseKey
	closeKey.SetHelp("esc", s.com.L("key.cancel"))
	s.keyMap.Close = closeKey
}

// HandleMsg implements [Dialog].
func (s *Status) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, s.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, s.keyMap.Refresh):
			return ActionRefreshStatus{}
		}
	case common.CoalescedWheelMsg:
		if s.scrollbarZone.track.Visible() &&
			msg.Mouse.X >= s.bodyRect.Min.X && msg.Mouse.X < s.bodyRect.Max.X &&
			msg.Mouse.Y >= s.bodyRect.Min.Y && msg.Mouse.Y < s.bodyRect.Max.Y {
			s.scrollBy(int(msg.DeltaY))
		}
	case tea.MouseClickMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg:
		s.scrollbarZone.HandleMsg(msg, s.scrollTo)
	}
	return nil
}

func (s *Status) scrollTo(offset int) {
	s.scrollOffset = offset
}

func (s *Status) scrollBy(delta int) {
	s.scrollOffset = max(0, s.scrollOffset+delta)
}

// Draw implements [Dialog].
func (s *Status) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := min(statusDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize())
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	allLines := s.contentLines(innerWidth)
	total := len(allLines)

	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() + 1 +
		t.Dialog.View.GetVerticalFrameSize()
	viewport := total
	if maxBody := area.Dy() - t.Dialog.View.GetVerticalBorderSize() - heightOffset; total > maxBody {
		viewport = max(0, maxBody)
	}
	s.scrollOffset = min(s.scrollOffset, total-viewport)
	visible := allLines[s.scrollOffset : s.scrollOffset+viewport]

	rc := NewRenderContext(t, width)
	rc.Title = s.com.L("status.title")
	body := joinScrollbar(t, lipgloss.JoinVertical(lipgloss.Left, visible...), viewport, total, viewport, s.scrollOffset)
	rc.AddPart(body)
	rc.Help = renderDialogHelp(t, &s.help, s, innerWidth)

	view := rc.Render()
	if total > viewport {
		s.bodyRect = dialogBodyRect(area, dialogRectCentered(area, view), body, rc.Help, rc.ViewStyle, t.Dialog.List, innerWidth, viewport)
		s.scrollbarZone.Painted(s.bodyRect.Min, body, viewport, total, viewport, s.scrollOffset)
	} else {
		s.bodyRect = image.Rect(0, 0, 0, 0)
		s.scrollbarZone.Clear()
	}

	DrawCenter(scr, area, view)
	return nil
}

func (s *Status) contentLines(width int) []string {
	t := s.com.Styles
	section := t.Dialog.PrimaryText
	label := t.Dialog.ListItem.InfoBlurred

	accountEmail := s.data.AccountEmail
	if accountEmail == "" {
		accountEmail = s.com.L("auth.not_signed_in")
	}

	lines := []string{
		section.Render(s.com.L("auth.account_label")),
		label.Render(ansi.Truncate(accountEmail, max(1, width), "…")),
	}
	if s.data.PrimeAILimits != nil && s.data.PrimeAILimits.Available {
		lines = append(lines, "", section.Render(s.com.L("primeai.limits_title")))
		lines = append(lines, s.primeAILimitsSection(label, width)...)
	}
	lines = append(lines, "", section.Render(s.com.L("status.usage")))
	lines = append(lines, s.usageSection(label, width)...)

	lines = append(lines, "", section.Render(s.com.L("status.mcp_servers")))
	lines = append(lines, s.mcpSection(label, width)...)

	lines = append(lines, "", section.Render(s.com.L("status.lsp_servers")))
	lines = append(lines, s.lspSection(label, width)...)

	lines = append(lines, "", section.Render(s.com.L("status.skills")))
	lines = append(lines, s.skillsSection(label, width)...)

	return lines
}

func (s *Status) primeAILimitsSection(label lipgloss.Style, width int) []string {
	lim := s.data.PrimeAILimits
	if lim == nil || !lim.Available {
		return []string{label.Render(s.com.L("primeai.unavailable"))}
	}
	row := func(k, v string) string {
		return label.Render(ansi.Truncate(fmt.Sprintf("%s  %s", k, v), max(1, width), "…"))
	}
	var lines []string
	if lim.PlanName != "" {
		lines = append(lines, row(s.com.L("primeai.plan_label"), lim.PlanName))
	}
	formatWindow := func(key string, w *primeai.LimitWindow) {
		if w == nil {
			return
		}
		if w.LimitTokens <= 0 && w.LimitRequests <= 0 && w.Percent <= 0 && w.ResetInSeconds <= 0 {
			lines = append(lines, row(s.com.L(key), s.com.L("primeai.unlimited")))
			return
		}
		val := fmt.Sprintf("%.0f%%", w.Percent)
		if w.ResetInSeconds > 0 {
			val += " (" + s.com.LSprintf("primeai.resets_in", common.FormatResetDuration(w.ResetInSeconds)) + ")"
		}
		lines = append(lines, row(s.com.L(key), val))
	}
	formatWindow("primeai.window_5h", lim.FiveHour)
	formatWindow("primeai.window_weekly", lim.Weekly)
	return lines
}

func (s *Status) skillsSection(label lipgloss.Style, width int) []string {
	if len(s.data.Skills) == 0 {
		return []string{label.Render(s.com.L("status.none"))}
	}
	names := make([]string, 0, len(s.data.Skills))
	for _, entry := range s.data.Skills {
		names = append(names, entry.Name)
	}
	slices.Sort(names)

	var lines []string
	for _, name := range names {
		lines = append(lines, label.Render(name))
	}
	return lines
}

func (s *Status) usageSection(label lipgloss.Style, width int) []string {
	if s.data.Session == nil {
		return []string{label.Render(s.com.L("status.no_session"))}
	}
	sess := s.data.Session
	totalTokens := sess.PromptTokens + sess.CompletionTokens
	costStr := fmt.Sprintf("$%.4f", sess.Cost)
	if sess.EstimatedUsage {
		costStr += " " + s.com.L("status.estimated")
	}

	row := func(k, v string) string {
		return label.Render(fmt.Sprintf("%s  ", k) + v)
	}

	return []string{
	row(s.com.L("status.session"), ansi.Truncate(sess.Title, width-12, "…")),
		row(s.com.L("status.tokens_prompt"), fmt.Sprintf("%d", sess.PromptTokens)),
		row(s.com.L("status.tokens_completion"), fmt.Sprintf("%d", sess.CompletionTokens)),
		row(s.com.L("status.tokens_total"), fmt.Sprintf("%d", totalTokens)),
		row(s.com.L("status.messages"), fmt.Sprintf("%d", sess.MessageCount)),
		row(s.com.L("status.cost"), costStr),
	}
}

func (s *Status) mcpSection(label lipgloss.Style, width int) []string {
	if len(s.data.MCPStates) == 0 {
		return []string{label.Render(s.com.L("status.none"))}
	}
	names := make([]string, 0, len(s.data.MCPStates))
	for name := range s.data.MCPStates {
		names = append(names, name)
	}
	slices.Sort(names)

	var lines []string
	for _, name := range names {
		info := s.data.MCPStates[name]
		stateText := s.mcpStateText(info)
		line := label.Render(fmt.Sprintf("%s: %s", name, stateText))
		if info.Error != nil {
			line += " — " + ansi.Truncate(info.Error.Error(), width-30, "…")
		}
		lines = append(lines, line)
	}
	return lines
}

func (s *Status) mcpStateText(info mcp.ClientInfo) string {
	switch info.State {
	case mcp.StateConnected:
		return s.com.L("status.state_started")
	case mcp.StateStarting:
		return s.com.L("status.state_connecting")
	case mcp.StateNeedsAuth:
		return s.com.L("status.state_auth")
	case mcp.StateError:
		return s.com.L("status.state_error")
	default:
		return s.com.L("status.state_stopped")
	}
}

func (s *Status) lspSection(label lipgloss.Style, width int) []string {
	if len(s.data.LSPStates) == 0 {
		return []string{label.Render(s.com.L("status.none"))}
	}
	names := make([]string, 0, len(s.data.LSPStates))
	for name := range s.data.LSPStates {
		names = append(names, name)
	}
	slices.Sort(names)

	var lines []string
	for _, name := range names {
		info := s.data.LSPStates[name]
		var stateText string
		switch info.State {
		case lsp.StateReady:
			stateText = s.com.L("status.state_started")
		case lsp.StateStarting:
			stateText = s.com.L("status.state_connecting")
		case lsp.StateError:
			stateText = s.com.L("status.state_error")
		default:
			stateText = s.com.L("status.state_stopped")
		}
		line := label.Render(fmt.Sprintf("%s: %s", name, stateText))
		if diags, ok := s.data.LSPDiags[name]; ok {
			issues := diags.Error + diags.Warning
			if issues > 0 {
				line += fmt.Sprintf(" (%d err, %d warn)", diags.Error, diags.Warning)
			}
		}
		lines = append(lines, line)
	}
	return lines
}

// ShortHelp implements [help.KeyMap].
func (s *Status) ShortHelp() []key.Binding {
	return []key.Binding{s.keyMap.Close}
}

// FullHelp implements [help.KeyMap].
func (s *Status) FullHelp() [][]key.Binding {
	return [][]key.Binding{{s.keyMap.Refresh, s.keyMap.Close}}
}
