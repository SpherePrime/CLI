package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
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

func imageGenProvider(serverURL string) ProviderImageModelFunc {
	return func() (config.ProviderImageModel, bool) {
		return config.ProviderImageModel{ProviderID: "test", BaseURL: serverURL, APIKey: "test-key", Model: "test-model"}, true
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

func TestImageGenToolWithoutProviderSuggestsProviders(t *testing.T) {
	t.Parallel()

	tool := NewImageGenTool(nil, t.TempDir(), config.IdentityResolver(), nil, nil)

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "no connected provider has an image model")
	require.Contains(t, resp.Content, "prime://settings/providers")
}

func TestImageGenToolRequiresPrompt(t *testing.T) {
	t.Parallel()

	tool := NewImageGenTool(nil, t.TempDir(), config.IdentityResolver(), nil, imageGenProvider(imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {}).URL))

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

	tool := NewImageGenTool(&mockPermissionService{}, workingDir, config.IdentityResolver(), nil, imageGenProvider(server.URL))

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
	require.Len(t, resp.Images, 1)
	require.Equal(t, "image/png", resp.Images[0].MediaType)
	require.True(t, strings.HasSuffix(resp.Images[0].Path, ".png"))
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

	tool := NewImageGenTool(&mockPermissionService{}, workingDir, config.IdentityResolver(), nil, imageGenProvider(apiServer.URL))

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

	tool := NewImageGenTool(&mockPermissionService{}, t.TempDir(), config.IdentityResolver(), nil, imageGenProvider(server.URL))

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

	tool := NewImageGenTool(&mockPermissionService{}, t.TempDir(), config.IdentityResolver(), nil, imageGenProvider(server.URL))

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "Bad Gateway")
}

func TestImageGenToolSendsRequestedSize(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	var requestBody map[string]any
	server := imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.NoError(t, json.NewDecoder(r.Body).Decode(&requestBody))
		imageData := base64.StdEncoding.EncodeToString([]byte(testPNGMagic))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": imageData}},
		}))
	})

	tool := NewImageGenTool(&mockPermissionService{}, workingDir, config.IdentityResolver(), nil, imageGenProvider(server.URL))

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat", Size: "1024x1536"})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.Equal(t, "1024x1536", requestBody["size"])
}

func TestImageGenToolSendsDefaults(t *testing.T) {
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

	tool := NewImageGenTool(&mockPermissionService{}, workingDir, config.IdentityResolver(), nil, imageGenProvider(server.URL))

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat", Count: 99})
	require.NoError(t, err)
	require.False(t, resp.IsError)
	require.NotContains(t, requestBody, "response_format")
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

func TestImageGenToolUsesProviderImageModel(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	var requestBody map[string]any
	server := imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/generations", r.URL.Path)
		require.Equal(t, "Bearer relay-key", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&requestBody))
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString([]byte(testPNGMagic + "relay"))}},
		}))
	})
	providerModel := func() (config.ProviderImageModel, bool) {
		return config.ProviderImageModel{ProviderID: "relay", BaseURL: server.URL + "/v1", APIKey: "relay-key", Model: "antigravity/gemini-3.1-flash-image"}, true
	}
	tool := NewImageGenTool(&mockPermissionService{}, workingDir, config.IdentityResolver(), nil, providerModel)

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a hero banner of a mountain at dawn", Size: "1536x1024"})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Equal(t, "antigravity/gemini-3.1-flash-image", requestBody["model"])
	require.Equal(t, "a hero banner of a mountain at dawn", requestBody["prompt"])
	require.Equal(t, "1536x1024", requestBody["size"])
	require.NotContains(t, requestBody, "messages")
	require.Len(t, resp.Images, 1)
}

func TestImageGenToolWithoutProviderImageModelSuggestsSettings(t *testing.T) {
	t.Parallel()

	tool := NewImageGenTool(nil, t.TempDir(), config.IdentityResolver(), nil, func() (config.ProviderImageModel, bool) { return config.ProviderImageModel{}, false })
	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "a cat"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "prime://settings/providers")
}

func TestImageGenToolEditsSourceImages(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(workingDir, "assets"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "assets", "cube.png"), []byte(testPNGMagic+"red"), 0o644))

	var fields map[string]string
	var files []string
	server := imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/images/edits", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		fields = map[string]string{"model": r.FormValue("model"), "prompt": r.FormValue("prompt"), "n": r.FormValue("n")}
		for _, header := range r.MultipartForm.File["image[]"] {
			file, err := header.Open()
			require.NoError(t, err)
			data, err := io.ReadAll(file)
			require.NoError(t, err)
			files = append(files, header.Filename+":"+header.Header.Get("Content-Type")+":"+string(data))
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString([]byte(testPNGMagic + "blue"))}},
		}))
	})
	providerModel := func() (config.ProviderImageModel, bool) {
		return config.ProviderImageModel{ProviderID: "relay", BaseURL: server.URL + "/v1", APIKey: "k", Model: "antigravity/gemini-3.1-flash-image"}, true
	}
	tool := NewImageGenTool(&mockPermissionService{}, workingDir, config.IdentityResolver(), nil, providerModel)

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "make the cube blue, keep everything else", SourceImages: []string{"assets/cube.png"}, OutputDir: "assets", FilenamePrefix: "cube-blue"})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Equal(t, map[string]string{"model": "antigravity/gemini-3.1-flash-image", "prompt": "make the cube blue, keep everything else", "n": "1"}, fields)
	require.Equal(t, []string{"cube.png:image/png:" + testPNGMagic + "red"}, files)
	original, err := os.ReadFile(filepath.Join(workingDir, "assets", "cube.png"))
	require.NoError(t, err)
	require.Equal(t, testPNGMagic+"red", string(original))
}

func TestImageGenToolRejectsBadSourceImages(t *testing.T) {
	t.Parallel()

	workingDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(workingDir, "notes.txt"), []byte("hello"), 0o644))
	server := imageGenTestServer(t, func(w http.ResponseWriter, r *http.Request) { t.Fatal("no request expected") })
	tool := NewImageGenTool(&mockPermissionService{}, workingDir, config.IdentityResolver(), nil, imageGenProvider(server.URL))

	resp, err := runImageGen(t, tool, ImageGenParams{Prompt: "x", SourceImages: []string{"notes.txt"}})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "not a png, jpeg or webp")

	resp, err = runImageGen(t, tool, ImageGenParams{Prompt: "x", SourceImages: []string{"missing.png"}})
	require.NoError(t, err)
	require.True(t, resp.IsError)

	resp, err = runImageGen(t, tool, ImageGenParams{Prompt: "x", SourceImages: []string{"a", "b", "c", "d", "e"}})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "up to 4")
}
