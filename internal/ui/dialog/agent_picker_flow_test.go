package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/workspace"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// agentPickerWorkspace carries a provider with real models, so the agent's model
// picker has something to pick. Without one the list holds only the Default row
// and "picking a model" cannot be exercised at all.
type agentPickerWorkspace struct {
	workspace.Workspace
	cfg        *config.Config
	writtenKey string
}

func (w *agentPickerWorkspace) Config() *config.Config            { return w.cfg }
func (w *agentPickerWorkspace) Language() string                  { return "en" }
func (w *agentPickerWorkspace) WorkingDir() string                { return "/tmp" }
func (w *agentPickerWorkspace) Resolver() config.VariableResolver { return nil }

func (w *agentPickerWorkspace) SetConfigField(scope config.Scope, key string, value any) error {
	w.writtenKey = key
	return nil
}

func newAgentPicker(t *testing.T) (*Agents, *Models, *agentPickerWorkspace) {
	t.Helper()

	cfg := &config.Config{
		Options: &config.Options{DisabledTools: []string{}},
		Agents:  map[string]config.Agent{},
	}
	cfg.SetupAgents()

	cfg.Providers = csync.NewMap[string, config.ProviderConfig]()
	cfg.Providers.Set("acme", config.ProviderConfig{
		ID:             "acme",
		UserConfigured: true,
		Models: []catwalk.Model{
			{ID: "fast", ContextWindow: 200_000},
			{ID: "slow", ContextWindow: 400_000},
		},
	})
	cfg.Models = map[config.SelectedModelType]config.SelectedModel{
		config.SelectedModelTypeLarge: {Provider: "acme", Model: "fast"},
	}

	ws := &agentPickerWorkspace{cfg: cfg}
	com := &common.Common{Workspace: ws, Styles: providerTestStyles()}

	agents := NewAgents(com)
	picker, err := NewModelsForAgent(com, false, config.AgentCode)
	require.NoError(t, err)

	return agents, picker, ws
}

// The whole point of pressing a worker: the picker that opens has to offer
// concrete models, and Default has to sit above them.
//
// The row index is walked generously rather than bounded by the item count,
// because this list mixes group headings and spacers into the same index space
// as the items. Bounding the walk by the item count is what made an earlier
// version of this file report an empty picker on a picker that was populated.
func TestAgentModelPickerOffersTheProvidersModels(t *testing.T) {
	_, picker, _ := newAgentPicker(t)

	var modelIDs []string
	defaultRow := -1

	for i := range 20 {
		switch item := picker.list.ItemAt(i).(type) {
		case *ModelItem:
			modelIDs = append(modelIDs, item.model.ID)
		case *agentDefaultItem:
			defaultRow = i
		default:
		}
	}

	require.Contains(t, modelIDs, "fast", "the picker must list the provider's models")
	require.Contains(t, modelIDs, "slow")

	require.NotEqual(t, -1, defaultRow, "the worker's picker must offer Default")
	for i := range 20 {
		if _, ok := picker.list.ItemAt(i).(*ModelItem); ok {
			require.Greater(t, i, defaultRow,
				"Default must sit above the models, not among them")
		}
	}
}

// Activating a model row in an agent's picker has to produce a pin for that
// agent. Returning nil, or a pin for the wrong agent, is what "cannot change a
// sub-agent's model" looks like from the outside.
func TestAgentModelPickerPinNamesTheAgent(t *testing.T) {
	_, picker, _ := newAgentPicker(t)

	pickerIdx := -1
	for i := range 12 {
		if item, ok := picker.list.ItemAt(i).(*ModelItem); ok && item.model.ID == "slow" {
			pickerIdx = i
			break
		}
	}
	require.NotEqual(t, -1, pickerIdx, "the model must be reachable in the list")

	action := picker.activateModel(pickerIdx, false)
	require.IsType(t, ActionSetAgentModel{}, action,
		"choosing a model must pin it, not do nothing")

	set := action.(ActionSetAgentModel)
	require.Equal(t, config.AgentCode, set.AgentID, "the pin must name the worker that was opened")
	require.Equal(t, "slow", set.Model.Model)
	require.Equal(t, "acme", set.Model.Provider)
}

// Default belongs in this picker and nowhere else: pressing a worker is the one
// step that decides what it runs on, and Default there means "the model the main
// agent is using".
func TestAgentModelPickerDefaultMeansTheMainModel(t *testing.T) {
	_, picker, ws := newAgentPicker(t)

	i := -1
	for j := range 6 {
		if _, ok := picker.list.ItemAt(j).(*agentDefaultItem); ok {
			i = j
			break
		}
	}
	require.NotEqual(t, -1, i, "the agent's picker must offer Default")

	action := picker.activateModel(i, false)
	require.IsType(t, ActionClearAgentModel{}, action)
	require.Equal(t, config.AgentCode, action.(ActionClearAgentModel).AgentID)

	// And it must mean what the user thinks: with no pin the worker resolves to
	// the same model as the main agent, which is fast here.
	require.Nil(t, ws.cfg.Agents[config.AgentCode].ModelOverride)
	resolved := ws.cfg.GetModelForAgent(ws.cfg.Agents[config.AgentCode])
	require.NotNil(t, resolved)
	require.Equal(t, ws.cfg.GetModelForAgent(ws.cfg.Agents[config.AgentGeneral]).ID, resolved.ID,
		"Default must resolve the worker to whatever the main agent is running")
}

// The agents list must not carry a Default row of its own. Picking the same
// choice from two places meant picking it here cleared every worker instead of
// opening a picker, so pressing enter appeared to do nothing.
func TestAgentsListHasNoDefaultRow(t *testing.T) {
	agents, _, _ := newAgentPicker(t)

	require.Equal(t, len(agents.agentIDs), agents.list.Len(),
		"the list must hold the workers and nothing else")

	// Every row is a worker, and every one opens a picker.
	for i := range agents.list.Len() {
		require.IsType(t, ActionOpenAgentModel{}, agents.activate(i),
			"row %d must open the model picker", i)
	}
}

func TestAgentsListCtrlXClearsTheWorkersPin(t *testing.T) {
	agents, _, ws := newAgentPicker(t)

	code := ws.cfg.Agents[config.AgentCode]
	code.ModelOverride = &config.SelectedModel{Provider: "acme", Model: "slow"}
	ws.cfg.Agents[config.AgentCode] = code

	agents.setAgentsItems()
	require.IsType(t, ActionClearAgentModel{}, agents.clearPin())
}