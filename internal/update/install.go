package update

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Install downloads the latest release binary for the current platform and
// replaces executable in place. It returns the update info either way;
// check Info.Available first if the caller only wants to act on updates.
func Install(ctx context.Context, current, executable string, client Client, out io.Writer) (Info, error) {
	info, err := Check(ctx, current, client)
	if err != nil {
		return info, err
	}

	release, err := client.Latest(ctx)
	if err != nil {
		return info, err
	}

	asset, err := pickAsset(release)
	if err != nil {
		return info, err
	}
	checksums := findAsset(release.Assets, "checksums.txt")

	tmpDir, err := os.MkdirTemp("", "prime-update-*")
	if err != nil {
		return info, err
	}
	defer func() {
		if fi, err := os.Stat(tmpDir); err == nil && fi.IsDir() {
			_ = os.RemoveAll(tmpDir)
		}
	}()

	archivePath := filepath.Join(tmpDir, asset.Name)
	fmt.Fprintf(out, "Downloading %s...\n", asset.Name)
	if err := download(ctx, asset.BrowserDownloadURL, archivePath); err != nil {
		return info, err
	}

	if checksums != nil {
		sumPath := filepath.Join(tmpDir, "checksums.txt")
		if err := download(ctx, checksums.BrowserDownloadURL, sumPath); err != nil {
			return info, err
		}
		if err := verifyChecksum(archivePath, sumPath, asset.Name); err != nil {
			return info, err
		}
		fmt.Fprintln(out, "Checksum verified.")
	}

	data, err := extractBinary(archivePath)
	if err != nil {
		return info, err
	}

	if err := swapBinary(executable, data); err != nil {
		return info, err
	}
	return info, nil
}

func pickAsset(release *Release) (*Asset, error) {
	osToken, err := osToken()
	if err != nil {
		return nil, err
	}
	archToken, err := archToken()
	if err != nil {
		return nil, err
	}
	ext := "tar.gz"
	if runtime.GOOS == "windows" {
		ext = "zip"
	}
	version := strings.TrimPrefix(release.TagName, "v")
	want := fmt.Sprintf("prime_%s_%s_%s.%s", version, osToken, archToken, ext)
	if a := findAsset(release.Assets, want); a != nil {
		return a, nil
	}
	return nil, fmt.Errorf("release %s has no asset %s", release.TagName, want)
}

func findAsset(assets []Asset, name string) *Asset {
	for i := range assets {
		if assets[i].Name == name {
			return &assets[i]
		}
	}
	return nil
}

func osToken() (string, error) {
	switch runtime.GOOS {
	case "linux":
		return "Linux", nil
	case "darwin":
		return "Darwin", nil
	case "windows":
		return "Windows", nil
	case "freebsd":
		return "Freebsd", nil
	case "openbsd":
		return "Openbsd", nil
	case "netbsd":
		return "Netbsd", nil
	default:
		return "", fmt.Errorf("no release binaries for %s", runtime.GOOS)
	}
}

func archToken() (string, error) {
	switch runtime.GOARCH {
	case "amd64":
		return "x86_64", nil
	case "arm64":
		return "arm64", nil
	case "386":
		return "i386", nil
	case "arm":
		return "armv7", nil
	default:
		return "", fmt.Errorf("no release binaries for %s", runtime.GOARCH)
	}
}

func download(ctx context.Context, url, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: unexpected status %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return f.Close()
}

func verifyChecksum(archivePath, sumsPath, name string) error {
	sums, err := os.ReadFile(sumsPath)
	if err != nil {
		return err
	}
	var want string
	for _, line := range strings.Split(string(sums), "\n") {
		fields := strings.Fields(line)
		if len(fields) >= 2 && filepath.Base(fields[1]) == name {
			want = fields[0]
			break
		}
	}
	if want == "" {
		return fmt.Errorf("%s missing from checksums.txt", name)
	}
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return err
	}
	sum := sha256.Sum256(data)
	if !strings.EqualFold(hex.EncodeToString(sum[:]), want) {
		return errors.New("checksum mismatch, refusing to install")
	}
	return nil
}

func extractBinary(archivePath string) ([]byte, error) {
	data, err := os.ReadFile(archivePath)
	if err != nil {
		return nil, err
	}
	binaryName := "prime"
	if runtime.GOOS == "windows" {
		binaryName = "prime.exe"
	}
	if strings.HasSuffix(archivePath, ".zip") {
		return extractZip(data, binaryName)
	}
	return extractTarGz(data, binaryName)
}

func extractZip(data []byte, name string) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) != name || f.FileInfo().IsDir() {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(rc)
	}
	return nil, fmt.Errorf("%s not found in archive", name)
}

func extractTarGz(data []byte, name string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("%s not found in archive", name)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg || filepath.Base(hdr.Name) != name {
			continue
		}
		return io.ReadAll(tr)
	}
}
