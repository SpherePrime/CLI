package dialog

import (
	"github.com/SpherePrime/CLI/internal/projects"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/sahilm/fuzzy"
)

type projectItem struct {
	*list.Versioned
	project projects.Project
	styles  *styles.Styles
	match   fuzzy.Match
	focused bool
}

var _ list.FilterableItem = (*projectItem)(nil)
var _ list.Focusable = (*projectItem)(nil)
var _ list.MatchSettable = (*projectItem)(nil)

func (p *projectItem) Filter() string { return p.project.Path }
func (p *projectItem) SetFocused(focused bool) {
	if p.focused != focused {
		p.focused = focused
		p.Bump()
	}
}
func (p *projectItem) SetMatch(match fuzzy.Match) {
	p.match = match
	p.Bump()
}
func (p *projectItem) Finished() bool { return true }
func (p *projectItem) Render(width int) string {
	style := p.styles.Dialog.NormalItem
	if p.focused {
		style = p.styles.Dialog.SelectedItem
	}
	return lipgloss.NewStyle().Width(width).Render(style.Render(p.project.Path))
}
