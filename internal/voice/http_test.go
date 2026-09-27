package voice

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// testAudio builds a two second recording for engine tests.
func testAudio(t *testing.T) *Audio {
	t.Helper()

	pcm := make([]byte, SampleRate*bytesPerSample*2)
	return &Audio{
		WAV:      encodeWAV(pcm),
		Duration: pcmDuration(pcm),
		Recorder: "test",
	}
}

// formOf parses an uploaded multipart body into plain fields plus the file.
func formOf(t *testing.T, req *http.Request) (map[string]string, []byte, string) {
	t.Helper()

	reader, err := req.MultipartReader()
	require.NoError(t, err)

	fields := map[string]string{}
	var file []byte
	var fileName string
	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		body := make([]byte, 0, 4096)
		buf := make([]byte, 1024)
		for {
			n, readErr := part.Read(buf)
			body = append(body, buf[:n]...)
			if readErr != nil {
				break
			}
		}
		if part.FormName() == "file" {
			file = body
			fileName = part.FileName()
			continue
		}
		fields[part.FormName()] = string(body)
	}
	return fields, file, fileName
}

func TestOpenAITranscriberIsTranscribed(t *testing.T) {
	t.Parallel()

	var got *http.Request
	var fields map[string]string
	var upload []byte
	var fileName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r
		fields, upload, fileName = formOf(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"text":"  привет  мир  "}`))
	}))
	defer server.Close()

	audio := testAudio(t)
	transcriber := newOpenAITranscriber(server.URL+"/audio/transcriptions", Settings{
		Model:    "whisper-1",
		APIKey:   "sk-test",
		Language: "ru",
	})

	text, err := transcriber.Transcribe(context.Background(), audio)
	require.NoError(t, err)
	require.Equal(t, "привет мир", text)

	require.Equal(t, http.MethodPost, got.Method)
	require.Equal(t, "Bearer sk-test", got.Header.Get("Authorization"))
	require.Equal(t, "whisper-1", fields["model"])
	require.Equal(t, "ru", fields["language"])
	require.Equal(t, "dictation.wav", fileName)
	require.Equal(t, audio.WAV, upload)
}

func TestOpenAITranscriberOmitsLanguageForAutoDetect(t *testing.T) {
	t.Parallel()

	var fields map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields, _, _ = formOf(t, r)
		_, _ = w.Write([]byte(`{"text":"ok"}`))
	}))
	defer server.Close()

	transcriber := newOpenAITranscriber(server.URL+"/audio/transcriptions", Settings{APIKey: "k"})
	_, err := transcriber.Transcribe(context.Background(), testAudio(t))
	require.NoError(t, err)
	require.NotContains(t, fields, "language")
}

func TestOpenAITranscriberReportsServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid model"}}`))
	}))
	defer server.Close()

	transcriber := newOpenAITranscriber(server.URL+"/audio/transcriptions", Settings{APIKey: "k"})
	_, err := transcriber.Transcribe(context.Background(), testAudio(t))
	require.ErrorIs(t, err, ErrTranscribeFailed)
	require.Contains(t, err.Error(), "invalid model")
}

func TestOpenAITranscriberNeedsAPIKey(t *testing.T) {
	t.Parallel()

	transcriber := newOpenAITranscriber("http://127.0.0.1:1/audio/transcriptions", Settings{})
	_, err := transcriber.Transcribe(context.Background(), testAudio(t))
	require.ErrorIs(t, err, ErrTranscribeFailed)
	require.Contains(t, err.Error(), "no API key")
}

func TestDecodeTranscriptionJSONAcceptsPlainText(t *testing.T) {
	t.Parallel()

	text, err := decodeTranscriptionJSON([]byte("just text\n"))
	require.NoError(t, err)
	require.Equal(t, "just text", strings.TrimSpace(text))

	_, err = decodeTranscriptionJSON([]byte(`{"error":{"message":"boom"}}`))
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

// newGoogleStubServer answers Web Speech requests with one transcript line
// and reports the query values each request carried.
func newGoogleStubServer(t *testing.T, transcript string, status int) (*httptest.Server, *map[string]string) {
	t.Helper()
	if status == 0 {
		status = http.StatusOK
	}
	queries := map[string]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries["lang"] = r.URL.Query().Get("lang")
		queries["key"] = r.URL.Query().Get("key")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NotEmpty(t, body, "the Web Speech endpoint expects raw PCM, not a multipart form")
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":{"code":3,"message":"api key invalid"}}`))
			return
		}
		_, _ = w.Write([]byte(`{"result":[{"alternative":[{"transcript":"` + transcript + `","confidence":0.9}]}]}` + "\n"))
	}))
	t.Cleanup(server.Close)
	return server, &queries
}

// googleAt builds a googleTranscriber pointed at a test stub.
func googleAt(settings Settings, server *httptest.Server) *googleTranscriber {
	return &googleTranscriber{settings: settings, endpoint: server.URL}
}

func TestGoogleTranscriberParsesTranscript(t *testing.T) {
	t.Parallel()

	server, queries := newGoogleStubServer(t, "привет мир", 0)
	transcriber := googleAt(Settings{Language: "ru"}, server)

	text, err := transcriber.Transcribe(context.Background(), testAudio(t))
	require.NoError(t, err)
	require.Equal(t, "привет мир", text)
	require.Equal(t, "ru", (*queries)["lang"])
	require.Equal(t, googleChromiumKey, (*queries)["key"], "no personal key falls back to the chromium key")
}

func TestGoogleTranscriberUsesPersonalKey(t *testing.T) {
	t.Parallel()

	server, queries := newGoogleStubServer(t, "ok", 0)
	transcriber := googleAt(Settings{APIKey: "personal"}, server)

	_, err := transcriber.Transcribe(context.Background(), testAudio(t))
	require.NoError(t, err)
	require.Equal(t, "personal", (*queries)["key"])
}

// The Web Speech route answers 400 "Missing parameter: lang" instead of
// detecting a language, so every request has to name one.
func TestGoogleTranscriberAlwaysSendsLanguage(t *testing.T) {
	t.Parallel()

	for name, settings := range map[string]Settings{
		"unset":      {},
		"auto":       {Language: "auto"},
		"configured": {Language: "ru-RU"},
		"list":       {Language: "ru-RU,en-US"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server, asked := newGoogleLangServer(t, map[string]googleAnswer{
				"ru-ru": {transcript: "привет", confidence: 0.9},
				"en-us": {transcript: "hello", confidence: 0.9},
			})
			transcriber := googleAt(settings, server)

			_, err := transcriber.Transcribe(context.Background(), testAudio(t))
			require.NoError(t, err)

			require.NotEmpty(t, *asked, "no request was made")
			for _, lang := range *asked {
				require.NotEmpty(t, lang, "the endpoint rejects a request without a lang")
			}

			candidates := languageCandidates(settings.Language)
			if len(candidates) == 0 {
				candidates = defaultGoogleLanguages
			}
			require.Contains(t, candidates, (*asked)[0])

			if sole := soleLanguage(settings.Language); sole != "" {
				// A single named language is a decision, not a search.
				require.Equal(t, []string{sole}, *asked)
			} else {
				// A search starts at the first candidate, because the list is
				// ordered by preference.
				require.Equal(t, candidates[0], (*asked)[0])
			}
		})
	}
}

// A language list is a search order, not a coin toss: on Russian speech the
// Russian model answers Cyrillic while the English one answers Latin gibberish
// with less than half the confidence. Confidence alone would call that a tie,
// so the alphabet is what picks the right one.
func TestGoogleTranscriberPicksLanguageByAlphabet(t *testing.T) {
	t.Parallel()

	// Measured on the live endpoint, Russian speech against English speech.
	answers := map[string]googleAnswer{
		"ru-ru": {transcript: "привет мир это проверка", confidence: 0.86},
		"en-us": {transcript: "previous mirror at the private adjective", confidence: 0.48},
	}
	server, _ := newGoogleLangServer(t, answers)

	text, err := googleAt(Settings{}, server).Transcribe(context.Background(), testAudio(t))
	require.NoError(t, err)
	require.Equal(t, "привет мир это проверка", text)
}

// The other direction: the Russian model on English speech answers Latin at
// 0.93, nearly the 0.97 of the right model, so only the alphabet tells them
// apart and the search has to reach the second candidate.
func TestGoogleTranscriberPicksLanguageByAlphabetOnEnglishAudio(t *testing.T) {
	t.Parallel()

	answers := map[string]googleAnswer{
		"ru-ru": {transcript: "Hello world This is education Test", confidence: 0.93},
		"en-us": {transcript: "hello world this is a dictation test", confidence: 0.97},
	}
	server, asked := newGoogleLangServer(t, answers)

	text, err := googleAt(Settings{}, server).Transcribe(context.Background(), testAudio(t))
	require.NoError(t, err)
	require.Equal(t, "hello world this is a dictation test", text)
	require.Equal(t, defaultGoogleLanguages, *asked, "the search must reach the second language")
}

// A transcript already written in the language's own alphabet and confident
// enough ends the search, which is why the common case costs one request.
func TestGoogleTranscriberStopsAtConfidentMatch(t *testing.T) {
	t.Parallel()

	server, asked := newGoogleLangServer(t, map[string]googleAnswer{
		"ru-ru": {transcript: "привет мир", confidence: 0.86},
		"en-us": {transcript: "hello world", confidence: 0.97},
	})

	text, err := googleAt(Settings{}, server).Transcribe(context.Background(), testAudio(t))
	require.NoError(t, err)
	require.Equal(t, "привет мир", text)
	require.Equal(t, []string{"ru-ru"}, *asked, "a confident own-alphabet answer needs no second request")
}

// The configured list decides the order, so a user who speaks English first
// gets the cheaper single request on English speech.
func TestGoogleTranscriberFollowsConfiguredOrder(t *testing.T) {
	t.Parallel()

	answers := map[string]googleAnswer{
		"ru-ru": {transcript: "привет", confidence: 0.86},
		"en-us": {transcript: "hello", confidence: 0.97},
	}

	for name, testCase := range map[string]struct {
		language string
		want     string
	}{
		"russian first": {language: "ru-RU,en-US", want: "привет"},
		"english first": {language: "en-US,ru-RU", want: "hello"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server, asked := newGoogleLangServer(t, answers)
			text, err := googleAt(Settings{Language: testCase.language}, server).
				Transcribe(context.Background(), testAudio(t))
			require.NoError(t, err)
			require.Equal(t, testCase.want, text)
			require.Len(t, *asked, 1, "the first language already answered in its own alphabet")
		})
	}
}

// Silence answers empty for every candidate, which is not a failure: see
// TestGoogleTranscriberTreatsEmptyResultAsNoSpeech. Nothing can be detected
// out of an empty answer, so every candidate still has to be asked.
func TestGoogleTranscriberSearchOverEmptyAnswers(t *testing.T) {
	t.Parallel()

	asked := &[]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*asked = append(*asked, r.URL.Query().Get("lang"))
		_, _ = w.Write([]byte(`{"result":[]}` + "\n"))
	}))
	t.Cleanup(server.Close)

	text, err := googleAt(Settings{}, server).Transcribe(context.Background(), testAudio(t))
	require.NoError(t, err)
	require.Empty(t, text)
	require.Equal(t, defaultGoogleLanguages, *asked)
}

// googleAnswer is one language's canned reply from the Web Speech stub.
type googleAnswer struct {
	transcript string
	confidence float64
}

// newGoogleLangServer answers each request with the reply registered for the
// language it asked for, and reports the tags that were asked in order.
func newGoogleLangServer(t *testing.T, answers map[string]googleAnswer) (*httptest.Server, *[]string) {
	t.Helper()

	asked := &[]string{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		*asked = append(*asked, lang)

		answer, known := answers[lang]
		if !known {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":{"code":3,"message":"bad language"}}`))
			return
		}
		body, err := json.Marshal(map[string]any{"result": []any{map[string]any{
			"alternative": []any{map[string]any{
				"transcript": answer.transcript,
				"confidence": answer.confidence,
			}},
		}}})
		require.NoError(t, err)
		_, _ = w.Write(append(body, '\n'))
	}))
	t.Cleanup(server.Close)
	return server, asked
}

func TestGoogleTranscriberTreatsEmptyResultAsNoSpeech(t *testing.T) {
	t.Parallel()

	// Google answers 200 with nothing usable in it when it heard nothing it
	// could recognize: silence, or a clip too short to transcribe. The request
	// itself succeeded, so failing it hands the user a Google internal to
	// puzzle over instead of "nothing was heard". Both empty shapes count.
	for name, response := range map[string]string{
		"no results":      `{"result":[]}` + "\n",
		"no alternatives": `{"result":[{"alternative":[]}]}` + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(response))
			}))
			t.Cleanup(server.Close)

			text, err := googleAt(Settings{}, server).Transcribe(context.Background(), testAudio(t))
			require.NoError(t, err)
			require.Empty(t, text)
		})
	}
}

func TestGoogleTranscriberRejectsHTTPError(t *testing.T) {
	t.Parallel()

	server, _ := newGoogleStubServer(t, "", http.StatusForbidden)
	transcriber := googleAt(Settings{}, server)

	_, err := transcriber.Transcribe(context.Background(), testAudio(t))
	require.ErrorIs(t, err, ErrTranscribeFailed)
}

// An empty result is not a failure: Google answering 200 with nothing usable
// means it heard nothing it could recognize, which the UI reports as "no
// speech" rather than as a Google internal. See
// TestGoogleTranscriberTreatsEmptyResultAsNoSpeech.
