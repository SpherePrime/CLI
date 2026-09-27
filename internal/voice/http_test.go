package voice

import (
	"context"
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
		_, _ = w.Write([]byte(`{"result":[{"alternative":[{"transcript":"`+transcript+`","confidence":0.9}]}]}`+"\n"))
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

func TestGoogleTranscriberRejectsHTTPError(t *testing.T) {
	t.Parallel()

	server, _ := newGoogleStubServer(t, "", http.StatusForbidden)
	transcriber := googleAt(Settings{}, server)

	_, err := transcriber.Transcribe(context.Background(), testAudio(t))
	require.ErrorIs(t, err, ErrTranscribeFailed)
}

func TestGoogleTranscriberRejectsEmptyTranscript(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"result":[{"alternative":[]}]}`+"\n"))
	}))
	defer server.Close()

	transcriber := googleAt(Settings{}, server)
	_, err := transcriber.Transcribe(context.Background(), testAudio(t))
	require.ErrorIs(t, err, ErrTranscribeFailed)
}
