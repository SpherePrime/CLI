package voice

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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

// defaultGoogleLanguages are the locales tried, in order, when
// options.voice.language names no language of its own.
//
// This route has no detect mode: it answers 400 "Missing parameter: lang" when
// the tag is absent, and it answers 400 to "auto" while quietly serving English
// anyway. So detection here means asking, and every request must name one
// language. Two well-separated languages are enough to tell a match from a
// foreign model: on Russian speech the Russian model answers Cyrillic at 0.86
// while the English one answers Latin gibberish at 0.48, and on English speech
// the English model answers 0.97 against the Russian model's 0.93 of Latin.
//
// Closely related languages are the known weak spot and the reason the list is
// short: on Russian speech a German model returns 0.91 and a Ukrainian one
// 0.94, both above the correct language, so a list mixing neighbours turns
// confidence into a coin flip. Set options.voice.language to widen it.
//
// The tags are lowercased because that is the form every configured tag is
// normalized into, and the endpoint serves both spellings alike.
var defaultGoogleLanguages = []string{"ru-ru", "en-us"}

// confidentEnough is the bar a transcript has to clear to end the search
// without asking the remaining candidates, and it only applies once the
// alphabet already agreed. Measured on the live endpoint, a wrong model on
// foreign speech runs from 0.47 to 0.93, so confidence alone decides nothing.
const confidentEnough = 0.6

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

// candidates lists the languages to try, in order. A configured tag is a fixed
// choice and costs one request; an unset option (or the "auto" that means the
// same thing for engines which can detect) means a choice has to be made from
// the answers, which costs a request per candidate until one is written in the
// alphabet its own language uses.
func (t googleTranscriber) candidates() []string {
	if configured := languageCandidates(t.settings.Language); len(configured) > 0 {
		return configured
	}
	return defaultGoogleLanguages
}

// attempt is one candidate language's answer.
type attempt struct {
	language   string
	text       string
	confidence float64
	// onScript records that the transcript is written in the alphabet the
	// requested language uses, which is the only signal that separates the
	// model that understood the speech from one that guessed.
	onScript bool
}

// better reports whether a should replace b. The alphabet decides first and
// confidence only breaks the tie, because a wrong model on this endpoint is
// often the more confident of the two.
func better(a, b attempt) bool {
	if b.text == "" {
		return true
	}
	if a.onScript != b.onScript {
		return a.onScript
	}
	return a.confidence > b.confidence
}

// Transcribe uploads the raw PCM of the recording to the Web Speech endpoint
// and returns the transcript of whichever language fits it best.
func (t googleTranscriber) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	pcm, rate, err := barePCM(audio)
	if err != nil {
		return "", err
	}

	key := strings.TrimSpace(t.settings.resolveAPIKey())
	if key == "" {
		key = googleChromiumKey
	}

	candidates := t.candidates()
	var best attempt
	for _, language := range candidates {
		got, err := t.recognize(ctx, language, pcm, rate, key)
		if err != nil {
			return "", err
		}
		if better(got, best) {
			best = got
		}
		if best.onScript && best.confidence >= confidentEnough {
			break
		}
	}

	if best.text == "" {
		// A 200 with no alternatives is not a transport failure: the request
		// landed and Google had nothing to give back, which is what silence or
		// a clip too short to recognize looks like from in here. Reporting that
		// as a failed transcription names a Google internal the user cannot act
		// on, so it comes back as an empty result and the UI says in words that
		// nothing was heard.
		slog.Debug("Google returned no transcript",
			"duration", audio.Duration, "languages", candidates)
		return "", nil
	}
	slog.Debug("Google transcript",
		"language", best.language, "confidence", best.confidence,
		"on_script", best.onScript)
	return tidyTranscript(best.text), nil
}

// barePCM strips the RIFF header the recorder writes, because the Web Speech
// endpoint wants raw 16-bit little-endian samples.
func barePCM(audio *Audio) ([]byte, int, error) {
	if audio == nil || len(audio.WAV) == 0 {
		return nil, 0, ErrNoSpeech
	}
	pcm := audio.WAV
	rate := SampleRate
	if len(pcm) >= 44 && bytes.Equal(pcm[:4], []byte("RIFF")) {
		rate = int(binary.LittleEndian.Uint32(pcm[24:28]))
		pcm = pcm[44:]
	}
	if len(pcm) == 0 {
		return nil, 0, ErrNoSpeech
	}
	return pcm, rate, nil
}

// recognize asks the endpoint about one language and returns its best
// alternative. The request always names that language, which is what keeps the
// endpoint from answering 400 "Missing parameter: lang".
func (t googleTranscriber) recognize(
	ctx context.Context, language string, pcm []byte, rate int, key string,
) (attempt, error) {
	params := url.Values{
		"client":  {"chromium"},
		"pfilter": {"0"},
		"lang":    {language},
	}
	params.Set("key", key)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		t.target()+"?"+params.Encode(), bytes.NewReader(pcm))
	if err != nil {
		return attempt{}, fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	req.Header.Set("Content-Type", fmt.Sprintf("audio/l16; rate=%d; endian=little", rate))

	res, err := googleSTTClient.Do(req)
	if err != nil {
		return attempt{}, fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	defer func() { _ = res.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(res.Body, commandOutputLimit))
	if err != nil {
		return attempt{}, fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return attempt{}, fmt.Errorf("%w: google speech returned %s for %s: %s",
			ErrTranscribeFailed, res.Status, language, strings.TrimSpace(string(body)))
	}

	// The endpoint answers with newline-delimited JSON; every result line
	// carries alternatives, so keep the single most confident transcript.
	var text string
	var confidence float64
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
				if alt.Transcript != "" && alt.Confidence > confidence {
					text, confidence = alt.Transcript, alt.Confidence
				}
			}
		}
	}
	return attempt{
		language:   language,
		text:       text,
		confidence: confidence,
		onScript:   onExpectedScript(language, text),
	}, nil
}
