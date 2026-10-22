package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	purego "github.com/SpherePrime/CLI/vendordeps/ebitengine/purego"
)

// This file binds libvosk through purego, which dlopens the shared library
// at run time without cgo, so the CGO_ENABLED=0 build stays intact. The
// symbols are the ones vosk_api.h exports; anything missing is reported as
// a setup problem, never a crash.

// voskAPI holds the dlopened library handle and the registered C functions.
var voskAPI struct {
	mu         sync.Mutex
	loadedPath string

	setLogLevel      func(level int32)
	createModel      func(path string) uintptr
	freeModel        func(model uintptr)
	createRecognizer func(model uintptr, sampleRate float32) uintptr
	acceptWaveform   func(recognizer uintptr, data []byte, length int32) int32
	finalResult      func(recognizer uintptr) string
	freeRecognizer   func(recognizer uintptr)
}

var voskLoadedModels = newVoskModelCache(func(path string) uintptr {
	voskAPI.mu.Lock()
	defer voskAPI.mu.Unlock()
	return voskAPI.createModel(path)
}, func(handle uintptr) {
	voskAPI.mu.Lock()
	defer voskAPI.mu.Unlock()
	voskAPI.freeModel(handle)
}, 10*time.Minute)

// voskTranscriber transcribes with a local Vosk model through libvosk. The
// model decides the language, so the configured language setting plays no
// role here.
type voskTranscriber struct {
	settings Settings
}

var _ Transcriber = voskTranscriber{}

func newVoskTranscriber(settings Settings) Transcriber {
	return voskTranscriber{settings: settings}
}

func (t voskTranscriber) Name() string { return "vosk" }

// Transcribe feeds the whole recording to a fresh recognizer and reads the
// final result. A recognizer is cheap; the model underneath is not, which
// is why only the recognizer is created per call.
func (t voskTranscriber) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	if audio == nil || len(audio.WAV) == 0 {
		return "", ErrNoSpeech
	}
	dataDir := sttDataDir(t.settings)
	modelPath := voskModelDir(dataDir, t.settings.Model)
	if t.settings.Model == "" || !dirExists(modelPath) {
		return "", fmt.Errorf("%w: vosk model not installed; download it in the speech recognition settings",
			ErrTranscribeFailed)
	}
	if err := ensureVoskAPI(dataDir); err != nil {
		return "", err
	}

	model, releaseModel, err := voskLoadedModels.acquire(modelPath)
	if err != nil {
		return "", err
	}
	defer releaseModel()

	pcm, err := decodePCM(audio.WAV)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if len(pcm) == 0 {
		return "", ErrNoSpeech
	}

	voskAPI.mu.Lock()
	recognizer := voskAPI.createRecognizer(model, SampleRate)
	finalResult, acceptWaveform, freeRecognizer := voskAPI.finalResult, voskAPI.acceptWaveform, voskAPI.freeRecognizer
	voskAPI.mu.Unlock()
	if recognizer == 0 {
		return "", fmt.Errorf("%w: vosk recognizer could not be created", ErrTranscribeFailed)
	}
	defer freeRecognizer(recognizer)

	if acceptWaveform(recognizer, pcm, int32(len(pcm))) < 0 {
		return "", fmt.Errorf("%w: vosk rejected the audio", ErrTranscribeFailed)
	}
	return decodeVoskResult([]byte(finalResult(recognizer))), nil
}

// decodeVoskResult pulls the transcript out of Vosk's result JSON, which is
// {"text": "..."}. An unparseable answer yields no text rather than an
// error: Vosk emits partial-state JSON in some flows and a blank result is
// the honest reading of it.
func decodeVoskResult(payload []byte) string {
	var result struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(payload), &result); err != nil {
		return ""
	}
	return strings.TrimSpace(result.Text)
}

// ensureVoskAPI dlopens the installed libvosk once and binds the functions
// the transcriber needs. Subsequent calls are no-ops.
func ensureVoskAPI(dataDir string) error {
	libPath := voskLibPath(dataDir)
	if !fileExists(libPath) {
		return fmt.Errorf("%w: vosk runtime not installed; download a vosk model in the speech recognition settings",
			ErrTranscribeFailed)
	}

	voskAPI.mu.Lock()
	defer voskAPI.mu.Unlock()
	if voskAPI.loadedPath == libPath {
		return nil
	}
	handle, err := loadDynamicLibrary(libPath)
	if err != nil {
		return fmt.Errorf("%w: loading libvosk failed: %v", ErrTranscribeFailed, err)
	}
	// RegisterLibFunc panics on a missing symbol, so the required ones are
	// probed first; a wrong or truncated library must fail with a message
	// rather than take the process down.
	for _, symbol := range []string{
		"vosk_set_log_level",
		"vosk_model_new",
		"vosk_model_free",
		"vosk_recognizer_new",
		"vosk_recognizer_accept_waveform",
		"vosk_recognizer_final_result",
		"vosk_recognizer_free",
	} {
		if _, err := findSymbol(handle, symbol); err != nil {
			return fmt.Errorf("%w: libvosk does not export %s: %v", ErrTranscribeFailed, symbol, err)
		}
	}

	purego.RegisterLibFunc(&voskAPI.setLogLevel, handle, "vosk_set_log_level")
	purego.RegisterLibFunc(&voskAPI.createModel, handle, "vosk_model_new")
	purego.RegisterLibFunc(&voskAPI.freeModel, handle, "vosk_model_free")
	purego.RegisterLibFunc(&voskAPI.createRecognizer, handle, "vosk_recognizer_new")
	purego.RegisterLibFunc(&voskAPI.acceptWaveform, handle, "vosk_recognizer_accept_waveform")
	purego.RegisterLibFunc(&voskAPI.finalResult, handle, "vosk_recognizer_final_result")
	purego.RegisterLibFunc(&voskAPI.freeRecognizer, handle, "vosk_recognizer_free")

	// Kaldi's info logs go to stderr, which the terminal owns; silence them.
	voskAPI.setLogLevel(-1)
	voskAPI.loadedPath = libPath
	return nil
}
