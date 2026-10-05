package dialog

import (
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// Deleting a provider was impossible from the TUI: the list offered enter to
// open it and nothing else, and the only way to delete one was to leave the app
// and run the CLI command.
func TestProviderSettingsDeleteAsksBeforeRemoving(t *testing.T) {
	t.Parallel()

	d, ws := newProviderTestDialog(t, nil)

	// A key that is not the delete key must not delete anything.
	d.HandleMsg(keyPress("j"))
	require.Equal(t, providerSettingsStateProviders, d.state)
	require.Empty(t, d.deleteTarget)

	d.HandleMsg(keyPress(d.keyMap.Delete.Keys()[0]))
	require.Equal(t, providerSettingsStateDeleteConfirm, d.state)
	require.Equal(t, "osnova", d.deleteTarget)

	// Answering no leaves everything as it was.
	d.HandleMsg(keyPress("n"))
	require.Equal(t, providerSettingsStateProviders, d.state)
	_, still := ws.cfg.Providers.Get("osnova")
	require.True(t, still, "the provider must survive a cancelled delete")
}

// esc is the reflex for backing out of a question, so it must answer no rather
// than close the dialog. Closing instead would throw away the settings screen
// the user came from, which is not what "no" means.
func TestProviderSettingsEscapeCancelsTheDeleteNotTheDialog(t *testing.T) {
	t.Parallel()

	d, _ := newProviderTestDialog(t, nil)
	d.startDelete()
	require.Equal(t, providerSettingsStateDeleteConfirm, d.state)

	action := d.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEscape})
	require.Nil(t, action, "esc must not close the dialog")
	require.Equal(t, providerSettingsStateProviders, d.state)
	require.Empty(t, d.deleteTarget)
}

// While the confirmation is up it owns every key, so an unrelated key cannot
// fall through to the search field and filter the list behind the question.
func TestProviderSettingsConfirmationSwallowsUnrelatedKeys(t *testing.T) {
	t.Parallel()

	d, _ := newProviderTestDialog(t, nil)
	d.startDelete()
	d.input.SetValue("")

	d.HandleMsg(keyPress("q"))
	require.Empty(t, d.input.Value(), "typing must not reach the filter behind the question")
	require.Equal(t, providerSettingsStateDeleteConfirm, d.state)
}

func TestProviderSettingsConfirmEmitsDelete(t *testing.T) {
	t.Parallel()

	d, _ := newProviderTestDialog(t, nil)
	d.startDelete()

	action := d.confirmDelete()
	require.IsType(t, ActionDeleteProvider{}, action)
	require.Equal(t, "osnova", action.(ActionDeleteProvider).ProviderID)
	require.Equal(t, providerSettingsStateProviders, d.state)
	require.Empty(t, d.deleteTarget, "the target must not linger after the action")
}

// Deleting also unpins the agents that were running one of its models, which is
// invisible from a list of providers. Saying so up front is the difference
// between a deliberate change and a surprise found later.
func TestProviderSettingsDeleteConfirmNamesAffectedAgents(t *testing.T) {
	t.Parallel()

	d, ws := newProviderTestDialog(t, nil)
	ws.cfg.Agents = map[string]config.Agent{
		config.AgentCode: {
			ID:            config.AgentCode,
			ModelOverride: &config.SelectedModel{Provider: "osnova", Model: "m"},
		},
		config.AgentPlan: {
			ID:            config.AgentPlan,
			ModelOverride: &config.SelectedModel{Provider: "anthropic", Model: "m"},
		},
	}

	d.startDelete()
	require.Equal(t, "osnova", d.deleteTarget)

	view := d.renderDeleteConfirm(providerTestStyles(), 60)
	require.Contains(t, view, "osnova")
	require.Contains(t, view, "code", "the affected agent must be named")
	require.NotContains(t, view, "plan", "an agent on another provider is unaffected")
}

func TestProviderSettingsDeleteWithNothingHighlighted(t *testing.T) {
	t.Parallel()

	d, _ := newProviderTestDialog(t, nil)
	d.list.SetItems()

	d.startDelete()
	require.Equal(t, providerSettingsStateProviders, d.state,
		"with no provider selected there is nothing to confirm")
}

// keyPress builds a key event from a key name the keymap uses.
//
// key.Matches compares on String(), so the modifier matters: without ModCtrl a
// "ctrl+delete" chord stringifies as plain "delete" and nothing matches.
func keyPress(name string) tea.KeyPressMsg {
	code, mod := rune(0), tea.KeyMod(0)

	if rest, ok := strings.CutPrefix(name, "ctrl+"); ok {
		mod = tea.ModCtrl
		switch rest {
		case "delete":
			code = tea.KeyDelete
		case "backspace":
			code = tea.KeyBackspace
		default:
			code = rune(rest[0])
		}
	} else if len(name) > 0 {
		code = rune(name[len(name)-1])
	}

	return tea.KeyPressMsg{Code: code, Mod: mod}
}