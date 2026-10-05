package model

import (
	"image"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// drawWorkers paints the open worker list for real and returns everything it
// put on screen.
//
// Asserting on the rendered rows rather than on config is the whole point of
// these tests: the complaint being pinned down is that the change was made,
// the config was right, and the list in front of the user said otherwise. A
// test that reads the config back cannot see that.
func drawWorkers(t *testing.T, u *UI) string {
	t.Helper()

	workers, ok := u.dialog.Dialog(dialog.AgentsID).(*dialog.Agents)
	require.True(t, ok, "the worker list must be open")

	scr := uv.NewScreenBuffer(120, 40)
	workers.Draw(scr, image.Rect(0, 0, 120, 40))

	var sb strings.Builder
	for y := range scr.Bounds().Dy() {
		for x := range scr.Bounds().Dx() {
			if cell := scr.CellAt(x, y); cell != nil {
				sb.WriteString(cell.Content)
			}
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

// The list deliberately stays open after a pin so several workers can be
// configured in one visit. It builds its rows when it opens, so the pin has to
// be handed back to it - otherwise the row just set keeps reading as unpinned,
// which from the terminal is the same as the change never having happened.
func TestTheOpenWorkerListShowsAPinThatWasJustSet(t *testing.T) {
	u, ws := newAgentFlowUI(t)

	require.Nil(t, u.openAgentsDialog())
	// Enter opens the picker for the highlighted worker, the way a user does it.
	_ = u.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, u.dialog.ContainsDialog(dialog.ModelsID))

	cmd := u.handleSetAgentModel(dialog.ActionSetAgentModel{
		AgentID: config.AgentCode,
		Model:   config.SelectedModel{Provider: "acme", Model: "slow"},
	})
	require.NotNil(t, cmd)

	require.Equal(t, "slow", ws.cfg.Agents[config.AgentCode].ModelOverride.Model,
		"the pin is in the config")
	require.Contains(t, drawWorkers(t, u), "slow",
		"the list that is still on screen must show the pin that was just set")
}

// Choosing Default is a decision like picking a model, so it has to close the
// picker and leave the list following the main agent again.
func TestChoosingDefaultClosesThePickerAndTheListFollows(t *testing.T) {
	u, ws := newAgentFlowUI(t)

	require.Nil(t, u.openAgentsDialog())
	_ = u.handleSetAgentModel(dialog.ActionSetAgentModel{
		AgentID: config.AgentCode,
		Model:   config.SelectedModel{Provider: "acme", Model: "slow"},
	})
	require.Contains(t, drawWorkers(t, u), "slow")

	// Reopen the picker on the same worker and choose Default, which is the
	// ActionClearAgentModel the Default row emits.
	_ = u.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.True(t, u.dialog.ContainsDialog(dialog.ModelsID))

	_ = u.handleClearAgentModel(dialog.ActionClearAgentModel{AgentID: config.AgentCode})

	require.False(t, u.dialog.ContainsDialog(dialog.ModelsID),
		"the picker must close when Default is chosen, or the choice reads as unregistered")
	require.Nil(t, ws.cfg.Agents[config.AgentCode].ModelOverride,
		"the pin is gone from the config")
	require.NotContains(t, drawWorkers(t, u), "slow",
		"the list must stop showing a pin that no longer exists")
}

// Rebuilding the rows must not throw the cursor back to the top: pinning the
// second worker and having the selection jump to the first means the next
// keypress configures a different agent than the one just configured.
func TestRefreshingTheWorkerListKeepsTheRowUnderTheCursor(t *testing.T) {
	u, _ := newAgentFlowUI(t)

	require.Nil(t, u.openAgentsDialog())

	// Move off the first worker so a reset to the top would be visible.
	agents, ok := u.dialog.Dialog(dialog.AgentsID).(*dialog.Agents)
	require.True(t, ok)
	agents.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})

	cmd := u.handleSetAgentModel(dialog.ActionSetAgentModel{
		AgentID: config.AgentTask,
		Model:   config.SelectedModel{Provider: "acme", Model: "slow"},
	})
	require.NotNil(t, cmd)

	// ctrl+x acts on the highlighted row, so it reports which worker is
	// selected without reaching into the dialog's private list.
	action := agents.HandleMsg(tea.KeyPressMsg{Code: 'x', Mod: tea.ModCtrl})
	require.IsType(t, dialog.ActionClearAgentModel{}, action,
		"the refreshed list must still have the worker that was pinned selected")
	require.Equal(t, config.AgentTask, action.(dialog.ActionClearAgentModel).AgentID,
		"the cursor has to stay on the worker that was just configured")
}
