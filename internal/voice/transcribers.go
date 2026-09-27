package voice

import (
	"context"
	"os"
	"strings"
	"time"
)

// transcriptionTimeout bounds one transcription request so a wedged endpoint
// cannot hold the dictation flow open forever.
const transcriptionTimeout = 60 * time.Second

// openAIAPIModel is the hosted transcription model used when
// options.voice.model is unset.
const openAIAPIModel = "whisper-1"

// hostedFallbacks are credentials that unlock hosted OpenAI-compatible
// transcription with no local install. They are consulted last because they
// mean sending audio off the machine, which should be a config choice rather
// than a surprise from the environment.
var hostedFallbacks = []struct {
	apiKeyEnv string
	baseURL   string
	model     string
}{
	{apiKeyEnv: "OPENAI_API_KEY", baseURL: "https://api.openai.com/v1", model: openAIAPIModel},
	{apiKeyEnv: "GROQ_API_KEY", baseURL: "https://api.groq.com/openai/v1", model: "whisper-large-v3-turbo"},
}

// newTranscribers returns the transcription engines this machine can use,
// best first: a configured OpenAI-compatible endpoint, then Google Web
// Speech, which answers with nothing installed, then hosted credentials from
// the environment.
func newTranscribers(ctx context.Context, settings Settings, p prober) []Transcriber {
	if settings.TranscribeCommand != "" {
		return []Transcriber{commandTranscriber{
			command: settings.TranscribeCommand,
			lang:    settings.Language,
		}}
	}
	if settings.Engine != "" && settings.Engine != EngineAuto {
		return pinnedTranscribers(ctx, settings, p)
	}

	var transcribers []Transcriber
	if settings.BaseURL != "" {
		transcribers = append(transcribers, newOpenAITranscriber(openAIEndpoint(settings.BaseURL), settings))
	}
	// Google Web Speech is the default engine: no local model, no install,
	// a few seconds of audio go out over HTTPS and the transcript comes
	// back, which beats every local CPU path on speed.
	transcribers = append(transcribers, newGoogleTranscriber(settings))
	for _, fallback := range hostedFallbacks {
		key := os.Getenv(fallback.apiKeyEnv)
		if key == "" {
			continue
		}
		resolved := settings
		resolved.APIKey = key
		if resolved.Model == "" {
			resolved.Model = fallback.model
		}
		transcribers = append(transcribers, newOpenAITranscriber(openAIEndpoint(fallback.baseURL), resolved))
	}
	return transcribers
}

// pinnedTranscribers builds exactly the engine named by options.voice.engine,
// so a machine with several options uses the chosen one.
func pinnedTranscribers(ctx context.Context, settings Settings, p prober) []Transcriber {
	switch settings.Engine {
	case EngineGoogle:
		return []Transcriber{newGoogleTranscriber(settings)}
	case EngineOpenAI:
		base := settings.BaseURL
		if base == "" {
			base = hostedFallbacks[0].baseURL
		}
		return []Transcriber{newOpenAITranscriber(openAIEndpoint(base), settings)}
	case EngineCommand:
		if settings.TranscribeCommand != "" {
			return []Transcriber{commandTranscriber{
				command: settings.TranscribeCommand,
				lang:    settings.Language,
			}}
		}
	}
	return nil
}

// openAIEndpoint turns a configured base URL into the transcriptions route.
func openAIEndpoint(base string) string {
	trimmed := strings.TrimSuffix(strings.TrimSpace(base), "/")
	if !strings.HasSuffix(strings.ToLower(trimmed), "/audio/transcriptions") {
		trimmed += "/audio/transcriptions"
	}
	return trimmed
}
