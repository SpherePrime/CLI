package model

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/plugins"
	"github.com/SpherePrime/CLI/internal/ui/util"
	"github.com/SpherePrime/CLI/internal/voice"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
)

// voiceRefreshInterval is how often the recording badge refreshes its timer.
const voiceRefreshInterval = 250 * time.Millisecond

// voiceState is where dictation stands.
type voiceState uint8

const (
	voiceIdle voiceState = iota
	voicePreparing
	voiceRecording
	voiceTranscribing
)

// voiceRuntime keeps dictation state on the UI model, so a recording can
// never outlive the screen that shows it. Dictation itself runs in one
// goroutine: capture, silence detection, and transcription. The hotkey can
// end the recording early through stopCh.
type voiceRuntime struct {
	state     voiceState
	settings  voice.Settings
	stopCh    chan struct{}
	stopCtx   context.CancelFunc
	startedAt time.Time
	elapsed   time.Duration
	// warmStarted records that the startup warm-up already ran.
	warmStarted bool
}

// voiceTranscriptMsg carries dictated text (or the failure) back to the
// UI thread.
type voiceTranscriptMsg struct {
	text     string
	engine   string
	err      error
	tooShort bool
}

// voiceTickMsg refreshes the recording timer.
type voiceTickMsg time.Time

// voiceSettings reads the engine options; a model without a workspace (tests,
// early boot) falls back to working defaults.
func (m *UI) voiceSettings() voice.Settings {
	if m.com == nil {
		return voice.Settings{Enabled: true}
	}
	cfg := m.com.Config()
	var resolver voice.VariableResolver
	if m.com.Workspace != nil {
		resolver = m.com.Workspace.Resolver()
	}
	return plugins.SettingsFor(cfg, resolver)
}

// voiceEnabled decides whether the hotkey and its hint are offered: the
// voice plugin must be installed, and the engine options must not disable it.
func (m *UI) voiceEnabled() bool {
	return m.voicePluginOn() && m.voiceSettings().Enabled
}

// WarmVoiceServer loads the Whisper model in the background at startup, so
// the first dictation of a session meets an already-loaded server. It does
// nothing while the voice plugin is not installed.
func (m *UI) WarmVoiceServer() {
	if m.voice.warmStarted || !m.voiceEnabled() {
		return
	}
	m.voice.warmStarted = true
	settings := m.voiceSettings()
	go func() {
		_ = voice.WarmServer(context.Background(), settings)
	}()
}

// toggleVoiceInput starts dictation, and ends the recording early when the
// microphone is already live. Bound to the dictation hotkey.
func (m *UI) toggleVoiceInput() tea.Cmd {
	switch m.voice.state {
	case voiceIdle:
		return m.startVoiceInput()
	case voicePreparing, voiceRecording:
		return m.stopVoiceInput()
	case voiceTranscribing:
		return util.ReportInfo(m.com.L("info.voice_transcribing"))
	}
	return nil
}

func (m *UI) startVoiceInput() tea.Cmd {
	settings := m.voiceSettings()
	if !m.voiceEnabled() {
		return util.ReportWarn(m.com.L("info.voice_disabled"))
	}

	stopCh := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	m.voice.settings = settings
	m.voice.state = voicePreparing
	m.voice.stopCh = stopCh
	m.voice.stopCtx = cancel
	m.status.SetVoiceBadge(m.voiceBadge())

	return func() tea.Msg {
		defer cancel()
		dictation, err := m.voiceDictate(ctx, settings, stopCh)
		if err != nil {
			return voiceTranscriptMsg{err: err}
		}
		return voiceTranscriptMsg{text: dictation.Text, engine: dictation.Engine}
	}
}

// voiceDictate runs one full capture. A plugin switch that is on but whose
// components are missing (never installed, or removed later) triggers the
// download in place instead of failing, so the user only waits once.
func (m *UI) voiceDictate(ctx context.Context, settings voice.Settings, stop chan struct{}) (voice.Dictation, error) {
	detector := voice.NewDetector(settings)
	plan, err := detector.Plan(ctx)
	if errors.Is(err, voice.ErrNoRecorder) || errors.Is(err, voice.ErrNoTranscriber) {
		if plugin, ok := plugins.Get(plugins.VoiceName); ok && plugins.Running(plugin.Name) == nil {
			installCtx, installCancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer installCancel()
			if installErr := plugin.Install(installCtx, settings, nil); installErr != nil {
				return voice.Dictation{}, installErr
			}
			plan, err = detector.Plan(ctx)
		}
	}
	if err != nil {
		return voice.Dictation{}, err
	}
	return plan.DictateWithStop(ctx, voice.DictateOptions{MaxDuration: settings.MaxDurationOr()}, stop)
}

// stopVoiceInput ends a running recording early; what was said so far is
// still transcribed.
func (m *UI) stopVoiceInput() tea.Cmd {
	m.finishStopSignal()
	if m.voice.state != voiceRecording && m.voice.state != voicePreparing {
		return nil
	}
	m.voice.state = voiceTranscribing
	m.status.SetVoiceBadge(m.voiceBadge())
	return nil
}

func (m *UI) finishStopSignal() {
	if m.voice.stopCh != nil {
		close(m.voice.stopCh)
		m.voice.stopCh = nil
	}
}

// cancelVoiceInput drops an in-flight recording without transcribing it.
func (m *UI) cancelVoiceInput() tea.Cmd {
	switch m.voice.state {
	case voicePreparing, voiceRecording:
		if m.voice.stopCtx != nil {
			m.voice.stopCtx()
		}
		m.finishStopSignal()
		m.resetVoice()
		return util.ReportInfo(m.com.L("info.voice_canceled"))
	}
	return nil
}

// dictationActive reports whether the microphone pipeline is in use, which
// is when escape cancels instead of touching the chat view.
func (m *UI) dictationActive() bool {
	return m.voice.state == voicePreparing || m.voice.state == voiceRecording
}

// abortVoiceCapture kills a running capture without waiting for it.
func (m *UI) abortVoiceCapture() {
	m.finishStopSignal()
	if m.voice.stopCtx != nil {
		m.voice.stopCtx()
		m.voice.stopCtx = nil
	}
	m.resetVoice()
}

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

func formatVoiceElapsed(elapsed time.Duration) string {
	seconds := int(elapsed.Round(time.Second) / time.Second)
	return fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
}

// updateVoice handles dictation messages and reports whether the message was
// one of its own.
func (m *UI) updateVoice(msg tea.Msg) (tea.Cmd, bool) {
	switch msg := msg.(type) {
	case voiceTranscriptMsg:
		return m.handleVoiceTranscript(msg), true
	case voiceTickMsg:
		return m.handleVoiceTick(msg), true
	}
	return nil, false
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
	case errors.Is(msg.err, context.Canceled):
		return util.ReportInfo(m.com.L("info.voice_canceled"))
	case errors.Is(msg.err, voice.ErrNoSpeech):
		return util.ReportWarn(m.com.L("info.voice_too_short"))
	case msg.err != nil:
		if errors.Is(msg.err, voice.ErrNoRecorder) || errors.Is(msg.err, voice.ErrNoTranscriber) {
			return util.ReportError(fmt.Errorf("%s: %w", m.com.L("info.voice_needs_setup"), msg.err))
		}
		return util.ReportError(msg.err)
	}

	text := strings.TrimSpace(msg.text)
	if text == "" {
		return util.ReportWarn(m.com.L("info.voice_empty"))
	}

	var cmds []tea.Cmd
	if m.activeInline == nil {
		m.focus = uiFocusEditor
		m.textarea.Focus()
	}
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

func voiceTickCmd() tea.Cmd {
	return tea.Tick(voiceRefreshInterval, func(t time.Time) tea.Msg {
		return voiceTickMsg(t)
	})
}
