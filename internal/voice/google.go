package voice

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// googleChromiumKey is the public key Chromium uses for the free Web Speech
// API when no user key is configured. It is shared, so Google can throttle
// it; users with their own key set options.voice.api_key.
const googleChromiumKey = "AIzaSyBOti4mM-6x9WDnZIjIeyEU21OpBXqWBgw"

// googleSpeechURL is the free Web Speech endpoint, reachable without
// authentication when client=chromium is set.
const googleSpeechURL = "https://www.google.com/speech-api/v2/recognize"

// googleSTTClient is a shared HTTP client for Google Web Speech requests.
var googleSTTClient = &http.Client{Timeout: 30 * time.Second}

// googleTranscriber sends captured audio to Google's free Web Speech API
// (the same endpoint the browser's speech recognition uses). No local model,
// no install: a POST of raw PCM over HTTPS and back comes the transcript.
type googleTranscriber struct {
	settings Settings
	// endpoint defaults to googleSpeechURL; tests point it at a stub.
	endpoint string
}

var _ Transcriber = googleTranscriber{}

// newGoogleTranscriber builds the default engine. It always answers, which is
// why auto-detection lands there even on a machine with nothing installed.
func newGoogleTranscriber(settings Settings) Transcriber {
	return googleTranscriber{settings: settings}
}

// target returns the URL this transcriber POSTs to.
func (t googleTranscriber) target() string {
	if t.endpoint != "" {
		return t.endpoint
	}
	return googleSpeechURL
}

func (t googleTranscriber) Name() string { return "google-stt" }

// Transcribe uploads the raw PCM of the recording to the Web Speech endpoint
// and returns the highest-confidence transcript it got back.
func (t googleTranscriber) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	if audio == nil || len(audio.WAV) == 0 {
		return "", ErrNoSpeech
	}

	// Our recorder hands us a RIFF file; the Web Speech endpoint wants bare
	// 16-bit little-endian PCM, so strip the header.
	pcm := audio.WAV
	rate := SampleRate
	if len(pcm) >= 44 && bytes.Equal(pcm[:4], []byte("RIFF")) {
		rate = int(binary.LittleEndian.Uint32(pcm[24:28]))
		pcm = pcm[44:]
	}
	if len(pcm) == 0 {
		return "", ErrNoSpeech
	}

	key := strings.TrimSpace(t.settings.resolveAPIKey())
	if key == "" {
		key = googleChromiumKey
	}

	params := url.Values{
		"client":  {"chromium"},
		"pfilter": {"0"},
	}
	if lang := strings.TrimSpace(t.settings.Language); lang != "" {
		params.Set("lang", lang)
	}
	params.Set("key", key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		t.target()+"?"+params.Encode(), bytes.NewReader(pcm))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	req.Header.Set("Content-Type", fmt.Sprintf("audio/l16; rate=%d; endian=little", rate))

	res, err := googleSTTClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(res.Body, commandOutputLimit))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", fmt.Errorf("%w: google speech returned %s: %s",
			ErrTranscribeFailed, res.Status, strings.TrimSpace(string(body)))
	}

	// The endpoint answers with newline-delimited JSON; every result line
	// carries alternatives, so keep the single most confident transcript.
	var text string
	var best float64
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var doc struct {
			Result []struct {
				Alternative []struct {
					Transcript string  `json:"transcript"`
					Confidence float64 `json:"confidence"`
				} `json:"alternative"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &doc); err != nil {
			continue
		}
		for _, result := range doc.Result {
			for _, alt := range result.Alternative {
				if alt.Transcript != "" && alt.Confidence > best {
					text, best = alt.Transcript, alt.Confidence
				}
			}
		}
	}
	if text == "" {
		return "", fmt.Errorf("%w: google speech returned no transcript", ErrTranscribeFailed)
	}
	return tidyTranscript(text), nil
}
