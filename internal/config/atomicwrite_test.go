package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestAtomicWriteFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	require.NoError(t, atomicWriteFile(path, []byte(`{"key":"value"}`), 0o600))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	// JSON is formatted on the way to disk so config sections stay readable.
	require.Equal(t, "{\n  \"key\": \"value\"\n}\n", string(data))

	// No temp files should linger.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "test.json", entries[0].Name())
}

func TestAtomicWriteFile_LeavesNonJSONVerbatim(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "primerrc")

	require.NoError(t, atomicWriteFile(path, []byte("source something"), 0o600))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "source something", string(data))
}

func TestAtomicWriteFile_PermissionsApplied(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not support Unix file permissions")
	}
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.json")

	require.NoError(t, atomicWriteFile(path, []byte(`{}`), 0o600))

	info, err := os.Stat(path)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}
