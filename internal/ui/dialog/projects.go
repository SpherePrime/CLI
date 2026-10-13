package dialog

import (
	"strings"

	"github.com/SpherePrime/CLI/internal/projects"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/util"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/help"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/key"
	"github.com/SpherePrime/CLI/vendordeps/bubbles/v2/textinput"
	tea "github.com/SpherePrime/CLI/vendordeps/bubbletea/v2"
	uv "github.com/SpherePrime/CLI/vendordeps/dwertyfa288/ultraviolet"
)

// ProjectsID identifies the project selector dialog.
const ProjectsID = "projects"

type projectsMode uint8

const (
	projectsModeNormal projectsMode = iota
	projectsModeDeleting
	projectsModePath
)

// Projects is a selector for recently used project directories.
type Projects struct {
	com   *common.Common
	help  help.Model
	list  *list.FilterableList
	input textinput.Model
	items []*projectItem
	mode  projectsMode
}

var _ Dialog = (*Projects)(nil)

// NewProjects creates a project selector dialog.
func NewProjects(com *common.Common) (*Projects, error) {
	if err := projects.Prune(); err != nil {
		return nil, err
	}
	entries, err := projects.List()
	if err != nil {
		return nil, err
	}
	p := &Projects{com: com}
	h := help.New()
	h.Styles = com.Styles.DialogHelpStyles()
	p.help = h
	p.items = make([]*projectItem, 0, len(entries))
	filterable := make([]list.FilterableItem, 0, len(entries))
	for _, entry := range entries {
		item := &projectItem{Versioned: list.NewVersioned(), project: entry, styles: com.Styles}
		p.items = append(p.items, item)
		filterable = append(filterable, item)
	}
	p.list = list.NewFilterableList(filterable...)
	p.list.Focus()
	p.input = textinput.New()
	p.input.SetVirtualCursor(false)
	p.input.SetStyles(com.Styles.TextInput)
	p.input.Focus()
	return p, nil
}

// ID implements Dialog.
func (p *Projects) ID() string { return ProjectsID }

// HandleMsg implements Dialog.
func (p *Projects) HandleMsg(msg tea.Msg) Action {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if p.mode == projectsModeDeleting {
			switch msg.String() {
			case "y", "enter":
				item := p.selected()
				p.mode = projectsModeNormal
				if item != nil {
					if err := removeProject(item.project.Path); err != nil {
						return ActionCmd{util.ReportError(err)}
					}
					p.remove(item)
				}
			case "n", "esc":
				p.mode = projectsModeNormal
			}
			return nil
		}
		if p.mode == projectsModePath {
			switch msg.String() {
			case "esc":
				p.mode = projectsModeNormal
				return nil
			case "enter":
				path := strings.TrimSpace(p.input.Value())
				if path == "" {
					return nil
				}
				return ActionSwitchProject{Path: path}
			default:
				var cmd tea.Cmd
				p.input, cmd = p.input.Update(msg)
				return ActionCmd{cmd}
			}
		}
		switch msg.String() {
		case "esc":
			return ActionClose{}
		case "ctrl+x":
			if p.selected() != nil {
				p.mode = projectsModeDeleting
			}
			return nil
		case "ctrl+n", "ctrl+o":
			p.mode = projectsModePath
			p.input.SetValue("")
			return nil
		case "enter":
			if item := p.selected(); item != nil {
				return ActionSwitchProject{Path: item.project.Path}
			}
			return nil
		case "down", "ctrl+j":
			p.list.SelectNext()
		case "up", "ctrl+k":
			p.list.SelectPrev()
		default:
			old := p.input.Value()
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			if p.input.Value() != old {
				p.list.SetFilter(p.input.Value())
				p.list.SetSelected(0)
			}
			return ActionCmd{cmd}
		}
	case tea.PasteMsg:
		if p.mode == projectsModePath {
			var cmd tea.Cmd
			p.input, cmd = p.input.Update(msg)
			return ActionCmd{cmd}
		}
		return filterPaste(&p.input, p.list, msg)
	}
	return nil
}

// Draw implements Dialog.
func (p *Projects) Draw(scr uv.Screen, area uv.Rectangle) *tea.Cursor {
	t := p.com.Styles
	width := max(0, min(defaultDialogMaxWidth, area.Dx()-t.Dialog.View.GetHorizontalBorderSize()))
	height := max(0, min(defaultDialogHeight, area.Dy()-t.Dialog.View.GetVerticalBorderSize()))
	innerWidth := width - t.Dialog.View.GetHorizontalFrameSize()
	listHeight, listTotalHeight, _ := sizeDialogList(t, p.list, innerWidth, height)
	var rc = NewRenderContext(t, width)
	rc.Title = "Projects"
	if p.mode == projectsModeDeleting {
		rc.AddPart("Delete this project from the list? (y/n)")
	} else if p.mode == projectsModePath {
		p.input.SetWidth(dialogInputTextWidth(t, p.input, innerWidth))
		rc.AddPart(t.Dialog.InputPrompt.Render(p.input.View()))
	} else {
		p.input.SetWidth(dialogInputTextWidth(t, p.input, innerWidth))
		rc.AddPart(t.Dialog.InputPrompt.Render(p.input.View()))
	}
	bodyView := t.Dialog.List.Height(p.list.Height()).Render(p.list.Render())
	rc.AddPart(joinScrollbar(t, bodyView, listHeight, listTotalHeight, listHeight, p.list.Offset()))
	rc.Help = renderDialogHelp(t, &p.help, p, innerWidth)
	view := rc.Render()
	DrawCenterCursor(scr, area, view, p.Cursor())
	return p.Cursor()
}

// BackspaceDeletesText implements BackspaceAware.
func (p *Projects) BackspaceDeletesText() bool { return BackspaceDeletesText(p.input) }

func (p *Projects) Cursor() *tea.Cursor {
	if p.mode == projectsModePath {
		return InputCursor(p.com.Styles, p.input.Cursor())
	}
	return nil
}

func (p *Projects) selected() *projectItem {
	item, _ := p.list.SelectedItem().(*projectItem)
	return item
}

func (p *Projects) remove(target *projectItem) {
	for i, item := range p.items {
		if item == target {
			p.items = append(p.items[:i], p.items[i+1:]...)
			break
		}
	}
	items := make([]list.FilterableItem, 0, len(p.items))
	for _, item := range p.items {
		items = append(items, item)
	}
	p.list.SetItems(items...)
	p.list.SetFilter(p.input.Value())
	p.list.SetSelected(0)
}

func removeProject(path string) error {
	entries, err := projects.List()
	if err != nil {
		return err
	}
	kept := make([]projects.Project, 0, len(entries))
	for _, entry := range entries {
		if entry.Path != path {
			kept = append(kept, entry)
		}
	}
	return projects.Save(&projects.ProjectList{Projects: kept})
}

// ShortHelp implements help.KeyMap.
func (p *Projects) ShortHelp() []key.Binding {
	if p.mode == projectsModeDeleting {
		return []key.Binding{key.NewBinding(key.WithKeys("y", "enter"), key.WithHelp("y", "delete")), key.NewBinding(key.WithKeys("n", "esc"), key.WithHelp("n", "cancel"))}
	}
	if p.mode == projectsModePath {
		return []key.Binding{key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")), CloseKey}
	}
	return []key.Binding{
		key.NewBinding(key.WithKeys("up", "down"), key.WithHelp("↑↓", "choose")),
		key.NewBinding(key.WithKeys("ctrl+o"), key.WithHelp("ctrl+o", "go to path")),
		key.NewBinding(key.WithKeys("ctrl+x"), key.WithHelp("ctrl+x", "remove")),
		key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open")),
		CloseKey,
	}
}

// FullHelp implements help.KeyMap.
func (p *Projects) FullHelp() [][]key.Binding {
	short := p.ShortHelp()
	var rows [][]key.Binding
	for i := 0; i < len(short); i += 4 {
		rows = append(rows, short[i:min(i+4, len(short))])
	}
	return rows
}
