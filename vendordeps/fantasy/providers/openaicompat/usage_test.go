package openaicompat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

func TestProviderCacheUsage(t *testing.T) {
	cases := []struct {
		name    string
		details string
		cached  int64
	}{
		{"deepseek", `"prompt_cache_hit_tokens":80,"prompt_cache_miss_tokens":20`, 80},
		{"standard", `"prompt_tokens_details":{"cached_tokens":70}`, 70},
		{"standard_precedence", `"prompt_tokens_details":{"cached_tokens":70},"prompt_cache_hit_tokens":80`, 70},
		{"standard_zero", `"prompt_tokens_details":{"cached_tokens":0},"prompt_cache_hit_tokens":80`, 0},
		{"no_cache", `"prompt_cache_miss_tokens":100`, 0},
		{"negative", `"prompt_cache_hit_tokens":-1`, 0},
		{"excessive", `"prompt_cache_hit_tokens":101`, 0},
	}
	for _, test := range cases {
		for _, streaming := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stream=%v", test.name, streaming), func(t *testing.T) {
				usageJSON := fmt.Sprintf(`{"prompt_tokens":100,"completion_tokens":10,"total_tokens":110,%s}`, test.details)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var request struct {
						Stream bool `json:"stream"`
					}
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
						t.Error(err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if request.Stream {
						w.Header().Set("Content-Type", "text/event-stream")
						fmt.Fprint(w, "data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"ok\"},\"finish_reason\":\"stop\"}]}\n\n")
						fmt.Fprintf(w, "data: {\"id\":\"test\",\"object\":\"chat.completion.chunk\",\"choices\":[],\"usage\":%s}\n\ndata: [DONE]\n\n", usageJSON)
						return
					}
					w.Header().Set("Content-Type", "application/json")
					fmt.Fprintf(w, `{"id":"test","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":%s}`, usageJSON)
				}))
				defer server.Close()
				provider, err := New(WithBaseURL(server.URL), WithAPIKey("test"))
				if err != nil {
					t.Fatal(err)
				}
				model, err := provider.LanguageModel(context.Background(), "deepseek-chat")
				if err != nil {
					t.Fatal(err)
				}
				call := fantasy.Call{Prompt: fantasy.Prompt{fantasy.NewUserMessage("hello")}}
				var usage fantasy.Usage
				if streaming {
					response, err := model.Stream(context.Background(), call)
					if err != nil {
						t.Fatal(err)
					}
					for part := range response {
						if part.Error != nil {
							t.Fatal(part.Error)
						}
						if part.Type == fantasy.StreamPartTypeFinish {
							usage = part.Usage
						}
					}
				} else {
					response, err := model.Generate(context.Background(), call)
					if err != nil {
						t.Fatal(err)
					}
					usage = response.Usage
				}
				if usage.CacheReadTokens != test.cached || usage.InputTokens != 100-test.cached || usage.OutputTokens != 10 || usage.TotalTokens != 110 {
					t.Fatalf("unexpected usage: %+v; want cached=%d, input=%d, output=10, total=110", usage, test.cached, 100-test.cached)
				}
			})
		}
	}
}
