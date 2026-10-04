package dialog

import (
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/sahilm/fuzzy"
)

// DefaultAgentModelID is the row identifier for the "follow the main agent"
// choice in an agent's model picker.
const DefaultAgentModelID = "__default__"

// agentDefaultItem is the "Default" row at the top of an agent's model picker.
//
// Without it, choosing a model for a worker was all-or-nothing: the only ways
// out were pinning a concrete model or reaching for a hidden ctrl+x. The
// common case - "run on whatever I am running on" - had no row of its own, so
// it was invisible while being the default.
//
// For the main agent it reads as a statement of fact rather than a choice: it
// always follows the model picker, because that is what the picker changes.
type agentDefaultItem struct {
	*list.Versioned

	t          *styles.Styles
	title      string
	info       string
	filterText string
	// isCurrent marks the row selected when the agent has no pin.
	isCurrent bool

	m       fuzzy.Match
	cache   map[int]string
	focused bool
}

var _ ListItem = (*agentDefaultItem)(nil)

// NewAgentDefaultItem creates the "Default" row.
func NewAgentDefaultItem(t *styles.Styles, title, info string, isCurrent bool) *agentDefaultItem {
	return &agentDefaultItem{
		Versioned: list.NewVersioned(),
		t:         t,
		title:     title,
		info:      info,
		isCurrent: isCurrent,
		cache:     make(map[int]string),
	}
}

func (i *agentDefaultItem) Finished() bool { return true }

func (i *agentDefaultItem) ID() string { return DefaultAgentModelID }

func (i *agentDefaultItem) Filter() string { return i.filterText }

func (i *agentDefaultItem) Render(width int) string {
	info := i.info
	if i.isCurrent {
		info = "current"
	}
	st := ListItemStyles{
		ItemBlurred:     i.t.Dialog.NormalItem,
		ItemFocused:     i.t.Dialog.SelectedItem,
		InfoTextBlurred: i.t.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: i.t.Dialog.ListItem.InfoFocused,
	}
	return renderItem(st, i.title, info, i.focused, width, i.cache, &i.m)
}

func (i *agentDefaultItem) SetFocused(focused bool) {
	if i.focused == focused {
		return
	}
	i.cache = nil
	i.focused = focused
	if i.Versioned != nil {
		i.Bump()
	}
}

func (i *agentDefaultItem) SetMatch(fm fuzzy.Match) {
	if sameFuzzyMatch(i.m, fm) {
		return
	}
	i.cache = nil
	i.m = fm
	if i.Versioned != nil {
		i.Bump()
	}
}
