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

// httpTranscriber talks to an OpenAI-compatible audio/transcriptions
// endpoint: OpenAI, Groq, llama.cpp servers, and friends.
type httpTranscriber struct {
	label    string
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
		endpoint: endpoint,
		model:    settings.Model,
		apiKey:   settings.resolveAPIKey(),
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
	if t.apiKey == "" {
		return "", fmt.Errorf("%w: no API key configured for %s", ErrTranscribeFailed, t.endpoint)
	}

	body, contentType, err := t.uploadBody(audio)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.endpoint, body)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", "Bearer "+t.apiKey)

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

	text, err := decodeTranscriptionJSON(payload)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTranscribeFailed, err)
	}
	if text == "" {
		return "", fmt.Errorf("%w: %s returned no text", ErrTranscribeFailed, t.label)
	}
	return tidyTranscript(text), nil
}

// uploadBody packages the recording as multipart form data, which the
// OpenAI shape expects.
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

	fields := map[string]string{
		"model":           orDefault(t.model, openAIAPIModel),
		"response_format": "json",
	}
	if t.language != "" {
		fields["language"] = t.language
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
type transcriptionPayload struct {
	Text    string `json:"text"`
	Error   any    `json:"error"`
	Message struct {
		Message string `json:"message"`
	} `json:"message"`
}

// decodeTranscriptionJSON reads a transcript out of a JSON answer, and also
// accepts a plain text body so servers configured for text output still work.
func decodeTranscriptionJSON(payload []byte) (string, error) {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 {
		return "", nil
	}
	if trimmed[0] != '{' {
		return string(trimmed), nil
	}
	var decoded transcriptionPayload
	if err := json.Unmarshal(trimmed, &decoded); err != nil {
		return "", fmt.Errorf("unexpected response: %w", err)
	}
	if decoded.Error != nil {
		return "", fmt.Errorf("server error: %v", decoded.Error)
	}
	return strings.TrimSpace(decoded.Text), nil
}

// orDefault returns value, or fallback when it is empty.
func orDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
