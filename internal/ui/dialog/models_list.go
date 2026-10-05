package dialog

import (
	"sort"
	"strings"

	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/sahilm/fuzzy"
)

// ModelsList is a list specifically for model items and groups.
type ModelsList struct {
	*list.List
	groups []ModelGroup
	query  string
	t      *styles.Styles
}

// NewModelsList creates a new list suitable for model items and groups.
func NewModelsList(sty *styles.Styles, groups ...ModelGroup) *ModelsList {
	f := &ModelsList{
		List:   list.NewList(),
		groups: groups,
		t:      sty,
	}
	f.RegisterRenderCallback(list.FocusedRenderCallback(f.List))
	return f
}

// Len returns the number of model items across all groups.
func (f *ModelsList) Len() int {
	n := 0
	for _, g := range f.groups {
		n += len(g.Items)
	}
	return n
}

// SetGroups sets the model groups and updates the list items.
func (f *ModelsList) SetGroups(groups ...ModelGroup) {
	f.groups = groups
	items := []list.Item{}
	for _, g := range f.groups {
		items = append(items, &g)
		for _, item := range g.Items {
			items = append(items, item)
		}
		// Add a space separator after each provider section
		items = append(items, list.NewSpacerItem(1))
	}
	f.SetItems(items...)
}

// SetFilter sets the filter query and updates the list items.
func (f *ModelsList) SetFilter(q string) {
	f.query = q
	f.SetItems(f.VisibleItems()...)
}

// selectableRow reports whether the cursor may rest on this row.
//
// The picker interleaves the rows it can act on - model entries and a worker's
// Default entry - with headings and spacers that only separate them. This read
// "is a *ModelItem", written when that was the only actionable row there was,
// so the Default entry added later was skipped by every arrow key: painted on
// screen, named in the help text, and unreachable without reaching for the
// mouse. Testing for the rows the picker can act on, rather than for one kind
// of them, is what lets the next kind work without anyone remembering to come
// back here.
func selectableRow(item list.Item) bool {
	_, ok := item.(ListItem)
	return ok
}

// SetSelected sets the selected item index. It overrides the base method to
// skip non-model items.
func (f *ModelsList) SetSelected(index int) {
	if index < 0 || index >= f.Len() {
		f.List.SetSelected(index)
		return
	}

	f.List.SetSelected(index)
	for {
		selectedItem := f.SelectedItem()
		if selectableRow(selectedItem) {
			return
		}
		f.List.SetSelected(index + 1)
		index++
		if index >= f.Len() {
			return
		}
	}
}

// SetSelectedItem sets the selected item in the list by item ID.
func (f *ModelsList) SetSelectedItem(itemID string) {
	if itemID == "" {
		return
	}

	// Walk the selectable model items using the same helpers that
	// keyboard navigation uses, so we stay in sync with the flat
	// list layout.
	for ok := f.SelectFirst(); ok; ok = f.SelectNext() {
		if item, is := f.SelectedItem().(ListItem); is && item.ID() == itemID {
			return
		}
	}
}

// SelectNext selects the next selectable item, skipping any non-focusable
// items like group headers and spacers.
func (f *ModelsList) SelectNext() (v bool) {
	v = f.List.SelectNext()
	for v {
		selectedItem := f.SelectedItem()
		if selectableRow(selectedItem) {
			return v
		}
		v = f.List.SelectNext()
	}
	return v
}

// SelectPrev selects the previous selectable item, skipping any non-focusable
// items like group headers and spacers.
func (f *ModelsList) SelectPrev() (v bool) {
	v = f.List.SelectPrev()
	for v {
		selectedItem := f.SelectedItem()
		if selectableRow(selectedItem) {
			return v
		}
		v = f.List.SelectPrev()
	}
	return v
}

// SelectFirst selects the first selectable item in the list.
func (f *ModelsList) SelectFirst() (v bool) {
	v = f.List.SelectFirst()
	for v {
		if selectableRow(f.SelectedItem()) {
			return v
		}
		v = f.List.SelectNext()
	}
	return v
}

// SelectLast selects the last selectable item in the list.
func (f *ModelsList) SelectLast() (v bool) {
	v = f.List.SelectLast()
	for v {
		if selectableRow(f.SelectedItem()) {
			return v
		}
		v = f.List.SelectPrev()
	}
	return v
}

// IsSelectedFirst checks if the selected item is the first model item.
func (f *ModelsList) IsSelectedFirst() bool {
	originalIndex := f.Selected()
	f.SelectFirst()
	isFirst := f.Selected() == originalIndex
	f.List.SetSelected(originalIndex)
	return isFirst
}

// IsSelectedLast checks if the selected item is the last model item.
func (f *ModelsList) IsSelectedLast() bool {
	originalIndex := f.Selected()
	f.SelectLast()
	isLast := f.Selected() == originalIndex
	f.List.SetSelected(originalIndex)
	return isLast
}

// VisibleItems returns the visible items after filtering.
func (f *ModelsList) VisibleItems() []list.Item {
	query := strings.ToLower(strings.ReplaceAll(f.query, " ", ""))

	if query == "" {
		// No filter, return all items with group headers
		items := []list.Item{}
		for _, g := range f.groups {
			items = append(items, &g)
			for _, item := range g.Items {
				item.SetMatch(fuzzy.Match{})
				items = append(items, item)
			}
			// Add a space separator after each provider section
			items = append(items, list.NewSpacerItem(1))
		}
		return items
	}

	items := []list.Item{}
	visitedGroups := map[int]bool{}

	// Reconstruct groups with matched items.
	//
	// Matching runs one group at a time against that group's own rows. It
	// used to run against every row in the list while labelling them all
	// with the current group's title, then discard the rows that belonged
	// somewhere else - and it reached for the row's concrete type to do it.
	// The picker holds more than one kind of row (the worker's Default entry
	// among them), so typing a character into the filter panicked on the
	// first assertion and took the whole program down with it.
	for gi, g := range f.groups {
		addedCount := 0
		name := strings.ToLower(g.Title) + " "

		names := make([]string, len(g.Items))
		for i, item := range g.Items {
			names[i] = name + item.Filter()
		}

		matches := fuzzy.Find(query, names)

		// Sort by original index to preserve order within the group
		sort.SliceStable(matches, func(i, j int) bool {
			return matches[i].Index < matches[j].Index
		})

		for _, match := range matches {
			item := g.Items[match.Index]
			idxs := []int{}
			for _, idx := range match.MatchedIndexes {
				// Adjusts removing provider name highlights
				if idx < len(name) {
					continue
				}
				idxs = append(idxs, idx-len(name))
			}

			match.MatchedIndexes = idxs
			if !visitedGroups[gi] {
				// Add section header
				items = append(items, &g)
				visitedGroups[gi] = true
			}
			// Add the matched item
			item.SetMatch(match)
			items = append(items, item)
			addedCount++
		}
		if addedCount > 0 {
			// Add a space separator after each provider section
			items = append(items, list.NewSpacerItem(1))
		}
	}

	return items
}

// Render renders the filterable list.
func (f *ModelsList) Render() string {
	return f.List.Render()
}

type modelGroups []ModelGroup

func (m modelGroups) Len() int {
	n := 0
	for _, g := range m {
		n += len(g.Items)
	}
	return n
}

func (m modelGroups) String(i int) string {
	count := 0
	for _, g := range m {
		if i < count+len(g.Items) {
			return g.Items[i-count].Filter()
		}
		count += len(g.Items)
	}
	return ""
}
