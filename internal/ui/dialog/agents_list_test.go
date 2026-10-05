package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// agentsDialogWorkspace is a minimal workspace: the dialog only reads config.
type agentsDialogWorkspace struct {
	workspace.Workspace
	cfg config.Config
}

func (w *agentsDialogWorkspace) Config() *config.Config            { return &w.cfg }
func (w *agentsDialogWorkspace) Language() string                  { return "en" }
func (w *agentsDialogWorkspace) WorkingDir() string                { return "/tmp" }
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

// The main agent is not configurable in this list. Its model comes from the
// model picker, so an entry here was a second place to change the same setting,
// and one that read as configuring a delegate while actually switching the
// session's own model.
func TestAgentsDialogHidesTheMainAgent(t *testing.T) {
	d, _ := newAgentsDialog(t)

	for _, id := range d.agentIDs {
		require.NotEqual(t, config.AgentGeneral, id,
			"the main agent must not be listed as a configurable worker")
	}

	// Every worker that can be delegated to must still be reachable.
	require.Len(t, d.agentIDs, 3)
	for _, id := range []string{config.AgentCode, config.AgentTask, config.AgentPlan} {
		require.Contains(t, d.agentIDs, id)
	}
}

// The list holds exactly the workers: no Default row, and no gap left behind by
// one. Picking the same choice from two places meant picking it here cleared
// every worker rather than opening a picker, which read as enter doing nothing.
func TestAgentsDialogHoldsOnlyWorkers(t *testing.T) {
	d, _ := newAgentsDialog(t)

	require.Equal(t, d.list.Len(), len(d.agentIDs),
		"the list must contain the workers and nothing else")

	for i := range d.list.Len() {
		item, ok := d.list.ItemAt(i).(*AgentsItem)
		require.True(t, ok, "row %d must be a worker, got %T", i, d.list.ItemAt(i))
		require.NotEmpty(t, item.agent.Name)
	}
}

// Selecting a worker opens that worker's model picker, and nothing else does.
func TestAgentsDialogEveryRowOpensItsOwnPicker(t *testing.T) {
	d, _ := newAgentsDialog(t)

	for i, want := range d.agentIDs {
		action := d.activate(i)
		require.IsType(t, ActionOpenAgentModel{}, action, "row %d", i)
		require.Equal(t, want, action.(ActionOpenAgentModel).AgentID,
			"row %d must open the picker for the worker on it", i)
	}
}

// ctrl+x clears the highlighted worker's pin, and does nothing when there is no
// pin to clear rather than reporting a change that did not happen.
func TestAgentsDialogCtrlXClearsOnlyPinnedWorkers(t *testing.T) {
	d, ws := newAgentsDialog(t)
	d.list.SetSelected(0)

	require.Nil(t, d.clearPin(), "nothing pinned means nothing to clear")

	code := ws.cfg.Agents[config.AgentCode]
	code.ModelOverride = &config.SelectedModel{Provider: "p", Model: "m"}
	ws.cfg.Agents[config.AgentCode] = code
	d.setAgentsItems()

	action := d.clearPin()
	require.IsType(t, ActionClearAgentModel{}, action)
	require.Equal(t, config.AgentCode, action.(ActionClearAgentModel).AgentID)
}

// The row has to say what the worker actually runs on, so a pinned one is not
// mistaken for a following one.
func TestAgentsDialogRowShowsTheResolvedModel(t *testing.T) {
	d, ws := newAgentsDialog(t)

	item, ok := d.list.ItemAt(0).(*AgentsItem)
	require.True(t, ok)

	// Unpinned: the row shows the resolved model, with no pin marker.
	require.NotContains(t, item.Render(70), d.com.L("cmd.pinned"))

	code := ws.cfg.Agents[config.AgentCode]
	code.ModelOverride = &config.SelectedModel{Provider: "p", Model: "pinned-model"}
	ws.cfg.Agents[config.AgentCode] = code
	d.setAgentsItems()

	item, ok = d.list.ItemAt(0).(*AgentsItem)
	require.True(t, ok)
	rendered := item.Render(70)
	require.Contains(t, rendered, "pinned-model")
	require.Contains(t, rendered, d.com.L("cmd.pinned"))
}