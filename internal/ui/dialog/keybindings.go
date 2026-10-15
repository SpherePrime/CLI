package dialog

import (
	"fmt"
	"strings"
	"unicode"

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

// KeybindingsID is the identifier for the keybinding remap dialog.
const KeybindingsID = "keybindings"

const keybindingsDialogMaxWidth = 73

// Keybindings is a dialog for remapping the application's keybindings.
//
// It lists every rebindable action and lets the user capture a new key
// combination for the selected one. Each save is persisted immediately, so
// the user can walk the list and rebind as many actions as they want without
// a separate confirm step: entering a row starts the capture, the next key
// press becomes the new binding, and the config is written at once.
type Keybindings struct {
	com *common.Common

	infos   []common.KeyInfo
	changes map[string]string

	// capture is the action being re-bound. While it is set the next key
	// press is recorded as the new binding rather than driving the list.
	capture *common.KeyInfo

	help  help.Model
	list  *list.FilterableList
	input textinput.Model
	mouse ListMouse

	keyMap struct {
		Select   key.Binding
		Next     key.Binding
		Previous key.Binding
		Unbind   key.Binding
		Reset    key.Binding
		Close    key.Binding
	}
}

// KeybindingsItem is a single rebindable action in the Keybindings dialog.
type KeybindingsItem struct {
	*list.Versioned

	info         common.KeyInfo
	remap        string
	t            *styles.Styles
	unboundLabel string
	m            fuzzy.Match
	cache        map[int]string
	focused      bool
}

var (
	_ Dialog   = (*Keybindings)(nil)
	_ ListItem = (*KeybindingsItem)(nil)
)

// NewKeybindings creates a new Keybindings dialog. infos are the rebindable
// actions in name order, and current is the user's existing remaps (name to
// key string). The effective binding for each action is the remap when one
// exists, else the default, and that is what the list shows and edits.
func NewKeybindings(com *common.Common, infos []common.KeyInfo, current map[string]string) *Keybindings {
	k := &Keybindings{com: com, infos: infos}

	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	k.help = h

	k.list = list.NewFilterableList()
	k.list.Focus()

	k.input = textinput.New()
	k.input.SetVirtualCursor(false)
	k.input.Placeholder = k.com.L("cmd.type_to_filter")
	k.input.SetStyles(com.Styles.TextInput)
	k.input.Focus()

	k.keyMap.Select = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", k.com.L("keys.rebind")),
	)
	k.keyMap.Next = key.NewBinding(
		key.WithKeys("down", "ctrl+n"),
		key.WithHelp("↓", k.com.L("keys.next")),
	)
	k.keyMap.Previous = key.NewBinding(
		key.WithKeys("up", "ctrl+p"),
		key.WithHelp("↑", k.com.L("keys.prev")),
	)
	k.keyMap.Unbind = key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", k.com.L("keys.unbind")),
	)
	k.keyMap.Reset = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", k.com.L("keys.default")),
	)
	k.keyMap.Close = CloseKey

	k.changes = make(map[string]string, len(infos))
	for _, info := range infos {
		value := info.Default
		if remap, ok := current[info.Name]; ok {
			value = remap
		}
		k.changes[info.Name] = value
	}

	k.setItems()
	return k
}

// ID implements Dialog.
func (k *Keybindings) ID() string {
	return KeybindingsID
}

// BackspaceDeletesText implements [BackspaceAware].
func (k *Keybindings) BackspaceDeletesText() bool {
	if k.capture != nil {
		return false
	}
	return BackspaceDeletesText(k.input)
}

// Cursor returns the cursor position relative to the dialog.
func (k *Keybindings) Cursor() *tea.Cursor {
	return InputCursor(k.com.Styles, k.input.Cursor())
}

// HandleMsg implements [Dialog].
func (k *Keybindings) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case common.CoalescedWheelMsg, tea.MouseClickMsg, tea.MouseMotionMsg, tea.MouseReleaseMsg:
		if k.capture != nil {
			// Pointer input does not bind a key; ignore it while capturing.
			return nil
		}
		return k.mouse.HandleMsg(msg, k.list, nil)
	case tea.PasteMsg:
		if k.capture != nil {
			return nil
		}
		return filterPaste(&k.input, k.list, msg)
	case tea.KeyPressMsg:
		if k.capture != nil {
			return k.handleCapture(msg)
		}
		return k.handleListKey(msg)
	}
	return nil
}

// handleListKey routes key presses while the dialog is showing its list.
func (k *Keybindings) handleListKey(msg tea.KeyPressMsg) Action {
	switch {
	case key.Matches(msg, k.keyMap.Close):
		return ActionClose{}
	case key.Matches(msg, k.keyMap.Previous):
		k.moveSelection(-1)
	case key.Matches(msg, k.keyMap.Next):
		k.moveSelection(1)
	case key.Matches(msg, k.keyMap.Select):
		return k.startCapture()
	case key.Matches(msg, k.keyMap.Unbind):
		return k.saveSelected("")
	case key.Matches(msg, k.keyMap.Reset):
		info := k.selectedInfo()
		if info == nil {
			return nil
		}
		return k.saveSelected(info.Default)
	default:
		prevValue := k.input.Value()
		var cmd tea.Cmd
		k.input, cmd = k.input.Update(msg)
		value := k.input.Value()
		if value != prevValue {
			k.list.SetFilter(value)
			k.list.ScrollToTop()
			k.list.SetSelected(0)
		}
		return ActionCmd{cmd}
	}
	return nil
}

// handleCapture records the next press as the new binding for the action in
// capture. Only the dialog's close key is reserved, and it cancels the
// capture rather than closing the dialog, so a capture that goes wrong can
// always be abandoned without losing the whole remap session.
func (k *Keybindings) handleCapture(msg tea.KeyPressMsg) Action {
	if key.Matches(msg, k.keyMap.Close) {
		k.capture = nil
		return nil
	}
	// String is exactly what key.Matches compares against on a future
	// press (the character for printable keys, the canonical
	// "ctrl+alt+shift+key" for everything else), so what is captured is
	// guaranteed to be what the matcher will later see.
	return k.saveSelected(msg.String())
}

// startCapture begins recording a new binding for the selected action.
func (k *Keybindings) startCapture() Action {
	info := k.selectedInfo()
	if info == nil {
		return nil
	}
	k.capture = info
	return nil
}

// moveSelection moves the list selection by delta with wrap.
func (k *Keybindings) moveSelection(delta int) {
	if k.list.Len() == 0 {
		return
	}
	k.list.Focus()
	if delta > 0 {
		if k.list.IsSelectedLast() {
			k.list.SelectFirst()
		} else {
			k.list.SelectNext()
		}
	} else {
		if k.list.IsSelectedFirst() {
			k.list.SelectLast()
		} else {
			k.list.SelectPrev()
		}
	}
	k.list.ScrollToSelected()
}

// selectedInfo returns the KeyInfo under the cursor, or nil when the list is
// empty.
func (k *Keybindings) selectedInfo() *common.KeyInfo {
	item, ok := k.list.SelectedItem().(*KeybindingsItem)
	if !ok || item == nil {
		return nil
	}
	return &item.info
}

// saveSelected persists the new binding for the selected action, refreshes the
// row, and returns the action the model should handle to write it to config.
func (k *Keybindings) saveSelected(keyString string) Action {
	info := k.selectedInfo()
	if info == nil {
		k.capture = nil
		return nil
	}
	k.changes[info.Name] = keyString
	k.capture = nil
	k.setItems()
	return ActionSaveKeybindings{Bindings: k.collectSave()}
}

// collectSave produces the map that should be written to the config.
//
// An empty value is the "unbind" marker and is kept: dropping it would make
// "unbind" and "no remap" indistinguishable on the next load. Entries that
// match the default are dropped so the config only ever stores deviations.
// When nothing deviates from the defaults the whole map is nil so the field
// can be cleared entirely.
func (k *Keybindings) collectSave() map[string]string {
	out := make(map[string]string, len(k.changes))
	for name, value := range k.changes {
		defaultValue := value
		for _, info := range k.infos {
			if info.Name == name {
				defaultValue = info.Default
			}
		}
		if value != defaultValue {
			out[name] = value
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Refresh re-reads the user's remaps from the config and rebuilds the list.
// The rows are a snapshot taken when the dialog opens and nothing repainted
// them when the config changed, so this is how an external edit is picked up.
func (k *Keybindings) Refresh() {
	cfg := k.com.Config()
	var current map[string]string
	if cfg != nil && cfg.Options != nil {
		current = cfg.Options.KeyBindings
	}
	for _, info := range k.infos {
		value := info.Default
		if remap, ok := current[info.Name]; ok {
			value = remap
		}
		k.changes[info.Name] = value
	}
	k.setItems()
}

// Draw implements [Dialog].
func (k *Keybindings) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := k.com.Styles
	width := max(0, min(keybindingsDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	listHeight, listTotalHeight, listWidth := sizeDialogList(t, k.list, innerWidth, height)

	rc := NewRenderContext(t, width)
	rc.Title = k.com.L("cmd.key_bindings")
	rc.TitleInfo = k.titleInfo()

	k.input.SetWidth(dialogInputTextWidth(t, k.input, innerWidth))
	rc.AddPart(t.Dialog.InputPrompt.Render(k.input.View()))

	listView := t.Dialog.List.Height(k.list.Height()).Render(k.list.Render())
	scrollable := listView
	listView = joinScrollbar(t, listView, listHeight, listTotalHeight, listHeight, k.list.Offset())
	rc.AddPart(listView)
	rc.Help = renderDialogHelp(t, &k.help, k, innerWidth)

	view := rc.Render()
	body := dialogBodyRect(area, dialogRectCentered(area, view), listView, rc.Help, rc.ViewStyle, t.Dialog.List, listWidth, listHeight)
	k.mouse.Painted(body)
	k.mouse.PaintedJoin(body.Min, scrollable, listHeight, listTotalHeight, listHeight, k.list.Offset())

	DrawCenterCursor(scr, area, view, nil)
	return nil
}

// titleInfo builds the status line shown next to the title: the prompt while a
// binding is being captured, empty otherwise. The action's friendly name is
// shown (with the dot-path as a fallback) so the user can confirm which
// binding they are re-targeting without flipping the locale or guessing.
func (k *Keybindings) titleInfo() string {
	if k.capture == nil {
		return ""
	}
	name := k.capture.Description
	if name == "" {
		name = k.capture.Name
	}
	if runes := []rune(name); len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
		name = string(runes)
	}
	return fmt.Sprintf("%s · %s", name, k.com.L("keys.press_key"))
}

// setItems rebuilds the list from the current changes, keeping the selection
// stable across refreshes.
func (k *Keybindings) setItems() {
	selectedName := ""
	if item, ok := k.list.SelectedItem().(*KeybindingsItem); ok && item != nil {
		selectedName = item.info.Name
	}

	items := make([]list.FilterableItem, 0, len(k.infos))
	selected := 0
	for i, info := range k.infos {
		item := &KeybindingsItem{
			Versioned:    list.NewVersioned(),
			info:         info,
			remap:        k.changes[info.Name],
			t:            k.com.Styles,
			unboundLabel: k.com.L("keys.unbound"),
		}
		if info.Name == selectedName {
			selected = i
		}
		items = append(items, item)
	}

	k.list.SetItems(items...)
	k.list.SetSelected(selected)
	k.list.ScrollToSelected()
}

// ShortHelp implements [help.KeyMap].
func (k *Keybindings) ShortHelp() []key.Binding {
	if k.capture != nil {
		return []key.Binding{k.keyMap.Close}
	}
	return []key.Binding{
		k.keyMap.Next,
		k.keyMap.Previous,
		k.keyMap.Select,
		k.keyMap.Close,
	}
}

// FullHelp implements [help.KeyMap].
func (k *Keybindings) FullHelp() [][]key.Binding {
	var slice []key.Binding
	if k.capture == nil {
		slice = []key.Binding{
			k.keyMap.Select,
			k.keyMap.Next,
			k.keyMap.Previous,
			k.keyMap.Unbind,
			k.keyMap.Reset,
			k.keyMap.Close,
		}
	} else {
		slice = []key.Binding{k.keyMap.Close}
	}
	var rows [][]key.Binding
	for i := 0; i < len(slice); i += 4 {
		end := min(i+4, len(slice))
		rows = append(rows, slice[i:end])
	}
	return rows
}

// Filter returns the filter value for the keybinding item.
func (i *KeybindingsItem) Filter() string {
	return i.info.Name + " " + i.info.Description
}

// ID returns the unique identifier for the keybinding.
func (i *KeybindingsItem) ID() string {
	return i.info.Name
}

// SetFocused sets the focus state of the item.
func (i *KeybindingsItem) SetFocused(focused bool) {
	if i.focused == focused {
		return
	}
	i.cache = nil
	i.focused = focused
	if i.Versioned != nil {
		i.Versioned.Bump()
	}
}

// SetMatch sets the fuzzy match for the item.
func (i *KeybindingsItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(i.m, m) {
		return
	}
	i.cache = nil
	i.m = m
	if i.Versioned != nil {
		i.Versioned.Bump()
	}
}

// Finished implements list.Item.
func (i *KeybindingsItem) Finished() bool {
	return true
}

// Render returns the string representation of the item. The title is the
// friendly localized description ("Voice input") and the info column is the
// current effective binding, title-cased ("Alt+V"), with several keys joined
// by " / ". An empty remap is an explicit "unbind", which gets its own label
// rather than falling back to the default, so the two read differently on
// purpose.
func (i *KeybindingsItem) Render(width int) string {
	display := i.remap
	if display == "" {
		if i.info.Default == "" {
			display = i.unboundLabel
		} else {
			display = formatKeyCombo(i.info.Default, i.unboundLabel)
		}
	} else {
		display = formatKeyCombo(display, i.unboundLabel)
	}
	title := i.info.Description
	if title == "" {
		title = i.info.Name
	}
	// Help labels read as lowercase words ("voice input"); capitalize the
	// first rune so the row reads like a sentence fragment.
	if runes := []rune(title); len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
		title = string(runes)
	}
	st := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: i.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(st, title, display, i.focused, width, i.cache, &i.m)
}

// formatKeyCombo renders raw key strings for display: "alt+v" becomes "Alt+V",
// "shift+enter" becomes "Shift+Enter", and a comma-separated list like
// "shift+enter, ctrl+j" is joined with " / " so a binding that answers to
// several keys reads as alternatives. An empty combination falls back to the
// localized unboundLabel so the word reads in the user's language.
func formatKeyCombo(s, unboundLabel string) string {
	if s == "" {
		return unboundLabel
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		segs := strings.Split(part, "+")
		for i, seg := range segs {
			if seg == "" {
				continue
			}
			// Title-case each key name: "alt" -> "Alt", "shift" -> "Shift".
			// Non-ASCII keys ("←", "esc") pass through with only the first
			// rune upper-cased.
			runes := []rune(seg)
			runes[0] = unicode.ToUpper(runes[0])
			segs[i] = string(runes)
		}
		out = append(out, strings.Join(segs, "+"))
	}
	if len(out) == 0 {
		return unboundLabel
	}
	return strings.Join(out, " / ")
}
