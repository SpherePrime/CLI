package voice

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// writeTestZip builds a zip archive from path → content.
func writeTestZip(t *testing.T, dest string, members map[string]string) {
	t.Helper()

	file, err := os.Create(dest)
	require.NoError(t, err)
	writer := zip.NewWriter(file)
	for name, content := range members {
		entry, err := writer.Create(name)
		require.NoError(t, err)
		_, err = entry.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	require.NoError(t, file.Close())
}

func TestInstallableFileAcceptsRealArchiveMembers(t *testing.T) {
	t.Parallel()

	accepted := []string{
		"Release/whisper-cli.exe",
		"Release/ggml-base.dll",
		"whisper-bin-ubuntu-x64/whisper-cli",
		"whisper-bin-ubuntu-x64/libwhisper.so.1",
		"whisper-bin-ubuntu-x64/libggml-base.so.0.23.0",
		"bin/whisper-cli",
		"lib/libwhisper.so",
		"whisper-cli.exe",
	}
	for _, name := range accepted {
		require.True(t, installableFile(name), name)
	}

	rejected := []string{
		"Release/whisper-cli.h",
		"Release/notes.md",
		"whisper-bin-ubuntu-x64/CMakeLists.txt",
		"include/whisper.h",
		"docs/intro.md",
		"../escape.exe",
	}
	for _, name := range rejected {
		require.False(t, installableFile(name), name)
	}
}

func TestExtractArchiveFlattensWindowsLayout(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	archive := filepath.Join(dir, "whisper-bin-x64.zip")
	writeTestZip(t, archive, map[string]string{
		"Release/whisper-cli.exe": "binary",
		"Release/ggml-base.dll":   "library",
		"Release/whisper-cli.h":   "header",
	})

	dest := filepath.Join(dir, "bin")
	written, err := extractArchive(archive, dest)
	require.NoError(t, err)
	require.Contains(t, written, "whisper-cli.exe")
	require.Contains(t, written, "ggml-base.dll")
	require.NotContains(t, written, "whisper-cli.h")

	_, err = os.Stat(filepath.Join(dest, "whisper-cli.exe"))
	require.NoError(t, err, "the binary must land next to its libraries")
}
