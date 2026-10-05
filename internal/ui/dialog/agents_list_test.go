package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type agentsDialogWorkspace struct {
	workspace.Workspace
	cfg config.Config
}

func (w *agentsDialogWorkspace) Config() *config.Config     { return &w.cfg }
func (w *agentsDialogWorkspace) Language() string           { return "en" }
func (w *agentsDialogWorkspace) WorkingDir() string         { return "/tmp" }
func (w *agentsDialogWorkspace) Resolver() config.VariableResolver { return nil }

func newAgentsDialog(t *testing.T) (*Agents, *agentsDialogWorkspace) {
	t.Helper()

	cfg := config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Agents:    map[string]config.Agent{},
		Options:   &config.Options{},
	}
	cfg.SetupAgents()

	ws := &agentsDialogWorkspace{cfg: cfg}
	return NewAgents(&common.Common{Workspace: ws, Styles: providerTestStyles()}), ws
}

// The main agent is not configurable here. Its model comes from the model
// picker, so an entry for it in this list was a second place to change the same
// setting - and one that read as configuring a delegate while actually
// switching the session's own model.
func TestAgentsDialogHidesTheMainAgent(t *testing.T) {
	d, _ := newAgentsDialog(t)

	for _, id := range d.agentIDs {
		require.NotEqual(t, config.AgentGeneral, id,
			"the main agent must not be listed as a configurable worker")
	}

	for i := range len(d.agentIDs) + 1 {
		if item, ok := d.list.ItemAt(i).(*AgentsItem); ok {
			require.NotEqual(t, config.AgentGeneral, item.agentID)
		}
	}

	// Every worker that can be delegated to must still be reachable.
	require.Len(t, d.agentIDs, 3)
	for _, id := range []string{config.AgentCode, config.AgentTask, config.AgentPlan} {
		require.Contains(t, d.agentIDs, id)
	}
}

// Default has to be the first row: it is the setting most workers belong on, so
// it cannot be something reached only by scrolling.
func TestAgentsDialogOffersDefaultFirst(t *testing.T) {
	d, _ := newAgentsDialog(t)

	item, ok := d.list.ItemAt(0).(*AgentsDefaultItem)
	require.True(t, ok, "the first row must be Default, got %T", d.list.ItemAt(0))
	require.Equal(t, AgentsDefaultID, item.ID())
}

// Picking it must clear every worker's pin, not open a model picker: it is the
// absence of a pin, applied to all of them at once.
func TestAgentsDialogDefaultClearsEveryWorker(t *testing.T) {
	d, _ := newAgentsDialog(t)

	action := d.activate(0)
	require.IsType(t, ActionSetAllAgentsDefault{}, action)

	ids := action.(ActionSetAllAgentsDefault).AgentIDs
	require.ElementsMatch(t, []string{config.AgentCode, config.AgentTask, config.AgentPlan}, ids)
}

// The row must not claim everything already follows the main model when some
// worker is still pinned.
func TestAgentsDialogDefaultReportsPinnedWorkers(t *testing.T) {
	d, ws := newAgentsDialog(t)

	item := d.list.ItemAt(0).(*AgentsDefaultItem)
	// Nothing pinned: the row states the setting rather than offering an action.
	require.Contains(t, item.Render(60), "Follow the main agent")

	for _, id := range []string{config.AgentCode, config.AgentPlan} {
		agent := ws.cfg.Agents[id]
		agent.ModelOverride = &config.SelectedModel{Provider: "p", Model: "m"}
		ws.cfg.Agents[id] = agent
	}

	require.Contains(t, item.Render(60), "2",
		"the row must say how many workers are pinned")
}

// ctrl+x on the Default row must do the same thing as picking it, or the same
// intent would have two different results depending on the row.
func TestAgentsDialogClearKeyOnDefaultRow(t *testing.T) {
	d, _ := newAgentsDialog(t)

	require.IsType(t, ActionSetAllAgentsDefault{}, d.clearPin())
}

// Selecting a worker still opens its model picker.
func TestAgentsDialogWorkerOpensModelPicker(t *testing.T) {
	d, _ := newAgentsDialog(t)

	action := d.activate(1)
	require.IsType(t, ActionOpenAgentModel{}, action)
	require.Equal(t, config.AgentCode, action.(ActionOpenAgentModel).AgentID)
}