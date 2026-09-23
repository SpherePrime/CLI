package dialog

import (
	"fmt"
	"strings"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/ui/common"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/sahilm/fuzzy"
)

// providerListItem is a row in the provider settings dialog: a title, a short
// right-aligned info column, and fuzzy match highlighting while filtering.
// The title doubles as the identifier the dialog acts on, so provider rows use
// the config key and model rows use the model ID.
type providerListItem struct {
	*list.Versioned
	t          *styles.Styles
	title      string
	info       string
	filterText string
	m          fuzzy.Match
	cache      map[int]string
	focused    bool
}

var _ ListItem = (*providerListItem)(nil)

func (p *providerListItem) ID() string     { return p.title }
func (p *providerListItem) Filter() string { return p.filterText }
func (p *providerListItem) Finished() bool { return true }

func (p *providerListItem) SetFocused(focused bool) {
	if p.focused == focused {
		return
	}
	p.cache = nil
	p.focused = focused
	p.Bump()
}

func (p *providerListItem) SetMatch(m fuzzy.Match) {
	if sameFuzzyMatch(p.m, m) {
		return
	}
	p.cache = nil
	p.m = m
	p.Bump()
}

func (p *providerListItem) Render(width int) string {
	return renderItem(
		ListItemStyles{
			ItemBlurred:     p.t.Dialog.NormalItem,
			ItemFocused:     p.t.Dialog.SelectedItem,
			InfoTextBlurred: p.t.Dialog.ListItem.InfoBlurred,
			InfoTextFocused: p.t.Dialog.ListItem.InfoFocused,
		},
		p.title, p.info, p.focused, width, p.cache, &p.m,
	)
}

// configuredProviderItems lists providers the user added to their own config,
// skipping the built-in catalog.
func configuredProviderItems(t *styles.Styles, cfg *config.Config) []list.FilterableItem {
	type entry struct {
		id   string
		info string
	}

	var entries []entry
	for id, pc := range cfg.Providers.Seq2() {
		if !pc.UserConfigured {
			continue
		}
		switch {
		case pc.Disable:
			entries = append(entries, entry{id: id, info: "disabled"})
		default:
			info := fmt.Sprintf("%d models", len(pc.Models))
			if name := strings.TrimSpace(pc.Name); name != "" && name != id {
				info = name + "  " + info
			}
			entries = append(entries, entry{id: id, info: info})
		}
	}

	items := make([]list.FilterableItem, 0, len(entries))
	for _, e := range entries {
		items = append(items, newProviderListItem(t, e.id, e.info, e.id+" "+e.info))
	}
	return items
}

// providerModelItems lists a provider's models with their context windows.
func providerModelItems(t *styles.Styles, models []catwalk.Model) []list.FilterableItem {
	items := make([]list.FilterableItem, 0, len(models))
	for _, model := range models {
		info := formatContextWindow(model.ContextWindow)
		items = append(items, newProviderListItem(t, model.ID, info, model.ID+" "+model.Name))
	}
	return items
}

func newProviderListItem(t *styles.Styles, title, info, filterText string) *providerListItem {
	return &providerListItem{
		Versioned:  list.NewVersioned(),
		t:          t,
		title:      title,
		info:       info,
		filterText: filterText,
	}
}

// formatContextWindow renders a token count for the info column.
func formatContextWindow(tokens int64) string {
	if tokens <= 0 {
		return ""
	}
	return common.FormatContextTokens(tokens)
}
