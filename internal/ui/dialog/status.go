package dialog

import (
	"fmt"
	"slices"

	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/internal/agent/tools/mcp"
	"github.com/SpherePrime/CLI/internal/lsp"
	"github.com/SpherePrime/CLI/internal/session"
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
	Session  *session.Session
	MCPStates map[string]mcp.ClientInfo
	LSPStates map[string]workspace.LSPClientInfo
	LSPDiags  map[string]lsp.DiagnosticCounts
}

// Status is a read-only dialog showing session usage and server states.
type Status struct {
	com  *common.Common
	data StatusData
	keyMap struct {
		Refresh key.Binding
		Close   key.Binding
	}
}

var _ Dialog = (*Status)(nil)

// NewStatus creates a new status dialog with the given data.
func NewStatus(com *common.Common, data StatusData) *Status {
	s := &Status{com: com, data: data}
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
	}
	return nil
}

// Draw implements [Dialog].
func (s *Status) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := s.com.Styles
	width := min(statusDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize())
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	rc := NewRenderContext(t, width)
	rc.Title = s.com.L("status.title")
	rc.AddPart(s.renderContent(innerWidth))
	rc.Help = renderDialogHelp(t, nil, s, innerWidth)

	view := rc.Render()
	DrawCenter(scr, area, view)
	return nil
}

func (s *Status) renderContent(width int) string {
	t := s.com.Styles
	section := t.Dialog.PrimaryText
	label := t.Dialog.ListItem.InfoBlurred

	lines := []string{
		section.Render(s.com.L("status.usage")),
	}
	lines = append(lines, s.usageSection(label, width)...)

	lines = append(lines, "", section.Render(s.com.L("status.mcp_servers")))
	lines = append(lines, s.mcpSection(label, width)...)

	lines = append(lines, "", section.Render(s.com.L("status.lsp_servers")))
	lines = append(lines, s.lspSection(label, width)...)

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
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
