package voice

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	stdout, stderr, err := runCommandBudget(ctx, args[0], args[1:], nil, whisperTimeout)
	if err != nil {
		return "", engineError(err, stderr)
	}
	return tidyTranscript(stdout), nil
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
func whisperArgs(binary, modelPath, language, wavPath string) []string {
	return []string{binary, "-m", modelPath, "-l", language, "-nt", "-f", wavPath}
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
