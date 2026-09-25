package voice

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// downloadTimeout bounds one file download. A model is a few hundred megabytes,
// which is generous on a slow link but still ends a dead connection.
const downloadTimeout = 30 * time.Minute

// progressChunk is how often a download reports progress.
const progressChunk = 2 << 20

// downloadFile writes a URL to disk and verifies its SHA-256 when a digest is
// given. The file lands at dest, which is removed on any failure so a partial
// download never looks like an installed tool.
func downloadFile(ctx context.Context, client *http.Client, url, dest, wantSHA string, progress func(done, total int64)) error {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "prime-voice-setup")

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("cannot download %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("downloading %s returned %s", path.Base(url), resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	file, err := os.Create(dest)
	if err != nil {
		return err
	}

	hasher := sha256.New()
	reader := io.TeeReader(resp.Body, hasher)
	if progress != nil {
		reader = &progressReader{reader: reader, total: resp.ContentLength, report: progress}
	}
	if _, err := io.Copy(file, reader); err != nil {
		_ = file.Close()
		_ = os.Remove(dest)
		return fmt.Errorf("download of %s stopped: %w", path.Base(url), err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(dest)
		return err
	}
	if wantSHA != "" {
		got := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(got, wantSHA) {
			_ = os.Remove(dest)
			return fmt.Errorf("checksum mismatch for %s, refusing to install", path.Base(url))
		}
	}
	return nil
}

// progressReader reports download progress every progressChunk bytes. A unknown
// content length is reported as a negative total, which callers render as a
// plain byte count.
type progressReader struct {
	reader io.Reader
	done   int64
	total  int64
	report func(done, total int64)
}

func (r *progressReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.done += int64(n)
	if n > 0 && r.done%progressChunk < int64(n) {
		r.report(r.done, r.total)
	}
	return n, err
}

// installableFile decides which archive members belong in the tool directory.
// whisper.cpp archives also carry headers, cmake files and docs, which are
// noise next to a binary.
func installableFile(name string) bool {
	clean := path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if strings.HasPrefix(clean, "..") || path.IsAbs(clean) {
		return false
	}
	base := path.Base(clean)
	if base == "." || base == "/" || strings.HasPrefix(base, ".") {
		return false
	}
	switch strings.ToLower(filepath.Ext(base)) {
	case ".h", ".hpp", ".md", ".txt", ".cmake", ".pc", ".pdb", ".lib", ".a", ".obj":
		return false
	}
	dir := strings.ToLower(path.Dir(clean))
	if dir == "." {
		return true
	}
	if strings.Contains(dir, "include") || strings.Contains(dir, "cmake") ||
		strings.Contains(dir, "pkgconfig") || strings.Contains(dir, "docs") {
		return false
	}
	return true
}

// extractArchive unpacks a .zip or .tar(.gz) archive into dest, keeping file
// names but dropping the archive's own directory layout, so an executable and
// the libraries beside it end up next to each other. It returns the names of
// the files written.
func extractArchive(archivePath, dest string) ([]string, error) {
	info, err := os.Stat(archivePath)
	if err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		return nil, fmt.Errorf("%s is empty", filepath.Base(archivePath))
	}

	switch {
	case strings.HasSuffix(strings.ToLower(archivePath), ".zip"):
		return extractZip(archivePath, dest)
	case strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz"),
		strings.HasSuffix(strings.ToLower(archivePath), ".tgz"):
		return extractTarGz(archivePath, dest)
	default:
		return nil, fmt.Errorf("unsupported archive %s", filepath.Base(archivePath))
	}
}

func extractZip(archivePath, dest string) ([]string, error) {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", filepath.Base(archivePath), err)
	}
	defer func() { _ = reader.Close() }()

	var written []string
	for _, member := range reader.File {
		if member.FileInfo().IsDir() || !installableFile(member.Name) {
			continue
		}
		opened, err := member.Open()
		if err != nil {
			return written, err
		}
		base, err := writeMember(dest, opened, member.Name, member.FileInfo().Mode())
		_ = opened.Close()
		if err != nil {
			return written, err
		}
		written = append(written, base)
	}
	return written, nil
}

func extractTarGz(archivePath, dest string) ([]string, error) {
	file, err := os.Open(archivePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()

	var source io.Reader = file
	if strings.HasSuffix(strings.ToLower(archivePath), ".tar.gz") ||
		strings.HasSuffix(strings.ToLower(archivePath), ".tgz") {
		gz, err := gzip.NewReader(file)
		if err != nil {
			return nil, fmt.Errorf("cannot read %s: %w", filepath.Base(archivePath), err)
		}
		defer func() { _ = gz.Close() }()
		source = gz
	}

	reader := tar.NewReader(source)
	var written []string
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return written, err
		}
		if header.Typeflag != tar.TypeReg || !installableFile(header.Name) {
			continue
		}
		base, err := writeMember(dest, reader, header.Name, header.FileInfo().Mode())
		if err != nil {
			return written, err
		}
		written = append(written, base)
	}
	return written, nil
}

// writeMember copies one archive member into dest under its base name and
// returns the name it was written as.
func writeMember(dest string, source io.Reader, name string, mode os.FileMode) (string, error) {
	base := filepath.Base(path.Clean(strings.ReplaceAll(name, "\\", "/")))
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return "", err
	}
	target := filepath.Join(dest, base)

	file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(file, source); err != nil {
		_ = file.Close()
		return "", fmt.Errorf("cannot write %s: %w", target, err)
	}
	if err := file.Close(); err != nil {
		return "", err
	}
	// Executables and shared libraries need the execute bit; archives built on
	// Windows carry no useful permissions, so anything without an extension
	// that came from a bin directory is made executable.
	if mode.IsRegular() && (mode&0o111 != 0 || filepath.Ext(base) == "") {
		if err := os.Chmod(target, 0o755); err != nil {
			return "", err
		}
	}
	return base, nil
}
