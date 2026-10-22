package voice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// This file is the catalog and layout half of the local STT engines:
// what can be downloaded, where it lands under the data directory, and
// whether it is already there. The download and extraction machinery
// lives in stt_install.go; the transcribers that consume the results are
// whisper.go and vosk.go.
//
// Everything installs under <dataDir>/voice/stt/<provider>/{bin,models,lib}
// so Remove is a plain delete and a moved data directory carries the
// multi-gigabyte models along with it.

const (
	// SttGoogle is the hosted engine; it has no local files at all.
	SttGoogle = EngineGoogle
	// SttWhisper keys the whisper.cpp provider.
	SttWhisper = EngineWhisper
	// SttVosk keys the Vosk provider.
	SttVosk = EngineVosk
)

// SttProvider is one transcription engine the desktop can offer, with the
// models a user can download for it.
type SttProvider struct {
	ID          string
	Title       string
	Description string
	// Available says whether Prime can install this provider's runtime on
	// this platform at all.
	Available bool
	Models    []SttModelInfo
}

// SttModelInfo is one downloadable model.
type SttModelInfo struct {
	ID          string
	Label       string
	Language    string
	SizeBytes   int64
	Recommended bool
	Installed   bool
}

// SttModel is one catalog entry behind SttModelInfo: where to download it
// and which local file proves it is installed.
type SttModel struct {
	ID          string
	Label       string
	Language    string
	SizeBytes   int64
	Recommended bool
}

// whisperModels are the whisper.cpp GGML models on Hugging Face. Most are
// multilingual, so Language stays empty and the engine decides the language
// at run time from the audio; the ".en" variants are pinned to English and
// say so, which lets the desktop filter the list the same way it filters
// the single-language vosk models.
var whisperModels = []SttModel{
	{ID: "ggml-tiny", Label: "Tiny", SizeBytes: 85_000_000},
	{ID: "ggml-tiny.en", Label: "Tiny (English)", Language: "en", SizeBytes: 85_000_000},
	{ID: "ggml-base", Label: "Base", SizeBytes: 155_000_000},
	{ID: "ggml-base.en", Label: "Base (English)", Language: "en", SizeBytes: 155_000_000},
	{ID: "ggml-base-q5_1", Label: "Base (Q5)", SizeBytes: 63_000_000},
	{ID: "ggml-small", Label: "Small", SizeBytes: 512_000_000},
	{ID: "ggml-small.en", Label: "Small (English)", Language: "en", SizeBytes: 512_000_000},
	{ID: "ggml-small-q5_1", Label: "Small (Q5)", SizeBytes: 200_000_000},
	{ID: "ggml-medium", Label: "Medium", SizeBytes: 1_530_000_000},
	{ID: "ggml-medium.en", Label: "Medium (English)", Language: "en", SizeBytes: 1_530_000_000},
	{ID: "ggml-medium-q5_0", Label: "Medium (Q5)", SizeBytes: 567_000_000},
	{ID: "ggml-large-v3-turbo-q5_0", Label: "Large v3 Turbo (Q5)", SizeBytes: 602_000_000, Recommended: true},
	{ID: "ggml-large-v3-turbo-q8_0", Label: "Large v3 Turbo (Q8)", SizeBytes: 917_000_000},
	{ID: "ggml-large-v3-turbo", Label: "Large v3 Turbo", SizeBytes: 1_740_000_000},
	{ID: "ggml-large-v3-q5_0", Label: "Large v3 (Q5)", SizeBytes: 1_130_000_000},
}

// voskModels are the offline models on alphacephei.com. Each model speaks
// exactly one language, which is why the language is part of the entry. The
// small models are the recommendation: they load in a moment and answer
// fast, which is what dictation needs; the full models are for quality.
var voskModels = []SttModel{
	{ID: "vosk-model-small-ru-0.22", Label: "Russian (small)", Language: "ru", SizeBytes: 47_000_000, Recommended: true},
	{ID: "vosk-model-ru-0.42", Label: "Russian", Language: "ru", SizeBytes: 1_930_000_000},
	{ID: "vosk-model-small-en-us-0.15", Label: "English, US (small)", Language: "en", SizeBytes: 42_000_000, Recommended: true},
	{ID: "vosk-model-en-us-0.22", Label: "English, US", Language: "en", SizeBytes: 1_930_000_000},
	{ID: "vosk-model-small-en-us-zamia-0.5", Label: "English, US (Zamia)", Language: "en", SizeBytes: 49_000_000, Recommended: true},
	{ID: "vosk-model-small-es-0.42", Label: "Spanish (small)", Language: "es", SizeBytes: 39_000_000, Recommended: true},
	{ID: "vosk-model-es-0.42", Label: "Spanish", Language: "es", SizeBytes: 1_400_000_000},
	{ID: "vosk-model-small-cn-0.22", Label: "Chinese (small)", Language: "zh", SizeBytes: 42_000_000, Recommended: true},
	{ID: "vosk-model-cn-0.22", Label: "Chinese", Language: "zh", SizeBytes: 1_300_000_000},
	{ID: "vosk-model-small-fa-0.5", Label: "Persian (small)", Language: "fa", SizeBytes: 60_000_000, Recommended: true},
	{ID: "vosk-model-small-fr-0.22", Label: "French (small)", Language: "fr", SizeBytes: 41_000_000, Recommended: true},
	{ID: "vosk-model-fr-0.22", Label: "French", Language: "fr", SizeBytes: 1_400_000_000},
	{ID: "vosk-model-small-de-0.15", Label: "German (small)", Language: "de", SizeBytes: 45_000_000, Recommended: true},
	{ID: "vosk-model-de-0.21", Label: "German", Language: "de", SizeBytes: 1_900_000_000},
	{ID: "vosk-model-small-pt-0.3", Label: "Portuguese (small)", Language: "pt", SizeBytes: 31_000_000, Recommended: true},
}

const (
	whisperModelURLPrefix = "https://huggingface.co/ggerganov/whisper.cpp/resolve/main/"
	voskModelURLPrefix    = "https://alphacephei.com/vosk/models/"

	// whisperSourceURL is the versioned whisper.cpp source tarball the
	// Vulkan runtime is compiled from: whisper.cpp publishes no prebuilt
	// Vulkan binaries, so building from source with -DGGML_VULKAN=1 is the
	// supported way to get GPU acceleration for any vendor's GPU.
	whisperSourceURL = "https://codeload.github.com/ggml-org/whisper.cpp/tar.gz/refs/tags/v1.9.5"
	// whisperSourceDir is the single root directory inside that tarball.
	whisperSourceDir = "whisper.cpp-1.9.5"
)

// whisperRuntimes are the prebuilt whisper.cpp binaries, one per platform
// and accelerator the project publishes a build for. A CPU build exists
// everywhere the provider is installable; the CUDA build exists where
// whisper.cpp publishes one and is chosen by the accelerator setting. macOS
// has none, so the whisper provider is marked unavailable there and install
// explains the manual path. There is no prebuilt Vulkan build at all: that
// runtime is compiled from source instead (stt_install.go).
var whisperRuntimes = []struct{ goos, goarch, accelerator, url string }{
	{"windows", "amd64", AcceleratorCPU, "https://github.com/ggml-org/whisper.cpp/releases/download/b5454/whisper-bin-x64.zip"},
	{"windows", "amd64", AcceleratorCUDA, "https://github.com/ggml-org/whisper.cpp/releases/download/b5454/whisper-bin-win-cuda-12.4.0-x64.zip"},
	{"linux", "amd64", AcceleratorCPU, "https://github.com/ggml-org/whisper.cpp/releases/download/b5454/whisper-bin-ubuntu-x64.tar.gz"},
}

// voskRuntimes are the prebuilt libvosk shared libraries from vosk-api
// releases. An empty goarch matches every architecture on that platform.
var voskRuntimes = []struct{ goos, goarch, url string }{
	{"linux", "amd64", "https://github.com/alphacep/vosk-api/releases/download/v0.3.45/vosk-linux-x86_64-0.3.45.zip"},
	{"windows", "amd64", "https://github.com/alphacep/vosk-api/releases/download/v0.3.45/vosk-win64-0.3.45.zip"},
	{"darwin", "", "https://github.com/alphacep/vosk-api/releases/download/v0.3.42/vosk-osx-0.3.42.zip"},
}

// sttURLRewrites lets tests point the official download URLs at a local
// server. Keys are URL prefixes; the longest match wins.
var (
	sttURLRewritesMu sync.Mutex
	sttURLRewrites   = map[string]string{}
)

// setSttURLRewrites swaps the URL prefix table. For tests only.
func setSttURLRewrites(rewrites map[string]string) {
	sttURLRewritesMu.Lock()
	defer sttURLRewritesMu.Unlock()
	sttURLRewrites = rewrites
}

// SetSttDownloadRewritesForTest points the official download URLs at a
// local mirror for tests outside this package. An empty map restores the
// official URLs.
func SetSttDownloadRewritesForTest(rewrites map[string]string) {
	setSttURLRewrites(rewrites)
}

// sttDownloadURL rewrites an official URL through the test table, keeping
// production untouched when no prefix matches.
func sttDownloadURL(official string) string {
	sttURLRewritesMu.Lock()
	defer sttURLRewritesMu.Unlock()
	best, base := "", ""
	for prefix, target := range sttURLRewrites {
		if strings.HasPrefix(official, prefix) && len(prefix) > len(best) {
			best, base = prefix, target
		}
	}
	if best == "" {
		return official
	}
	return base + official[len(best):]
}

// sttRoot is where every local STT runtime and model lives.
func sttRoot(dataDir string) string {
	return filepath.Join(dataDir, "voice", "stt")
}

// whisperBinDir holds the extracted whisper.cpp command-line binary.
func whisperBinDir(dataDir string) string {
	return filepath.Join(sttRoot(dataDir), "whisper", "bin")
}

// whisperModelPath is the GGML file of one model. The catalog ID is also
// the on-disk file name, which keeps installed checks trivial.
func whisperModelPath(dataDir, model string) string {
	return filepath.Join(sttRoot(dataDir), "whisper", "models", model+".bin")
}

// voskLibDir holds the extracted libvosk shared library.
func voskLibDir(dataDir string) string {
	return filepath.Join(sttRoot(dataDir), "vosk", "lib")
}

// voskLibPath names the shared library file for this platform.
func voskLibPath(dataDir string) string {
	name := "libvosk.so"
	switch runtime.GOOS {
	case "windows":
		name = "vosk.dll"
	case "darwin":
		name = "libvosk.dylib"
	}
	return filepath.Join(voskLibDir(dataDir), name)
}

// voskModelDir is the extracted model folder. The zips ship a top-level
// folder named after the model, so extraction preserves that layout.
func voskModelDir(dataDir, model string) string {
	return filepath.Join(sttRoot(dataDir), "vosk", "models", model)
}

// sttDataDir returns a usable data directory. Settings built outside the
// full config flow may carry none; ".prime" in the working directory is
// the same fallback the config loader applies.
func sttDataDir(settings Settings) string {
	if settings.DataDir != "" {
		return settings.DataDir
	}
	return ".prime"
}

// whisperRuntime returns the runtime download URL and the accelerator the
// binary is built for, on this machine. A requested accelerator with no
// published variant falls back to the CPU build: the CPU binary runs
// everywhere, it just leaves the GPU idle, which is a working engine rather
// than an error the user has to interpret.
func whisperRuntime(accelerator string) (url, variant string, ok bool) {
	for _, candidate := range whisperRuntimes {
		if candidate.goos == runtime.GOOS && candidate.goarch == runtime.GOARCH &&
			candidate.accelerator == accelerator {
			return candidate.url, candidate.accelerator, true
		}
	}
	for _, candidate := range whisperRuntimes {
		if candidate.goos == runtime.GOOS && candidate.goarch == runtime.GOARCH &&
			candidate.accelerator == AcceleratorCPU {
			return candidate.url, AcceleratorCPU, true
		}
	}
	return "", "", false
}

// WhisperAccelerators lists the accelerators a prebuilt whisper.cpp runtime
// is published for on this platform, in catalog order, regardless of whether
// the machine actually has usable hardware. The CPU build always exists;
// CUDA appears only where a prebuilt CUDA binary is published. The
// hardware-aware offer list lives in WhisperAcceleratorOptions.
func WhisperAccelerators() []string {
	offers := []string{AcceleratorCPU}
	if _, ok := whisperRuntimeVariant(AcceleratorCUDA); ok {
		offers = append(offers, AcceleratorCUDA)
	}
	return offers
}

// whisperRuntimeVariant returns the runtime URL published for exactly the
// requested accelerator, with no fallback.
func whisperRuntimeVariant(accelerator string) (string, bool) {
	for _, candidate := range whisperRuntimes {
		if candidate.goos == runtime.GOOS && candidate.goarch == runtime.GOARCH &&
			candidate.accelerator == accelerator {
			return candidate.url, true
		}
	}
	return "", false
}

// whisperAcceleratorMarker names the file in the whisper bin directory that
// records which build is unpacked there.
func whisperAcceleratorMarker(dataDir string) string {
	return filepath.Join(whisperBinDir(dataDir), ".accelerator")
}

// installedWhisperAccelerator reads the accelerator marker. A missing file
// means CPU: the marker arrived with CUDA support, so every runtime unpacked
// before it exists is necessarily the CPU build. A marker naming an
// accelerator the engine knows is taken literally; anything else is old
// output treated as CPU.
func installedWhisperAccelerator(dataDir string) string {
	data, err := os.ReadFile(whisperAcceleratorMarker(dataDir))
	if err != nil {
		return AcceleratorCPU
	}
	switch variant := strings.TrimSpace(string(data)); variant {
	case AcceleratorCUDA, AcceleratorVulkan:
		return variant
	}
	return AcceleratorCPU
}

// writeWhisperAcceleratorMarker records which whisper.cpp build is unpacked
// in the bin directory, so a later install with a different accelerator
// knows the runtime has to be replaced instead of reused.
func writeWhisperAcceleratorMarker(dataDir, accelerator string) error {
	if err := os.MkdirAll(whisperBinDir(dataDir), 0o755); err != nil {
		return err
	}
	return os.WriteFile(whisperAcceleratorMarker(dataDir), []byte(accelerator), 0o644)
}

// voskRuntime returns the libvosk download URL for this machine.
func voskRuntime() (string, bool) {
	for _, candidate := range voskRuntimes {
		if candidate.goos == runtime.GOOS && (candidate.goarch == "" || candidate.goarch == runtime.GOARCH) {
			return candidate.url, true
		}
	}
	return "", false
}

// findWhisperBinary locates the extracted command-line executable. The
// archives differ in layout and in whether the entry is called
// whisper-cli or main, so the directory is searched instead of guessing.
func findWhisperBinary(dir string) string {
	var found, fallback string
	_ = filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil //nolint:nilerr // a missing directory just means "not installed"
		}
		base := strings.TrimSuffix(entry.Name(), ".exe")
		switch {
		case strings.HasPrefix(base, "whisper-cli"):
			if found == "" {
				found = path
			}
		case base == "main":
			if fallback == "" {
				fallback = path
			}
		}
		return nil
	})
	if found != "" {
		return found
	}
	return fallback
}

// ErrSttUnknownModel reports a provider/model pair that is not in the
// catalog, which is a caller bug rather than a machine state.
var ErrSttUnknownModel = errors.New("unknown STT provider or model")

// SttCatalog lists the transcription engines Prime can offer, with per-model
// install state read from the data directory.
func SttCatalog(dataDir string) []SttProvider {
	_, _, whisperInstallable := whisperRuntime(AcceleratorCPU)
	_, voskInstallable := voskRuntime()
	// A runtime installed by hand counts too: availability is about whether
	// the machine can run the engine, not only about Prime's own downloads.
	whisperAvailable := whisperInstallable || findWhisperBinary(whisperBinDir(dataDir)) != ""
	voskAvailable := voskInstallable

	return []SttProvider{
		{
			ID:          SttGoogle,
			Title:       "Google STT",
			Description: "Online Google Web Speech transcription, no models to download.",
			Available:   true,
			Models:      []SttModelInfo{},
		},
		{
			ID:          SttWhisper,
			Title:       "Whisper STT",
			Description: "Local offline transcription with whisper.cpp.",
			Available:   whisperAvailable,
			Models: sttModelInfos(whisperModels, whisperInstallable, func(model string) bool {
				return fileExists(whisperModelPath(dataDir, model))
			}),
		},
		{
			ID:          SttVosk,
			Title:       "Vosk STT",
			Description: "Local offline transcription with Vosk.",
			Available:   voskAvailable,
			Models: sttModelInfos(voskModels, voskInstallable, func(model string) bool {
				return dirExists(voskModelDir(dataDir, model))
			}),
		},
	}
}

// sttModelInfos copies catalog entries into the public shape, attaching
// per-model installed state.
func sttModelInfos(models []SttModel, available bool, installed func(string) bool) []SttModelInfo {
	infos := make([]SttModelInfo, 0, len(models))
	for _, model := range models {
		infos = append(infos, SttModelInfo{
			ID:          model.ID,
			Label:       model.Label,
			Language:    model.Language,
			SizeBytes:   model.SizeBytes,
			Recommended: model.Recommended,
			// A model cannot be installed when the runtime cannot be:
			// offering it would just end in a confusing error.
			Installed: available && installed(model.ID),
		})
	}
	return infos
}

// SttIsInstalled reports whether a catalog model is already on disk.
// The hosted engine is always installed.
func SttIsInstalled(dataDir, provider, model string) bool {
	switch provider {
	case SttGoogle:
		return true
	case SttWhisper:
		return fileExists(whisperModelPath(dataDir, model))
	case SttVosk:
		return dirExists(voskModelDir(dataDir, model))
	}
	return false
}

// SttInstall downloads and unpacks one catalog model, and the provider's
// runtime first when it is missing. It is idempotent and never runs inside
// Transcribe: installation is a user decision with visible progress. The
// accelerator is the compute device the whisper runtime is installed for
// ("cuda", "vulkan", or anything else for CPU); it is read once by the
// caller so a config change mid-install cannot re-target a download already
// running.
func SttInstall(ctx context.Context, dataDir, provider, model, accelerator string, emit func(SetupProgress)) error {
	if emit == nil {
		emit = func(SetupProgress) {}
	}
	switch provider {
	case SttWhisper:
		return installWhisperModel(ctx, dataDir, model, accelerator, emit)
	case SttVosk:
		return installVoskModel(ctx, dataDir, model, emit)
	case SttGoogle:
		return fmt.Errorf("Google STT has nothing to install")
	default:
		return fmt.Errorf("%w: %s", ErrSttUnknownModel, provider)
	}
}

// SttRemove deletes the files of one installed model. Runtimes stay: they
// are shared by every model of the provider and cost one download.
func SttRemove(dataDir, provider, model string) error {
	switch provider {
	case SttWhisper:
		return os.RemoveAll(whisperModelPath(dataDir, model))
	case SttVosk:
		return os.RemoveAll(voskModelDir(dataDir, model))
	case SttGoogle:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrSttUnknownModel, provider)
	}
}

// fileExists and dirExists keep the installed checks readable.
func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
