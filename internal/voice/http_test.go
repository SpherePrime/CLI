package voice

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

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

func TestOpenAIHTTPIsTranscribed(t *testing.T) {
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

func TestWhisperServerShapeIsTranscribed(t *testing.T) {
	t.Parallel()

	var fields map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields, _, _ = formOf(t, r)
		// whisper.cpp has answered with "message" in some releases.
		_, _ = w.Write([]byte(`{"message":"сделай коммит"}`))
	}))
	defer server.Close()

	transcriber := newServerTranscriber(server.URL+"/inference", Settings{Language: "ru"})
	text, err := transcriber.Transcribe(context.Background(), testAudio(t))
	require.NoError(t, err)
	require.Equal(t, "сделай коммит", text)
	require.Equal(t, "ru", fields["language"])
	require.NotContains(t, fields, "model", "the server loads its own model")
}

func TestHTTPTranscriberOmitsLanguageForAutoDetect(t *testing.T) {
	t.Parallel()

	var fields map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields, _, _ = formOf(t, r)
		_, _ = w.Write([]byte(`{"text":"ok"}`))
	}))
	defer server.Close()

	transcriber := newOpenAITranscriber(server.URL, Settings{APIKey: "k"})
	_, err := transcriber.Transcribe(context.Background(), testAudio(t))
	require.NoError(t, err)
	require.NotContains(t, fields, "language")
}

func TestHTTPTranscriberReportsServerError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid model"}}`))
	}))
	defer server.Close()

	transcriber := newOpenAITranscriber(server.URL, Settings{APIKey: "k"})
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
