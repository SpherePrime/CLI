package fsext

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestPrimeIgnore(t *testing.T) {
	tempDir := t.TempDir()
	t.Chdir(tempDir)

	// Create test files
	require.NoError(t, os.WriteFile("test1.txt", []byte("test"), 0o644))
	require.NoError(t, os.WriteFile("test2.log", []byte("test"), 0o644))
	require.NoError(t, os.WriteFile("test3.tmp", []byte("test"), 0o644))

	// Create a .primeignore file that ignores .log files
	require.NoError(t, os.WriteFile(".primeignore", []byte("*.log\n"), 0o644))

	dl := NewDirectoryLister(tempDir)
	require.True(t, dl.shouldIgnore("test2.log", nil, false), ".log files should be ignored")
	require.False(t, dl.shouldIgnore("test1.txt", nil, false), ".txt files should not be ignored")
	require.True(t, dl.shouldIgnore("test3.tmp", nil, false), ".tmp files should be ignored by common patterns")
}

func TestShouldExcludeFile(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure for testing
	tempDir := t.TempDir()

	// Create directories that should be ignored
	nodeModules := filepath.Join(tempDir, "node_modules")
	target := filepath.Join(tempDir, "target")
	customIgnored := filepath.Join(tempDir, "custom_ignored")
	normalDir := filepath.Join(tempDir, "src")

	for _, dir := range []string{nodeModules, target, customIgnored, normalDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create .gitignore file
	gitignoreContent := "node_modules/\ntarget/\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte(gitignoreContent), 0o644); err != nil {
		t.Fatalf("Failed to create .gitignore: %v", err)
	}

	// Create .primeignore file
	primeignoreContent := "custom_ignored/\n"
	if err := os.WriteFile(filepath.Join(tempDir, ".primeignore"), []byte(primeignoreContent), 0o644); err != nil {
		t.Fatalf("Failed to create .primeignore: %v", err)
	}

	// Test that ignored directories are properly ignored
	require.True(t, ShouldExcludeFile(tempDir, nodeModules), "Expected node_modules to be ignored by .gitignore")
	require.True(t, ShouldExcludeFile(tempDir, target), "Expected target to be ignored by .gitignore")
	require.True(t, ShouldExcludeFile(tempDir, customIgnored), "Expected custom_ignored to be ignored by .primeignore")

	// Test that normal directories are not ignored
	require.False(t, ShouldExcludeFile(tempDir, normalDir), "Expected src directory to not be ignored")

	// Test that the workspace root itself is not ignored
	require.False(t, ShouldExcludeFile(tempDir, tempDir), "Expected workspace root to not be ignored")
}

func TestShouldExcludeFileHierarchical(t *testing.T) {
	t.Parallel()

	// Create a nested directory structure for testing hierarchical ignore
	tempDir := t.TempDir()

	// Create nested directories
	subDir := filepath.Join(tempDir, "subdir")
	nestedNormal := filepath.Join(subDir, "normal_nested")

	for _, dir := range []string{subDir, nestedNormal} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Create .primeignore in subdir that ignores normal_nested
	subPrimeignore := "normal_nested/\n"
	if err := os.WriteFile(filepath.Join(subDir, ".primeignore"), []byte(subPrimeignore), 0o644); err != nil {
		t.Fatalf("Failed to create subdir .primeignore: %v", err)
	}

	// Test hierarchical ignore behavior - this should work because the .primeignore is in the parent directory
	require.True(t, ShouldExcludeFile(tempDir, nestedNormal), "Expected normal_nested to be ignored by subdir .primeignore")
	require.False(t, ShouldExcludeFile(tempDir, subDir), "Expected subdir itself to not be ignored")
}

func TestShouldExcludeFileCommonPatterns(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()

	// Create directories that should be ignored by common patterns
	commonIgnored := []string{
		filepath.Join(tempDir, ".git"),
		filepath.Join(tempDir, "node_modules"),
		filepath.Join(tempDir, "__pycache__"),
		filepath.Join(tempDir, "target"),
		filepath.Join(tempDir, ".vscode"),
	}

	for _, dir := range commonIgnored {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
	}

	// Test that common patterns are ignored even without explicit ignore files
	for _, dir := range commonIgnored {
		require.True(t, ShouldExcludeFile(tempDir, dir), "Expected %s to be ignored by common patterns", filepath.Base(dir))
	}
}

func TestDirectoryListerMatcherInheritance(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "sub")
	nestedDir := filepath.Join(subDir, "nested")
	emptyIgnoreDir := filepath.Join(tempDir, "empty")

	require.NoError(t, os.MkdirAll(nestedDir, 0o755))
	require.NoError(t, os.MkdirAll(emptyIgnoreDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte("*.log\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(subDir, ".gitignore"), []byte("!keep.log\n*.bak\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(emptyIgnoreDir, ".gitignore"), []byte(""), 0o644))

	dl := NewDirectoryLister(tempDir)

	rootEntry := dl.getCombinedMatcherEntry(tempDir)
	subEntry := dl.getCombinedMatcherEntry(subDir)
	nestedEntry := dl.getCombinedMatcherEntry(nestedDir)
	emptyEntry := dl.getCombinedMatcherEntry(emptyIgnoreDir)

	require.Same(t, rootEntry, emptyEntry)
	require.NotSame(t, rootEntry, subEntry)
	require.Same(t, subEntry, nestedEntry)

	require.True(t, dl.shouldIgnore(filepath.Join(tempDir, "root.log"), nil, false))
	require.False(t, dl.shouldIgnore(filepath.Join(subDir, "keep.log"), nil, false))
	require.False(t, dl.shouldIgnore(filepath.Join(nestedDir, "keep.log"), nil, false))
	require.True(t, dl.shouldIgnore(filepath.Join(nestedDir, "drop.log"), nil, false))
	require.True(t, dl.shouldIgnore(filepath.Join(nestedDir, "old.bak"), nil, false))
}

func TestDirectoryListerConcurrentAccess(t *testing.T) {
	t.Parallel()

	tempDir := t.TempDir()
	nested := filepath.Join(tempDir, "a", "b", "c")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, ".gitignore"), []byte("*.tmp\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "a", ".primeignore"), []byte("*.secret\n"), 0o644))

	dl := NewDirectoryLister(tempDir)
	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			require.True(t, dl.shouldIgnore(filepath.Join(nested, "x.secret"), nil, false))
			require.False(t, dl.shouldIgnore(filepath.Join(nested, "main.go"), nil, false))
		})
	}
	wg.Wait()
}
