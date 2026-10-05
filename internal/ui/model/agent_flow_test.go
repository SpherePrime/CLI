package model

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/dialog"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// agentFlowWorkspace carries a provider with real models and records writes.
type agentFlowWorkspace struct {
	testWorkspace
	written []string
}

func (w *agentFlowWorkspace) SetConfigField(scope config.Scope, key string, value any) error {
	w.written = append(w.written, key)
	if agent, ok := value.(config.Agent); ok && w.cfg != nil {
		// Mirror the store so a later read observes the write.
		w.cfg.Agents[agent.ID] = agent
	}
	return nil
}

func newAgentFlowUI(t *testing.T) (*UI, *agentFlowWorkspace) {
	t.Helper()

	cfg := &config.Config{
		Options:   &config.Options{DisabledTools: []string{}},
		Agents:    map[string]config.Agent{},
		Providers: csync.NewMap[string, config.ProviderConfig](),
	}
	cfg.SetupAgents()
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

	ws := &agentFlowWorkspace{}
	ws.cfg = cfg
	u := newTestUI()
	u.com = common.DefaultCommon(ws)
	u.dialog = dialog.NewOverlay()
	return u, ws
}

// Pressing enter on the first worker has to open that worker's model picker.
//
// This is where it broke. The agent list used to carry a Default row at the top
// whose enter cleared pins instead of opening a picker, so the very first thing
// a user pressed did nothing visible when nothing was pinned, and picking a
// sub-agent's model looked broken.
func TestPressingAWorkerOpensItsModelPicker(t *testing.T) {
	u, _ := newAgentFlowUI(t)

	require.Nil(t, u.openAgentsDialog())

	// The key is fed to the UI the way bubbletea delivers it. handleDialogMsg
	// is what turns the dialog's return value into an action, so going
	// straight to the overlay would skip the half where the picker is opened.
	_ = u.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	require.True(t, u.dialog.ContainsDialog(dialog.ModelsID),
		"pressing a worker must open its model picker")

	front, ok := u.dialog.DialogLast().(*dialog.Models)
	require.True(t, ok, "the front dialog must be a model picker")
	require.Equal(t, config.AgentCode, front.AgentID(),
		"the picker must be scoped to the worker that was pressed")
}

// The picker that opens has to offer Default, which is where "the same model as
// the main agent" lives. It belongs here and nowhere else: the worker list used
// to carry one too, where enter cleared pins instead of opening a picker.
//
// What the picker contains is asserted in the dialog package, where the list is
// reachable. Adding accessors here to look at it from outside gave them two
// incompatible index scales - Len counts items while ItemAt counts headings and
// spacers as well - which is the kind of API that makes a caller wrong instead
// of making the code wrong.
func TestWorkerPickerIsScopedAndOffersDefault(t *testing.T) {
	u, _ := newAgentFlowUI(t)

	require.Nil(t, u.openAgentsDialog())
	_ = u.handleDialogMsg(tea.KeyPressMsg{Code: tea.KeyEnter})

	front, ok := u.dialog.DialogLast().(*dialog.Models)
	require.True(t, ok, "the front dialog must be a model picker")
	require.Equal(t, config.AgentCode, front.AgentID(),
		"the picker must be scoped to the worker that was pressed")
}

// Choosing a model has to write the pin and leave the agent list open, so the
// next worker can be set without reopening anything.
func TestChoosingAModelPinsItAndKeepsTheList(t *testing.T) {
	u, ws := newAgentFlowUI(t)

	require.Nil(t, u.openAgentsDialog())
	require.True(t, u.dialog.ContainsDialog(dialog.AgentsID))

	cmd := u.handleSetAgentModel(dialog.ActionSetAgentModel{
		AgentID: config.AgentCode,
		Model:   config.SelectedModel{Provider: "acme", Model: "slow"},
	})
	require.NotNil(t, cmd)

	require.Contains(t, ws.written, "agents.code")
	require.NotNil(t, ws.cfg.Agents[config.AgentCode].ModelOverride)
	require.Equal(t, "slow", ws.cfg.Agents[config.AgentCode].ModelOverride.Model)

	require.False(t, u.dialog.ContainsDialog(dialog.ModelsID),
		"the picker closes once a model is chosen")
	require.True(t, u.dialog.ContainsDialog(dialog.AgentsID),
		"the worker list stays open so the next one can be set")
}

// Default in the picker clears the pin, which is what that row promises.
func TestSubAgentCanGoBackToTheMainModel(t *testing.T) {
	u, ws := newAgentFlowUI(t)

	code := ws.cfg.Agents[config.AgentCode]
	code.ModelOverride = &config.SelectedModel{Provider: "acme", Model: "slow"}
	ws.cfg.Agents[config.AgentCode] = code

	require.Nil(t, u.openAgentsDialog())
	require.NotNil(t, u.handleClearAgentModel(dialog.ActionClearAgentModel{AgentID: config.AgentCode}))
	require.Nil(t, ws.cfg.Agents[config.AgentCode].ModelOverride,
		"cleared workers follow the main agent's model again")

	require.True(t, u.dialog.ContainsDialog(dialog.AgentsID),
		"the worker list stays open after clearing too")
}