package i18n

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// localizedKeyMatches the keys handed to the translation helpers: dotted
// lowercase identifiers such as "cmd.agent_models".
var localizedKeyMatches = []*regexp.Regexp{
	regexp.MustCompile(`\.L\("([a-z0-9_]+(?:\.[a-z0-9_]+)+)"\)`),
	regexp.MustCompile(`\.LSprintf\("([a-z0-9_]+(?:\.[a-z0-9_]+)+)"`),
	regexp.MustCompile(`\.T\("([a-z0-9_]+(?:\.[a-z0-9_]+)+)"\)`),
}

// TestUsedKeysExistInCatalog walks the source tree and fails on any lookup key
// the English catalog does not define, which is what makes a dialog render the
// raw key instead of a label.
func TestUsedKeysExistInCatalog(t *testing.T) {
	root := moduleRoot(t)
	catalog := Catalogs[En].Strings

	missing := map[string][]string{}
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skippedDirs[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		for _, re := range localizedKeyMatches {
			for _, match := range re.FindAllStringSubmatch(string(data), -1) {
				key := match[1]
				if _, ok := catalog[key]; !ok {
					missing[key] = append(missing[key], rel)
				}
			}
		}
		return nil
	})
	require.NoError(t, err)

	keys := make([]string, 0, len(missing))
	for key := range missing {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var report []string
	for _, key := range keys {
		report = append(report, key+" used in "+strings.Join(missing[key], ", "))
	}
	require.Empty(t, report, "translation keys missing from the English catalog:\n%s", strings.Join(report, "\n"))
}

// skippedDirs are trees that hold vendored code or generated fixtures rather
// than Prime's own user-facing strings.
var skippedDirs = map[string]bool{
	"vendordeps":   true,
	"node_modules": true,
	".git":         true,
	".prime":       true,
	"testdata":     true,
	"dist":         true,
	"build":        true,
}

func moduleRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		require.NotEqual(t, parent, dir, "no go.mod found above the test working directory")
		dir = parent
	}
}
