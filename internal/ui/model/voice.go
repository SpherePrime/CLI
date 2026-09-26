package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/util"
	"github.com/SpherePrime/CLI/internal/voice"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
)

// voiceRefreshInterval is how often the recording badge refreshes its timer.
const voiceRefreshInterval = 250 * time.Millisecond

// voiceState is where dictation stands.
type voiceState uint8

const (
	// voiceIdle means no capture is running.
	voiceIdle voiceState = iota
	// voicePreparing means the recorder is being found and started.
	voicePreparing
	// voiceRecording means the microphone is live.
	voiceRecording
	// voiceTranscribing means audio is on its way to Whisper.
	voiceTranscribing
)

// voiceRuntime keeps dictation state and the capture it owns. It lives on the
// UI model so a recording cannot outlive the screen that shows it.
type voiceRuntime struct {
	state    voiceState
	settings voice.Settings
	detector *voice.Detector
	session  *voice.Session
	// stopCtx ends the capture process if dictation is abandoned.
	stopCtx   context.CancelFunc
	startedAt time.Time
	elapsed   time.Duration
	// warmStarted records that the background server warm-up already ran, so
	// a keymap rebuild does not launch a second load attempt.
	warmStarted bool
}

// voiceStartedMsg reports that capture is running or failed to start.
type voiceStartedMsg struct {
	session  *voice.Session
	recorder string
	err      error
}

// voiceTranscriptMsg carries dictated text back to the UI thread.
type voiceTranscriptMsg struct {
	text     string
	engine   string
	err      error
	tooShort bool
}

// voiceTickMsg refreshes the recording timer.
type voiceTickMsg time.Time

// voiceLimitMsg fires when a recording hits options.voice.max-duration.
type voiceLimitMsg struct{}

// voiceSettings reads options.voice into pipeline settings. An absent options
// block still yields working defaults.
func (m *UI) voiceSettings() voice.Settings {
	var options *config.VoiceOptions
	if cfg := m.com.Config(); cfg != nil && cfg.Options != nil {
		options = cfg.Options.Voice
	}
	var resolver voice.VariableResolver
	if m.com.Workspace != nil {
		resolver = m.com.Workspace.Resolver()
	}
	return voice.SettingsFrom(options, resolver)
}

// voiceEnabled reports whether dictation is switched on, which decides whether
// the hotkey and its hint are offered at all.
func (m *UI) voiceEnabled() bool {
	return m.voiceSettings().Enabled
}

// applyVoiceHotkey rebinds dictation from options.voice.hotkey. It runs
// wherever the keymap is rebuilt so a config reload takes effect.
func (m *UI) applyVoiceHotkey() {
	m.keyMap.SetVoiceHotkey(m.voiceSettings().Hotkeys())
}

// WarmVoiceServer starts the resident Whisper server in the background so
// the first dictation of a session meets an already-loaded model. Prime
// calls it when the TUI launches; the engine list prefers the server
// automatically once it answers.
func (m *UI) WarmVoiceServer() {
	if m.voice.warmStarted {
		return
	}
	m.voice.warmStarted = true
	settings := m.voiceSettings()
	go func() {
		_ = voice.WarmServer(context.Background(), settings)
	}()
}

// voiceDetector returns a detector reused while the settings are unchanged,
// because finding a recorder costs a process probe.
func (m *UI) voiceDetector(settings voice.Settings) *voice.Detector {
	if m.voice.detector != nil && m.voice.detector.Settings() == settings {
		return m.voice.detector
	}
	detector := voice.NewDetector(settings)
	m.voice.detector = detector
	m.voice.settings = settings
	return detector
}

// invalidateVoiceDetector drops the cached recorder and engine lookup, so the
// next attempt re-reads the machine after a failure or a settings change.
func (m *UI) invalidateVoiceDetector() {
	m.voice.detector = nil
}

// toggleVoiceInput starts capture, and stops it and transcribes when the
// microphone is already live. It is bound to the dictation hotkey.
func (m *UI) toggleVoiceInput() tea.Cmd {
	switch m.voice.state {
	case voiceIdle:
		return m.startVoiceInput()
	case voicePreparing:
		return util.ReportInfo(m.com.L("info.voice_preparing"))
	case voiceTranscribing:
		return util.ReportInfo(m.com.L("info.voice_transcribing"))
	case voiceRecording:
		return m.stopVoiceInput(false)
	}
	return nil
}

// startVoiceInput launches capture off the UI thread, since finding the
// recorder can spawn a probe process.
func (m *UI) startVoiceInput() tea.Cmd {
	settings := m.voiceSettings()
	if !settings.Enabled {
		return util.ReportWarn(m.com.L("info.voice_disabled"))
	}
	m.voice.settings = settings
	m.voice.state = voicePreparing
	m.status.SetVoiceBadge(m.voiceBadge())

	detector := m.voiceDetector(settings)
	ctx, cancel := context.WithCancel(context.Background())
	m.voice.stopCtx = cancel

	return func() tea.Msg {
		plan, err := detector.Plan(ctx)
		if err != nil {
			cancel()
			return voiceStartedMsg{err: err}
		}
		session, err := plan.Start(ctx)
		if err != nil {
			cancel()
			return voiceStartedMsg{err: err}
		}
		return voiceStartedMsg{session: session, recorder: session.Recorder()}
	}
}

// stopVoiceInput ends capture. When discard is set the audio is thrown away
// instead of being sent to Whisper.
func (m *UI) stopVoiceInput(discard bool) tea.Cmd {
	session := m.voice.session
	m.voice.session = nil
	if session == nil {
		m.resetVoice()
		return nil
	}
	m.resetVoice()

	if discard {
		return tea.Batch(
			func() tea.Msg {
				session.Abort()
				return nil
			},
			util.CmdHandler(util.NewInfoMsg(m.com.L("info.voice_canceled"))),
		)
	}

	m.voice.state = voiceTranscribing
	m.status.SetVoiceBadge(m.voiceBadge())

	detector := m.voiceDetector(m.voice.settings)
	ctx := context.Background()

	return func() tea.Msg {
		plan, err := detector.Plan(ctx)
		if err != nil {
			return voiceTranscriptMsg{err: err}
		}
		audio, err := session.Stop()
		if err != nil {
			return voiceTranscriptMsg{err: err}
		}
		if !audio.WorthTranscribing() {
			return voiceTranscriptMsg{tooShort: true}
		}
		text, err := plan.Transcribe(ctx, audio)
		if err != nil {
			return voiceTranscriptMsg{err: err}
		}
		return voiceTranscriptMsg{text: text, engine: plan.Transcriber.Name()}
	}
}

// cancelVoiceInput drops an in-flight recording without transcribing it.
func (m *UI) cancelVoiceInput() tea.Cmd {
	switch m.voice.state {
	case voicePreparing:
		m.abortVoiceCapture()
		m.invalidateVoiceDetector()
		return util.ReportInfo(m.com.L("info.voice_canceled"))
	case voiceRecording:
		return m.stopVoiceInput(true)
	case voiceIdle, voiceTranscribing:
		return nil
	}
	return nil
}

// dictationActive reports whether the microphone or the pipeline is in use,
// which is when escape cancels instead of touching the chat view.
func (m *UI) dictationActive() bool {
	return m.voice.state == voicePreparing || m.voice.state == voiceRecording
}

// abortVoiceCapture kills a running capture without waiting for it.
func (m *UI) abortVoiceCapture() {
	if m.voice.session != nil {
		m.voice.session.Abort()
		m.voice.session = nil
	}
	if m.voice.stopCtx != nil {
		m.voice.stopCtx()
		m.voice.stopCtx = nil
	}
	m.resetVoice()
}

// resetVoice returns dictation to idle and clears the microphone indicator.
func (m *UI) resetVoice() {
	m.voice.state = voiceIdle
	m.voice.elapsed = 0
	m.voice.startedAt = time.Time{}
	m.status.SetVoiceBadge(m.voiceBadge())
}

// voiceBadge is the microphone indicator shown in the status bar.
func (m *UI) voiceBadge() string {
	switch m.voice.state {
	case voicePreparing:
		return m.com.L("voice.preparing")
	case voiceRecording:
		return fmt.Sprintf("%s %s", m.com.L("voice.recording"), formatVoiceElapsed(m.voice.elapsed))
	case voiceTranscribing:
		return m.com.L("voice.transcribing")
	}
	return ""
}

// formatVoiceElapsed renders a recording timer as minutes:seconds.
func formatVoiceElapsed(elapsed time.Duration) string {
	seconds := int(elapsed.Round(time.Second) / time.Second)
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

// updateVoice handles dictation messages and reports whether the message was
// one of its own.
func (m *UI) updateVoice(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case voiceStartedMsg:
		return m.handleVoiceStarted(msg), true
	case voiceTranscriptMsg:
		return m.handleVoiceTranscript(msg), true
	case voiceTickMsg:
		return m.handleVoiceTick(msg), true
	case voiceLimitMsg:
		if m.voice.state != voiceRecording {
			return nil, true
		}
		cmds := []tea.Cmd{util.ReportWarn(m.com.L("info.voice_limit_reached"))}
		if cmd := m.stopVoiceInput(false); cmd != nil {
			cmds = append(cmds, cmd)
		}
		return tea.Batch(cmds...), true
	}
	return nil, false
}

// handleVoiceStarted applies the outcome of a capture launch.
func (m *UI) handleVoiceStarted(msg voiceStartedMsg) tea.Cmd {
	if msg.err != nil {
		m.abortVoiceCapture()
		m.invalidateVoiceDetector()
		return util.ReportError(msg.err)
	}
	if m.voice.state != voicePreparing {
		// Dictation was cancelled while the recorder was starting.
		msg.session.Abort()
		m.abortVoiceCapture()
		return nil
	}
	m.voice.session = msg.session
	m.voice.state = voiceRecording
	m.voice.startedAt = time.Now()
	m.voice.elapsed = 0
	m.status.SetVoiceBadge(m.voiceBadge())

	cmds := []tea.Cmd{
		util.CmdHandler(util.NewInfoMsg(m.com.LSprintf("info.voice_started", msg.recorder))),
		voiceTickCmd(),
	}
	// Dictation belongs in the prompt, so focus the editor even when the key
	// was pressed with the chat or the sidebar focused.
	m.focusVoiceEditor()
	if limit := m.voice.settings.MaxDurationOr(); limit > 0 {
		cmds = append(cmds, voiceLimitCmd(limit))
	}
	return tea.Batch(cmds...)
}

// focusVoiceEditor moves focus to the prompt so the transcript lands where the
// user can see it.
func (m *UI) focusVoiceEditor() {
	if m.activeInline != nil {
		return
	}
	m.focus = uiFocusEditor
	m.textarea.Focus()
}

// handleVoiceTick refreshes the recording timer while the microphone is live.
func (m *UI) handleVoiceTick(_ voiceTickMsg) tea.Cmd {
	if m.voice.state != voiceRecording {
		return nil
	}
	m.voice.elapsed = time.Since(m.voice.startedAt)
	m.status.SetVoiceBadge(m.voiceBadge())
	return voiceTickCmd()
}

// handleVoiceTranscript inserts dictated text into the prompt.
func (m *UI) handleVoiceTranscript(msg voiceTranscriptMsg) tea.Cmd {
	m.resetVoice()

	switch {
	case msg.tooShort:
		return util.ReportWarn(m.com.L("info.voice_too_short"))
	case msg.err != nil:
		if errors.Is(msg.err, voice.ErrNoTranscriber) {
			m.invalidateVoiceDetector()
		}
		return util.ReportError(msg.err)
	}

	text := strings.TrimSpace(msg.text)
	if text == "" {
		return util.ReportWarn(m.com.L("info.voice_empty"))
	}

	var cmds []tea.Cmd
	m.focusVoiceEditor()
	prevHeight := m.textarea.Height()
	m.textarea.InsertString(voiceSpacing(m.textarea.Value(), text))
	if cmd := m.handleTextareaHeightChange(prevHeight); cmd != nil {
		cmds = append(cmds, cmd)
	}
	cmds = append(cmds, util.CmdHandler(util.NewInfoMsg(m.com.LSprintf("info.voice_done", msg.engine))))
	return tea.Batch(cmds...)
}

// voiceSpacing keeps a space between existing prompt text and a transcript
// appended to it.
func voiceSpacing(existing string, text string) string {
	if existing == "" || text == "" {
		return text
	}
	if strings.HasSuffix(existing, " ") {
		return text
	}
	return " " + text
}

// voiceTickCmd schedules the next recording timer refresh.
func voiceTickCmd() tea.Cmd {
	return tea.Tick(voiceRefreshInterval, func(t time.Time) tea.Msg {
		return voiceTickMsg(t)
	})
}

// voiceLimitCmd stops a recording that ran into options.voice.max-duration.
func voiceLimitCmd(after time.Duration) tea.Cmd {
	return tea.Tick(after, func(time.Time) tea.Msg {
		return voiceLimitMsg{}
	})
}
