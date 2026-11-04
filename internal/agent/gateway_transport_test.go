package agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/fantasy/providers/openaicompat"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestGatewayTransportStripsOnlyDeclaredGenerationParameters(t *testing.T) {
	var model catwalk.Model
	require.NoError(t, json.Unmarshal([]byte(`{"id":"codex/model","options":{"provider_options":{"gateway":{"type":"codex","unsupported_generation_parameters":["max_tokens","max_completion_tokens","max_output_tokens","temperature","top_p","frequency_penalty","presence_penalty","messages","reasoning_effort"]}}}}`), &model))
	for _, test := range []struct {
		name, model, path string
		stripped          bool
	}{
		{"chat", "codex/model", "/v1/chat/completions", true},
		{"responses", "codex/model", "/v1/responses", true},
		{"ordinary model", "ordinary", "/v1/chat/completions", false},
		{"unrelated endpoint", "codex/model", "/v1/other", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var body map[string]any
				require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
				for _, key := range []string{"max_tokens", "max_completion_tokens", "max_output_tokens", "temperature", "top_p", "frequency_penalty", "presence_penalty"} {
					_, exists := body[key]
					require.Equal(t, !test.stripped, exists, key)
				}
				require.Equal(t, "high", body["reasoning_effort"])
				require.NotEmpty(t, body["messages"])
				require.NotEmpty(t, body["tools"])
				require.Equal(t, "session", r.Header.Get("x-session-id"))
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()
			client := withGatewayCapabilities(server.Client(), []catwalk.Model{model})
			payload := `{"model":"` + test.model + `","max_tokens":40,"max_completion_tokens":40,"max_output_tokens":40,"temperature":0.5,"top_p":0.9,"frequency_penalty":0,"presence_penalty":0,"reasoning_effort":"high","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"data:image/png;base64,AAAA"}}]}],"tools":[{"type":"function","function":{"name":"inspect"}}]}`
			request, err := http.NewRequest(http.MethodPost, server.URL+test.path, strings.NewReader(payload))
			require.NoError(t, err)
			request.Header.Set("x-session-id", "session")
			response, err := client.Do(request)
			require.NoError(t, err)
			_, _ = io.Copy(io.Discard, response.Body)
			require.NoError(t, response.Body.Close())
		})
	}
}

func TestGatewayTransportLeavesUnmarkedClientUnchanged(t *testing.T) {
	client := &http.Client{}
	require.Same(t, client, withGatewayCapabilities(client, []catwalk.Model{{ID: "ordinary"}}))
}

func TestGatewayTransportTitleRequestThroughSDK(t *testing.T) {
	var catalog catwalk.Model
	require.NoError(t, json.Unmarshal([]byte(`{"id":"codex/model","options":{"provider_options":{"gateway":{"type":"codex","unsupported_generation_parameters":["max_tokens"]}}}}`), &catalog))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
		require.NotContains(t, body, "max_tokens")
		require.Equal(t, "codex/model", body["model"])
		require.Equal(t, true, body["stream"])
		require.Equal(t, map[string]any{"include_usage": true}, body["stream_options"])
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = io.WriteString(w, "data: {\"id\":\"title\",\"object\":\"chat.completion.chunk\",\"model\":\"codex/model\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Title\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	}))
	defer server.Close()
	provider, err := openaicompat.New(openaicompat.WithBaseURL(server.URL+"/v1"), openaicompat.WithAPIKey("test"), openaicompat.WithHTTPClient(withGatewayCapabilities(server.Client(), []catwalk.Model{catalog})))
	require.NoError(t, err)
	model, err := provider.LanguageModel(context.Background(), catalog.ID)
	require.NoError(t, err)
	agent := fantasy.NewAgent(model, fantasy.WithMaxOutputTokens(40))
	result, err := agent.Stream(context.Background(), fantasy.AgentStreamCall{Prompt: "Generate a title"})
	require.NoError(t, err)
	require.Equal(t, "Title", result.Response.Content.Text())
}
