package voice

import "sync"

// detectedLanguage remembers the language a whisper.cpp server last answered
// with while Prime was running with language auto-detect. whisper.cpp reuses
// its loaded model per request, but language detection still costs a full
// encoding pass on short dictations; pinning the once-detected language for
// later requests skips that pass entirely.
var detectedLanguage struct {
	mu   sync.Mutex
	lang string
}

func rememberDetectedLanguage(lang string) {
	if lang == "" || lang == "auto" {
		return
	}
	detectedLanguage.mu.Lock()
	detectedLanguage.lang = lang
	detectedLanguage.mu.Unlock()
}

func recallDetectedLanguage() string {
	detectedLanguage.mu.Lock()
	defer detectedLanguage.mu.Unlock()
	return detectedLanguage.lang
}

// clearDetectedLanguage forgets the cached language, for tests and for a
// server swap that may change the model's language coverage.
func clearDetectedLanguage() {
	detectedLanguage.mu.Lock()
	detectedLanguage.lang = ""
	detectedLanguage.mu.Unlock()
}
