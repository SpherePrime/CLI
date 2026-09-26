package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

// httpTranscriber talks to a Whisper server over HTTP. Two shapes are served:
// OpenAI-compatible audio/transcriptions endpoints, which include local
// llama.cpp and Groq servers, and whisper.cpp's own inference route.
type httpTranscriber struct {
	label    string
	style    httpStyle
	endpoint string
	model    string
	apiKey   string
	language string
	client   *http.Client
}

// newOpenAITranscriber builds a transcriber for an OpenAI-compatible endpoint.
func newOpenAITranscriber(endpoint string, settings Settings) Transcriber {
	return httpTranscriber{
		label:    "whisper api",
		style:    styleOpenAI,
		endpoint: endpoint,
		model:    settings.Model,
		apiKey:   settings.resolveAPIKey(),
		language: settings.Language,
	}
}

// newServerTranscriber builds a transcriber for whisper.cpp's server, which
// loads its own model and therefore ignores options.voice.model.
func newServerTranscriber(endpoint string, settings Settings) Transcriber {
	return httpTranscriber{
		label:    "whisper.cpp server",
		style:    styleWhisperServer,
		endpoint: endpoint,
		language: settings.Language,
	}
}

func (t httpTranscriber) Name() string { return t.label }

// Transcribe uploads the recording and returns the text the server answered
// with.
func (t httpTranscriber) Transcribe(ctx context.Context, audio *Audio) (string, error) {
	if audio == nil || len(audio.WAV) == 0 {
		return "", ErrNoSpeech
	}
	if t.apiKey == "" && t.style == styleOpenAI {
		return "", fmt.Errorf("%w: no API key configured for %s", ErrTranscribeFailed, t.endpoint)
	}

	body, contentType, err := t.uploadBody(audio)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}

	ctx, cancel := context.WithTimeout(ctx, transcriptionTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, body)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	req.Header.Set("Content-Type", contentType)
	if t.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+t.apiKey)
	}

	client := t.client
	if client == nil {
		client = &http.Client{Timeout: transcriptionTimeout}
	}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	defer func() { _ = res.Body.Close() }()

	payload, err := io.ReadAll(io.LimitReader(res.Body, commandOutputLimit))
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return "", fmt.Errorf("%w: %s returned %s: %s",
			ErrTranscribeFailed, t.endpoint, res.Status, strings.TrimSpace(string(payload)))
	}

	text, language, err := decodeTranscription(payload)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if text == "" {
		return "", fmt.Errorf("%w: %s returned no text", ErrTranscribeFailed, t.label)
	}
	// A server run with auto-detect reports which language it heard; later
	// requests pin it and skip the detection pass, which on short phrases
	// costs about as much as the transcription itself.
	if t.style == styleWhisperServer && t.language == "" {
		rememberDetectedLanguage(language)
	}
	return tidyTranscript(text), nil
}

// uploadBody packages the recording as multipart form data, which both HTTP
// shapes expect.
func (t httpTranscriber) uploadBody(audio *Audio) (io.Reader, string, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", "dictation.wav")
	if err != nil {
		return nil, "", err
	}
	if _, err := part.Write(audio.WAV); err != nil {
		return nil, "", err
	}

	fields := map[string]string{}
	switch t.style {
	case styleOpenAI:
		fields["model"] = orDefault(t.model, openAIAPIModel)
		fields["response_format"] = "json"
	case styleWhisperServer:
		fields["response_format"] = "json"
		fields["temperature"] = "0"
		fields["no_timestamps"] = "1"
	}
	language := t.language
	if language == "" && t.style == styleWhisperServer {
		language = recallDetectedLanguage()
	}
	if language != "" {
		fields["language"] = language
	}
	for name, value := range fields {
		if value == "" {
			continue
		}
		if err := writer.WriteField(name, value); err != nil {
			return nil, "", err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, "", err
	}
	return &buf, writer.FormDataContentType(), nil
}

// transcriptionPayload covers the response shapes of the supported servers.
// whisper.cpp has used both "text" and "message" across releases.
type transcriptionPayload struct {
	Text        string `json:"text"`
	Message     string `json:"message"`
	Translation string `json:"translation"`
	Transcribed string `json:"transcribed"`
	Language    string `json:"language"`
	Error       any    `json:"error"`
}

// transcript returns whichever field carries the dictated text.
func (p transcriptionPayload) transcript() string {
	for _, candidate := range []string{p.Text, p.Message, p.Translation, p.Transcribed} {
		if strings.TrimSpace(candidate) != "" {
			return candidate
		}
	}
	return ""
}

// decodeTranscriptionJSON reads a transcript out of a JSON answer, and also
// accepts a plain text body so servers configured for text output still work.
func decodeTranscriptionJSON(payload []byte) (string, error) {
	text, _, err := decodeTranscription(payload)
	return text, err
}

// decodeTranscription returns the transcript and the language the server
// says it heard, if it says so at all.
func decodeTranscription(payload []byte) (text string, language string, err error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return "", "", nil
	}
	if trimmed[0] != '{' {
		return string(trimmed), "", nil
	}
	var decoded transcriptionPayload
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return "", "", fmt.Errorf("unexpected response: %w", err)
	}
	if decoded.Error != nil {
		return "", "", fmt.Errorf("server reported: %v", decoded.Error)
	}
	return decoded.transcript(), decoded.Language, nil
}

// orDefault returns value, or fallback when it is empty.
func orDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
