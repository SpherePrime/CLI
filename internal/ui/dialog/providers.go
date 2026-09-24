package dialog

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/help"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/spinner"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/textinput"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/discover"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/util"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
)

// ProvidersID is the identifier for the provider add dialog.
const ProvidersID = "providers"

// providerContextWindowOverride is the context window assigned to every
// discovered model.
const providerContextWindowOverride = 270_000

type providersState int

const (
	providersStateID providersState = iota
	providersStateBaseURL
	providersStateAPIKey
	providersStateDiscovering
	providersStateError
	providersStateModels
)

// Providers is a step-based form dialog for adding a custom LLM provider.
type Providers struct {
	com *common.Common

	state        providersState
	providerID   string
	errorMessage string

	idInput  textinput.Model
	urlInput textinput.Model
	keyInput textinput.Model

	// modelInputs holds one text field per manually added model. Entering
	// a model ID and pressing enter appends a fresh field for the next one.
	modelInputs []textinput.Model
	modelIDs    []string

	spinner spinner.Model
	help    help.Model

	keyMap struct {
		Submit key.Binding
		Add    key.Binding
		Remove key.Binding
		Close  key.Binding
	}

	width int
}

var _ Dialog = (*Providers)(nil)

// NewProviders creates a new provider add dialog.
func NewProviders(com *common.Common) *Providers {
	m := &Providers{com: com}

	m.idInput = textinput.New()
	m.idInput.SetVirtualCursor(false)
	m.idInput.Prompt = "ID: "
	m.idInput.Placeholder = "provider ID, e.g. my-llm"
	m.idInput.SetStyles(com.Styles.TextInput)
	m.idInput.Focus()

	m.urlInput = textinput.New()
	m.urlInput.SetVirtualCursor(false)
	m.urlInput.Prompt = "Base URL: "
	m.urlInput.Placeholder = "https://host/v1"
	m.urlInput.SetStyles(com.Styles.TextInput)

	m.keyInput = textinput.New()
	m.keyInput.SetVirtualCursor(false)
	m.keyInput.Prompt = "API key: "
	m.keyInput.Placeholder = "sk-..."
	m.keyInput.SetStyles(com.Styles.TextInput)

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(com.Styles.Dialog.APIKey.Spinner),
	)

	m.help = help.New()
	m.help.Styles = com.Styles.DialogHelpStyles()

	m.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "next"),
	)
	m.keyMap.Add = key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", "add model"),
	)
	m.keyMap.Remove = key.NewBinding(
		key.WithKeys("ctrl+x"),
		key.WithHelp("ctrl+x", "remove model"),
	)
	m.keyMap.Close = CloseKey

	return m
}

// ID implements [Dialog].
func (m *Providers) ID() string {
	return ProvidersID
}

// activeInput returns the text input for the current form step.
func (m *Providers) activeInput() *textinput.Model {
	switch m.state {
	case providersStateID, providersStateError:
		return &m.idInput
	case providersStateBaseURL:
		return &m.urlInput
	case providersStateModels:
		if len(m.modelInputs) == 0 {
			m.ensureModelInput()
		}
		return &m.modelInputs[len(m.modelInputs)-1]
	default:
		return &m.keyInput
	}
}

// HandleMsg implements [Dialog].
func (m *Providers) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case providersDiscoveredMsg:
		if msg.err != nil {
			m.state = providersStateModels
			m.errorMessage = msg.err.Error()
			if len(m.modelInputs) == 0 {
				m.ensureModelInput()
			}
			m.modelInputs[len(m.modelInputs)-1].Focus()
			return nil
		}
		if err := m.persistProvider(msg.models); err != nil {
			return ActionCmd{util.ReportError(fmt.Errorf("failed to save provider: %w", err))}
		}
		return ActionSaveProvider{}
	case spinner.TickMsg:
		if m.state == providersStateDiscovering {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case m.state == providersStateDiscovering:
			// Ignore keys while the discovery request is in flight.
		case key.Matches(msg, m.keyMap.Submit):
			if m.state == providersStateModels {
				if len(m.modelInputs) == 0 {
					return nil
				}
				m.CommitModelInputs()
				models := make([]catwalk.Model, 0, len(m.modelIDs))
				for _, id := range m.modelIDs {
					models = append(models, catwalk.Model{ID: id, Name: id})
				}
				if err := m.persistProvider(models); err != nil {
					m.errorMessage = err.Error()
					return nil
				}
				return ActionSaveProvider{}
			}
			m.advance()
			if m.state == providersStateDiscovering {
				return ActionCmd{tea.Batch(m.spinner.Tick, m.discoverModels())}
			}
		case m.state == providersStateModels && key.Matches(msg, m.keyMap.Add):
			if len(m.modelInputs) > 0 {
				m.CommitModelInputs()
			}
			m.ensureModelInput()
			m.modelInputs[len(m.modelInputs)-1].Focus()
	case m.state == providersStateModels && key.Matches(msg, m.keyMap.Remove):
		if len(m.modelInputs) > 1 {
			m.modelInputs = m.modelInputs[:len(m.modelInputs)-1]
			m.modelInputs[len(m.modelInputs)-1].Focus()
		}
	default:
			input := m.activeInput()
			var cmd tea.Cmd
			*input, cmd = input.Update(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	case tea.PasteMsg:
		if m.state == providersStateDiscovering {
			return nil
		}
		input := m.activeInput()
		var cmd tea.Cmd
		*input, cmd = input.Update(msg)
		if cmd != nil {
			return ActionCmd{cmd}
		}
	}
	return nil
}

// ensureModelInput creates the model input fields: one per manually added
// model plus one fresh field waiting for input.
func (m *Providers) ensureModelInput() {
	for _, id := range m.modelIDs {
		input := m.buildModelInput()
		input.SetValue(id)
		m.modelInputs = append(m.modelInputs, input)
	}
	if len(m.modelInputs) == 0 {
		m.modelInputs = append(m.modelInputs, m.buildModelInput())
	} else {
		last := &m.modelInputs[len(m.modelInputs)-1]
		last.SetValue("")
		last.Focus()
	}
}

// CommitModelInputs finalizes the manually entered model fields: filled-in
// values become the committed list, and a fresh empty field is opened for
// the next entry.
func (m *Providers) CommitModelInputs() {
	trimmed := make([]string, 0, len(m.modelIDs)+1)
	for _, input := range m.modelInputs {
		value := strings.TrimSpace(input.Value())
		if value != "" && !slices.Contains(trimmed, value) {
			trimmed = append(trimmed, value)
		}
	}
	m.modelIDs = trimmed

	committed := m.modelInputs
	m.modelInputs = nil
	for _, input := range committed {
		input.SetValue("")
	}
	for range m.modelIDs {
		m.modelInputs = append(m.modelInputs, m.buildModelInput())
	}
	m.modelInputs = append(m.modelInputs, m.buildModelInput())
	m.modelInputs[len(m.modelInputs)-1].Focus()
}

func (m *Providers) buildModelInput() textinput.Model {
	input := textinput.New()
	input.SetVirtualCursor(false)
	input.Prompt = "Model: "
	input.Placeholder = "model ID, e.g. llama-3-70b"
	input.SetStyles(m.com.Styles.TextInput)
	return input
}

// advance moves to the next form step when the current value is valid.
func (m *Providers) advance() {
	switch m.state {
	case providersStateID:
		id := strings.TrimSpace(m.idInput.Value())
		if id == "" || strings.ContainsAny(id, "/ ") {
			return
		}
		m.providerID = id
		m.state = providersStateBaseURL
		m.urlInput.SetValue("")
		m.urlInput.Focus()
	case providersStateBaseURL:
		baseURL := strings.TrimSpace(m.urlInput.Value())
		if baseURL == "" {
			return
		}
		if u, err := url.Parse(baseURL); err != nil || (u.Scheme == "" && u.Host == "") {
			return
		}
		m.state = providersStateAPIKey
		m.keyInput.SetValue("")
		m.keyInput.Focus()
	case providersStateError:
		m.state = providersStateModels
		m.ensureModelInput()
	case providersStateAPIKey:
		if strings.TrimSpace(m.keyInput.Value()) == "" {
			return
		}
		m.state = providersStateDiscovering
	case providersStateModels:
		if len(m.modelInputs) == 0 {
			return
		}
		m.CommitModelInputs()
		models := make([]catwalk.Model, 0, len(m.modelIDs))
		for _, id := range m.modelIDs {
			models = append(models, catwalk.Model{ID: id, Name: id})
		}
		if err := m.persistProvider(models); err != nil {
			m.errorMessage = err.Error()
			return
		}
	}
}

// persistProvider writes the provider's configuration to the global config.
func (m *Providers) persistProvider(models []catwalk.Model) error {
	baseURL := strings.TrimRight(strings.TrimSpace(m.urlInput.Value()), "/")
	apiKey := strings.TrimSpace(m.keyInput.Value())
	providerType := string(catwalk.TypeOpenAICompat)
	id := "providers." + m.providerID

	for i := range models {
		models[i].ContextWindow = providerContextWindowOverride
	}

	// One dotted path per field: a nested object passed through
	// sjson.Set would clobber sibling keys, so set each key on its own.
	return m.com.Workspace.SetConfigFields(config.ScopeGlobal, map[string]any{
		id + ".name":            m.providerID,
		id + ".base_url":        baseURL,
		id + ".api_key":         apiKey,
		id + ".type":            providerType,
		id + ".discover_models": true,
		id + ".models":          models,
	})
}

// discoverModels runs the /models fetch and returns a command that reports
// the result to the dialog.
func (m *Providers) discoverModels() tea.Cmd {
	baseURL := strings.TrimRight(strings.TrimSpace(m.urlInput.Value()), "/")
	apiKey := strings.TrimSpace(m.keyInput.Value())

	ctx, cancel := context.WithTimeout(context.Background(), discover.ProviderDiscoveryTimeout)
	cfg := discover.Config{
		ID:      m.providerID,
		BaseURL: baseURL,
		APIKey:  apiKey,
	}
	resolver := m.com.Workspace.Resolver()

	return func() tea.Msg {
		defer cancel()
		models, err := discover.DiscoverModels(ctx, cfg, resolver)
		return providersDiscoveredMsg{models: models, err: err}
	}
}

// providersDiscoveredMsg reports the outcome of the model discovery.
type providersDiscoveredMsg struct {
	models []catwalk.Model
	err    error
}

// Cursor returns the cursor position for the active input. Only the states
// that render an input line may report a cursor: InputCursor assumes the
// input sits directly beneath the dialog title, so returning a cursor for a
// state without a rendered input would place it on the wrong line.
func (m *Providers) Cursor() *tea.Cursor {
	switch m.state {
	case providersStateID, providersStateBaseURL, providersStateAPIKey, providersStateModels:
		return InputCursor(m.com.Styles, m.activeInput().Cursor())
	}
	return nil
}

// Draw implements [Dialog].
func (m *Providers) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles

	m.width = max(0, min(60, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := m.width - t.Dialog.View.GetHorizontalFrameSize()
	activeInput := m.activeInput()
	activeInput.SetWidth(dialogInputTextWidth(t, *activeInput, innerWidth))

	textStyle := t.Dialog.SecondaryText
	helpView := renderDialogHelp(t, &m.help, m, innerWidth)

	rc := NewRenderContext(t, m.width)

	switch m.state {
	case providersStateID:
		rc.Title = "Add Provider"
		rc.AddPart(t.Dialog.InputPrompt.Render(activeInput.View()))
		rc.AddPart(textStyle.Render("A short unique identifier, e.g. my-llm"))
	case providersStateBaseURL:
		rc.Title = "Add Provider"
		rc.AddPart(t.Dialog.InputPrompt.Render(activeInput.View()))
		rc.AddPart(textStyle.Render("ID: " + m.providerID))
		rc.AddPart(textStyle.Render("e.g. https://api.my-llm.com/v1"))
	case providersStateAPIKey:
		rc.Title = "Add Provider"
		rc.AddPart(t.Dialog.InputPrompt.Render(activeInput.View()))
		rc.AddPart(textStyle.Render("ID: " + m.providerID))
		rc.AddPart(textStyle.Render("Base URL: " + strings.TrimRight(strings.TrimSpace(m.urlInput.Value()), "/")))
	case providersStateDiscovering:
		rc.Title = "Adding Provider"
		rc.AddPart(textStyle.Render(m.spinner.View() + " Fetching models from " + strings.TrimRight(strings.TrimSpace(m.urlInput.Value()), "/") + "..."))
	case providersStateModels:
		rc.Title = "Add Models"
		for _, input := range m.modelInputs {
			input.SetWidth(dialogInputTextWidth(t, input, innerWidth))
			rc.AddPart(t.Dialog.InputPrompt.Render(input.View()))
		}
		if m.errorMessage != "" {
			rc.AddPart(textStyle.Render(m.errorMessage))
			m.errorMessage = ""
		}
		rc.AddPart(textStyle.Render("Ctrl+n adds a field, Ctrl+x removes the active one, enter saves the provider."))
	}

	rc.Help = helpView
	view := rc.Render()
	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// ShortHelp implements [help.KeyMap].
func (m *Providers) ShortHelp() []key.Binding {
	if m.state == providersStateModels {
		return []key.Binding{m.keyMap.Submit, m.keyMap.Add, m.keyMap.Remove, m.keyMap.Close}
	}
	return []key.Binding{m.keyMap.Submit, m.keyMap.Close}
}

// FullHelp implements [help.KeyMap].
func (m *Providers) FullHelp() [][]key.Binding {
	if m.state == providersStateModels {
		return [][]key.Binding{{m.keyMap.Submit, m.keyMap.Add, m.keyMap.Remove, m.keyMap.Close}}
	}
	return [][]key.Binding{{m.keyMap.Submit, m.keyMap.Close}}
}
