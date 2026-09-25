package model

import (
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/util"
	"github.com/SpherePrime/CLI/internal/voice"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestVoiceHotkeyDefaults(t *testing.T) {
	t.Parallel()

	u := newTestUIWithConfig(t, &config.Config{})
	u.keyMap = DefaultKeyMap()
	u.applyVoiceHotkey()

	require.Equal(t, voice.DefaultHotkeys, u.keyMap.Voice.Keys())
	require.NotEmpty(t, u.keyMap.Voice.Help().Desc)
}

func TestVoiceHotkeyFromConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Options: &config.Options{
		Voice: &config.VoiceOptions{Hotkey: "f7, ctrl+shift+space"},
	}}
	u := newTestUIWithConfig(t, cfg)
	u.keyMap = DefaultKeyMap()
	u.applyVoiceHotkey()

	require.Equal(t, []string{"f7", "ctrl+shift+space"}, u.keyMap.Voice.Keys())
	require.Equal(t, "f7", u.keyMap.Voice.Help().Key)
}

func TestToggleVoiceInputRespectsDisabledOption(t *testing.T) {
	t.Parallel()

	off := false
	cfg := &config.Config{Options: &config.Options{
		Voice: &config.VoiceOptions{Enabled: &off},
	}}
	u := newTestUIWithConfig(t, cfg)

	cmd := u.toggleVoiceInput()
	require.NotNil(t, cmd)

	msg, ok := cmd().(util.InfoMsg)
	require.True(t, ok)
	require.Equal(t, util.InfoTypeWarn, msg.Type)
	require.Equal(t, voiceIdle, u.voice.state)
}

func TestVoiceTranscriptLandsInPrompt(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("сделай ")
	u.voice.state = voiceTranscribing

	cmd := u.handleVoiceTranscript(voiceTranscriptMsg{text: "коммит и пуш", engine: "whisper.cpp"})
	require.NotNil(t, cmd)

	require.Equal(t, "сделай коммит и пуш", u.textarea.Value())
	require.Equal(t, voiceIdle, u.voice.state)
	require.Empty(t, u.status.VoiceBadge())
}

func TestVoiceTranscriptOnEmptyPrompt(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.handleVoiceTranscript(voiceTranscriptMsg{text: "  привет мир  "})

	require.Equal(t, "привет мир", u.textarea.Value())
}

func TestVoiceTranscriptFailureKeepsPromptUntouched(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.textarea.SetValue("уже набрано")

	cmd := u.handleVoiceTranscript(voiceTranscriptMsg{err: voice.ErrNoTranscriber})
	require.NotNil(t, cmd)

	msg, ok := cmd().(util.InfoMsg)
	require.True(t, ok)
	require.Equal(t, util.InfoTypeError, msg.Type)
	require.Equal(t, "уже набрано", u.textarea.Value())
	require.Nil(t, u.voice.detector, "a missing engine invalidates the cached probe")
}

func TestVoiceTranscriptTooShortIsWarned(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	cmd := u.handleVoiceTranscript(voiceTranscriptMsg{tooShort: true})
	require.NotNil(t, cmd)

	msg, ok := cmd().(util.InfoMsg)
	require.True(t, ok)
	require.Equal(t, util.InfoTypeWarn, msg.Type)
	require.Empty(t, u.textarea.Value())
}

func TestVoiceBadgeShowsTimer(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	require.Empty(t, u.voiceBadge())

	u.voice.state = voiceRecording
	u.voice.elapsed = 65 * time.Second
	badge := u.voiceBadge()
	require.Contains(t, badge, "1:05")

	u.voice.state = voiceTranscribing
	require.NotEmpty(t, u.voiceBadge())
}

func TestVoiceTickStopsWhenNotRecording(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.voice.state = voiceIdle

	require.Nil(t, u.handleVoiceTick(voiceTickMsg(time.Now())))

	u.voice.state = voiceRecording
	u.voice.startedAt = time.Now().Add(-time.Second)
	require.NotNil(t, u.handleVoiceTick(voiceTickMsg(time.Now())))
	require.Greater(t, u.voice.elapsed, time.Duration(0))
}

func TestCancelVoiceInputWhenIdleDoesNothing(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	require.Nil(t, u.cancelVoiceInput())
	require.False(t, u.dictationActive())
}

func TestAbortVoiceCaptureClearsState(t *testing.T) {
	t.Parallel()

	u := newTestUI()
	u.voice.state = voiceRecording
	u.abortVoiceCapture()

	require.Equal(t, voiceIdle, u.voice.state)
	require.Nil(t, u.voice.session)
	require.Empty(t, u.status.VoiceBadge())
}

func TestVoiceSpacingBeforeTranscript(t *testing.T) {
	t.Parallel()

	require.Equal(t, "text", voiceSpacing("", "text"))
	require.Equal(t, "text", voiceSpacing("already ", "text"))
	require.Equal(t, " text", voiceSpacing("already", "text"))
}

func TestFormatVoiceElapsed(t *testing.T) {
	t.Parallel()

	require.Equal(t, "0:00", formatVoiceElapsed(200*time.Millisecond))
	require.Equal(t, "1:05", formatVoiceElapsed(65*time.Second))
	require.Equal(t, "10:00", formatVoiceElapsed(10*time.Minute))
}
