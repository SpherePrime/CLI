package dialog

import (
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/help"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/sahilm/fuzzy"
)

// AgentsID is the identifier for the subagent model settings dialog.
const AgentsID = "agents"

const agentsDialogMaxWidth = 73

// Agents lists the built-in subagents and the model each one runs on.
// Pressing enter on an agent opens the model picker scoped to that agent.
// Pressing ctrl+x clears a pinned model.
type Agents struct {
	com *common.Common

	help help.Model
	list *list.FilterableList

	// allIDs is every worker the dialog can configure, used to render the
	// Default entry. agentIDs is the list minus general, since the main
	// agent is configured from the model picker instead.
	allIDs   []string
	agentIDs []string

	// mouse gives the list full pointer support: click a row to select
	// it, click it again to open, drag the scrollbar, wheel to scroll.
	mouse ListMouse

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Clear    key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*Agents)(nil)

// NewAgents creates a new Agents dialog.
func NewAgents(com *common.Common) *Agents {
	t := com.Styles
	m := &Agents{com: com}

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	// Only the workers. The main agent is deliberately not listed: it is the
	// agent this session runs on and its model is chosen from the model picker,
	// so an entry here would be a second place to change the same thing, and
	// picking a "model for an agent" in it would switch the session's model
	// while reading as configuring a delegate.
	//
	// There is no Default row here either. Choosing what a worker runs on is
	// one step: press a worker, and the picker that opens offers Default,
	// meaning the model the main agent is using. A Default entry in this list
	// put the same choice in a second place, where picking it cleared every
	// worker instead of opening a picker, so pressing enter on it looked like
	// nothing happened.
	m.agentIDs = []string{config.AgentCode, config.AgentTask, config.AgentPlan}

	m.list = list.NewFilterableList()
	m.list.Focus()

	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "pick model"),
	)
	m.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	m.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	m.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	m.keyMap.Clear = key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("ctrl+x", "clear pin"),
	)
	m.keyMap.Close = CloseKey

	m.setAgentsItems()

	return m
}

// ID implements Dialog.
func (m *Agents) ID() string {
	return AgentsID
}

// setAgentsItems builds the agent list from the current config.
func (m *Agents) setAgentsItems() {
	cfg := m.com.Config()

	items := make([]list.FilterableItem, 0, len(m.agentIDs))

	for _, agentID := range m.agentIDs {
		agent, ok := cfg.Agents[agentID]
		if !ok {
			continue
		}
		item := &AgentsItem{
			Versioned: list.NewVersioned(),
			agentID:   agentID,
			agent:     agent,
			com:       m.com,
			t:         m.com.Styles,
			cache:     make(map[int]string),
		}
		items = append(items, item)
	}
	// The row under the cursor has to be the same row after a rebuild, or
	// pinning the second worker throws the selection back to the first and
	// the next keypress configures the wrong agent. A list that has not been
	// given anything yet reports no selection at all, so both ends clamp:
	// an out-of-range index here made the first keypress land on no row and
	// opening a worker's picker silently do nothing.
	selected := m.list.Selected()
	m.list.SetItems(items...)
	if len(items) == 0 {
		return
	}
	if selected < 0 || selected >= len(items) {
		selected = 0
	}
	m.list.SetSelected(selected)
	m.list.ScrollToSelected()
}

// Refresh re-reads the workers from config.
//
// The rows are a snapshot taken when the dialog opens, and the dialog stays
// open across a pin on purpose, so nothing that writes an agent would ever
// repaint it: the pin landed, the row under it still read as unpinned, and
// from the chair in front of the terminal that is indistinguishable from the
// change not having been made at all.
func (m *Agents) Refresh() {
	m.setAgentsItems()
}

// HandleMsg implements [Dialog].
func (m *Agents) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case common.CoalescedWheelMsg, tea.MouseClickMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg:
		return m.mouse.HandleMsg(msg, m.list, m.activate)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, m.keyMap.Previous):
			m.list.Focus()
			if m.list.IsSelectedFirst() {
				m.list.SelectLast()
				m.list.ScrollToBottom()
				break
			}
			m.list.SelectPrev()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Next):
			m.list.Focus()
			if m.list.IsSelectedLast() {
				m.list.SelectFirst()
				m.list.ScrollToTop()
				break
			}
			m.list.SelectNext()
			m.list.ScrollToSelected()
		case key.Matches(msg, m.keyMap.Select):
			return m.activate(m.list.Selected())
		case key.Matches(msg, m.keyMap.Clear):
			return m.clearPin()
		}
	}
	return nil
}

// Draw implements [Dialog].
func (m *Agents) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(agentsDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	listHeight, listTotalHeight, _ := sizeDialogList(t, m.list, innerWidth, height)

	rc := NewRenderContext(t, width)
	rc.Title = m.com.L("cmd.agent_models")

	listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	scrollable := listView
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, m.list.Offset())
	rc.AddPart(listView)

	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)

	view := rc.Render()
	body := dialogBodyRect(area, dialogRectCentered(area, view), listView, rc.Help, rc.ViewStyle, t.Dialog.List, innerWidth, listHeight)
	m.mouse.Painted(body)
	m.mouse.PaintedJoin(body.Min, scrollable, listHeight, listTotalHeight, listHeight, m.list.Offset())

	DrawCenterCursor(scr, area, view, nil)

	return nil
}

// clearPin removes the pin from the highlighted agent.
func (m *Agents) clearPin() Action {
	item, ok := m.list.SelectedItem().(*AgentsItem)
	if !ok || item == nil || item.agent.ModelOverride == nil {
		return nil
	}
	return ActionClearAgentModel{AgentID: item.agentID}
}

// activate opens the agent at idx. Shared by the enter key and by a
// click on a row.
func (m *Agents) activate(idx int) Action {
	item, ok := m.list.ItemAt(idx).(*AgentsItem)
	if !ok || item == nil {
		return nil
	}
	return ActionOpenAgentModel{AgentID: item.agentID}
}

// ShortHelp implements [help.KeyMap].
func (m *Agents) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.UpDown,
		m.keyMap.Select,
		m.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (m *Agents) FullHelp() [][]key.Binding {
	slice := []key.Binding{
		m.keyMap.Select,
		m.keyMap.Next,
		m.keyMap.Previous,
		m.keyMap.Clear,
		m.keyMap.Close,
	}
	var rows [][]key.Binding
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		rows = append(rows, slice[i:end])
	}
	return rows
}

// AgentsItem is a single agent entry in the Agents dialog.
type AgentsItem struct {
	*list.Versioned

	agentID string
	agent   config.Agent
	com     *common.Common

	t *styles.Styles

	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var _ ListItem = (*AgentsItem)(nil)

// Finished implements list.Item.
func (i *AgentsItem) Finished() bool {
	return true
}

// Filter implements list.FilterableItem.
func (i *AgentsItem) Filter() string {
	return i.agent.Name
}

// ID implements list.Item.
func (i *AgentsItem) ID() string {
	return i.agentID
}

// Render implements list.Item.
func (i *AgentsItem) Render(width int) string {
	info := i.modelInfo()
	styles := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: i.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(styles, i.agent.Name, info, i.focused, width, i.cache, &i.m)
}

// SetFocused implements list.Item.
func (i *AgentsItem) SetFocused(focused bool) {
	if i.focused == focused {
		return
	}
	i.cache = nil
	i.focused = focused
	if i.Versioned != nil {
		i.Bump()
	}
}

// SetMatch implements list.Item.
func (i *AgentsItem) SetMatch(fm fuzzy.Match) {
	if sameFuzzyMatch(i.m, fm) {
		return
	}
	i.cache = nil
	i.m = fm
	if i.Versioned != nil {
		i.Bump()
	}
}

// modelInfo returns the model the agent currently runs on.
func (i *AgentsItem) modelInfo() string {
	cfg := i.com.Config()

	if override := i.agent.ModelOverride; override != nil {
		pinLabel := i.com.L("cmd.pinned")
		// The pinned model is looked up on its own: GetModelForAgent falls back
		// to the agent's model type, which would print the default model name
		// as if it were the pinned one.
		//
		// A catalog entry may carry no name - a model configured by hand
		// usually does - and printing the empty name produced a row that said
		// "(pinned)" with nothing pinned to it, which is the same as not
		// showing the choice at all. The id is what the toast falls back to
		// for the same reason.
		if model := cfg.GetModel(override.Provider, override.Model); model != nil && model.Name != "" {
			return model.Name + " (" + pinLabel + ")"
		}
		return override.Model + " (" + pinLabel + ")"
	}

	model := cfg.GetModelForAgent(i.agent)
	if model != nil && model.Name != "" {
		return model.Name
	}
	if model != nil {
		return model.ID
	}
	return string(i.agent.Model)
}
