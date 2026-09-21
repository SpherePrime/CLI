package dialog

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/dwertyfa288/CLI/vendordeps/bubbles/v2/help"
	"github.com/dwertyfa288/CLI/vendordeps/bubbles/v2/key"
	"github.com/dwertyfa288/CLI/vendordeps/bubbles/v2/spinner"
	"github.com/dwertyfa288/CLI/vendordeps/bubbles/v2/textinput"
	tea "github.com/dwertyfa288/CLI/vendordeps/bubbletea/v2"
	"github.com/dwertyfa288/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/dwertyfa288/CLI/internal/config"
	"github.com/dwertyfa288/CLI/internal/ui/common"
	"github.com/dwertyfa288/CLI/internal/ui/util"
	uv "github.com/dwertyfa288/CLI/vendordeps/dwertyfa288/ultraviolet"
)

// ModelsConfigID is the identifier for the model settings dialog.
const ModelsConfigID = "models_config"

const modelsConfigDialogMaxWidth = 73

// modelConfigField is one editable input in the model settings dialog.
type modelConfigField int

const (
	fieldContextWindow modelConfigField = iota
	fieldPriceIn
	fieldPriceOut
)

// modelConfigState is the current step of the settings form.
type modelConfigState int

const (
	modelConfigStateEditing modelConfigState = iota
	modelConfigStateSaving
)

// ModelsConfig is a dialog for configuring the context window and per-token
// price overrides of the currently selected model.
type ModelsConfig struct {
	com *common.Common

	modelType ModelType
	selected  config.SelectedModel
	catalog   catwalk.Model

	state    modelConfigState
	editing  modelConfigField
	fields   [3]string
	input    textinput.Model
	spinner  spinner.Model
	help     help.Model
	errorMsg string

	keyMap struct {
		NextField  key.Binding
		PrevField  key.Binding
		Select     key.Binding
		ToggleType key.Binding
		Close      key.Binding
	}
}

var _ Dialog = (*ModelsConfig)(nil)

// NewModelsConfig creates a new model settings dialog for the given model
// type, loading the currently selected model and its catalog entry.
func NewModelsConfig(com *common.Common, modelType ModelType) (*ModelsConfig, error) {
	t := com.Styles
	m := &ModelsConfig{modelType: modelType}
	m.com = com

	cfg := com.Config()
	selected, ok := cfg.Models[modelType.Config()]
	if !ok {
		return nil, fmt.Errorf("no %s model selected", modelType)
	}
	m.selected = selected

	catalog := cfg.GetModel(selected.Provider, selected.Model)
	if catalog == nil {
		return nil, fmt.Errorf("model %s not found for provider %s", selected.Model, selected.Provider)
	}
	m.catalog = *catalog

	m.help = help.New()
	m.help.Styles = t.DialogHelpStyles()

	m.input = textinput.New()
	m.input.SetVirtualCursor(false)
	m.input.CharLimit = 24
	m.input.SetStyles(com.Styles.TextInput)
	m.input.Focus()

	m.spinner = spinner.New(
		spinner.WithSpinner(spinner.Dot),
		spinner.WithStyle(t.Dialog.APIKey.Spinner),
	)

	m.keyMap.NextField = key.NewBinding(
		key.WithKeys("tab", "down"),
		key.WithHelp("tab", "next field"),
	)
	m.keyMap.PrevField = key.NewBinding(
		key.WithKeys("shift+tab", "up"),
		key.WithHelp("btab", "prev field"),
	)
	m.keyMap.ToggleType = key.NewBinding(
		key.WithKeys("ctrl+tab"),
		key.WithHelp("ctrl+tab", "toggle type"),
	)
	m.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "save"),
	)
	m.keyMap.Close = CloseKey

	m.loadFields()
	m.editing = fieldContextWindow
	m.state = modelConfigStateEditing
	m.applyFieldInput()
	m.setFieldValue()

	return m, nil
}

// ID implements Dialog.
func (m *ModelsConfig) ID() string {
	return ModelsConfigID
}

// loadFields populates the editable fields with the current selected-model
// override values, falling back to the catalog values when unset.
func (m *ModelsConfig) loadFields() {
	cw := m.selected.ContextWindow
	if cw <= 0 {
		cw = m.catalog.ContextWindow
	}
	priceIn := m.selected.PriceIn
	if priceIn <= 0 {
		priceIn = m.catalog.CostPer1MIn
	}
	priceOut := m.selected.PriceOut
	if priceOut <= 0 {
		priceOut = m.catalog.CostPer1MOut
	}
	m.fields[fieldContextWindow] = strconv.FormatInt(cw, 10)
	m.fields[fieldPriceIn] = strconv.FormatFloat(priceIn, 'f', -1, 64)
	m.fields[fieldPriceOut] = strconv.FormatFloat(priceOut, 'f', -1, 64)
}

// applyFieldInput updates the active input's prompt and placeholder. The
// input value is only set explicitly when switching between fields, so the
// user's in-progress edits are preserved across frames.
func (m *ModelsConfig) applyFieldInput() {
	var prompt, placeholder string
	switch m.editing {
	case fieldContextWindow:
		prompt = "Max context: "
		placeholder = "e.g. 200000"
	case fieldPriceIn:
		prompt = "Input $/1M: "
		placeholder = "price per 1M input tokens, 0 = use catalog"
	case fieldPriceOut:
		prompt = "Output $/1M: "
		placeholder = "price per 1M output tokens, 0 = use catalog"
	}
	m.input.Prompt = prompt
	m.input.Placeholder = placeholder
}

// setFieldValue loads the current field value into the input, used when
// switching to a different field.
func (m *ModelsConfig) setFieldValue() {
	m.input.SetValue(m.fields[m.editing])
}

// cycleField moves to the next or previous input field and focuses it.
func (m *ModelsConfig) cycleField(direction int) {
	m.fields[m.editing] = strings.TrimSpace(m.input.Value())
	total := len(m.fields)
	next := int(m.editing) + direction
	if next < 0 {
		next = total - 1
	}
	m.editing = modelConfigField(next % total)
	m.errorMsg = ""
	m.applyFieldInput()
	m.setFieldValue()
}

// HandleMsg implements Dialog.
func (m *ModelsConfig) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case modelsConfigSavedMsg:
		m.state = modelConfigStateEditing
		if msg.err != nil {
			m.errorMsg = msg.err.Error()
			return nil
		}
		m.errorMsg = ""
		m.selected.ContextWindow = msg.contextWindow
		m.selected.PriceIn = msg.priceIn
		m.selected.PriceOut = msg.priceOut
		return ActionModelConfigSaved{ModelType: m.modelType.Config()}
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		if cmd != nil {
			return ActionCmd{cmd}
		}
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, m.keyMap.Close):
			return ActionClose{}
		case m.state == modelConfigStateSaving:
			// Input is locked while the config write is in flight.
		case m.state == modelConfigStateEditing:
			switch {
			case key.Matches(msg, m.keyMap.Select):
				return m.save()
			case key.Matches(msg, m.keyMap.NextField):
				m.cycleField(1)
			case key.Matches(msg, m.keyMap.PrevField):
				m.cycleField(-1)
			case key.Matches(msg, m.keyMap.ToggleType):
				if m.modelType == ModelTypeLarge {
					m.modelType = ModelTypeSmall
				} else {
					m.modelType = ModelTypeLarge
				}
				if err := m.reloadForType(); err != nil {
					return util.ReportError(err)
				}
				m.errorMsg = ""
			default:
				cmd := m.focusAndInputUpdate(msg)
				if cmd != nil {
					return ActionCmd{cmd}
				}
			}
		}
	case tea.PasteMsg:
		if m.state == modelConfigStateEditing {
			cmd := m.focusAndInputUpdate(msg)
			if cmd != nil {
				return ActionCmd{cmd}
			}
		}
	}
	return nil
}

// reloadForType re-loads the selected model and catalog entry for the new
// model type after a type toggle.
func (m *ModelsConfig) reloadForType() error {
	cfg := m.com.Config()
	selected, ok := cfg.Models[m.modelType.Config()]
	if !ok {
		return fmt.Errorf("no %s model selected", m.modelType)
	}
	m.selected = selected
	catalog := cfg.GetModel(selected.Provider, selected.Model)
	if catalog == nil {
		return fmt.Errorf("model %s not found for provider %s", selected.Model, selected.Provider)
	}
	m.catalog = *catalog
	m.loadFields()
	m.editing = fieldContextWindow
	m.state = modelConfigStateEditing
	m.applyFieldInput()
	m.setFieldValue()
	return nil
}

// save validates the input fields and persists the overrides into the
// selected model config in the global config.
func (m *ModelsConfig) save() Action {
	m.fields[m.editing] = strings.TrimSpace(m.input.Value())

	cw, err := strconv.ParseInt(m.fields[fieldContextWindow], 10, 64)
	if err != nil || cw < 0 {
		m.errorMsg = "Max context must be a non-negative integer"
		return nil
	}
	priceIn, err := strconv.ParseFloat(m.fields[fieldPriceIn], 64)
	if err != nil || priceIn < 0 {
		m.errorMsg = "Input price must be a non-negative number"
		m.editing = fieldPriceIn
		m.applyFieldInput()
		m.setFieldValue()
		return nil
	}
	priceOut, err := strconv.ParseFloat(m.fields[fieldPriceOut], 64)
	if err != nil || priceOut < 0 {
		m.errorMsg = "Output price must be a non-negative number"
		m.editing = fieldPriceOut
		m.applyFieldInput()
		m.setFieldValue()
		return nil
	}

	m.state = modelConfigStateSaving
	return ActionCmd{func() tea.Msg {
		updated := m.selected
		updated.ContextWindow = cw
		updated.PriceIn = priceIn
		updated.PriceOut = priceOut

		err := m.com.Workspace.UpdatePreferredModel(config.ScopeGlobal, m.modelType.Config(), updated)
		return modelsConfigSavedMsg{
			contextWindow: cw,
			priceIn:       priceIn,
			priceOut:      priceOut,
			err:           err,
		}
	}}
}

// modelsConfigSavedMsg reports the outcome of a config write.
type modelsConfigSavedMsg struct {
	contextWindow int64
	priceIn       float64
	priceOut      float64
	err           error
}

// focusAndInputUpdate ensures the shared input is focused before routing a
// key event to it, and returns any follow-up command (e.g. a cursor-blink
// tick) that the caller must run.
func (m *ModelsConfig) focusAndInputUpdate(msg tea.Msg) tea.Cmd {
	if !m.input.Focused() {
		cmd := m.input.Focus()
		m.input.CursorEnd()
		return cmd
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return cmd
}

// Cursor returns the cursor for the dialog. The vertical offset is computed
// from the content rows rendered above the input view, so it stays aligned
// with the visible value even when the error line is present.
func (m *ModelsConfig) Cursor() *tea.Cursor {
	if m.state != modelConfigStateEditing {
		return nil
	}
	cur := InputCursor(m.com.Styles, m.input.Cursor())
	if cur != nil {
		rowsAbove := 1
		if m.errorMsg != "" {
			rowsAbove++
		}
		cur.Y += rowsAbove
	}
	return cur
}

// Draw implements [Dialog].
func (m *ModelsConfig) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := m.com.Styles
	width := max(0, min(modelsConfigDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	if m.state != modelConfigStateSaving {
		m.input.SetWidth(dialogInputTextWidth(t, m.input, innerWidth))
	}

	rc := NewRenderContext(t, width)
	rc.Title = "Model Settings"
	rc.TitleInfo = m.modelTypeRadioView()

	switch m.state {
	case modelConfigStateEditing:
		title := "Configure: " + m.catalog.Name
		rc.AddPart(t.Dialog.SecondaryText.Render(title))
		if m.errorMsg != "" {
			rc.AddPart(t.Dialog.TitleError.Render(m.errorMsg))
		}
		m.applyFieldInput()
		rc.AddPart(t.Dialog.InputPrompt.Render(m.input.View()))
	case modelConfigStateSaving:
		rc.AddPart(t.Dialog.SecondaryText.Render(m.spinner.View() + " Saving..."))
	}

	rc.Help = renderDialogHelp(t, &m.help, m, innerWidth)

	view := rc.Render()
	cur := m.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// modelTypeRadioView returns the radio view for model type selection.
func (m *ModelsConfig) modelTypeRadioView() string {
	t := m.com.Styles
	textStyle := t.Radio.Label
	largeRadioStyle := t.Radio.Off
	smallRadioStyle := t.Radio.Off
	if m.modelType == ModelTypeLarge {
		largeRadioStyle = t.Radio.On
	} else {
		smallRadioStyle = t.Radio.On
	}

	largeRadio := largeRadioStyle.Padding(0, 1).Render()
	smallRadio := smallRadioStyle.Padding(0, 1).Render()

	return fmt.Sprintf("%s%s  %s%s",
		largeRadio, textStyle.Render(ModelTypeLarge.String()),
		smallRadio, textStyle.Render(ModelTypeSmall.String()))
}

// ShortHelp returns the short help view.
func (m *ModelsConfig) ShortHelp() []key.Binding {
	return []key.Binding{
		m.keyMap.NextField,
		m.keyMap.PrevField,
		m.keyMap.Select,
		m.keyMap.ToggleType,
		m.keyMap.Close,
	}
}

// FullHelp returns the full help view.
func (m *ModelsConfig) FullHelp() [][]key.Binding {
	return [][]key.Binding{m.ShortHelp()}
}

// ActionModelConfigSaved is sent when the model settings dialog has
// persisted the context window and price overrides to the config.
type ActionModelConfigSaved struct {
	ModelType config.SelectedModelType
}
