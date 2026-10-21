package tools

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const imageAPIMaxErrorLength = 500

type imageAPIRequest struct {
	Model          string `json:"model"`
	Prompt         string `json:"prompt"`
	N              int    `json:"n"`
	Size           string `json:"size,omitempty"`
	ResponseFormat string `json:"response_format,omitempty"`
}

type imageAPIResponseItem struct {
	B64JSON      string `json:"b64_json"`
	URL          string `json:"url"`
	RevisedPrompt string `json:"revised_prompt"`
}

type imageAPIResponse struct {
	Data  []imageAPIResponseItem `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type generatedImage struct {
	Bytes         []byte
	RevisedPrompt string
}

// summarizeErrorBody collapses an error response body into one short line:
// an HTML page becomes its title, anything else is whitespace-compacted and
// truncated so a wall of markup never reaches the model.
func summarizeErrorBody(rawBody string) string {
	if start := strings.Index(rawBody, "<title"); start >= 0 {
		if rest := rawBody[start:]; strings.Contains(rest, ">") {
			rest = rest[strings.Index(rest, ">")+1:]
			if end := strings.Index(rest, "</title>"); end >= 0 {
				if title := strings.TrimSpace(rest[:end]); title != "" {
					return title
				}
			}
		}
	}
	compact := strings.Join(strings.Fields(rawBody), " ")
	if len(compact) > imageAPIMaxErrorLength {
		return compact[:imageAPIMaxErrorLength] + "…"
	}
	return compact
}

func readErrorMessage(rawBody string) string {
	var parsed imageAPIResponse
	if err := json.Unmarshal([]byte(rawBody), &parsed); err == nil && parsed.Error != nil && parsed.Error.Message != "" {
		return parsed.Error.Message
	}
	return summarizeErrorBody(rawBody)
}

// generateImages calls POST {baseURL}/images/generations and returns the
// decoded image bytes. Both b64_json and url result styles are supported.
func generateImages(ctx context.Context, client *http.Client, baseURL, apiKey string, request imageAPIRequest, timeout time.Duration) ([]generatedImage, error) {
	requestCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(requestCtx, http.MethodPost, baseURL+"/images/generations", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	resp, err := client.Do(req)
	if err != nil {
		if requestCtx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("image API request timed out after %s", timeout)
		}
		return nil, fmt.Errorf("image API request failed: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read image API response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("image API error HTTP %d: %s", resp.StatusCode, readErrorMessage(string(raw)))
	}

	var payload imageAPIResponse
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("failed to parse image API response: %w", err)
	}
	if len(payload.Data) == 0 {
		return nil, fmt.Errorf("image API returned no images")
	}

	images := make([]generatedImage, 0, len(payload.Data))
	for _, item := range payload.Data {
		switch {
		case item.B64JSON != "":
			decoded, err := base64.StdEncoding.DecodeString(item.B64JSON)
			if err != nil {
				return nil, fmt.Errorf("failed to decode image data: %w", err)
			}
			images = append(images, generatedImage{Bytes: decoded, RevisedPrompt: item.RevisedPrompt})
		case item.URL != "":
			imageBytes, err := downloadImage(ctx, client, item.URL, timeout)
			if err != nil {
				return nil, err
			}
			images = append(images, generatedImage{Bytes: imageBytes, RevisedPrompt: item.RevisedPrompt})
		default:
			return nil, fmt.Errorf("image API response item has neither b64_json nor url")
		}
	}
	return images, nil
}

func downloadImage(ctx context.Context, client *http.Client, imageURL string, timeout time.Duration) ([]byte, error) {
	downloadCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(downloadCtx, http.MethodGet, imageURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create image download request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to download image from %s: %w", imageURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download image from %s: HTTP %d", imageURL, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to download image from %s: %w", imageURL, err)
	}
	return data, nil
}
