package fsext

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestListDirectory(t *testing.T) {
	tmp := t.TempDir()

	testFiles := map[string]string{
		"regular.txt":     "content",
		".hidden":         "hidden content",
		".gitignore":      ".*\n*.log\n",
		"subdir/file.go":  "package main",
		"subdir/.another": "more hidden",
		"build.log":       "build output",
	}

	for name, content := range testFiles {
		fp := filepath.Join(tmp, name)
		dir := filepath.Dir(fp)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		require.NoError(t, os.WriteFile(fp, []byte(content), 0o644))
	}

	t.Run("no limit", func(t *testing.T) {
		files, truncated, err := ListDirectory(tmp, nil, -1, -1)
		require.NoError(t, err)
		require.False(t, truncated)
		// The .gitignore has ".*" pattern which ignores hidden files anywhere
		// (like real git does), so subdir/.another is ignored.
		require.Len(t, files, 3)
		require.ElementsMatch(t, []string{
			"regular.txt",
			"subdir",
			"subdir/file.go",
		}, relPaths(t, files, tmp))
	})
	t.Run("limit", func(t *testing.T) {
		files, truncated, err := ListDirectory(tmp, nil, -1, 2)
		require.NoError(t, err)
		require.True(t, truncated)
		require.Len(t, files, 2)
	})
	t.Run("depth 1 fast path", func(t *testing.T) {
		files, truncated, err := ListDirectory(tmp, nil, 1, -1)
		require.NoError(t, err)
		require.False(t, truncated)
		require.ElementsMatch(t, []string{
			"regular.txt",
			"subdir",
		}, relPaths(t, files, tmp))
		require.True(t, strings.HasSuffix(files[0], string(filepath.Separator)))
	})
	t.Run("depth 1 limit and truncation", func(t *testing.T) {
		files, truncated, err := ListDirectory(tmp, nil, 1, 1)
		require.NoError(t, err)
		require.True(t, truncated)
		require.Len(t, files, 1)
		require.True(t, strings.HasSuffix(files[0], string(filepath.Separator)))
	})
	t.Run("depth 1 missing path", func(t *testing.T) {
		_, _, err := ListDirectory(filepath.Join(tmp, "missing"), nil, 1, -1)
		require.Error(t, err)
	})
	t.Run("depth 1 regular file", func(t *testing.T) {
		files, truncated, err := ListDirectory(filepath.Join(tmp, "regular.txt"), nil, 1, -1)
		require.NoError(t, err)
		require.False(t, truncated)
		require.Empty(t, files)
	})
}

func relPaths(tb testing.TB, in []string, base string) []string {
	tb.Helper()
	out := make([]string, 0, len(in))
	for _, p := range in {
		rel, err := filepath.Rel(base, p)
		require.NoError(tb, err)
		out = append(out, filepath.ToSlash(rel))
	}
	return out
}
