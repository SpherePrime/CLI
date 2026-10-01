package dialog

import (
	"strings"

	"github.com/SpherePrime/CLI/internal/i18n"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/help"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/textinput"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
	"github.com/SpherePrime/CLI/vendordeps/sahilm/fuzzy"
)

const (
	// VoiceLanguageID is the identifier for the dictation language picker.
	VoiceLanguageID              = "voice_language"
	voiceLanguageDialogMaxWidth  = 50
	voiceLanguageDialogMaxHeight = 14
	// autoVoiceTag is the internal sentinel for "no preference": the engine
	// decides, led by the interface language.
	autoVoiceTag = "auto"
)

// voiceLanguageTag renders a configured options.voice.language value into the
// sentinel used for comparison: blanks and "auto" collapse to auto, otherwise
// tags keep their order lowercased.
func voiceLanguageTag(language string) string {
	normalized := strings.ToLower(strings.ReplaceAll(language, " ", ""))
	if normalized == "" || normalized == autoVoiceTag {
		return autoVoiceTag
	}
	return normalized
}

// VoiceLanguage is a dialog for selecting the dictation language, the
// options.voice.language value tried by transcription engines (Google Web
// Speech first among them).
type VoiceLanguage struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model

	// mouse gives the list full pointer support: click a row to select
	// it, click it again to apply, wheel to scroll.
	mouse ListMouse

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		UpDown   key.Binding
		Close    key.Binding
	}
}

// VoiceLanguageItem represents a selectable dictation language option.
type VoiceLanguageItem struct {
	*list.Versioned
	tag       string
	title     string
	isCurrent bool
	t         *styles.Styles
	m         fuzzy.Match
	cache     map[int]string
	focused   bool
}

// Finished implements list.Item.
func (v *VoiceLanguageItem) Finished() bool {
	return true
}

var (
	_ Dialog   = (*VoiceLanguage)(nil)
	_ ListItem = (*VoiceLanguageItem)(nil)
)

// NewVoiceLanguage creates a new dictation language picker dialog.
func NewVoiceLanguage(com *common.Common) *VoiceLanguage {
	v := &VoiceLanguage{com: com}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	v.help = h

	v.list = list.NewFilterableList()
	v.list.Focus()

	v.input = textinput.New()
	v.input.SetVirtualCursor(false)
	v.input.Placeholder = v.com.L("cmd.type_to_filter")
	v.input.SetStyles(com.Styles.TextInput)
	v.input.Focus()

	v.keyMap.Select = key.NewBinding(
		key.WithKeys("enter", "ctrl+y"),
		key.WithHelp("enter", "confirm"),
	)
	v.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", "next item"),
	)
	v.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", "previous item"),
	)
	v.keyMap.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑/↓", "choose"),
	)
	v.keyMap.Close = CloseKey

	v.setItems()
	return v
}

// ID implements Dialog.
func (*VoiceLanguage) ID() string {
	return VoiceLanguageID
}

// HandleMsg implements [Dialog].
func (v *VoiceLanguage) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case common.CoalescedWheelMsg, tea.MouseClickMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg:
		return v.mouse.HandleMsg(msg, v.list, v.activate)
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, v.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, v.keyMap.Previous):
			v.list.Focus()
			if v.list.IsSelectedFirst() {
				v.list.SelectLast()
				v.list.ScrollToBottom()
				break
			}
			v.list.SelectPrev()
			v.list.ScrollToSelected()
		case key.Matches(msg, v.keyMap.Next):
			v.list.Focus()
			if v.list.IsSelectedLast() {
				v.list.SelectFirst()
				v.list.ScrollToTop()
				break
			}
			v.list.SelectNext()
			v.list.ScrollToSelected()
		case key.Matches(msg, v.keyMap.Select):
			return v.activate(v.list.Selected())
		default:
			prevValue := v.input.Value()
			var cmd tea.Cmd
			v.input, cmd = v.input.Update(msg)
			value := v.input.Value()
			if value != prevValue {
				v.list.SetFilter(value)
				v.list.ScrollToTop()
				v.list.SetSelected(0)
			}
			return ActionCmd{cmd}
		}
	}
	return nil
}

// Cursor returns the cursor position relative to the dialog.
func (v *VoiceLanguage) Cursor() *tea.Cursor {
	return InputCursor(v.com.Styles, v.input.Cursor())
}

// Draw implements [Dialog].
func (v *VoiceLanguage) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := v.com.Styles
	width := max(0, min(voiceLanguageDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(voiceLanguageDialogMaxHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	heightOffset := t.Dialog.Title.GetVerticalFrameSize() + titleContentHeight +
		t.Dialog.InputPrompt.GetVerticalFrameSize() + inputContentHeight +
		t.Dialog.HelpView.GetVerticalFrameSize() +
		t.Dialog.View.GetVerticalFrameSize()

	v.input.SetWidth(dialogInputTextWidth(t, v.input, innerWidth))
	v.list.SetSize(innerWidth, max(0, height-heightOffset))

	tr := v.translator()
	rc := NewRenderContext(t, width)
	rc.Title = tr.Label("cmd.voice_language")
	inputView := t.Dialog.InputPrompt.Render(v.input.View())
	rc.AddPart(inputView)

	visibleCount := len(v.list.FilteredItems())
	if v.list.Height() >= visibleCount {
		v.list.ScrollToTop()
	} else {
		v.list.ScrollToSelected()
	}

	listView := t.Dialog.List.Height(v.list.Height()).Render(v.list.Render())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &v.help, v, innerWidth)

	view := rc.Render()

	// Record where the list painted so clicks can be hit-tested.
	v.mouse.Painted(dialogBodyRect(area, dialogRectCentered(area, view), listView, rc.Help, rc.ViewStyle, t.Dialog.List, innerWidth, v.list.Height()))

	cur := v.Cursor()
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// activate applies the dictation language at idx. Shared by the enter key
// and by a click on a row.
func (v *VoiceLanguage) activate(idx int) Action {
	item, ok := v.list.ItemAt(idx).(*VoiceLanguageItem)
	if !ok || item == nil {
		return nil
	}
	return ActionSelectVoiceLanguage{Tag: item.tag, Label: item.title}
}

// ShortHelp implements [help.KeyMap].
func (v *VoiceLanguage) ShortHelp() []key.Binding {
	return []key.Binding{
		v.keyMap.UpDown,
		v.keyMap.Select,
		v.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (v *VoiceLanguage) FullHelp() [][]key.Binding {
	m := [][]key.Binding{}
	slice := []key.Binding{
		v.keyMap.Select,
		v.keyMap.Next,
		v.keyMap.Previous,
		v.keyMap.Close,
	}
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		m = append(m, slice[i:end])
	}
	return m
}

func (v *VoiceLanguage) translator() i18n.Translator {
	cfg := v.com.Config()
	locale := i18n.En
	if cfg != nil && cfg.Options != nil && cfg.Options.Language != "" {
		locale = cfg.Options.Language
	}
	return i18n.New(locale)
}

// voiceLanguageChoices are the options offered by the picker. Tags go into
// options.voice.language verbatim; lists are what the engines try in order,
// which is how "understand me in either language" is expressed. The native
// spellings stay untranslated on purpose: they identify the language better
// than any translation would.
func (v *VoiceLanguage) choices() []struct{ tag, title string } {
	tr := v.translator()
	return []struct{ tag, title string }{
		{autoVoiceTag, tr.Label("voice.language_auto")},
		{"ru-RU", "Русский (ru-RU)"},
		{"en-US", "English (en-US)"},
		{"ru-RU,en-US", tr.Label("voice.language_ru_en")},
		{"en-US,ru-RU", tr.Label("voice.language_en_ru")},
		{"uk-UA", "Українська (uk-UA)"},
		{"de-DE", "Deutsch (de-DE)"},
		{"es-ES", "Español (es-ES)"},
		{"fr-FR", "Français (fr-FR)"},
		{"pl-PL", "Polski (pl-PL)"},
		{"tr-TR", "Türkçe (tr-TR)"},
	}
}

func (v *VoiceLanguage) setItems() {
	cfg := v.com.Config()
	current := autoVoiceTag
	if cfg != nil && cfg.Options != nil && cfg.Options.Voice != nil {
		current = voiceLanguageTag(cfg.Options.Voice.Language)
	}

	choices := v.choices()
	items := make([]list.FilterableItem, 0, len(choices))
	selectedIndex := 0
	for i, choice := range choices {
		item := &VoiceLanguageItem{
			Versioned: list.NewVersioned(),
			tag:       choice.tag,
			title:     choice.title,
			isCurrent: voiceLanguageTag(choice.tag) == current,
			t:         v.com.Styles,
		}
		if item.isCurrent {
			selectedIndex = i
		}
		items = append(items, item)
	}

	v.list.SetItems(items...)
	v.list.SetSelected(selectedIndex)
	v.list.ScrollToSelected()
}

// Filter returns the filter value for the dictation language item.
func (v *VoiceLanguageItem) Filter() string {
	return strings.ToLower(v.title + " " + v.tag)
}

// ID returns the unique identifier for the option.
func (v *VoiceLanguageItem) ID() string {
	return v.tag
}

// SetFocused sets the focus state of the dictation language item.
func (v *VoiceLanguageItem) SetFocused(focused bool) {
	if v.focused == focused {
		return
	}
	v.cache = nil
	v.focused = focused
	if v.Versioned != nil {
		v.Bump()
	}
}

// SetMatch sets the fuzzy match for the dictation language item.
func (v *VoiceLanguageItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(v.m, m) {
		return
	}
	v.cache = nil
	v.m = m
	if v.Versioned != nil {
		v.Bump()
	}
}

// Render returns the string representation of the dictation language item.
func (v *VoiceLanguageItem) Render(width int) string {
	info := ""
	if v.isCurrent {
		info = "current"
	}
	st := ListItemStyles{
		ItemBlurred:     v.t.Dialog.NormalItem,
		ItemFocused:     v.t.Dialog.SelectedItem,
		InfoTextBlurred: v.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: v.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(st, v.title, info, v.focused, width, v.cache, &v.m)
}
