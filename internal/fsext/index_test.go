package fsext

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestWorkspaceIndexCachingAndInvalidation(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	firstFile := filepath.Join(tmp, "a.go")
	ignoredDir := filepath.Join(tmp, "node_modules")
	require.NoError(t, os.WriteFile(firstFile, []byte("package a"), 0o644))
	require.NoError(t, os.MkdirAll(ignoredDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(ignoredDir, "dep.js"), []byte("x"), 0o644))

	idx := NewWorkspaceIndex(WorkspaceIndexConfig{
		MaxRoots:     4,
		MaxEntries:   1000,
		FallbackTTL:  time.Minute,
		WatchEnabled: false,
	})
	defer idx.Close()

	entries, ok := idx.IndexEntries(context.Background(), tmp)
	require.True(t, ok)
	require.Len(t, entries, 1)
	require.Equal(t, "a.go", entries[0].SlashPath)
	require.Equal(t, "a.go", entries[0].Name)
	require.False(t, entries[0].IsDir)
	require.Equal(t, int64(len("package a")), entries[0].Size)

	secondFile := filepath.Join(tmp, "sub", "b.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(secondFile), 0o755))
	require.NoError(t, os.WriteFile(secondFile, []byte("package sub"), 0o644))

	cached, ok := idx.IndexEntries(context.Background(), tmp)
	require.True(t, ok)
	require.Len(t, cached, 1)

	idx.InvalidateIndex(secondFile)
	refreshed, ok := idx.IndexEntries(context.Background(), tmp)
	require.True(t, ok)
	require.Len(t, refreshed, 3)

	idx.InvalidateRoot(tmp)
	idx.mu.Lock()
	_, stillCached := idx.roots[filepath.Clean(tmp)]
	idx.mu.Unlock()
	require.False(t, stillCached)
}

func TestWorkspaceIndexBoundsAndEviction(t *testing.T) {
	t.Parallel()

	idx := NewWorkspaceIndex(WorkspaceIndexConfig{
		MaxRoots:     2,
		MaxEntries:   3,
		FallbackTTL:  time.Minute,
		WatchEnabled: false,
	})
	defer idx.Close()

	root1 := t.TempDir()
	root2 := t.TempDir()
	root3 := t.TempDir()
	for _, r := range []string{root1, root2, root3} {
		require.NoError(t, os.WriteFile(filepath.Join(r, "f.txt"), []byte("1"), 0o644))
	}

	_, ok := idx.IndexEntries(context.Background(), root1)
	require.True(t, ok)
	_, ok = idx.IndexEntries(context.Background(), root2)
	require.True(t, ok)
	_, ok = idx.IndexEntries(context.Background(), root1)
	require.True(t, ok)
	_, ok = idx.IndexEntries(context.Background(), root3)
	require.True(t, ok)

	idx.mu.Lock()
	_, hasRoot1 := idx.roots[filepath.Clean(root1)]
	_, hasRoot2 := idx.roots[filepath.Clean(root2)]
	_, hasRoot3 := idx.roots[filepath.Clean(root3)]
	idx.mu.Unlock()

	require.True(t, hasRoot1)
	require.False(t, hasRoot2)
	require.True(t, hasRoot3)

	overCapRoot := t.TempDir()
	for i := range 5 {
		require.NoError(t, os.WriteFile(filepath.Join(overCapRoot, fmt.Sprintf("f%d.txt", i)), []byte("x"), 0o644))
	}
	entries, ok := idx.IndexEntries(context.Background(), overCapRoot)
	require.False(t, ok)
	require.Nil(t, entries)
}

func TestWorkspaceIndexTTLFallbackAndCancellation(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "a.txt"), []byte("a"), 0o644))

	idx := NewWorkspaceIndex(WorkspaceIndexConfig{
		MaxRoots:     4,
		MaxEntries:   1000,
		FallbackTTL:  30 * time.Millisecond,
		WatchEnabled: false,
	})
	defer idx.Close()

	entries, ok := idx.IndexEntries(context.Background(), tmp)
	require.True(t, ok)
	require.Len(t, entries, 1)

	require.NoError(t, os.WriteFile(filepath.Join(tmp, "b.txt"), []byte("b"), 0o644))
	time.Sleep(50 * time.Millisecond)

	entries, ok = idx.IndexEntries(context.Background(), tmp)
	require.True(t, ok)
	require.Len(t, entries, 2)

	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	entries, ok = idx.IndexEntries(cancelledCtx, t.TempDir())
	require.False(t, ok)
	require.Nil(t, entries)
}

func TestWorkspaceIndexConcurrentBuilds(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	for i := range 20 {
		require.NoError(t, os.WriteFile(filepath.Join(tmp, fmt.Sprintf("file_%d.go", i)), []byte("package p"), 0o644))
	}

	idx := NewWorkspaceIndex(WorkspaceIndexConfig{
		MaxRoots:     4,
		MaxEntries:   1000,
		FallbackTTL:  time.Minute,
		WatchEnabled: false,
	})
	defer idx.Close()

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			entries, ok := idx.IndexEntries(context.Background(), tmp)
			require.True(t, ok)
			require.Len(t, entries, 20)
		})
	}
	wg.Wait()
}

func TestWorkspaceIndexWatcherLifecycle(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tmp, "init.txt"), []byte("1"), 0o644))

	idx := NewWorkspaceIndex(WorkspaceIndexConfig{
		MaxRoots:     4,
		MaxEntries:   1000,
		FallbackTTL:  time.Minute,
		WatchEnabled: true,
	})
	defer idx.Close()

	entries, ok := idx.IndexEntries(context.Background(), tmp)
	require.True(t, ok)
	require.Len(t, entries, 1)

	require.NoError(t, os.WriteFile(filepath.Join(tmp, "added.txt"), []byte("2"), 0o644))

	require.Eventually(t, func() bool {
		idx.mu.Lock()
		_, cached := idx.roots[filepath.Clean(tmp)]
		idx.mu.Unlock()
		return !cached
	}, 2*time.Second, 20*time.Millisecond)

	entries, ok = idx.IndexEntries(context.Background(), tmp)
	require.True(t, ok)
	require.Len(t, entries, 2)
}

func TestPackageIndexIntegrationWithGlobAndList(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	fileOld := filepath.Join(tmp, "old.go")
	fileNew := filepath.Join(tmp, "sub", "new.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(fileNew), 0o755))
	require.NoError(t, os.WriteFile(fileOld, []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(fileNew, []byte("package sub"), 0o644))

	base := time.Now().Add(-2 * time.Hour)
	require.NoError(t, os.Chtimes(fileOld, base, base))
	require.NoError(t, os.Chtimes(fileNew, base.Add(time.Hour), base.Add(time.Hour)))

	defer InvalidateRoot(tmp)

	entries, ok := IndexEntries(context.Background(), tmp)
	require.True(t, ok)
	require.Len(t, entries, 3)

	globMatches, truncated, err := GlobGitignoreAwareCtx(context.Background(), "**/*.go", tmp, 10)
	require.NoError(t, err)
	require.False(t, truncated)
	require.Equal(t, []string{fileNew, fileOld}, globMatches)

	listed, truncated, err := ListDirectory(tmp, nil, 2, -1)
	require.NoError(t, err)
	require.False(t, truncated)
	require.ElementsMatch(t, []string{"old.go", "sub", "sub/new.go"}, relPaths(t, listed, tmp))

	added := filepath.Join(tmp, "sub", "fresh.go")
	require.NoError(t, os.WriteFile(added, []byte("package sub"), 0o644))
	require.NoError(t, os.Chtimes(added, base.Add(2*time.Hour), base.Add(2*time.Hour)))

	InvalidateIndex(added)

	globMatches, truncated, err = GlobGitignoreAwareCtx(context.Background(), "**/*.go", tmp, 2)
	require.NoError(t, err)
	require.True(t, truncated)
	require.Equal(t, []string{added, fileNew}, globMatches)
}
