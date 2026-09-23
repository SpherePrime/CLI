package dialog

import (
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/help"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
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

	agentIDs []string

	scrollbarZone ScrollbarZone
	mouseScrolled bool

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

	m.agentIDs = []string{config.AgentCoder, config.AgentTask, config.AgentPlan}

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
	m.list.SetItems(items...)
	if len(items) > 0 {
		m.list.SetSelected(0)
		m.list.ScrollToSelected()
	}
}

// HandleMsg implements [Dialog].
func (m *Agents) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.MouseClickMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg:
		m.scrollbarZone.HandleMsg(msg, m.scrollListTo)
		return nil
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
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			agentItem, ok := selectedItem.(*AgentsItem)
			if !ok {
				break
			}
			return ActionOpenAgentModel{AgentID: agentItem.agentID}
		case key.Matches(msg, m.keyMap.Clear):
			selectedItem := m.list.SelectedItem()
			if selectedItem == nil {
				break
			}
			agentItem, ok := selectedItem.(*AgentsItem)
			if !ok || agentItem.agent.ModelOverride == nil {
				break
			}
			return ActionClearAgentModel{AgentID: agentItem.agentID}
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
	rc.Title = m.com.L("cmd.agents_models")

	listView := t.Dialog.List.Height(m.list.Height()).Render(m.list.Render())
	scrollable := listView
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, m.list.Offset())
	rc.AddPart(listView)

	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)

	view := rc.Render()
	body := dialogBodyRect(area, dialogRectCentered(area, view), listView, rc.Help, rc.ViewStyle, t.Dialog.List, innerWidth, listHeight)
	m.scrollbarZone.Painted(body.Min, scrollable, listHeight, listTotalHeight, listHeight, m.list.Offset())

	DrawCenterCursor(scr, area, view, nil)

	return nil
}

// scrollListTo scrolls the agents list so the scrollbar thumb lines up with
// the pointer.
func (m *Agents) scrollListTo(offset int) {
	m.mouseScrolled = true
	m.list.ScrollBy(offset - m.list.Offset())
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
	model := cfg.GetModelForAgent(i.agent)

	if i.agent.ModelOverride != nil {
		pinLabel := i.com.L("cmd.pinned")
		if model != nil {
			return model.Name + " (" + pinLabel + ")"
		}
		return i.agent.ModelOverride.Model + " (" + pinLabel + ")"
	}

	if model != nil {
		return model.Name
	}
	return string(i.agent.Model)
}
