package model

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/attachments"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// mainModelSlot is the model slot the main agent runs on in these tests.
func mainModelSlot(t *testing.T, u *UI) config.SelectedModelType {
	t.Helper()
	cfg := u.com.Config()
	require.NotNil(t, cfg, "the UI must have a config")
	agent, ok := cfg.Agents[config.AgentGeneral]
	require.True(t, ok, "the main agent must be configured")
	return agent.Model
}

// Thinking mode is a property of the main agent's current model, so the
// toggle has to flip it and write it back. Flipping a local copy alone would
// pass a looser test while the next read showed the old value, which is the
// failure a user sees as "the hotkey does nothing".
func TestToggleThinkingFlipsTheMainAgentModel(t *testing.T) {
	u, ws := newAgentFlowUI(t)
	slot := mainModelSlot(t, u)

	require.False(t, u.com.Config().Models[slot].Think, "thinking starts off")

	runCmds(u, u.toggleThinking())
	require.True(t, u.com.Config().Models[slot].Think, "the first toggle turns thinking on")
	require.Contains(t, ws.written, "preferred_model",
		"the flip has to be written back, or the next read still shows it off")

	runCmds(u, u.toggleThinking())
	require.False(t, u.com.Config().Models[slot].Think, "the second toggle turns it back off")
}

// The toggle has two doors. Thinking mode used to be reachable only from the
// command palette, so the hotkey is the newer one and the one that would
// break quietly - a binding nobody presses and nothing asserts on simply
// stops reaching the handler, and the palette entry carries on looking like
// the only way in.
func TestBothDoorsToToggleThinkingAreWired(t *testing.T) {
	t.Run("hotkey", func(t *testing.T) {
		u, _ := newAgentFlowUI(t)
		slot := mainModelSlot(t, u)

		// The flow harness builds the config, not the whole keyboard path.
		// Update reads the attachments pane and the keymap before it reaches
		// the binding under test, and neither is set up here - the embedded
		// workspace interface is nil, so a missing one is a panic rather than
		// a value that simply does not match.
		u.keyMap = DefaultKeyMap()
		u.attachments = attachments.New(nil, attachments.Keymap{})

		_, cmd := u.Update(tea.KeyPressMsg{Code: 't', Mod: tea.ModCtrl | tea.ModShift})
		runCmds(u, cmd)
		require.True(t, u.com.Config().Models[slot].Think,
			"ctrl+shift+t must turn thinking on")
	})

	t.Run("palette", func(t *testing.T) {
		u, ws := newAgentFlowUI(t)
		slot := mainModelSlot(t, u)

		// The palette entry is conditional: only a model that reasons the
		// Anthropic way gets it - CanReason set, with no reasoning levels to
		// pick from, since those open a different dialog. A model without
		// that shape has no entry to press, and the test would pass while
		// proving nothing.
		ws.cfg.Providers.Set("acme", config.ProviderConfig{
			ID:             "acme",
			UserConfigured: true,
			Models: []catwalk.Model{
				{ID: "reasoner", ContextWindow: 200_000, CanReason: true},
			},
		})
		ws.cfg.Models[slot] = config.SelectedModel{Provider: "acme", Model: "reasoner"}

		_ = u.openCommandsDialog()

		// Filter and select the way a user does. Reaching into the list
		// would skip the part where the entry has to survive the filter to
		// be reachable at all.
		_ = u.handleDialogMsg(tea.PasteMsg{Content: "thinking"})
		cmd := u.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
		runCmds(u, cmd)

		require.True(t, u.com.Config().Models[slot].Think,
			"the palette entry must turn thinking on")
	})
}
