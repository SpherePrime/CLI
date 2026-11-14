package model

import (
	"context"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/account"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/internal/ui/util"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/pkg/browser"
)

const (
	authPollInterval    = time.Second
	authSyncInterval    = 2 * time.Second
	authMaxCardWidth    = 72
	authMinCardWidth    = 28
	authCompactHeightAt = 21
)

type authStateMsg struct {
	State       account.State
	Gen         uint64
	FromRestore bool
	FromLogout  bool
	Err         error
}

type authPollTickMsg struct {
	gen uint64
}

type authSyncTickMsg struct {
	seq uint64
}

type authOpenURLResultMsg struct {
	Err error
}

func (m *UI) SetAccountService(svc *account.Service) {
	m.accountSvc = svc
	if m.authGen == 0 {
		m.authGen = 1
	}
}

func (m *UI) CloseAccountService() {
	if m == nil || m.accountSvc == nil {
		return
	}
	m.accountSvc.Close()
}

func (m *UI) nextAuthGen() uint64 {
	m.authGen++
	return m.authGen
}

func (m *UI) currentAuthGen() uint64 {
	if m.authGen == 0 {
		m.authGen = 1
	}
	return m.authGen
}

func (m *UI) restoreAccountCmd() tea.Cmd {
	svc := m.accountSvc
	if svc == nil {
		return nil
	}
	gen := m.nextAuthGen()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return authStateMsg{
			State:       svc.Restore(ctx),
			Gen:         gen,
			FromRestore: true,
		}
	}
}

func (m *UI) startAccountLoginCmd() tea.Cmd {
	svc := m.accountSvc
	if svc == nil {
		return nil
	}
	gen := m.nextAuthGen()
	return func() tea.Msg {
		st, err := svc.StartLogin(context.Background())
		return authStateMsg{
			State: st,
			Gen:   gen,
			Err:   err,
		}
	}
}

func (m *UI) cancelOrLogoutAccountCmd(fromLogout bool) tea.Cmd {
	svc := m.accountSvc
	gen := m.nextAuthGen()
	if svc == nil {
		return func() tea.Msg {
			return authStateMsg{Gen: gen, FromLogout: fromLogout}
		}
	}
	return func() tea.Msg {
		return authStateMsg{
			State:      svc.CancelOrLogout(context.Background()),
			Gen:        gen,
			FromLogout: fromLogout,
		}
	}
}

func (m *UI) syncAccountStateCmd() tea.Cmd {
	svc := m.accountSvc
	if svc == nil {
		return nil
	}
	gen := m.currentAuthGen()
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return authStateMsg{
			State: svc.State(ctx),
			Gen:   gen,
		}
	}
}

func (m *UI) scheduleAuthPollTick() tea.Cmd {
	if m.state != uiAuthGate || m.accountSvc == nil {
		return nil
	}
	m.authPollGen++
	gen := m.authPollGen
	return tea.Tick(authPollInterval, func(time.Time) tea.Msg {
		return authPollTickMsg{gen: gen}
	})
}

func (m *UI) scheduleAuthSyncTick() tea.Cmd {
	if m.state == uiAuthGate || m.accountSvc == nil {
		return nil
	}
	m.authSyncSeq++
	seq := m.authSyncSeq
	return tea.Tick(authSyncInterval, func(time.Time) tea.Msg {
		return authSyncTickMsg{seq: seq}
	})
}

func (m *UI) handleAuthStateMsg(msg authStateMsg) []tea.Cmd {
	if msg.Gen != 0 && msg.Gen != m.authGen {
		return nil
	}

	var cmds []tea.Cmd
	if msg.Err != nil && msg.State.Error == "" {
		msg.State.Error = msg.Err.Error()
	}
	prevURL := m.authState.LoginURL
	m.authState = msg.State
	if msg.State.LoginURL != prevURL || !msg.State.Waiting {
		m.authCopied = false
	}

	if msg.State.SignedIn {
		if m.state == uiAuthGate {
			cmds = append(cmds, m.completeAuthGate()...)
		}
		return cmds
	}

	if m.state != uiAuthGate {
		cmds = append(cmds, m.enterAuthGate(msg.FromLogout)...)
	}
	return cmds
}

func (m *UI) completeAuthGate() []tea.Cmd {
	var cmds []tea.Cmd
	m.authCopied = false
	m.authPollGen++

	target := m.authPostLoginState
	if target != uiInitialize && target != uiLanding {
		target = uiLanding
	}
	if m.com != nil && m.com.Workspace != nil {
		if needsInit, _ := m.com.Workspace.ProjectNeedsInitialization(); needsInit {
			target = uiInitialize
		} else if target == uiInitialize {
			target = uiLanding
		}
	}

	m.setState(target, uiFocusEditor)
	if target == uiLanding {
		cmds = append(cmds, m.textarea.Focus())
	}
	if m.authDeferredInit {
		m.authDeferredInit = false
		cmds = append(cmds, m.deferredInitCmds()...)
	}
	if cmd := m.scheduleAuthSyncTick(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return cmds
}

func (m *UI) enterAuthGate(fromLogout bool) []tea.Cmd {
	var cmds []tea.Cmd
	m.authSyncSeq++
	m.authCopied = false
	if m.dialog != nil && m.dialog.HasDialogs() {
		cmds = append(cmds, m.dialog.CloseAllDialogs()...)
	}
	m.activeInline = nil
	m.completionsOpen = false
	m.detailsOpen = false
	m.session = nil
	if m.chat != nil {
		m.chat.Blur()
		m.chat.ClearMessages()
	}
	if m.textarea.Focused() {
		m.textarea.Blur()
	}
	m.setState(uiAuthGate, uiFocusNone)
	if fromLogout {
		cmds = append(cmds, util.ReportInfo(m.com.L("auth.logged_out")))
	}
	if cmd := m.scheduleAuthPollTick(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return cmds
}

func (m *UI) logoutAccount() tea.Cmd {
	m.authState = account.State{}
	return m.cancelOrLogoutAccountCmd(true)
}

func (m *UI) updateAuthGate(msg tea.KeyPressMsg) tea.Cmd {
	switch msg.String() {
	case "q", "ctrl+c":
		return tea.Quit
	case "c":
		if m.authState.Waiting && m.authState.LoginURL != "" {
			m.authCopied = true
			return common.CopyToClipboard(m.authState.LoginURL, m.com.L("auth.copied_url"))
		}
	case "o":
		if m.authState.Waiting && m.authState.LoginURL != "" {
			loginURL := m.authState.LoginURL
			return func() tea.Msg {
				return authOpenURLResultMsg{Err: browser.OpenURL(loginURL)}
			}
		}
	case "esc", "alt+esc":
		if m.authState.Waiting {
			m.authState.Waiting = false
			m.authState.Error = ""
			m.authCopied = false
			return m.cancelOrLogoutAccountCmd(false)
		}
	case "r":
		if m.authState.NetworkError && m.authState.HasStoredSession && !m.authState.Checking && !m.authState.Waiting {
			m.authState.Waiting = true
			m.authState.NetworkError = false
			m.authState.Error = ""
			return m.startAccountLoginCmd()
		}
	case "enter":
		switch {
		case m.authState.Checking:
			return nil
		case m.authState.Waiting:
			m.authState.Waiting = false
			m.authState.Error = ""
			m.authCopied = false
			return m.cancelOrLogoutAccountCmd(false)
		case m.authState.NetworkError && m.authState.HasStoredSession:
			m.authState.Checking = true
			m.authState.Error = ""
			return m.restoreAccountCmd()
		default:
			m.authState.Waiting = true
			m.authState.Error = ""
			return m.startAccountLoginCmd()
		}
	}
	return nil
}

func (m *UI) authGateView() string {
	t := m.com.Styles
	availW := max(authMinCardWidth, m.layout.main.Dx())
	availH := max(1, m.layout.main.Dy())
	cardWidth := min(authMaxCardWidth, availW)
	compact := availH <= authCompactHeightAt

	padV := 1
	if compact {
		padV = 0
	}
	cardStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.Dialog.View.GetBorderTopForeground()).
		Padding(padV, 2).
		Width(cardWidth)

	innerWidth := max(16, cardWidth-cardStyle.GetHorizontalFrameSize())

	var sections []string
	sections = append(sections, t.Dialog.TitleText.Bold(true).Width(innerWidth).Render(m.com.L("auth.title")))
	if availH >= 15 {
		sections = append(sections, t.Dialog.OAuth.StatusText.Width(innerWidth).Render(m.com.L("auth.subtitle")))
	}
	sections = append(sections, m.renderAuthSteps(innerWidth))
	sections = append(sections, m.renderAuthStatus(innerWidth))

	if m.authState.Waiting && m.authState.LoginURL != "" {
		urlText := m.authState.LoginURL
		if compact {
			urlText = ansi.Truncate(urlText, innerWidth, "…")
		}
		if availH >= 12 {
			sections = append(sections,
				lipgloss.JoinVertical(
					lipgloss.Left,
					t.Dialog.OAuth.StatusText.Width(innerWidth).Render(m.com.L("auth.open_url_hint")),
					t.Dialog.OAuth.Link.Width(innerWidth).Render(urlText),
				),
			)
		} else {
			sections = append(sections, t.Dialog.OAuth.Link.Width(innerWidth).Render(urlText))
		}
	}

	if actions := m.renderAuthActions(innerWidth, compact); actions != "" {
		sections = append(sections, actions)
	}

	sections = append(sections, t.Dialog.OAuth.StatusText.Width(innerWidth).Render("[q / ctrl+c] "+m.com.L("auth.quit")))

	sep := "\n\n"
	if compact {
		sep = "\n"
	}
	card := cardStyle.Render(strings.Join(sections, sep))
	if availH >= lipgloss.Height(card) && availW >= lipgloss.Width(card) {
		return lipgloss.Place(availW, availH, lipgloss.Center, lipgloss.Center, card)
	}
	return lipgloss.NewStyle().MaxWidth(availW).MaxHeight(availH).Render(card)
}

func (m *UI) renderAuthSteps(width int) string {
	t := m.com.Styles

	type stepState uint8
	const (
		stepPending stepState = iota
		stepActive
		stepDone
		stepError
	)

	s1, s2, s3 := stepActive, stepPending, stepPending
	switch {
	case m.authState.SignedIn:
		s1, s2, s3 = stepDone, stepDone, stepDone
	case m.authState.Waiting:
		s1, s2, s3 = stepDone, stepActive, stepPending
	case m.authState.Checking:
		s1, s2, s3 = stepDone, stepActive, stepPending
	case m.authState.NetworkError && m.authState.HasStoredSession:
		s1, s2, s3 = stepDone, stepError, stepPending
	}

	renderStep := func(label string, st stepState) string {
		switch st {
		case stepDone:
			return t.Tool.IconSuccess.Render() + " " + t.Dialog.OAuth.Success.Render(label)
		case stepActive:
			return t.Dialog.Spinner.Render(styles.SpinnerIcon) + " " + t.Dialog.TitleText.Bold(true).Render(label)
		case stepError:
			return t.Tool.IconError.Render() + " " + t.Dialog.OAuth.ErrorText.Render(label)
		default:
			return t.Resource.OfflineIcon.Render() + " " + t.Dialog.OAuth.StatusText.Render(label)
		}
	}

	steps := []string{
		renderStep(m.com.L("auth.step_browser"), s1),
		renderStep(m.com.L("auth.step_confirm"), s2),
		renderStep(m.com.L("auth.step_ready"), s3),
	}

	for _, pad := range []string{"  ", " "} {
		sep := pad + t.Dialog.OAuth.StatusText.Render(styles.ArrowRightIcon) + pad
		horizontal := strings.Join(steps, sep)
		if lipgloss.Width(horizontal) <= width {
			return horizontal
		}
	}
	return strings.Join(steps, "\n")
}

func (m *UI) renderAuthStatus(width int) string {
	t := m.com.Styles
	switch {
	case m.authState.Checking:
		return t.Dialog.OAuth.StatusText.Width(width).Render(styles.LoadingIcon + " " + m.com.L("auth.status_checking"))
	case m.authState.Waiting:
		return t.Dialog.OAuth.Instructions.Width(width).Render(styles.SpinnerIcon + " " + m.com.L("auth.status_waiting"))
	case m.authState.SignedIn:
		text := styles.CheckIcon + " " + m.com.L("auth.status_signed_in")
		if m.authState.Email != "" {
			text += ": " + m.authState.Email
		}
		return t.Dialog.OAuth.Success.Width(width).Render(text)
	case m.authState.NetworkError && m.authState.HasStoredSession:
		msg := m.com.L("auth.status_network_error")
		if m.authState.Error != "" {
			msg += "\n" + m.authState.Error
		}
		return t.Dialog.OAuth.ErrorText.Width(width).Render(msg)
	case m.authState.Error != "":
		return t.Dialog.OAuth.ErrorText.Width(width).Render(m.authState.Error)
	default:
		return t.Dialog.OAuth.StatusText.Width(width).Render(m.com.L("auth.status_idle"))
	}
}

func (m *UI) renderAuthActions(width int, compact bool) string {
	t := m.com.Styles
	switch {
	case m.authState.Checking:
		return ""
	case m.authState.Waiting:
		buttons := common.ButtonGroup(t, []common.ButtonOpts{
			{Text: m.com.L("auth.cancel") + " (Esc)", Selected: true},
		}, "  ")
		if m.authState.LoginURL == "" {
			return buttons
		}
		copyLabel := m.com.L("auth.copy_url")
		if m.authCopied {
			copyLabel = m.com.L("auth.copied_url")
		}
		hints := t.Dialog.OAuth.StatusText.Width(width).Render(
			"[c] " + copyLabel + "   [o] " + m.com.L("auth.open_again"),
		)
		if compact {
			return lipgloss.JoinVertical(lipgloss.Left, buttons, hints)
		}
		return lipgloss.JoinVertical(lipgloss.Left, buttons, "", hints)
	case m.authState.NetworkError && m.authState.HasStoredSession:
		return common.ButtonGroup(t, []common.ButtonOpts{
			{Text: m.com.L("auth.retry_check") + " (Enter)", Selected: true},
			{Text: m.com.L("auth.sign_in_again") + " (r)", Selected: false},
		}, "  ")
	default:
		return common.ButtonGroup(t, []common.ButtonOpts{
			{Text: m.com.L("auth.sign_in_google") + " (Enter)", Selected: true},
		}, "  ")
	}
}

func (m *UI) authGateHelpBindings() []key.Binding {
	switch {
	case m.authState.Checking:
		return []key.Binding{
			key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", m.com.L("auth.quit"))),
		}
	case m.authState.Waiting:
		binds := []key.Binding{
			key.NewBinding(key.WithKeys("esc", "enter"), key.WithHelp("esc", m.com.L("auth.cancel"))),
		}
		if m.authState.LoginURL != "" {
			copyLabel := m.com.L("auth.copy_url")
			if m.authCopied {
				copyLabel = m.com.L("auth.copied_url")
			}
			binds = append(binds,
				key.NewBinding(key.WithKeys("c"), key.WithHelp("c", copyLabel)),
				key.NewBinding(key.WithKeys("o"), key.WithHelp("o", m.com.L("auth.open_again"))),
			)
		}
		binds = append(binds, key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", m.com.L("auth.quit"))))
		return binds
	case m.authState.NetworkError && m.authState.HasStoredSession:
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", m.com.L("auth.retry_check"))),
			key.NewBinding(key.WithKeys("r"), key.WithHelp("r", m.com.L("auth.sign_in_again"))),
			key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", m.com.L("auth.quit"))),
		}
	default:
		return []key.Binding{
			key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", m.com.L("auth.sign_in_google"))),
			key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/ctrl+c", m.com.L("auth.quit"))),
		}
	}
}
