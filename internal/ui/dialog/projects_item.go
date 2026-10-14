package dialog

import (
	"path/filepath"
	"strings"

	"github.com/SpherePrime/CLI/internal/projects"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/dwertyfa288/x/ansi"
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

var (
	_ list.FilterableItem = (*projectItem)(nil)
	_ list.Focusable      = (*projectItem)(nil)
	_ list.MatchSettable  = (*projectItem)(nil)
)

// name returns the folder name of the tracked project, the label the dialog
// shows as the item title. The full path stays available as the info column.
func (p *projectItem) name() string {
	base := filepath.Base(p.project.Path)
	if base == "" || base == "." || base == string(filepath.Separator) {
		return p.project.Path
	}
	return base
}

func (p *projectItem) Filter() string { return p.project.Path }
func (p *projectItem) SetFocused(focused bool) {
	if p.focused != focused {
		p.focused = focused
		p.Bump()
	}
}

func (p *projectItem) SetMatch(match fuzzy.Match) {
	if sameFuzzyMatch(p.match, match) {
		return
	}
	p.match = match
	p.Bump()
}
func (p *projectItem) Finished() bool { return true }

func (p *projectItem) Render(width int) string {
	styles := ListItemStyles{
		ItemBlurred:     p.styles.Dialog.NormalItem,
		ItemFocused:     p.styles.Dialog.SelectedItem,
		InfoTextBlurred: p.styles.Dialog.ListItem.InfoBlurred,
		InfoTextFocused: p.styles.Dialog.ListItem.InfoFocused,
	}
	return renderProjectItem(styles, p.name(), p.project.Path, p.focused, width, &p.match)
}

// renderProjectItem renders a "FolderName   /path/to/folder" row. It follows
// renderItem but marks the fuzzy match on the path: the filter string is the
// path, while the rendered line puts the folder name and a gap in front of
// it, so every match index shifts by the length of that prefix.
func renderProjectItem(t ListItemStyles, title, info string, focused bool, width int, m *fuzzy.Match) string {
	style := t.ItemBlurred
	if focused {
		style = t.ItemFocused
	}

	// Build content to the width left after the item style's own padding,
	// so the final rendered line is exactly `width` wide. Otherwise the
	// padding pushes the line past the list width and it wraps.
	lineWidth := max(0, width-style.GetHorizontalFrameSize())

	var infoText string
	var infoWidth int
	if len(info) > 0 {
		// Cap the info column so a long path can truncate instead of
		// overflowing the row or squeezing the folder name to nothing;
		// the name keeps at least half the width.
		if maxInfo := lineWidth / 2; lipgloss.Width(info)+2 > maxInfo {
			info = ansi.Truncate(info, max(0, maxInfo-2), "…")
		}
		infoText = " " + info + " "
		if focused {
			infoText = t.InfoTextFocused.Render(infoText)
		} else {
			infoText = t.InfoTextBlurred.Render(infoText)
		}
		infoWidth = lipgloss.Width(infoText)
	}

	title = ansi.Truncate(title, max(0, lineWidth-infoWidth), "…")
	titleWidth := lipgloss.Width(title)
	gap := strings.Repeat(" ", max(0, lineWidth-titleWidth-infoWidth))
	content := title
	if m != nil && len(m.MatchedIndexes) > 0 {
		content = underlineProjectMatch(title, gap, info, m)
	}

	return style.Render(content + gap + infoText)
}

// underlineProjectMatch re-renders the "name + gap + path" line with the
// fuzzy match indexes underlined. Match indexes point into the filter
// string, which is the path; the rendered line prepends the folder name and
// the gap, so an index i lands at len(title)+len(gap)+i. Indexes that fell
// off the line after truncation are dropped.
func underlineProjectMatch(title, gap, info string, m *fuzzy.Match) string {
	if len(info) == 0 {
		// Nothing rendered to the right of the name, and the match
		// indexes point into the path, so there is nothing to mark.
		return title
	}
	line := title + gap + info
	offset := len(title) + len(gap)
	indexes := make([]int, 0, len(m.MatchedIndexes))
	for _, i := range m.MatchedIndexes {
		if pos := offset + i; pos < len(line) {
			indexes = append(indexes, pos)
		}
	}
	if len(indexes) == 0 {
		return line
	}

	var lastPos int
	parts := make([]string, 0)
	for _, rng := range matchedRanges(indexes) {
		start, stop := bytePosToVisibleCharPos(line, rng)
		if start > lastPos {
			parts = append(parts, ansi.Cut(line, lastPos, start))
		}
		// NOTE: We're using [ansi.Style] here instead of [lipgloss.Style]
		// because we can control the underline start and stop more
		// precisely via [ansi.AttrUnderline] and [ansi.AttrNoUnderline]
		// which only affect the underline attribute without interfering
		// with other style attributes.
		parts = append(
			parts,
			ansi.NewStyle().Underline(true).String(),
			ansi.Cut(line, start, stop+1),
			ansi.NewStyle().Underline(false).String(),
		)
		lastPos = stop + 1
	}
	if lastPos < ansi.StringWidth(line) {
		parts = append(parts, ansi.Cut(line, lastPos, ansi.StringWidth(line)))
	}
	return strings.Join(parts, "")
}
