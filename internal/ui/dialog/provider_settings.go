package dialog

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/util"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/help"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/textinput"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
)

// ProviderSettingsID is the identifier for the provider settings dialog.
const ProviderSettingsID = "provider_settings"

// providerSettingsState selects which step the dialog is showing.
type providerSettingsState uint8

const (
	// providerSettingsStateProviders lists the configured providers.
	providerSettingsStateProviders providerSettingsState = iota
	// providerSettingsStateModels lists one provider's models.
	providerSettingsStateModels
	// providerSettingsStateModelID captures the ID of a model to add.
	providerSettingsStateModelID
	// providerSettingsStateModelContext captures the context window of a
	// model to add. Providers that do not report one leave this empty.
	providerSettingsStateModelContext
)

// ProviderSettings lists configured providers and lets the user attach models
// to them by hand, which is needed when a provider does not report its models
// through the API.
type ProviderSettings struct {
	com   *common.Common
	state providerSettingsState

	providerID string
	models     []catwalk.Model
	discover   bool

	list  *list.FilterableList
	input textinput.Model
	help  help.Model

	newModelID string

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		Add      key.Binding
		Remove   key.Binding
		Toggle   key.Binding
		Close    key.Binding
	}
}

var _ Dialog = (*ProviderSettings)(nil)

// ActionProviderSettingsChanged reports that a provider's models changed and
// the agents need to be rebuilt to pick the change up.
type ActionProviderSettingsChanged struct {
	ProviderID string
}

// NewProviderSettings creates the provider settings dialog.
func NewProviderSettings(com *common.Common) *ProviderSettings {
	p := &ProviderSettings{com: com, state: providerSettingsStateProviders}

	p.help = help.New()
	p.help.Styles = com.Styles.DialogHelpStyles()

	p.input = textinput.New()
	p.input.SetVirtualCursor(false)
	p.input.CharLimit = 120
	p.input.SetStyles(com.Styles.TextInput)
	p.input.Focus()

	p.list = list.NewFilterableList()
	p.list.Focus()
	p.reloadProviders()

	p.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "tab", "ctrl+y"),
		key.WithHelp("enter", "choose"),
	)
	p.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next"),
	)
	p.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous"),
	)
	p.keyMap.Add = key.NewBinding(key.WithKeys("a"), key.WithHelp("a", "add model"))
	p.keyMap.Remove = key.NewBinding(key.WithKeys("x", "ctrl+x"), key.WithHelp("x", "remove model"))
	p.keyMap.Toggle = key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "toggle discovery"))
	p.keyMap.Close = CloseKey

	return p
}

// ID implements Dialog.
func (p *ProviderSettings) ID() string {
	return ProviderSettingsID
}

// reloadProviders rebuilds the provider list from the current config.
func (p *ProviderSettings) reloadProviders() {
	cfg := p.com.Config()
	var items []list.FilterableItem
	if cfg != nil {
		items = configuredProviderItems(p.com.Styles, cfg)
	}
	p.list.SetItems(items...)
	if p.list.Len() > 0 {
		p.list.SetSelected(0)
	}
}

// openProvider loads one provider's models and moves to the model step.
func (p *ProviderSettings) openProvider(id string) bool {
	cfg := p.com.Config()
	if cfg == nil {
		return false
	}
	provider, ok := cfg.Providers.Get(id)
	if !ok {
		return false
	}
	p.providerID = id
	p.models = slices.Clone(provider.Models)
	p.discover = provider.AutoDiscoverModels == nil || *provider.AutoDiscoverModels
	p.state = providerSettingsStateModels
	p.resetFilter()
	p.reloadModels()
	return true
}

// resetFilter clears the search field and the active filter so a new step
// starts from the full list.
func (p *ProviderSettings) resetFilter() {
	p.input.SetValue("")
	p.list.SetFilter("")
}

func (p *ProviderSettings) reloadModels() {
	p.list.SetItems(providerModelItems(p.com.Styles, p.models)...)
	if p.list.Len() > 0 {
		p.list.SetSelected(0)
	}
}

// HandleMsg implements Dialog.
func (p *ProviderSettings) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case common.CoalescedWheelMsg:
		if p.state == providerSettingsStateProviders || p.state == providerSettingsStateModels {
			p.list.ScrollBy(int(msg.DeltaY))
		}
		return nil
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, p.keyMap.Close):
			return p.handleClose()
		case key.Matches(msg, p.keyMap.Next):
			p.moveSelection(1)
		case key.Matches(msg, p.keyMap.Previous):
			p.moveSelection(-1)
		}

		switch p.state {
		case providerSettingsStateProviders:
			if key.Matches(msg, p.keyMap.Select) {
				item, ok := p.list.SelectedItem().(ListItem)
				if !ok {
					return nil
				}
				if !p.openProvider(item.ID()) {
					return ActionCmd{util.ReportError(
						errors.New("provider is no longer configured"))}
				}
				return nil
			}
			return p.filterInput(msg)

		case providerSettingsStateModels:
			switch {
			case key.Matches(msg, p.keyMap.Add):
				p.startAddingModel()
			case key.Matches(msg, p.keyMap.Remove):
				return p.removeSelectedModel()
			case key.Matches(msg, p.keyMap.Toggle):
				return p.toggleDiscovery()
			}
			return nil

		case providerSettingsStateModelID:
			if key.Matches(msg, p.keyMap.Select) {
				return p.commitModelID()
			}
			return p.updateInput(msg)

		case providerSettingsStateModelContext:
			if key.Matches(msg, p.keyMap.Select) {
				return p.commitModelContext()
			}
			return p.updateInput(msg)
		}
	}
	return nil
}

func (p *ProviderSettings) handleClose() Action {
	switch p.state {
	case providerSettingsStateProviders:
		return ActionClose{}
	case providerSettingsStateModels:
		p.state = providerSettingsStateProviders
		p.resetFilter()
		p.reloadProviders()
		return nil
	default:
		// Abandon the pending model entry and return to the model list.
		p.state = providerSettingsStateModels
		p.reloadModels()
		return nil
	}
}

func (p *ProviderSettings) moveSelection(delta int) {
	if p.list.Len() == 0 {
		return
	}
	p.list.Focus()
	if delta > 0 {
		if p.list.IsSelectedLast() {
			p.list.SelectFirst()
		} else {
			p.list.SelectNext()
		}
	} else {
		if p.list.IsSelectedFirst() {
			p.list.SelectLast()
		} else {
			p.list.SelectPrev()
		}
	}
	p.list.ScrollToSelected()
}

// filterInput routes keystrokes to the filter field of the provider list.
func (p *ProviderSettings) filterInput(msg tea.Msg) Action {
	prev := p.input.Value()
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	if value := p.input.Value(); value != prev {
		p.list.SetFilter(value)
		p.list.SetSelected(0)
	}
	return ActionCmd{cmd}
}

// updateInput routes keystrokes to the active model form field.
func (p *ProviderSettings) updateInput(msg tea.Msg) Action {
	var cmd tea.Cmd
	p.input, cmd = p.input.Update(msg)
	return ActionCmd{cmd}
}

func (p *ProviderSettings) startAddingModel() {
	p.state = providerSettingsStateModelID
	p.newModelID = ""
	p.input.SetValue("")
	p.input.Prompt = "Model: "
	p.input.Placeholder = "model id as the provider names it"
}

// commitModelID validates the typed model ID and moves to the optional
// context window field.
func (p *ProviderSettings) commitModelID() Action {
	id := strings.TrimSpace(p.input.Value())
	if id == "" {
		return ActionCmd{util.ReportWarn("A model ID is required")}
	}
	for _, existing := range p.models {
		if existing.ID == id {
			return ActionCmd{util.ReportWarn("Model " + id + " is already configured")}
		}
	}
	p.newModelID = id
	p.state = providerSettingsStateModelContext
	p.input.SetValue("")
	p.input.Prompt = "Context: "
	p.input.Placeholder = "context window in tokens, empty for none"
	return nil
}

// commitModelContext stores the new model and persists the provider's models.
func (p *ProviderSettings) commitModelContext() Action {
	var contextWindow int64
	if raw := strings.TrimSpace(p.input.Value()); raw != "" {
		parsed, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || parsed <= 0 {
			return ActionCmd{util.ReportWarn("Enter a whole number of tokens, or leave empty")}
		}
		contextWindow = parsed
	}

	p.models = append(p.models, catwalk.Model{
		ID:            p.newModelID,
		Name:          p.newModelID,
		ContextWindow: contextWindow,
	})
	p.state = providerSettingsStateModels
	return p.saveModels()
}

func (p *ProviderSettings) removeSelectedModel() Action {
	item, ok := p.list.SelectedItem().(ListItem)
	if !ok {
		return nil
	}
	id := item.ID()
	kept := p.models[:0]
	for _, model := range p.models {
		if model.ID != id {
			kept = append(kept, model)
		}
	}
	p.models = kept
	return p.saveModels()
}

// toggleDiscovery turns model discovery on and off for the provider. When it
// is off, only the models listed here are used.
func (p *ProviderSettings) toggleDiscovery() Action {
	p.discover = !p.discover
	err := p.com.Workspace.SetConfigField(
		config.ScopeGlobal,
		"providers."+p.providerID+".discover_models",
		p.discover,
	)
	if err != nil {
		return ActionCmd{util.ReportError(err)}
	}
	return ActionProviderSettingsChanged{ProviderID: p.providerID}
}

// saveModels writes the provider's model list to the global config.
func (p *ProviderSettings) saveModels() Action {
	key := "providers." + p.providerID + ".models"
	err := p.com.Workspace.SetConfigField(config.ScopeGlobal, key, p.models)
	if err != nil {
		return ActionCmd{util.ReportError(err)}
	}
	p.reloadModels()
	return ActionProviderSettingsChanged{ProviderID: p.providerID}
}

// Cursor implements [dialog.LoadingDialog] behaviour for the input steps.
func (p *ProviderSettings) Cursor() *tea.Cursor {
	if p.state != providerSettingsStateModelID && p.state != providerSettingsStateModelContext {
		return nil
	}
	return InputCursor(p.com.Styles, p.input.Cursor())
}

// Draw implements Dialog.
func (p *ProviderSettings) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := p.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	rc := NewRenderContext(t, width)
	rc.Title = p.title()

	switch p.state {
	case providerSettingsStateModelID, providerSettingsStateModelContext:
		p.input.SetWidth(dialogInputTextWidth(t, p.input, innerWidth))
		rc.AddPart(t.Dialog.InputPrompt.Render(p.input.View()))
	case providerSettingsStateProviders:
		p.input.SetWidth(dialogInputTextWidth(t, p.input, innerWidth))
		rc.AddPart(t.Dialog.InputPrompt.Render(p.input.View()))
		fallthrough
	case providerSettingsStateModels:
		listHeight, listTotalHeight, _ := sizeDialogList(t, p.list, innerWidth, height)
		if p.list.Height() >= p.list.TotalHeight() {
			p.list.ScrollToTop()
		}
		listView := t.Dialog.List.Height(listHeight).Render(p.list.Render())
		rc.AddPart(joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, p.list.Offset()))
	}

	rc.Help = renderDialogHelp(t, &p.help, p, innerWidth)
	DrawCenter(scr, area, rc.Render())
	return p.Cursor()
}
func (p *ProviderSettings) title() string {
	switch p.state {
	case providerSettingsStateProviders:
		return p.com.L("provider_settings.title")
	case providerSettingsStateModels:
		return p.providerID
	case providerSettingsStateModelID:
		return p.com.L("provider_settings.add_model")
	default:
		return p.com.L("provider_settings.context_window")
	}
}

// ShortHelp implements [help.KeyMap].
func (p *ProviderSettings) ShortHelp() []key.Binding {
	switch p.state {
	case providerSettingsStateProviders:
		return []key.Binding{p.keyMap.Select, p.keyMap.Close}
	case providerSettingsStateModels:
		return []key.Binding{p.keyMap.Add, p.keyMap.Remove, p.keyMap.Toggle, p.keyMap.Close}
	default:
		return []key.Binding{p.keyMap.Select, p.keyMap.Close}
	}
}

// FullHelp implements [help.KeyMap].
func (p *ProviderSettings) FullHelp() [][]key.Binding {
	return [][]key.Binding{p.ShortHelp()}
}
