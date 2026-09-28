package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/internal/workspace"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestVoiceLanguageTag(t *testing.T) {
	t.Parallel()

	require.Equal(t, autoVoiceTag, voiceLanguageTag(""))
	require.Equal(t, autoVoiceTag, voiceLanguageTag(" auto "))
	require.Equal(t, "ru-ru", voiceLanguageTag("RU-RU"))
	require.Equal(t, "ru-ru,en-us", voiceLanguageTag("ru-RU, en-US"))
}

func voiceLanguageTestCommon(cfg *config.Config) *common.Common {
	s := styles.ColorTonePantera()
	return &common.Common{
		Workspace: &voiceLanguageWorkspace{cfg: cfg},
		Styles:    &s,
	}
}

// Selecting the item marked current returns its raw tag and display label.
func TestVoiceLanguageSelectReturnsTag(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Options: &config.Options{
		Voice: &config.VoiceOptions{Language: "en-US"},
	}}
	d := NewVoiceLanguage(voiceLanguageTestCommon(cfg))

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok := action.(ActionSelectVoiceLanguage)
	require.True(t, ok, "enter must select the current item")
	require.Equal(t, "en-US", selected.Tag)
	require.NotEmpty(t, selected.Label)

	// Stepping to the Russian item selects it instead.
	d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyUp}) // ru-RU
	action = d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok = action.(ActionSelectVoiceLanguage)
	require.True(t, ok)
	require.Equal(t, "ru-RU", selected.Tag)
}

// A missing voice options block is not an error: the picker defaults to auto.
func TestVoiceLanguageDefaultsToAutoWithoutConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{Options: &config.Options{}}
	d := NewVoiceLanguage(voiceLanguageTestCommon(cfg))

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	selected, ok := action.(ActionSelectVoiceLanguage)
	require.True(t, ok)
	require.Equal(t, autoVoiceTag, selected.Tag)
}

// voiceLanguageWorkspace is a minimal workspace serving a fixed config.
type voiceLanguageWorkspace struct {
	workspace.Workspace
	cfg *config.Config
}

func (w *voiceLanguageWorkspace) Config() *config.Config {
	return w.cfg
}

func (w *voiceLanguageWorkspace) Language() string {
	if w.cfg.Options != nil && w.cfg.Options.Language != "" {
		return w.cfg.Options.Language
	}
	return "en"
}
