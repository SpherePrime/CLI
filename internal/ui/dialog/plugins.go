package dialog

import (
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/i18n"
	"github.com/SpherePrime/CLI/internal/plugins"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/help"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/spinner"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/sahilm/fuzzy"
)

// PluginsID is the identifier for the plugins menu dialog.
const PluginsID = "plugins"

const (
	pluginsDialogMaxWidth  = 60
	pluginsDialogMaxHeight = 14
)

// Plugins lists Prime's own plugins and switches them on or off. Enabling a
// plugin that still needs to download its components shows a live progress
// block; a plugin whose components are already present switches on at once.
type Plugins struct {
	com   *common.Common
	help  help.Model
	list  *list.List
	items []*PluginItem

	spinner  spinner.Model
	install  string // plugin name being installed, empty when none
	snapshot plugins.Snapshot

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

// PluginItem is one plugin row in the list.
type PluginItem struct {
	*list.Versioned
	name        string
	title       string
	description string
	state       string
	t           *styles.Styles
	m           fuzzy.Match
	cache       map[int]string
	focused     bool
}

// Finished implements list.Item.
func (p *PluginItem) Finished() bool { return true }

var (
	_ Dialog   = (*Plugins)(nil)
	_ ListItem = (*PluginItem)(nil)
)

// NewPlugins creates the plugins menu and the command driving its progress
// animation while an install runs.
func NewPlugins(com *common.Common) (*Plugins, tea.Cmd) {
	p := &Plugins{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	p.help = h

	p.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(com.Styles.Dialog.OAuth.Spinner),
	)

	p.list = list.NewList()
	p.list.Focus()

	p.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "install or remove"),
	)
	p.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	p.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	p.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	p.keyMap.Close = CloseKey

	p.syncItems()
	if running := p.runningInstall(); running != "" {
		return p, p.spinner.Tick
	}
	return p, nil
}

// ID implements Dialog.
func (p *Plugins) ID() string { return PluginsID }

// HandleMsg implements Dialog.
func (p *Plugins) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case spinner.TickMsg:
		var cmd tea.Cmd
		p.spinner, cmd = p.spinner.Update(msg)
		if p.runningInstall() == "" {
			p.install = ""
			p.refreshStates()
			return nil
		}
		p.refreshStates()
		return ActionCmd{cmd}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, p.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, p.keyMap.Previous):
			p.list.SelectPrev()
		case key.Matches(msg, p.keyMap.Next):
			p.list.SelectNext()
		case key.Matches(msg, p.keyMap.Select):
			item, ok := p.list.SelectedItem().(*PluginItem)
			if !ok {
				break
			}
			if plugins.Running(item.name) != nil {
				break // one toggle at a time; the install speaks for itself
			}
			p.install = item.name
			return ActionTogglePlugin{Name: item.name}
		}
	}
	return nil
}

// StartLoading implements [LoadingDialog]: the progress heartbeat runs
// while an install task is active, refreshing the spinner and the bar.
func (p *Plugins) StartLoading() tea.Cmd {
	return p.spinner.Tick
}

// StopLoading implements [LoadingDialog].
func (p *Plugins) StopLoading() {}

// runningInstall keeps the dialog in step with the actual task registry, so
// reopening it mid-install resumes the animation and an install started from
// anywhere else is visible here too.
func (p *Plugins) runningInstall() string {
	for _, plugin := range plugins.All() {
		if task := plugins.Running(plugin.Name); task != nil {
			p.install = plugin.Name
			p.snapshot = task.Snapshot()
			return plugin.Name
		}
	}
	return ""
}

func (p *Plugins) syncItems() {
	items := make([]*PluginItem, 0, 4)
	listItems := make([]list.Item, 0, 4)
	for _, plugin := range plugins.All() {
		item := &PluginItem{
			Versioned:   list.NewVersioned(),
			name:        plugin.Name,
			title:       p.localized(p.translator(), "plugins."+plugin.Name+".title", plugin.Title),
			description: p.localized(p.translator(), "plugins."+plugin.Name+".desc", plugin.Description),
			t:           p.com.Styles,
		}
		items = append(items, item)
		listItems = append(listItems, item)
	}
	p.items = items
	p.list.SetItems(listItems...)
	p.refreshStates()
}

// refreshStates updates every row's on/off/installing label in place, so a
// redraw while an install runs never moves the selection.
func (p *Plugins) refreshStates() {
	tr := p.translator()
	cfg := p.com.Config()
	for _, item := range p.items {
		state := tr.Label("plugins.state.off")
		switch {
		case plugins.Running(item.name) != nil:
			state = tr.Label("plugins.state.installing")
		case cfg.IsPluginEnabled(item.name):
			state = tr.Label("plugins.state.on")
		}
		if item.state != state {
			item.state = state
			item.cache = nil
			item.Bump()
		}
	}
}

func (p *Plugins) localized(tr i18n.Translator, key, fallback string) string {
	if label := tr.Label(key); label != "" && label != key {
		return label
	}
	return fallback
}

func (p *Plugins) translator() i18n.Translator {
	cfg := p.com.Config()
	locale := i18n.En
	if cfg != nil && cfg.Options != nil && cfg.Options.Language != "" {
		locale = cfg.Options.Language
	}
	return i18n.New(locale)
}

// Cursor implements Dialog.
func (p *Plugins) Cursor() *tea.Cursor { return nil }

// Draw implements Dialog.
func (p *Plugins) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := p.com.Styles
	width := max(0, min(pluginsDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(pluginsDialogMaxHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	tr := p.translator()
	p.refreshStates()
	rc := NewRenderContext(t, width)
	rc.Title = tr.Label("dialog.plugins")

	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() + t.Dialog.View.GetVerticalFrameSize()
	progressHeight := 0
	if p.install != "" {
		progressHeight = 2
		heightOffset += progressHeight
	}

	p.list.SetSize(innerWidth, max(0, height-heightOffset))
	rc.AddPart(t.Dialog.List.Height(p.list.Height()).Render(p.list.Render()))

	if p.install != "" {
		rc.AddPart(p.progressView(tr, innerWidth))
	}

	rc.Help = renderDialogHelp(t, &p.help, p, innerWidth)
	DrawCenterCursor(scr, area, rc.Render(), nil)
	return nil
}

// progressView is the "please wait" block: spinner frame, human step, and a
// byte-progress bar when the download reports one.
func (p *Plugins) progressView(tr i18n.Translator, width int) string {
	snap := p.snapshot
	if task := plugins.Running(p.install); task != nil {
		snap = task.Snapshot()
	}

	line := tr.Label("plugins.waiting")
	if snap.Progress.Text != "" {
		line = snap.Progress.Text
	}
	if snap.Progress.Total > 0 {
		pct := float64(snap.Progress.Done) / float64(snap.Progress.Total)
		pct = min(pct, 1)
		const barWidth = 20
		filled := int(pct * barWidth)
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		line = fmt.Sprintf("%s %s %3.0f%%", snap.Progress.Phase, bar, pct*100)
	}

	view := fmt.Sprintf("%s %s", p.spinner.View(), line)
	return p.com.Styles.Dialog.NormalItem.Width(max(0, width)).Render(view)
}

// ShortHelp implements [help.KeyMap].
func (p *Plugins) ShortHelp() []key.Binding {
	return []key.Binding{p.keyMap.UpDown, p.keyMap.Select, p.keyMap.Close}
}

// FullHelp implements [help.KeyMap].
func (p *Plugins) FullHelp() [][]key.Binding {
	return [][]key.Binding{{p.keyMap.Select, p.keyMap.Next, p.keyMap.Previous, p.keyMap.Close}}
}

// Filter implements the list item contract.
func (p *PluginItem) Filter() string { return p.title + " " + p.name }

// ID implements the list item contract.
func (p *PluginItem) ID() string { return p.name }

// SetFocused implements the list item contract.
func (p *PluginItem) SetFocused(focused bool) {
	if p.focused == focused {
		return
	}
	p.cache = nil
	p.focused = focused
	if p.Versioned != nil {
		p.Bump()
	}
}

// SetMatch implements the list item contract.
func (p *PluginItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(p.m, m) {
		return
	}
	p.cache = nil
	p.m = m
	if p.Versioned != nil {
		p.Bump()
	}
}

// Render draws "Title (state)" with the description as secondary text.
func (p *PluginItem) Render(width int) string {
	st := ListItemStyles{
		ItemBlurred:     p.t.Dialog.NormalItem,
		ItemFocused:     p.t.Dialog.SelectedItem,
		InfoTextBlurred: p.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: p.t.Dialog.ListItem.InfoFocused,
	}
	title := fmt.Sprintf("%s  %s", p.title, p.state)
	return renderItem(st, title, p.description, p.focused, width, p.cache, &p.m)
}
