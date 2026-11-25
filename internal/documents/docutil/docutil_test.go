package docutil

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/golang/freetype/truetype"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestEmbeddedFontsSupportCyrillic(t *testing.T) {
	fonts := [][]byte{
		RegularFontBytes(),
		BoldFontBytes(),
		ItalicFontBytes(),
		MonoFontBytes(),
	}
	for _, data := range fonts {
		require.NotEmpty(t, data)
		font, err := truetype.Parse(data)
		require.NoError(t, err)
		for _, r := range []rune{'A', 'z', 'А', 'Я', 'а', 'я', 'Ё', 'ё', '0', '9', '№', '—'} {
			require.NotZero(t, font.Index(r), "missing glyph for %q", r)
		}
	}
}

func TestResolvePathAndAtomicWrite(t *testing.T) {
	dir := t.TempDir()

	resolved, err := ResolvePath(dir, "reports/out.pdf")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(dir, "reports", "out.pdf"), resolved)

	absTarget := filepath.Join(dir, "abs", "file.txt")
	resolvedAbs, err := ResolvePath(dir, absTarget)
	require.NoError(t, err)
	require.Equal(t, absTarget, resolvedAbs)

	_, err = ResolvePath(dir, "")
	require.Error(t, err)

	_, err = ResolvePath(dir, "   ")
	require.Error(t, err)

	_, err = ResolvePath("", "relative/path.pdf")
	require.Error(t, err)

	require.NoError(t, WriteFileAtomic(resolved, []byte("hello")))
	require.FileExists(t, resolved)
	got, err := os.ReadFile(resolved)
	require.NoError(t, err)
	require.Equal(t, []byte("hello"), got)

	require.NoError(t, WriteFileAtomic(resolved, []byte("updated-контент")))
	got, err = os.ReadFile(resolved)
	require.NoError(t, err)
	require.Equal(t, []byte("updated-контент"), got)

	entries, err := os.ReadDir(filepath.Dir(resolved))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}

func TestWriteZipAtomic(t *testing.T) {
	dir := t.TempDir()
	zipPath := filepath.Join(dir, "nested", "archive.docx")

	err := WriteZipAtomic(zipPath, func(zw *zip.Writer) error {
		w, err := zw.Create("word/document.xml")
		if err != nil {
			return err
		}
		_, err = w.Write([]byte("<doc>Привет мир</doc>"))
		return err
	})
	require.NoError(t, err)
	require.FileExists(t, zipPath)

	r, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer r.Close()
	require.Len(t, r.File, 1)
	require.Equal(t, "word/document.xml", r.File[0].Name)

	rc, err := r.File[0].Open()
	require.NoError(t, err)
	body, err := io.ReadAll(rc)
	require.NoError(t, rc.Close())
	require.NoError(t, err)
	require.Equal(t, "<doc>Привет мир</doc>", string(body))

	failZip := filepath.Join(dir, "nested", "failed.docx")
	err = WriteZipAtomic(failZip, func(zw *zip.Writer) error {
		return errors.New("boom")
	})
	require.Error(t, err)
	require.NoFileExists(t, failZip)

	entries, err := os.ReadDir(filepath.Dir(zipPath))
	require.NoError(t, err)
	require.Len(t, entries, 1)
}
