package voice

import (
	"context"
	"net/url"
	"os"
	"strings"
	"time"
)

// transcriptionTimeout bounds one Whisper run so a wedged engine cannot hold
// the dictation flow open forever.
const transcriptionTimeout = 2 * time.Minute

// engineProbeTimeout bounds the reachability check of a local Whisper server.
const engineProbeTimeout = 700 * time.Millisecond

// openAIAPIModel is the hosted transcription model used when
// options.voice.model is unset.
const openAIAPIModel = "whisper-1"

// hostedFallbacks are credentials that unlock hosted Whisper with no local
// install. They are consulted last because hosted transcription means sending
// audio off the machine, which should be a config choice rather than a
// surprise from the environment.
var hostedFallbacks = []struct {
	apiKeyEnv string
	baseURL   string
	model     string
}{
	{apiKeyEnv: "OPENAI_API_KEY", baseURL: "https://api.openai.com/v1", model: openAIAPIModel},
	{apiKeyEnv: "GROQ_API_KEY", baseURL: "https://api.groq.com/openai/v1", model: "whisper-large-v3-turbo"},
}

// newTranscribers returns the Whisper engines this machine can use, best first:
// a running local server, then local command line engines, then a hosted
// OpenAI-compatible endpoint.
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
	// A running server wins because it answers without reloading a model,
	// which is seconds faster per dictation than a cold CLI run.
	serverListening := false
	if transcriber, ok := autoServerTranscriber(ctx, settings, p); ok {
		transcribers = append(transcribers, transcriber)
		serverListening = true
	}
	if transcriber, ok := localWhisperTranscriber(settings, p); ok {
		if !serverListening {
			// With no server up yet the command is the seed the warm
			// wrapper promotes from. When one already answers, the plain
			// command stays listed as the second choice.
			transcriber = keepModelWarm(transcriber, settings, p)
		}
		transcribers = append(transcribers, transcriber)
	}
	if transcriber, ok := hostedTranscriber(settings); ok {
		transcribers = append(transcribers, transcriber)
	}
	return transcribers
}

// pinnedTranscribers builds exactly the engine named by options.voice.engine,
// so a machine with several engines installed uses the chosen one.
func pinnedTranscribers(ctx context.Context, settings Settings, p prober) []Transcriber {
	switch settings.Engine {
	case EngineServer:
		if settings.BaseURL == "" {
			// The managed server is started on demand here too: an engine
			// pinned to "server" means the user wants the resident model.
			_, _ = ensureServer(ctx, settings, p)
			if transcriber, ok := autoServerTranscriber(ctx, settings, p); ok {
				return []Transcriber{transcriber}
			}
			return nil
		}
		endpoint, style := transcriptionEndpoint(settings.BaseURL)
		if style != styleWhisperServer {
			return nil
		}
		return []Transcriber{newServerTranscriber(endpoint, settings)}
	case EngineOpenAI:
		endpoint := settings.BaseURL
		if endpoint == "" {
			endpoint = hostedFallbacks[0].baseURL
		}
		return []Transcriber{newOpenAITranscriber(endpoint, settings)}
	case EngineWhisperCPP, EngineWhisperPy, EngineCTranslate2:
		if transcriber, ok := namedWhisperTranscriber(settings, p, settings.Engine); ok {
			return []Transcriber{transcriber}
		}
	}
	return nil
}

// autoServerTranscriber uses a configured endpoint, otherwise probes for a
// whisper.cpp server listening locally.
func autoServerTranscriber(ctx context.Context, settings Settings, p prober) (Transcriber, bool) {
	if settings.BaseURL != "" {
		endpoint, style := transcriptionEndpoint(settings.BaseURL)
		transcriber := newServerTranscriber(endpoint, settings)
		if style == styleOpenAI {
			transcriber = newOpenAITranscriber(endpoint, settings)
		}
		return transcriber, true
	}
	if p.reachable == nil {
		return nil, false
	}
	ctx, cancel := context.WithTimeout(ctx, engineProbeTimeout)
	defer cancel()
	if !p.reachable(ctx, localWhisperServer) {
		return nil, false
	}
	return newServerTranscriber(inferenceEndpoint(localWhisperServer), settings), true
}

// localWhisperTranscriber finds a command line Whisper engine on PATH,
// which must have a model to use it.
func localWhisperTranscriber(settings Settings, p prober) (Transcriber, bool) {
	return namedWhisperTranscriber(settings, p, EngineAuto)
}

// namedWhisperTranscriber resolves one engine family, or the best available
// one when engine is EngineAuto.
func namedWhisperTranscriber(settings Settings, p prober, engine string) (Transcriber, bool) {
	if engine == EngineWhisperCPP || engine == EngineAuto {
		if transcriber, ok := whisperCPPFromPath(settings, p); ok {
			return transcriber, true
		}
	}
	if engine == EngineCTranslate2 || engine == EngineAuto {
		if bin, ok := p.lookPath(translate2Binary); ok {
			return ctranslate2Transcriber(bin, settings), true
		}
	}
	if engine == EngineWhisperPy || engine == EngineAuto {
		if bin, ok := p.lookPath(pythonWhisperBinary); ok {
			return pythonWhisperTranscriber(bin, settings), true
		}
	}
	return nil, false
}

// Command line names the engines ship under.
const (
	whisperCPPBinary       = "whisper-cli"
	whisperCPPLegacyBinary = "whisper-cpp"
	pythonWhisperBinary    = "whisper"
	translate2Binary       = "whisper-ctranslate2"
)

// inferenceRoute is whisper.cpp server's transcription endpoint, which is how
// a configured URL is recognized as that server rather than an
// OpenAI-compatible one.
const inferenceRoute = "/inference"

// whisperCPPFromPath resolves whisper.cpp, which is the only engine addressed
// by a model file instead of a model name. When no model is configured the
// one Prime downloaded for it is used.
func whisperCPPFromPath(settings Settings, p prober) (Transcriber, bool) {
	var bin string
	for _, candidate := range []string{whisperCPPBinary, whisperCPPLegacyBinary} {
		if path, ok := p.lookPath(candidate); ok {
			bin = path
			break
		}
	}
	if bin == "" {
		return nil, false
	}
	model, ok := ggmlModelFile(settings.Model, p.layout.modelSearchDirs())
	if !ok {
		return nil, false
	}
	resolved := settings
	resolved.Model = model
	return whisperCPPTranscriber(bin, model, resolved, p.layout.libraryPathEnv()), true
}

// hostedTranscriber falls back to a hosted endpoint when credentials are
// already configured or present in the environment.
func hostedTranscriber(settings Settings) (Transcriber, bool) {
	if key := settings.resolveAPIKey(); key != "" {
		resolved := settings
		resolved.APIKey = key
		base := settings.BaseURL
		if base == "" {
			base = hostedFallbacks[0].baseURL
		}
		endpoint, _ := transcriptionEndpoint(base)
		return newOpenAITranscriber(endpoint, resolved), true
	}
	for _, fallback := range hostedFallbacks {
		key := os.Getenv(fallback.apiKeyEnv)
		if key == "" {
			continue
		}
		resolved := settings
		resolved.APIKey = key
		resolved.Model = fallback.model
		endpoint, _ := transcriptionEndpoint(fallback.baseURL)
		return newOpenAITranscriber(endpoint, resolved), true
	}
	return nil, false
}

// httpStyle distinguishes the two JSON shapes a Whisper HTTP endpoint can
// speak.
type httpStyle int

const (
	// styleOpenAI is POST /audio/transcriptions with a model field.
	styleOpenAI httpStyle = iota
	// styleWhisperServer is whisper.cpp's own POST /inference.
	styleWhisperServer
)

// transcriptionEndpoint turns a configured base URL into the request endpoint
// and picks the shape it speaks. A localhost URL without an explicit route is
// assumed to be a whisper.cpp server, which is how that project documents
// running it.
func transcriptionEndpoint(base string) (string, httpStyle) {
	trimmed := strings.TrimSuffix(base, "/")
	lower := strings.ToLower(trimmed)
	switch {
	case strings.HasSuffix(lower, inferenceRoute):
		return trimmed, styleWhisperServer
	case strings.HasSuffix(lower, "/audio/transcriptions"):
		return trimmed, styleOpenAI
	case strings.HasPrefix(lower, "http://localhost") || strings.HasPrefix(lower, "http://127.0.0.1"):
		return trimmed + inferenceRoute, styleWhisperServer
	default:
		return trimmed + "/audio/transcriptions", styleOpenAI
	}
}

// inferenceEndpoint derives whisper.cpp's inference route from its health
// route, so the auto-probe can reuse a single URL constant.
func inferenceEndpoint(healthURL string) string {
	parsed, err := url.Parse(healthURL)
	if err != nil {
		return strings.TrimSuffix(healthURL, "/health") + inferenceRoute
	}
	parsed.Path = inferenceRoute
	parsed.RawQuery = ""
	return parsed.String()
}
