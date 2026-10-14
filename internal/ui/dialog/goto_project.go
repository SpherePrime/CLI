package dialog

import (
	"strings"

	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/help"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/textinput"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
)

// GoToProjectID identifies the "go to project" dialog.
const GoToProjectID = "go_to_project"

// GoToProject asks for a folder path and switches to it. It is the command
// palette entry point for opening a project by path, without going through
// the tracked-projects list first.
type GoToProject struct {
	com   *common.Common
	help  help.Model
	input textinput.Model

	keyMap struct {
		Submit key.Binding
		Close  key.Binding
	}
}

var _ Dialog = (*GoToProject)(nil)

// NewGoToProject creates the path prompt for opening a project by path.
func NewGoToProject(com *common.Common) *GoToProject {
	g := &GoToProject{com: com}
	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	g.help = h
	g.input = textinput.New()
	g.input.SetVirtualCursor(false)
	g.input.SetStyles(com.Styles.TextInput)
	g.input.Focus()
	g.input.Placeholder = com.L("projects.path_placeholder")
	g.keyMap.Submit = key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "open"),
	)
	g.keyMap.Close = CloseKey
	return g
}

// ID implements Dialog.
func (g *GoToProject) ID() string { return GoToProjectID }

// HandleMsg implements Dialog.
func (g *GoToProject) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		switch {
		case key.Matches(msg, g.keyMap.Close):
			return ActionClose{}
		case key.Matches(msg, g.keyMap.Submit):
			path := strings.TrimSpace(g.input.Value())
			if path == "" {
				return nil
			}
			return ActionSwitchProject{Path: path}
		default:
			var cmd tea.Cmd
			g.input, cmd = g.input.Update(msg)
			return ActionCmd{cmd}
		}
	case tea.PasteMsg:
		var cmd tea.Cmd
		g.input, cmd = g.input.Update(msg)
		return ActionCmd{cmd}
	}
	return nil
}

// Draw implements Dialog.
func (g *GoToProject) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := g.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()

	g.input.SetWidth(dialogInputTextWidth(t, g.input, innerWidth))

	rc := NewRenderContext(t, width)
	rc.Title = g.com.L("projects.go_to_title")
	rc.AddPart(t.Dialog.InputPrompt.Render(g.input.View()))
	rc.Help = renderDialogHelp(t, &g.help, g, innerWidth)
	view := rc.Render()

	cur := InputCursor(t, g.input.Cursor())
	DrawCenterCursor(scr, area, view, cur)
	return cur
}

// BackspaceDeletesText implements BackspaceAware.
func (g *GoToProject) BackspaceDeletesText() bool { return BackspaceDeletesText(g.input) }

// Cursor returns the input cursor so the terminal caret follows the field.
func (g *GoToProject) Cursor() *tea.Cursor {
	return InputCursor(g.com.Styles, g.input.Cursor())
}

// ShortHelp implements help.KeyMap.
func (g *GoToProject) ShortHelp() []key.Binding {
	return []key.Binding{g.keyMap.Submit, CloseKey}
}

// FullHelp implements help.KeyMap.
func (g *GoToProject) FullHelp() [][]key.Binding {
	return [][]key.Binding{g.ShortHelp()}
}
