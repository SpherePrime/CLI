package voice

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// sttHTTPClient downloads multi-hundred-megabyte models. No overall
// timeout: the install task's own context bounds the run, and a fixed
// client timeout would kill a slow but healthy multi-gigabyte transfer.
var sttHTTPClient = &http.Client{}

// downloadChunk is the read block size; one progress update flows out per
// chunk, which keeps the callback rate sane without stalling the UI.
const downloadChunk = 64 * 1024

// installWhisperModel brings in the whisper.cpp runtime when it is missing
// and then the GGML model file, which is a plain download with no archive
// to unpack.
func installWhisperModel(ctx context.Context, dataDir, model string, emit func(SetupProgress)) error {
	if _, ok := findSttModel(whisperModels, model); !ok {
		return fmt.Errorf("%w: whisper model %q", ErrSttUnknownModel, model)
	}
	if fileExists(whisperModelPath(dataDir, model)) {
		emitReady(emit, "Whisper model "+model+" is already installed")
		return nil
	}
	if err := ensureWhisperRuntime(ctx, dataDir, emit); err != nil {
		return err
	}

	target := whisperModelPath(dataDir, model)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	url := sttDownloadURL(whisperModelURLPrefix + model + ".bin")
	label := "Downloading " + model
	if err := downloadToFile(ctx, url, target, label, emit); err != nil {
		return err
	}
	emitReady(emit, "Whisper model "+model+" installed")
	return nil
}

// ensureWhisperRuntime downloads and unpacks the whisper.cpp command-line
// binary unless one is already in the bin directory.
func ensureWhisperRuntime(ctx context.Context, dataDir string, emit func(SetupProgress)) error {
	if findWhisperBinary(whisperBinDir(dataDir)) != "" {
		return nil
	}
	url, ok := whisperRuntime()
	if !ok {
		return fmt.Errorf("whisper.cpp publishes no prebuilt macOS binary Prime can install; " +
			"install it manually (for example with brew install whisper-cpp) and the whisper engine will use it")
	}
	emit(SetupProgress{Phase: "runtime", Text: "Downloading whisper.cpp runtime..."})
	temp, err := os.CreateTemp("", "prime-whisper-runtime-*")
	if err != nil {
		return fmt.Errorf("stage runtime download: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Close(); err != nil {
		return err
	}
	if err := downloadToFile(ctx, sttDownloadURL(url), tempPath, "Downloading whisper.cpp runtime", emit); err != nil {
		return err
	}

	binDir := whisperBinDir(dataDir)
	emit(SetupProgress{Phase: "runtime", Text: "Extracting whisper.cpp runtime..."})
	if strings.HasSuffix(url, ".zip") {
		err = extractZip(tempPath, binDir)
	} else {
		err = extractTarGz(tempPath, binDir)
	}
	if err != nil {
		return fmt.Errorf("extract whisper.cpp runtime: %w", err)
	}
	// Windows-built archives often lose the executable bit, so it is set
	// explicitly on every platform; it is a no-op on Windows.
	if binary := findWhisperBinary(binDir); binary != "" && runtime.GOOS != "windows" {
		if err := os.Chmod(binary, 0o755); err != nil {
			return fmt.Errorf("mark whisper binary executable: %w", err)
		}
	}
	return nil
}

// installVoskModel brings in the libvosk shared library when it is missing
// and then the model zip, which extracts into its own folder.
func installVoskModel(ctx context.Context, dataDir, model string, emit func(SetupProgress)) error {
	if _, ok := findSttModel(voskModels, model); !ok {
		return fmt.Errorf("%w: vosk model %q", ErrSttUnknownModel, model)
	}
	if dirExists(voskModelDir(dataDir, model)) {
		emitReady(emit, "Vosk model "+model+" is already installed")
		return nil
	}
	if err := ensureVoskRuntime(ctx, dataDir, emit); err != nil {
		return err
	}

	url := sttDownloadURL(voskModelURLPrefix + model + ".zip")
	emit(SetupProgress{Phase: "download", Text: "Downloading " + model + "...", Total: voskModelSize(model)})
	temp, err := os.CreateTemp("", "prime-vosk-model-*")
	if err != nil {
		return fmt.Errorf("stage model download: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Close(); err != nil {
		return err
	}
	if err := downloadToFile(ctx, url, tempPath, "Downloading "+model, emit); err != nil {
		return err
	}

	emit(SetupProgress{Phase: "extract", Text: "Extracting " + model + "..."})
	// The zip ships a top-level folder named after the model, so extracting
	// into models/ produces models/<model>/... directly.
	if err := extractZip(tempPath, filepath.Dir(voskModelDir(dataDir, model))); err != nil {
		return fmt.Errorf("extract vosk model: %w", err)
	}
	emitReady(emit, "Vosk model "+model+" installed")
	return nil
}

// ensureVoskRuntime downloads and unpacks the libvosk shared library
// unless it is already in the lib directory.
func ensureVoskRuntime(ctx context.Context, dataDir string, emit func(SetupProgress)) error {
	if fileExists(voskLibPath(dataDir)) {
		return nil
	}
	url, ok := voskRuntime()
	if !ok {
		return fmt.Errorf("vosk publishes no prebuilt library for %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	emit(SetupProgress{Phase: "runtime", Text: "Downloading Vosk runtime..."})
	temp, err := os.CreateTemp("", "prime-vosk-runtime-*")
	if err != nil {
		return fmt.Errorf("stage runtime download: %w", err)
	}
	tempPath := temp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := temp.Close(); err != nil {
		return err
	}
	if err := downloadToFile(ctx, sttDownloadURL(url), tempPath, "Downloading Vosk runtime", emit); err != nil {
		return err
	}

	emit(SetupProgress{Phase: "runtime", Text: "Extracting Vosk runtime..."})
	if err := extractZip(tempPath, voskLibDir(dataDir)); err != nil {
		return fmt.Errorf("extract vosk runtime: %w", err)
	}
	// The release zips ship the library inside a top-level folder named
	// after the release, so the file is moved up next to anything else the
	// archive carried before the presence check runs.
	if err := relocateFile(voskLibDir(dataDir), filepath.Base(voskLibPath(dataDir))); err != nil {
		return fmt.Errorf("relocate vosk runtime: %w", err)
	}
	if !fileExists(voskLibPath(dataDir)) {
		return fmt.Errorf("vosk runtime archive contained no %s", filepath.Base(voskLibPath(dataDir)))
	}
	return nil
}

// relocateFile lifts a nested file up to the root of dir so a platform
// check that expects a fixed layout does not care how the release packaged
// it. Only the first match moves; release zips carry exactly one.
func relocateFile(dir, name string) error {
	found := ""
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || found != "" {
			return nil //nolint:nilerr // a missing directory just means "nothing to move"
		}
		if entry.Name() == name && filepath.Dir(path) != dir {
			found = path
		}
		return nil
	})
	if err != nil || found == "" {
		return err
	}
	source, err := os.Open(found)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	if err := writeFileWithMode(filepath.Join(dir, name), source, info.Mode().Perm()); err != nil {
		return err
	}
	return os.Remove(found)
}

// findSttModel locates a catalog entry by id.
func findSttModel(models []SttModel, id string) (SttModel, bool) {
	for _, model := range models {
		if model.ID == id {
			return model, true
		}
	}
	return SttModel{}, false
}

// voskModelSize reads the advertised size out of the catalog so the
// progress bar can start from a sane total before the server answers.
func voskModelSize(model string) int64 {
	if entry, ok := findSttModel(voskModels, model); ok {
		return entry.SizeBytes
	}
	return 0
}

// emitReady closes an install run with the success line the menu shows.
func emitReady(emit func(SetupProgress), text string) {
	emit(SetupProgress{Phase: "ready", Text: text, Done: 1, Total: 1})
}

// downloadToFile streams url into dest, reporting byte progress under the
// given label. The file is written under its final name in place: the
// install task holds the only reference, and a failed download leaves a
// partial file that SttIsInstalled's checks will not mistake for a model
// because whisper models are validated by size on load and vosk models by
// their extracted directory, which only appears after a successful unpack.
func downloadToFile(ctx context.Context, url, dest, label string, emit func(SetupProgress)) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build download request: %w", err)
	}
	res, err := sttHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("download %s: %w", label, err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s answered %s", label, url, res.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create %s: %w", dest, err)
	}
	total := res.ContentLength
	done := int64(0)
	buf := make([]byte, downloadChunk)
	for {
		read, readErr := res.Body.Read(buf)
		if read > 0 {
			if _, writeErr := out.Write(buf[:read]); writeErr != nil {
				_ = out.Close()
				return fmt.Errorf("download %s: %w", label, writeErr)
			}
			done += int64(read)
			emit(SetupProgress{Phase: "download", Text: label, Done: done, Total: total})
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = out.Close()
			return fmt.Errorf("download %s: %w", label, readErr)
		}
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("download %s: %w", label, err)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

// safeExtractTarget resolves an archive entry name against dest and refuses
// anything that would escape it: absolute paths and ".." elements are the
// classic malicious-archive trick and are rejected rather than cleaned away.
func safeExtractTarget(dest, name string) (string, error) {
	if strings.ContainsRune(name, 0) || filepath.IsAbs(name) || strings.HasPrefix(name, "/") ||
		strings.Contains(name, `\`) {
		return "", fmt.Errorf("archive entry escapes destination: %q", name)
	}
	clean := filepath.Clean(filepath.Join(dest, filepath.FromSlash(name)))
	if clean != dest && !strings.HasPrefix(clean, dest+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive entry escapes destination: %q", name)
	}
	return clean, nil
}

// extractZip unpacks a zip archive into dest, creating directories as the
// entries demand. Entries keep their stored permissions where they are set.
func extractZip(archivePath, dest string) error {
	reader, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = reader.Close() }()
	for _, entry := range reader.File {
		target, err := safeExtractTarget(dest, entry.Name)
		if err != nil {
			return err
		}
		if err := extractZipEntry(entry, target); err != nil {
			return err
		}
	}
	return nil
}

// extractZipEntry writes one zip entry, which is either a directory or a
// regular file; other types (symlinks, in particular) are skipped because
// an archive must not plant links outside its own tree.
func extractZipEntry(entry *zip.File, target string) error {
	if entry.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if !entry.FileInfo().Mode().IsRegular() {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	source, err := entry.Open()
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()
	return writeFileWithMode(target, source, entry.FileInfo().Mode())
}

// extractTarGz unpacks a gzipped tarball into dest.
func extractTarGz(archivePath, dest string) error {
	archive, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer func() { _ = archive.Close() }()
	gz, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	defer func() { _ = gz.Close() }()

	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeExtractTarget(dest, header.Name)
		if err != nil {
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			// Only regular files are extracted: links and devices have no
			// place in a model directory and symlinks could point anywhere.
			if err := writeFileWithMode(target, reader, os.FileMode(header.Mode)&0o777); err != nil {
				return err
			}
		}
	}
}

// writeFileWithMode creates target and copies source into it.
func writeFileWithMode(target string, source io.Reader, mode os.FileMode) error {
	out, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, source); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}
