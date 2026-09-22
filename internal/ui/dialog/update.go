package dialog

import (
	"fmt"

	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/internal/ui/common"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
)

// UpdateID is the identifier for the update-available dialog.
const UpdateID = "update"

// Update asks the user whether to install a newly available Prime version.
type Update struct {
	com           *common.Common
	selectedLater bool
	current       string
	latest        string
	keyMap        struct {
		LeftRight,
		EnterSpace,
		Accept,
		Decline,
		Tab,
		Close key.Binding
	}
}

var _ Dialog = (*Update)(nil)

// NewUpdate creates a new update confirmation dialog.
func NewUpdate(com *common.Common, current, latest string) *Update {
	u := &Update{
		com:           com,
		selectedLater: true,
		current:       current,
		latest:        latest,
	}
	u.keyMap.LeftRight = key.NewBinding(
		key.WithKeys("left", "right"),
		key.WithHelp("←/→", "switch options"),
	)
	u.keyMap.EnterSpace = key.NewBinding(
		key.WithKeys("enter", " "),
		key.WithHelp("enter/space", "confirm"),
	)
	u.keyMap.Accept = key.NewBinding(
		key.WithKeys("y", "Y"),
		key.WithHelp("y/Y", "update"),
	)
	u.keyMap.Decline = key.NewBinding(
		key.WithKeys("n", "N"),
		key.WithHelp("n/N", "later"),
	)
	u.keyMap.Tab = key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "switch options"),
	)
	u.keyMap.Close = CloseKey
	return u
}

// ID implements [Dialog].
func (*Update) ID() string {
	return UpdateID
}

// HandleMsg implements [Dialog].
func (u *Update) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, u.keyMap.Close), key.Matches(msg, u.keyMap.Decline):
			return ActionClose{}
		case key.Matches(msg, u.keyMap.LeftRight, u.keyMap.Tab):
			u.selectedLater = !u.selectedLater
		case key.Matches(msg, u.keyMap.Accept):
			return ActionApplyUpdate{Latest: u.latest}
		case key.Matches(msg, u.keyMap.EnterSpace):
			if !u.selectedLater {
				return ActionApplyUpdate{Latest: u.latest}
			}
			return ActionClose{}
		}
	}
	return nil
}

// Draw implements [Dialog].
func (u *Update) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	var (
		baseStyle = u.com.Styles.Dialog.Quit.Content
		hintStyle = u.com.Styles.Dialog.Quit.Hint
	)
	question := fmt.Sprintf("Prime v%s is available (you have v%s).", u.latest, u.current)
	hint := "Update downloads in the background and installs when Prime exits."
	buttonOpts := []common.ButtonOpts{
		{Text: "Update", Selected: !u.selectedLater, Padding: 3},
		{Text: "Later", Selected: u.selectedLater, Padding: 3},
	}
	buttons := common.ButtonGroup(u.com.Styles, buttonOpts, " ")
	content := baseStyle.Render(
		lipgloss.JoinVertical(
			lipgloss.Center,
			question,
			"",
			buttons,
			"",
			hintStyle.Render(hint),
		),
	)

	frameStyle := u.com.Styles.Dialog.Quit.Frame
	maxWidth := area.Dx() - frameStyle.GetHorizontalBorderSize()
	if maxWidth < lipgloss.Width(content) {
		frameStyle = frameStyle.Padding(1, 0)
	}
	view := frameStyle.Render(content)
	DrawCenter(scr, area, view)
	return nil
}

// ShortHelp implements [help.KeyMap].
func (u *Update) ShortHelp() []key.Binding {
	return []key.Binding{
		u.keyMap.LeftRight,
		u.keyMap.EnterSpace,
	}
}

// FullHelp implements [help.KeyMap].
func (u *Update) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{u.keyMap.LeftRight, u.keyMap.EnterSpace, u.keyMap.Accept, u.keyMap.Decline},
		{u.keyMap.Tab, u.keyMap.Close},
	}
}
