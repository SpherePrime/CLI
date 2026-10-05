package model

import (
	"context"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// agentPickWorkspace records what the UI writes back to the config.
type agentPickWorkspace struct {
	workspace.Workspace
	cfg        *config.Config
	writtenKey string
}

func (w *agentPickWorkspace) Config() *config.Config        { return w.cfg }
func (w *agentPickWorkspace) Language() string              { return "en" }
func (w *agentPickWorkspace) WorkingDir() string            { return "/tmp" }
func (w *agentPickWorkspace) Resolver() config.VariableResolver { return nil }

func (w *agentPickWorkspace) SetConfigField(scope config.Scope, key string, value any) error {
	w.writtenKey = key
	return nil
}

func (w *agentPickWorkspace) UpdateAgentModel(context.Context) error { return nil }

func (w *agentPickWorkspace) AgentSetMain(string) error { return nil }
func (w *agentPickWorkspace) AgentIsBusy() bool         { return false }

func newAgentPickUI(t *testing.T) (*UI, *agentPickWorkspace) {
	t.Helper()

	cfg := &config.Config{
		Options: &config.Options{DisabledTools: []string{}},
		Agents:  map[string]config.Agent{},
	}
	cfg.SetupAgents()

	ws := &agentPickWorkspace{cfg: cfg}
	com := common.DefaultCommon(ws)
	u := newTestUI()
	u.com = com
	u.dialog = dialog.NewOverlay()
	return u, ws
}

// Picking a model for one worker used to close the agent list too, so a second
// worker could not be configured without reopening the whole dialog first. That
// reads as "choosing a sub-agent's model does not work" rather than as a missing
// keystroke: the first one appears to stick and everything after it does not.
func TestAgentModelPickLeavesTheAgentListOpen(t *testing.T) {
	u, ws := newAgentPickUI(t)

	require.Nil(t, u.openAgentsDialog())
	require.True(t, u.dialog.ContainsDialog(dialog.AgentsID))

	// Pick a concrete model for the code worker.
	cmd := u.handleSetAgentModel(dialog.ActionSetAgentModel{
		AgentID: config.AgentCode,
		Model:   config.SelectedModel{Provider: "p", Model: "m"},
	})
	_ = cmd

	require.Equal(t, "agents.code", ws.writtenKey)
	require.Equal(t, "m", ws.cfg.Agents[config.AgentCode].ModelOverride.Model)

	require.False(t, u.dialog.ContainsDialog(dialog.ModelsID),
		"the model picker must close once a model is chosen")
	require.True(t, u.dialog.ContainsDialog(dialog.AgentsID),
		"the agent list must stay open so another worker can be set next")
}

// Clearing a pin is the same operation and has the same requirement: the list
// has to survive it, or Default can only ever be used once per session.
func TestClearingAPinLeavesTheAgentListOpen(t *testing.T) {
	u, ws := newAgentPickUI(t)

	code := ws.cfg.Agents[config.AgentCode]
	code.ModelOverride = &config.SelectedModel{Provider: "p", Model: "m"}
	ws.cfg.Agents[config.AgentCode] = code

	require.Nil(t, u.openAgentsDialog())
	_ = u.handleClearAgentModel(dialog.ActionClearAgentModel{AgentID: config.AgentCode})

	require.Nil(t, ws.cfg.Agents[config.AgentCode].ModelOverride)
	require.True(t, u.dialog.ContainsDialog(dialog.AgentsID),
		"the agent list must stay open after clearing a pin")
}