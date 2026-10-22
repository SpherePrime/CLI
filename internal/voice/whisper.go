package voice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

// whisperTimeout bounds one whisper.cpp run. Local CPU inference of a
// two-minute recording on the larger models takes a while, so this is well
// above the hosted-engine budget.
const whisperTimeout = 120 * time.Second

// whisperTranscriber runs the whisper.cpp command-line binary that the
// install flow unpacked into the data directory, over the GGML model the
// user chose. Nothing leaves the machine.
type whisperTranscriber struct {
	settings Settings
}

var _ Transcriber = whisperTranscriber{}

func newWhisperTranscriber(settings Settings) Transcriber {
	return whisperTranscriber{settings: settings}
}

func (t whisperTranscriber) Name() string { return "whisper" }

// Transcribe writes the recording to a temp WAV file and asks whisper-cli
// for plain text. Missing pieces are reported as setup problems, because
// that is what they are: the model download lives in the speech settings.
func (t whisperTranscriber) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	if audio == nil || len(audio.WAV) == 0 {
		return "", ErrNoSpeech
	}
	dataDir := sttDataDir(t.settings)
	modelPath := whisperModelPath(dataDir, t.settings.Model)
	if t.settings.Model == "" || !fileExists(modelPath) {
		return "", fmt.Errorf("%w: whisper model not installed; download it in the speech recognition settings",
			ErrTranscribeFailed)
	}
	binary := findWhisperBinary(whisperBinDir(dataDir))
	if binary == "" {
		return "", fmt.Errorf("%w: whisper.cpp runtime not installed; download a whisper model in the speech recognition settings",
			ErrTranscribeFailed)
	}

	wavPath, err := writeTempWAV(audio)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.Remove(wavPath) }()

	args := whisperArgs(binary, modelPath, whisperLanguage(t.settings.Language), wavPath)
	libDir := whisperBinDir(sttDataDir(t.settings))
	// Transcription runs below normal priority like the build does: even a
	// capped engine can sit on several cores for a while, and the desktop
	// the user is working in keeps priority.
	command := lowPriorityCommand(args)
	stdout, stderr, err := runCommandBudget(ctx, command[0], command[1:], whisperEnv(t.settings, libDir), whisperTimeout)
	if err != nil {
		return "", engineError(err, stderr)
	}
	// The GPU accelerator is a promise the engine must keep: whisper.cpp
	// silently falls back to the CPU when the backend fails to initialize,
	// and the user picked the accelerator to spare that CPU. The backend
	// announces itself on stderr at startup, so its absence is a failed
	// promise worth reporting instead of a slow hot surprise. The success
	// banner ("ggml_vulkan: Found N Vulkan devices:") is matched specifically,
	// because the failure complaints also name Vulkan.
	if t.settings.Accelerator == AcceleratorVulkan && !strings.Contains(stderr, "ggml_vulkan: Found") {
		return "", fmt.Errorf("%w: the Vulkan backend did not initialize, transcription would have run on the CPU; check the GPU drivers", ErrTranscribeFailed)
	}
	return tidyTranscript(stdout), nil
}

// whisperEnv builds the environment for one whisper.cpp run. A CUDA run may
// be pinned to one GPU through CUDA_VISIBLE_DEVICES, a Vulkan run through
// GGML_VK_VISIBLE_DEVICES. The library directory is always added to the
// loader path on unix, because an installed runtime cannot rely on the
// build tree's runpath surviving the copy. CPU runs and GPU runs without a
// chosen device still get the loader path; only the device variable is
// omitted.
func whisperEnv(settings Settings, libDir string) []string {
	var extra []string
	device := strings.TrimSpace(settings.GPUDevice)
	switch settings.Accelerator {
	case AcceleratorCUDA:
		if device != "" {
			extra = append(extra, "CUDA_VISIBLE_DEVICES="+device)
		}
	case AcceleratorVulkan:
		if device != "" {
			extra = append(extra, "GGML_VK_VISIBLE_DEVICES="+device)
		}
	}
	return mergeEnv(append(loaderPath(libDir), extra...))
}

// loaderPath points the dynamic loader at the runtime's own directory, with
// any existing path preserved so system locations stay reachable.
func loaderPath(libDir string) []string {
	if runtime.GOOS == "windows" {
		return nil // the exe directory is searched automatically
	}
	name := "LD_LIBRARY_PATH"
	if runtime.GOOS == "darwin" {
		name = "DYLD_LIBRARY_PATH"
	}
	if current := os.Getenv(name); current != "" {
		return []string{name + "=" + libDir + string(os.PathListSeparator) + current}
	}
	return []string{name + "=" + libDir}
}

// whisperLanguage picks the language flag value: the first configured
// candidate, or "auto", which lets whisper detect the language itself.
func whisperLanguage(language string) string {
	if candidates := languageCandidates(language); len(candidates) > 0 {
		return candidates[0]
	}
	return "auto"
}

// whisperArgs builds the whisper.cpp command line. -nt suppresses the
// timestamp blocks so stdout is plain dictated text.
// whisperThreads caps the CPU threads whisper.cpp spins up. Without the cap
// the engine grabs every core, and a CPU fallback (or the CPU-side work of a
// GPU run) can overheat the machine the user is working on; four threads
// bound the worst case while staying fast enough.
const whisperThreads = 4

func whisperArgs(binary, modelPath, language, wavPath string) []string {
	threads := min(whisperThreads, runtime.NumCPU())
	return []string{binary, "-m", modelPath, "-l", language, "-t", strconv.Itoa(threads), "-nt", "-f", wavPath}
}

// writeTempWAV parks the recording in a temp file for the engine to read.
func writeTempWAV(audio *Audio) (string, error) {
	file, err := os.CreateTemp("", "prime-voice-whisper-*.wav")
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	path := file.Name()
	if _, err := file.Write(audio.WAV); err != nil {
		_ = file.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	return filepath.Clean(path), nil
}
