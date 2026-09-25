package model

import (
	"github.com/SpherePrime/CLI/internal/i18n"
	"github.com/SpherePrime/CLI/internal/voice"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
)

// SetVoiceHotkey rebinds dictation to the keys from options.voice.hotkey,
// keeping the translated help label.
func (km *KeyMap) SetVoiceHotkey(keys []string) {
	if len(keys) == 0 {
		return
	}
	km.Voice = key.NewBinding(
		key.WithKeys(keys...),
		key.WithHelp(keys[0], km.Voice.Help().Desc),
	)
}

type KeyMap struct {
	Editor struct {
		SendMessage key.Binding
		OpenEditor  key.Binding
		Newline     key.Binding
		AddImage    key.Binding
		PasteImage  key.Binding
		MentionFile key.Binding
		Commands    key.Binding

		// Attachments key maps
		AttachmentDeleteMode key.Binding
		Escape               key.Binding
		DeleteAllAttachments key.Binding

		// History navigation
		HistoryPrev key.Binding
		HistoryNext key.Binding

		// CopySelection copies the current textarea selection to the
		// clipboard.
		CopySelection key.Binding

		// CutSelection copies the current textarea selection to the
		// clipboard and deletes it from the textarea.
		CutSelection key.Binding

		// SelectAll selects all text in the textarea.
		SelectAll key.Binding

		// PasteText pastes clipboard text into the textarea, as an
		// alternative to bracketed paste.
		PasteText key.Binding
	}

	Chat struct {
		NewSession     key.Binding
		AddAttachment  key.Binding
		Cancel         key.Binding
		Tab            key.Binding
		Details        key.Binding
		TogglePills    key.Binding
		PillLeft       key.Binding
		PillRight      key.Binding
		Down           key.Binding
		Up             key.Binding
		UpDown         key.Binding
		DownOneItem    key.Binding
		UpOneItem      key.Binding
		UpDownOneItem  key.Binding
		PageDown       key.Binding
		PageUp         key.Binding
		HalfPageDown   key.Binding
		HalfPageUp     key.Binding
		Home           key.Binding
		End            key.Binding
		EndFollow      key.Binding
		Copy           key.Binding
		ClearHighlight key.Binding
		Expand         key.Binding
		ScrollLeft     key.Binding
		ScrollRight    key.Binding
		FocusSidebar   key.Binding
		FocusChat      key.Binding
	}

	Initialize struct {
		Yes,
		No,
		Enter,
		Switch key.Binding
	}

	// Global key maps
	Quit       key.Binding
	Help       key.Binding
	Commands   key.Binding
	Models     key.Binding
	Suspend    key.Binding
	Sessions   key.Binding
	Tab        key.Binding
	ToggleYolo key.Binding
	ShiftTab   key.Binding
	// Voice starts and stops microphone dictation.
	Voice key.Binding
}

// DefaultKeyMap builds a keymap with the default locale. The help labels
// are translated at build time from the English catalog.
func DefaultKeyMap() KeyMap {
	return BuildKeyMap(i18n.New(i18n.En))
}

// BuildKeyMap builds a keymap with translated help labels.
func BuildKeyMap(tr i18n.Translator) KeyMap {
	km := KeyMap{}

	km.Quit = key.NewBinding(
		key.WithKeys("ctrl+c"),
		key.WithHelp("ctrl+c", tr.Label("key.quit")),
	)
	km.Help = key.NewBinding(
		key.WithKeys("ctrl+g"),
		key.WithHelp("ctrl+g", tr.Label("key.more")),
	)
	km.Commands = key.NewBinding(
		key.WithKeys("ctrl+p"),
		key.WithHelp("ctrl+p", tr.Label("key.commands")),
	)
	km.Models = key.NewBinding(
		key.WithKeys("ctrl+m", "ctrl+l"),
		key.WithHelp("ctrl+l", tr.Label("key.models")),
	)
	km.Suspend = key.NewBinding(
		key.WithKeys("ctrl+z"),
		key.WithHelp("ctrl+z", tr.Label("key.suspend")),
	)
	km.Sessions = key.NewBinding(
		key.WithKeys("ctrl+s"),
		key.WithHelp("ctrl+s", tr.Label("key.sessions")),
	)
	km.Tab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", tr.Label("key.change_focus")),
	)
	km.ToggleYolo = key.NewBinding(
		key.WithKeys("ctrl+y"),
		key.WithHelp("ctrl+y", tr.Label("key.toggle_yolo")),
	)
	km.ShiftTab = key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", tr.Label("key.mode")),
	)
	km.Voice = key.NewBinding(
		key.WithKeys(voice.DefaultHotkeys...),
		key.WithHelp(voice.DefaultHotkeys[0], tr.Label("key.voice")),
	)

	km.Editor.SendMessage = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", tr.Label("key.send")),
	)
	km.Editor.OpenEditor = key.NewBinding(
		key.WithKeys("ctrl+o"),
		key.WithHelp("ctrl+o", tr.Label("key.open_editor")),
	)
	// "ctrl+j" is a common keybinding for newline in many editors. If
	// the terminal supports "shift+enter", we substitute the help text
	// to reflect that.
	km.Editor.Newline = key.NewBinding(
		key.WithKeys("shift+enter", "ctrl+j"),
		key.WithHelp("ctrl+j", tr.Label("key.newline")),
	)
	km.Editor.AddImage = key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", tr.Label("key.add_image")),
	)
	km.Editor.PasteImage = key.NewBinding(
		key.WithKeys("ctrl+v"),
		key.WithHelp("ctrl+v", tr.Label("key.paste_image")),
	)
	km.Editor.PasteText = key.NewBinding(
		key.WithKeys("ctrl+shift+v"),
		key.WithHelp("ctrl+shift+v", tr.Label("key.paste_text")),
	)
	km.Editor.MentionFile = key.NewBinding(
		key.WithKeys("@"),
		key.WithHelp("@", tr.Label("key.mention_file")),
	)
	km.Editor.Commands = key.NewBinding(
		key.WithKeys("/"),
		key.WithHelp("/", tr.Label("key.commands")),
	)
	km.Editor.AttachmentDeleteMode = key.NewBinding(
		key.WithKeys("ctrl+r"),
		key.WithHelp("ctrl+r+{i}", tr.Label("key.delete_attachment")),
	)
	km.Editor.Escape = key.NewBinding(
		key.WithKeys("esc", "alt+esc"),
		key.WithHelp("esc", tr.Label("key.cancel_delete_mode")),
	)
	km.Editor.DeleteAllAttachments = key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("ctrl+r+r", tr.Label("key.delete_all_attachments")),
	)
	km.Editor.HistoryPrev = key.NewBinding(
		key.WithKeys("up"),
	)
	km.Editor.HistoryNext = key.NewBinding(
		key.WithKeys("down"),
	)
	km.Editor.CopySelection = key.NewBinding(
		key.WithKeys("ctrl+shift+c"),
		key.WithHelp("ctrl+shift+c", tr.Label("key.copy_selection")),
	)
	km.Editor.CutSelection = key.NewBinding(
		key.WithKeys("ctrl+shift+x"),
		key.WithHelp("ctrl+shift+x", tr.Label("key.cut_selection")),
	)
	km.Editor.SelectAll = key.NewBinding(
		key.WithKeys("ctrl+shift+a"),
		key.WithHelp("ctrl+shift+a", tr.Label("key.select_all")),
	)

	km.Chat.NewSession = key.NewBinding(
		key.WithKeys("ctrl+n"),
		key.WithHelp("ctrl+n", tr.Label("key.new_session")),
	)
	km.Chat.AddAttachment = key.NewBinding(
		key.WithKeys("ctrl+f"),
		key.WithHelp("ctrl+f", tr.Label("key.add_attachment")),
	)
	km.Chat.Cancel = key.NewBinding(
		key.WithKeys("esc", "alt+esc"),
		key.WithHelp("esc", tr.Label("key.cancel")),
	)
	km.Chat.Tab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", tr.Label("key.change_focus")),
	)
	km.Chat.Details = key.NewBinding(
		key.WithKeys("ctrl+d"),
		key.WithHelp("ctrl+d", tr.Label("key.toggle_details")),
	)
	km.Chat.TogglePills = key.NewBinding(
		key.WithKeys("ctrl+t", "ctrl+space"),
		key.WithHelp("ctrl+t", tr.Label("key.toggle_tasks")),
	)
	km.Chat.PillLeft = key.NewBinding(
		key.WithKeys("left"),
		key.WithHelp("←/→", tr.Label("key.switch_section")),
	)
	km.Chat.PillRight = key.NewBinding(
		key.WithKeys("right"),
		key.WithHelp("←/→", tr.Label("key.switch_section")),
	)

	km.Chat.Down = key.NewBinding(
		key.WithKeys("down", "ctrl+j", "j"),
		key.WithHelp("↓", tr.Label("key.down")),
	)
	km.Chat.Up = key.NewBinding(
		key.WithKeys("up", "ctrl+k", "k"),
		key.WithHelp("↑", tr.Label("key.up")),
	)
	km.Chat.UpDown = key.NewBinding(
		key.WithKeys("up", "down"),
		key.WithHelp("↑↓", tr.Label("key.scroll")),
	)
	km.Chat.UpOneItem = key.NewBinding(
		key.WithKeys("shift+up", "K"),
		key.WithHelp("shift+↑", tr.Label("key.up_one_item")),
	)
	km.Chat.DownOneItem = key.NewBinding(
		key.WithKeys("shift+down", "J"),
		key.WithHelp("shift+↓", tr.Label("key.down_one_item")),
	)
	km.Chat.UpDownOneItem = key.NewBinding(
		key.WithKeys("shift+up", "shift+down"),
		key.WithHelp("shift+↑↓", tr.Label("key.scroll_one_item")),
	)
	km.Chat.HalfPageDown = key.NewBinding(
		key.WithKeys("d"),
		key.WithHelp("d", tr.Label("key.half_page_down")),
	)
	km.Chat.PageDown = key.NewBinding(
		key.WithKeys("pgdown", " ", "f"),
		key.WithHelp("f/pgdn", tr.Label("key.page_down")),
	)
	km.Chat.PageUp = key.NewBinding(
		key.WithKeys("pgup", "b"),
		key.WithHelp("b/pgup", tr.Label("key.page_up")),
	)
	km.Chat.HalfPageUp = key.NewBinding(
		key.WithKeys("u"),
		key.WithHelp("u", tr.Label("key.half_page_up")),
	)
	km.Chat.Home = key.NewBinding(
		key.WithKeys("g", "home"),
		key.WithHelp("g", tr.Label("key.home")),
	)
	km.Chat.End = key.NewBinding(
		key.WithKeys("G", "end"),
		key.WithHelp("G", tr.Label("key.end")),
	)
	km.Chat.EndFollow = key.NewBinding(
		key.WithKeys("ctrl+end"),
	)
	km.Chat.Copy = key.NewBinding(
		key.WithKeys("c", "y", "C", "Y"),
		key.WithHelp("c/y", tr.Label("key.copy")),
	)
	km.Chat.ClearHighlight = key.NewBinding(
		key.WithKeys("esc", "alt+esc"),
		key.WithHelp("esc", tr.Label("key.clear_selection")),
	)
	km.Chat.Expand = key.NewBinding(
		key.WithKeys("space"),
		key.WithHelp("space", tr.Label("key.expand_collapse")),
	)
	km.Chat.ScrollLeft = key.NewBinding(
		key.WithKeys("shift+left", "H"),
		key.WithHelp("shift+←/H", tr.Label("key.scroll_left")),
	)
	km.Chat.ScrollRight = key.NewBinding(
		key.WithKeys("shift+right", "L"),
		key.WithHelp("shift+→/L", tr.Label("key.scroll_right")),
	)
	km.Chat.FocusSidebar = key.NewBinding(
		key.WithKeys("l", "right"),
		key.WithHelp("l/→", tr.Label("key.focus_sidebar")),
	)
	km.Chat.FocusChat = key.NewBinding(
		key.WithKeys("h", "left"),
		key.WithHelp("h/←", tr.Label("key.focus_chat")),
	)
	km.Initialize.Yes = key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y", tr.Label("key.yes")),
	)
	km.Initialize.No = key.NewBinding(
		key.WithKeys("n", "N", "esc", "alt+esc"),
		key.WithHelp("n", tr.Label("key.no")),
	)
	km.Initialize.Switch = key.NewBinding(
		key.WithKeys("left", "right", "tab"),
		key.WithHelp("tab", tr.Label("key.switch")),
	)
	km.Initialize.Enter = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", tr.Label("key.select")),
	)

	return km
}
