package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

const testPNGMagic = "\x89PNG\r\n\x1a\n"

func imageGenTestContext() context.Context {
	return context.WithValue(context.Background(), SessionIDContextKey, "test-session")
}

func imageGenTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func imageGenOptions(serverURL string) *config.ImageGenOptions {
	return &config.ImageGenOptions{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: serverURL,
	}
}

func runImageGen(t *testing.T, tool fantasy.AgentTool, params ImageGenParams) (fantasy.ToolResponse, error) {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	return tool.Run(imageGenTestContext(), fantasy.ToolCall{
		ID:    "test-call",
		Name:  ImageGenToolName,
		Input: string(input),
	})
}

func TestImageGenToolWithoutAPIKeySuggestsSettings(t *testing.T) {
	t.Parallel()

	tool := NewImageGenTool(nil, t.TempDir(), &config.ImageGenOptions{}, config.IdentityResolver(), nil)

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "not configured")
	require.Contains(t, resp.Content, "prime://settings/image-generation")
}

func TestImageGenToolNilOptionsSuggestsSettings(t *testing.T) {
	t.Parallel()

	tool := NewImageGenTool(nil, t.TempDir(), nil, config.IdentityResolver(), nil)

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "prime://settings/image-generation")
}

func TestImageGenToolRequiresPrompt(t *testing.T) {
	t.Parallel()

	tool := NewImageGenTool(nil, t.TempDir(), imageGenOptions(imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {}).URL), config.IdentityResolver(), nil)

	resp, err := runImageGen(t, tool, ImageGenParams{})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "prompt is required")
}

func TestImageGenToolSavesBase64Response(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	var requestBody map[string]any
	server := imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/images/generations", r.URL.Path)
		require.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&requestBody))
		imageData := base64.StdEncoding.EncodeToString([]byte(testPNGMagic + "payload"))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": imageData, "revised_prompt": "a better cat"}},
		}))
	})

	tool := NewImageGenTool(&mockPermissionService{}, workingDir, imageGenOptions(server.URL), config.IdentityResolver(), nil)

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat", OutputDir: "images"})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	entries, err := os.ReadDir(filepath.Join(workingDir, "images"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.True(t, strings.HasSuffix(entries[0].Name(), ".png"))

	saved, err := os.ReadFile(filepath.Join(workingDir, "images", entries[0].Name()))
	require.NoError(t, err)
	require.Equal(t, testPNGMagic+"payload", string(saved))

	require.Equal(t, "image", resp.Type)
	require.Equal(t, "image/png", resp.MediaType)
	require.Equal(t, testPNGMagic+"payload", string(resp.Data))
	require.Contains(t, resp.Content, "Saved 1 image(s)")
	require.Contains(t, resp.Content, "a better cat")
	require.Equal(t, "test-model", requestBody["model"])
	require.Equal(t, float64(1), requestBody["n"])
}

func TestImageGenToolDownloadsURLResponse(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	imageServer := imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte(testPNGMagic + "downloaded"))
		require.NoError(t, err)
	})
	apiServer := imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"url": imageServer.URL + "/image.png"}},
		}))
	})

	tool := NewImageGenTool(&mockPermissionService{}, workingDir, imageGenOptions(apiServer.URL), config.IdentityResolver(), nil)

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat"})
	require.NoError(t, err)
	require.False(t, resp.IsError)

	entries, err := os.ReadDir(filepath.Join(workingDir, "generated-images"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	saved, err := os.ReadFile(filepath.Join(workingDir, "generated-images", entries[0].Name()))
	require.NoError(t, err)
	require.Equal(t, testPNGMagic+"downloaded", string(saved))
}

func TestImageGenToolReportsAPIError(t *testing.T) {
	t.Parallel()

	server := imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, err := w.Write([]byte(`{"error":{"message":"rate limited"}}`))
		require.NoError(t, err)
	})

	tool := NewImageGenTool(&mockPermissionService{}, t.TempDir(), imageGenOptions(server.URL), config.IdentityResolver(), nil)

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "rate limited")
}

func TestImageGenToolReportsHTMLApiError(t *testing.T) {
	t.Parallel()

	server := imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, err := w.Write([]byte("<html><head><title>Bad Gateway</title></head><body>  lots   of markup </body></html>"))
		require.NoError(t, err)
	})

	tool := NewImageGenTool(&mockPermissionService{}, t.TempDir(), imageGenOptions(server.URL), config.IdentityResolver(), nil)

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "Bad Gateway")
}

func TestImageGenToolSendsConfiguredDefaults(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	var requestBody map[string]any
	server := imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&requestBody))
		imageData := base64.StdEncoding.EncodeToString([]byte(testPNGMagic))
		items := make([]map[string]string, int(requestBody["n"].(float64)))
		for index := range items {
			items[index] = map[string]string{"b64_json": imageData}
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": items}))
	})

	options := imageGenOptions(server.URL)
	options.ResponseFormat = "b64_json"
	tool := NewImageGenTool(&mockPermissionService{}, workingDir, options, config.IdentityResolver(), nil)

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat", Count: 99})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Equal(t, "b64_json", requestBody["response_format"])
	require.NotContains(t, requestBody, "size")
	require.Equal(t, float64(10), requestBody["n"], "count must clamp to 10")

	entries, err := os.ReadDir(filepath.Join(workingDir, "generated-images"))
	require.NoError(t, err)
	require.Len(t, entries, 10)
}

func TestDetectImageExtension(t *testing.T) {
	t.Parallel()

	require.Equal(t, "png", detectImageExtension([]byte(testPNGMagic)))
	require.Equal(t, "jpg", detectImageExtension([]byte("\xff\xd8\xff\xe0")))
	require.Equal(t, "webp", detectImageExtension([]byte("RIFF\x00\x00\x00\x00WEBPVP8 ")))
	require.Equal(t, "png", detectImageExtension([]byte("unknown")))
}

func TestSanitizeImagePrefix(t *testing.T) {
	t.Parallel()

	require.Equal(t, "my-image", sanitizeImagePrefix("my image"))
	require.Equal(t, "image", sanitizeImagePrefix("///"))
}
