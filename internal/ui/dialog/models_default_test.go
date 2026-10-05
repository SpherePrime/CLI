package dialog

import (
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/csync"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/workspace"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

type defaultModelWorkspace struct {
	workspace.Workspace
	cfg config.Config
}

func (w *defaultModelWorkspace) Config() *config.Config { return &w.cfg }
func (w *defaultModelWorkspace) Language() string       { return "en" }

func newAgentModelPicker(t *testing.T, agents map[string]config.Agent) *Models {
	t.Helper()

	cfg := config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Agents:    agents,
		Options:   &config.Options{},
	}
	com := &common.Common{Workspace: &defaultModelWorkspace{cfg: cfg}, Styles: providerTestStyles()}
	m, err := NewModelsForAgent(com, false, config.AgentTask)
	require.NoError(t, err)
	return m
}

// A worker's picker has to offer "Default" as a visible row at the top. It is
// the setting most workers should be on, and before this it could only be
// reached through a hidden ctrl+x, so the common case was invisible while
// being the default.
// defaultRowIndex finds the Default row rather than assuming an index: a
// group header occupies a row when one is rendered, and the Default grouping is
// deliberately headerless.
func defaultRowIndex(m *Models) int {
	for i := range 6 {
		if _, ok := m.list.ItemAt(i).(*agentDefaultItem); ok {
			return i
		}
	}
	return -1
}

func TestAgentModelPickerOffersDefaultFirst(t *testing.T) {
	m := newAgentModelPicker(t, map[string]config.Agent{
		config.AgentTask: {ID: config.AgentTask},
	})

	i := defaultRowIndex(m)
	require.NotEqual(t, -1, i, "the picker must offer the Default choice")

	// It is the first item in the list. The row above it is the heading the
	// group needs, because a headerless group would render as a blank line.
	require.Equal(t, 1, i, "Default must be the first item, found at %d", i)

	item := m.list.ItemAt(i).(*agentDefaultItem)
	require.Equal(t, DefaultAgentModelID, item.ID())

	// No pin, so this is what the agent is on and the row says so.
	require.True(t, item.isCurrent)
}

// A headerless group still occupies a row and renders as an empty line, so the
// heading has to be real.
func TestAgentDefaultGroupHasNoBlankHeading(t *testing.T) {
	sty := providerTestStyles()

	titled := NewModelGroup(sty, "Default", false)
	require.NotEmpty(t, titled.Render(40))

	untitled := NewModelGroup(sty, "", false)
	require.Empty(t, untitled.Render(40),
		"an untitled group must render as nothing rather than a stray line")
}

// Picking Default must clear the pin rather than set one, which is the whole
// difference between "follows the main agent" and "pinned to something".
func TestAgentModelPickerDefaultClearsPin(t *testing.T) {
	m := newAgentModelPicker(t, map[string]config.Agent{
		config.AgentTask: {
			ID:            config.AgentTask,
			ModelOverride: &config.SelectedModel{Provider: "p", Model: "m"},
		},
	})

	i := defaultRowIndex(m)
	require.NotEqual(t, -1, i)

	action := m.activateModel(i, false)
	require.IsType(t, ActionClearAgentModel{}, action)
	require.Equal(t, config.AgentTask, action.(ActionClearAgentModel).AgentID)
}

// A pinned agent must not show Default as its current setting, or the row
// would claim it is following the main agent while it is not.
func TestAgentModelPickerDefaultNotCurrentWhenPinned(t *testing.T) {
	m := newAgentModelPicker(t, map[string]config.Agent{
		config.AgentTask: {
			ID:            config.AgentTask,
			ModelOverride: &config.SelectedModel{Provider: "p", Model: "m"},
		},
	})

	i := defaultRowIndex(m)
	require.NotEqual(t, -1, i)
	require.False(t, m.list.ItemAt(i).(*agentDefaultItem).isCurrent)
}

// The global picker has no agent scope, so Default has nothing to mean there
// and must not appear.
func TestGlobalModelPickerHasNoDefaultRow(t *testing.T) {
	cfg := config.Config{
		Providers: csync.NewMap[string, config.ProviderConfig](),
		Agents:    map[string]config.Agent{config.AgentTask: {ID: config.AgentTask}},
		Options:   &config.Options{},
	}
	com := &common.Common{
		Workspace: &defaultModelWorkspace{cfg: cfg},
		Styles:    providerTestStyles(),
	}

	m, err := NewModelsForAgent(com, false, "")
	require.NoError(t, err)

	for i := range 8 {
		if _, ok := m.list.ItemAt(i).(*agentDefaultItem); ok {
			t.Fatal("the global model picker must not offer the Default row")
		}
	}
}

// Typing into a picker has to filter the rows, not take the program down.
//
// The picker holds more than one kind of row - a worker's Default entry is not
// a ModelItem - and the filter walked them by asserting that each one was, so
// the first character typed into a worker's picker panicked and the session
// ended in a stack trace instead of a narrowed list. The assertion was also
// doing no work: the text it needed is on the interface every row already
// implements.
func TestTypingInTheWorkerPickerFiltersInsteadOfPanicking(t *testing.T) {
	m := newAgentModelPicker(t, map[string]config.Agent{
		config.AgentTask: {ID: config.AgentTask},
	})

	require.NotEqual(t, -1, defaultRowIndex(m),
		"the Default row is the row the filter has to survive")
	unfiltered := len(m.list.VisibleItems())
	require.NotZero(t, unfiltered, "the picker starts with rows in it")

	// A letter nothing matches: the filter still walks every row, which is
	// exactly where it used to assert each one was a *ModelItem.
	action := m.HandleMsg(tea.KeyPressMsg{Code: 'z', Text: "z"})
	require.NotNil(t, action, "a keystroke must be handed back as a command")
	require.Zero(t, len(m.list.VisibleItems()),
		"a query no row matches must leave the list empty")

	// Backspace empties the field, and the rows have to come back.
	m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyBackspace})
	require.Equal(t, unfiltered, len(m.list.VisibleItems()),
		"clearing the query must bring every row back")
}

// The Default row has to be reachable with the arrow keys.
//
// It is the setting most workers should be on, and the reason it became a row
// at all was that the common case was invisible behind a hidden ctrl+x. The
// cursor skips rows the picker cannot act on, and that skip was written when
// the only actionable row was a *ModelItem - so the row added later was
// skipped too: visible, announced in the help, and impossible to land on
// without reaching for the mouse.
func TestTheDefaultRowIsReachableWithTheArrowKeys(t *testing.T) {
	m := newAgentModelPicker(t, map[string]config.Agent{
		config.AgentTask: {ID: config.AgentTask},
	})

	reached := false
	for range 16 {
		m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyDown})
		if _, ok := m.list.SelectedItem().(*agentDefaultItem); ok {
			reached = true
			break
		}
	}
	require.True(t, reached, "arrow keys must be able to land on the Default row")

	// Landing on it and pressing enter is the choice that clears the pin.
	action := m.HandleMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	require.IsType(t, ActionClearAgentModel{}, action,
		"enter on the Default row is what makes it a real choice")
	require.Equal(t, config.AgentTask, action.(ActionClearAgentModel).AgentID)
}
