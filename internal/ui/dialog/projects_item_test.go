package dialog

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/projects"
	"github.com/SpherePrime/CLI/internal/ui/list"
	"github.com/SpherePrime/CLI/internal/ui/styles"
	"github.com/SpherePrime/CLI/vendordeps/lipgloss/v2"
	"github.com/SpherePrime/CLI/vendordeps/sahilm/fuzzy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

var ansiSequencePattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

func newProjectTestItem(path string) *projectItem {
	return &projectItem{
		Versioned: list.NewVersioned(),
		project:   projects.Project{Path: path},
		styles:    testDialogStyles(),
	}
}

// testDialogStyles returns the shared theme used by dialog tests.
func testDialogStyles() *styles.Styles {
	s := styles.ColorTonePantera()
	return &s
}

func TestProjectItemShowsFolderNameAndPath(t *testing.T) {
	t.Parallel()

	item := newProjectTestItem(filepath.Join("Users", "dev", "my-app"))
	plain := ansiSequencePattern.ReplaceAllString(item.Render(80), "")

	require.Contains(t, plain, "my-app", "folder name must be shown")
	require.Contains(t, plain, filepath.Join("Users", "dev", "my-app"),
		"the full path must be shown next to the folder name")
}

func TestProjectItemNameFallsBackToPath(t *testing.T) {
	t.Parallel()

	// A root path such as C:\ has an empty base name; the item must fall
	// back to the full path instead of rendering an empty title.
	item := newProjectTestItem("C:" + string(filepath.Separator))
	plain := ansiSequencePattern.ReplaceAllString(item.Render(80), "")
	require.NotEmpty(t, strings.TrimSpace(plain))
}

func TestProjectItemUnderlinesPathMatches(t *testing.T) {
	t.Parallel()

	path := filepath.Join("Users", "dev", "projects", "beta")
	item := newProjectTestItem(path)
	item.SetMatch(fuzzy.Match{
		Str:            path,
		MatchedIndexes: []int{0, 1},
	})
	rendered := item.Render(200)

	// The underline must land on the path, not on the folder name that
	// precedes it. The first two path characters are "Us", so the match
	// highlights the path's own start; the folder name must stay plain.
	nameStart := strings.Index(rendered, "beta")
	require.GreaterOrEqual(t, nameStart, 0, "folder name must be rendered")
	require.Greater(t, strings.Index(rendered, "\x1b[4m"), nameStart,
		"underline must start after the folder name, inside the path")
}

func TestProjectItemTruncatesLongPath(t *testing.T) {
	t.Parallel()

	item := newProjectTestItem(strings.Repeat("very-long-segment/", 20))
	rendered := item.Render(60)
	require.LessOrEqual(t, lipgloss.Width(rendered), 60, "row must never overflow its width")
}
