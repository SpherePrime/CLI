package voice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const (
	asrReleaseRepo  = "SpherePrime/prime-asr"
	asrReleaseTag   = "v0.1.0"
	asrModelFile    = "en-ru-uk.tzf.gz"
	vadModelFile    = "silero_vad.onnx.gz"
	asrModelSHA256  = "5215d4e6c90b9c119f1267862d83833ba5b6ea9c28c5df4ebefe4265e26f2f53"
	vadModelSHA256  = "cf8770c9376f65ffb8c6667bbca0edb993d9768902b3a0200b1041bcb16adaa4"
	tinyAsrModelRev = "e034289132667fa8e87dd8d474a4dc59d447b2dc"
)

type AsrAssets struct {
	BinaryPath   string
	AsrModelPath string
	VadModelPath string
}

func asrBinaryName() string {
	if runtime.GOOS == "windows" {
		return "prime-asr.exe"
	}
	return "prime-asr"
}

func asrReleaseAssetName() string {
	arch := "amd64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}
	if runtime.GOOS == "windows" {
		return fmt.Sprintf("prime-asr-windows-%s.exe", arch)
	}
	return fmt.Sprintf("prime-asr-linux-%s", arch)
}

func asrReleaseURL(name string) string {
	return fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", asrReleaseRepo, asrReleaseTag, name)
}

func asrFallbackModelURL(name string) string {
	return fmt.Sprintf("https://raw.githubusercontent.com/dkaraush/tinyasr/%s/models/%s", tinyAsrModelRev, name)
}

func asrDataDir(dataDir string) string {
	if strings.TrimSpace(dataDir) == "" {
		if dir, err := os.UserCacheDir(); err == nil && dir != "" {
			dataDir = filepath.Join(dir, "prime")
		} else {
			dataDir = filepath.Join(os.TempDir(), "prime")
		}
	}
	return filepath.Join(dataDir, "voice", "asr")
}

func nonEmptyFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Size() > 0
}

func asrCandidateDirs(dataDir string) []string {
	var dirs []string
	if explicit := strings.TrimSpace(os.Getenv("PRIME_ASR_DIR")); explicit != "" {
		dirs = append(dirs, explicit)
	}
	if exe, err := os.Executable(); err == nil && exe != "" {
		exeDir := filepath.Dir(exe)
		dirs = append(dirs,
			filepath.Join(exeDir, "asr"),
			filepath.Join(exeDir, "resources", "asr"),
			filepath.Join(exeDir, "..", "asr"),
			filepath.Join(exeDir, "..", "resources", "asr"),
		)
	}
	dirs = append(dirs, asrDataDir(dataDir))
	if runtime.GOOS == "windows" {
		if localAppData := strings.TrimSpace(os.Getenv("LOCALAPPDATA")); localAppData != "" {
			dirs = append(dirs,
				filepath.Join(localAppData, "Programs", "Prime Desktop", "resources", "asr"),
				filepath.Join(localAppData, "Prime Desktop", "resources", "asr"),
				filepath.Join(localAppData, "com.sphereprime.prime", "voice", "asr"),
			)
		}
	} else {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			dirs = append(dirs,
				filepath.Join(home, ".local", "share", "com.sphereprime.prime", "voice", "asr"),
				filepath.Join(home, ".local", "share", "prime", "voice", "asr"),
			)
		}
		dirs = append(dirs,
			"/usr/lib/prime-desktop/resources/asr",
			"/usr/share/prime-desktop/resources/asr",
		)
	}
	if cwd, err := os.Getwd(); err == nil && cwd != "" {
		dirs = append(dirs,
			filepath.Join(cwd, "desktop", "src-tauri", "asr"),
			filepath.Join(cwd, "..", "prime-asr", "target", "release"),
			filepath.Join(cwd, "..", "prime-asr", "models"),
		)
	}
	return dirs
}

func ResolveExistingAsrAssets(dataDir string) (AsrAssets, bool) {
	dirs := asrCandidateDirs(dataDir)
	binName := asrBinaryName()
	var assets AsrAssets
	for _, dir := range dirs {
		if assets.BinaryPath == "" && nonEmptyFile(filepath.Join(dir, binName)) {
			assets.BinaryPath = filepath.Join(dir, binName)
		}
		if assets.AsrModelPath == "" && nonEmptyFile(filepath.Join(dir, asrModelFile)) {
			assets.AsrModelPath = filepath.Join(dir, asrModelFile)
		}
		if assets.VadModelPath == "" && nonEmptyFile(filepath.Join(dir, vadModelFile)) {
			assets.VadModelPath = filepath.Join(dir, vadModelFile)
		}
	}
	if assets.BinaryPath != "" && assets.AsrModelPath != "" && assets.VadModelPath != "" {
		return assets, true
	}
	return AsrAssets{}, false
}

func EnsureAsrAssets(ctx context.Context, dataDir string, emit func(SetupProgress)) (AsrAssets, error) {
	if existing, ok := ResolveExistingAsrAssets(dataDir); ok {
		return existing, nil
	}

	targetDir := asrDataDir(dataDir)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return AsrAssets{}, fmt.Errorf("create ASR directory: %w", err)
	}

	dirs := asrCandidateDirs(dataDir)
	binName := asrBinaryName()
	assets := AsrAssets{}
	for _, dir := range dirs {
		if assets.BinaryPath == "" && nonEmptyFile(filepath.Join(dir, binName)) {
			assets.BinaryPath = filepath.Join(dir, binName)
		}
		if assets.AsrModelPath == "" && nonEmptyFile(filepath.Join(dir, asrModelFile)) {
			assets.AsrModelPath = filepath.Join(dir, asrModelFile)
		}
		if assets.VadModelPath == "" && nonEmptyFile(filepath.Join(dir, vadModelFile)) {
			assets.VadModelPath = filepath.Join(dir, vadModelFile)
		}
	}

	report := func(phase, text string, done, total int64) {
		if emit != nil {
			emit(SetupProgress{Phase: phase, Text: text, Done: done, Total: total})
		}
	}

	if assets.BinaryPath == "" {
		dest := filepath.Join(targetDir, binName)
		report("downloading", "Downloading prime-asr service", 0, 0)
		if err := downloadAsrFile(ctx, []string{asrReleaseURL(asrReleaseAssetName())}, dest, "", emit, "Downloading prime-asr service"); err != nil {
			return AsrAssets{}, err
		}
		if runtime.GOOS != "windows" {
			_ = os.Chmod(dest, 0o755)
		}
		assets.BinaryPath = dest
	}

	if assets.AsrModelPath == "" {
		dest := filepath.Join(targetDir, asrModelFile)
		report("downloading", "Downloading speech model (en-ru-uk)", 0, 0)
		urls := []string{asrReleaseURL(asrModelFile), asrFallbackModelURL(asrModelFile)}
		if err := downloadAsrFile(ctx, urls, dest, asrModelSHA256, emit, "Downloading speech model (en-ru-uk)"); err != nil {
			return AsrAssets{}, err
		}
		assets.AsrModelPath = dest
	}

	if assets.VadModelPath == "" {
		dest := filepath.Join(targetDir, vadModelFile)
		report("downloading", "Downloading VAD model", 0, 0)
		urls := []string{asrReleaseURL(vadModelFile), asrFallbackModelURL(vadModelFile)}
		if err := downloadAsrFile(ctx, urls, dest, vadModelSHA256, emit, "Downloading VAD model"); err != nil {
			return AsrAssets{}, err
		}
		assets.VadModelPath = dest
	}

	report("ready", "Speech model ready", 1, 1)
	return assets, nil
}

func downloadAsrFile(ctx context.Context, urls []string, dest, expectedSHA string, emit func(SetupProgress), label string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	var lastErr error
	for _, url := range urls {
		err := downloadOneAsrFile(ctx, client, url, dest, expectedSHA, emit, label)
		if err == nil {
			return nil
		}
		lastErr = err
	}
	return lastErr
}

func downloadOneAsrFile(ctx context.Context, client *http.Client, url, dest, expectedSHA string, emit func(SetupProgress), label string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("download %s returned HTTP %d", url, resp.StatusCode)
	}

	tmp := dest + ".part"
	out, err := os.Create(tmp)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	total := resp.ContentLength
	var done int64
	buf := make([]byte, 64*1024)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, writeErr := out.Write(buf[:n]); writeErr != nil {
				out.Close()
				_ = os.Remove(tmp)
				return writeErr
			}
			_, _ = hasher.Write(buf[:n])
			done += int64(n)
			if emit != nil {
				emit(SetupProgress{Phase: "downloading", Text: label, Done: done, Total: total})
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			out.Close()
			_ = os.Remove(tmp)
			return readErr
		}
	}
	if err := out.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if expectedSHA != "" {
		actual := hex.EncodeToString(hasher.Sum(nil))
		if !strings.EqualFold(actual, expectedSHA) {
			_ = os.Remove(tmp)
			return fmt.Errorf("checksum mismatch for %s: expected %s, got %s", dest, expectedSHA, actual)
		}
	}
	_ = os.Remove(dest)
	if err := os.Rename(tmp, dest); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
