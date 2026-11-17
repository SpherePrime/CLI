package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/fsext"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestGrepNativeContextCancel(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.go"), []byte("package main\nfunc needle() {}\n"), 0o644))

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	matches, err := searchFilesWithRegexCtx(ctx, "needle", dir, "", 100)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, matches)

	matches, truncated, err := searchFiles(ctx, "needle", dir, "", 100)
	require.ErrorIs(t, err, context.Canceled)
	require.False(t, truncated)
	require.Nil(t, matches)
}

func TestGrepNativeSkipsBinaryFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "text.txt"), []byte("findme here\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "image.png"), []byte("findme inside png"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "prog.exe"), []byte("findme inside exe"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "archive.zip"), []byte("findme inside zip"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nulbytes.dat"), []byte("findme\x00binary"), 0o644))

	matches, err := searchFilesWithRegexCtx(t.Context(), "findme", dir, "", 100)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, "text.txt", filepath.Base(matches[0].path))
}

func TestGrepNativeSkipsLargeFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(dir, "small.txt"), []byte("needle in small file\n"), 0o644))

	oversized := bytes.Repeat([]byte("a"), maxGrepFileSize)
	oversized = append(oversized, []byte("\nneedle in large file\n")...)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "large.txt"), oversized, 0o644))

	matches, err := searchFilesWithRegexCtx(t.Context(), "needle", dir, "", 100)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, "small.txt", filepath.Base(matches[0].path))
}

func TestGrepNativeSkipsHiddenDirsAndFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	files := map[string]string{
		"visible.go":           "package main // marker\n",
		".hidden.go":           "package hidden // marker\n",
		".hiddendir/nested.go": "package nested // marker\n",
		"sub/.secret/deep.go":  "package deep // marker\n",
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}

	matches, err := searchFilesWithRegexCtx(t.Context(), "marker", dir, "", 100)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, "visible.go", filepath.Base(matches[0].path))
}

func TestGrepNativeIncludePatterns(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	files := map[string]string{
		"root.go":       "target_token",
		"root.js":       "target_token",
		"sub/child.go":  "target_token",
		"sub/child.ts":  "target_token",
		"sub/child.txt": "target_token",
	}
	for rel, content := range files {
		full := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}

	t.Run("extension glob", func(t *testing.T) {
		matches, err := searchFilesWithRegexCtx(t.Context(), "target_token", dir, "*.go", 100)
		require.NoError(t, err)
		require.Len(t, matches, 2)
	})

	t.Run("brace expansion glob", func(t *testing.T) {
		matches, err := searchFilesWithRegexCtx(t.Context(), "target_token", dir, "*.{js,ts}", 100)
		require.NoError(t, err)
		require.Len(t, matches, 2)
	})

	t.Run("scoped directory glob", func(t *testing.T) {
		matches, err := searchFilesWithRegexCtx(t.Context(), "target_token", dir, "sub/*.go", 100)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		require.Equal(t, "child.go", filepath.Base(matches[0].path))
	})

	t.Run("backslash path separator normalized", func(t *testing.T) {
		matches, err := searchFilesWithRegexCtx(t.Context(), "target_token", dir, `sub\*.ts`, 100)
		require.NoError(t, err)
		require.Len(t, matches, 1)
		require.Equal(t, "child.ts", filepath.Base(matches[0].path))
	})

	t.Run("doublestar glob", func(t *testing.T) {
		matches, err := searchFilesWithRegexCtx(t.Context(), "target_token", dir, "**/*.go", 100)
		require.NoError(t, err)
		require.Len(t, matches, 2)
	})

	t.Run("invalid include pattern", func(t *testing.T) {
		_, err := searchFilesWithRegexCtx(t.Context(), "target_token", dir, "[unclosed", 100)
		require.Error(t, err)
	})
}

func TestGrepNativeParallelAndLineTruncation(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	const fileCount = 60
	for i := range fileCount {
		name := fmt.Sprintf("pkg%02d/file_%02d.go", i%6, i)
		full := filepath.Join(dir, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		content := fmt.Sprintf("package pkg\nvar v%d = \"parallel_needle\"\n", i)
		if i == 0 {
			content = fmt.Sprintf("package pkg\nvar long = \"parallel_needle_%s\"\n", strings.Repeat("x", maxGrepContentWidth+200))
		}
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}

	matches, err := searchFilesWithRegexCtx(t.Context(), "parallel_needle", dir, "*.go", 100)
	require.NoError(t, err)
	require.Len(t, matches, fileCount)

	for _, m := range matches {
		require.LessOrEqual(t, len(m.lineText), maxGrepContentWidth+3)
	}

	re, err := searchRegexCache.get("parallel_needle")
	require.NoError(t, err)

	fastwalkMatches, err := searchWithFastwalk(t.Context(), dir, "*.go", re, 500)
	require.NoError(t, err)
	require.Len(t, fastwalkMatches, fileCount)

	entries, ok := fsext.IndexEntries(t.Context(), dir)
	require.True(t, ok)
	require.NotEmpty(t, entries)

	indexedMatches, err := searchIndexedEntries(t.Context(), dir, entries, "*.go", re, 10)
	require.NoError(t, err)
	require.NotEmpty(t, indexedMatches)
	require.LessOrEqual(t, len(indexedMatches), 25)

	var _ *regexp.Regexp = re
}
