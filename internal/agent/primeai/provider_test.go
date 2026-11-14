package primeai

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

func TestSessionManager_SingleFlightRefreshAndInvalidate(t *testing.T) {
	t.Parallel()

	var sessionCalls atomic.Int32
	var mu sync.Mutex
	now := time.Unix(1_700_000_000, 0)
	acctToken := "acct_token_1"

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/plugin/session", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		n := sessionCalls.Add(1)
		mu.Lock()
		exp := now.Add(5 * time.Minute).UnixMilli()
		mu.Unlock()
		_ = json.NewEncoder(w).Encode(map[string]any{
			"token":     "sess_" + string(rune('0'+n)),
			"expiresAt": exp,
		})
	}))
	defer srv.Close()

	sm := NewSessionManager(
		func() string {
			mu.Lock()
			defer mu.Unlock()
			return acctToken
		},
		WithSessionBaseURL(func() string { return srv.URL }),
		WithSessionClock(func() time.Time {
			mu.Lock()
			defer mu.Unlock()
			return now
		}),
	)

	const workers = 8
	var wg sync.WaitGroup
	tokens := make([]string, workers)
	errs := make([]error, workers)
	for i := range workers {
		wg.Go(func() {
			tokens[i], errs[i] = sm.Get(context.Background())
		})
	}
	wg.Wait()

	for i := range workers {
		require.NoError(t, errs[i])
		require.Equal(t, "sess_1", tokens[i])
	}
	require.Equal(t, int32(1), sessionCalls.Load())

	tok, err := sm.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "sess_1", tok)
	require.Equal(t, int32(1), sessionCalls.Load())

	mu.Lock()
	now = now.Add(4*time.Minute + 5*time.Second)
	mu.Unlock()
	tok, err = sm.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "sess_2", tok)
	require.Equal(t, int32(2), sessionCalls.Load())

	sm.Invalidate("sess_2")
	tok, err = sm.Get(context.Background())
	require.NoError(t, err)
	require.Equal(t, "sess_3", tok)
	require.Equal(t, int32(3), sessionCalls.Load())
}

func TestFetchModels_PrependsAutoAndRetries401(t *testing.T) {
	t.Parallel()

	var sessionSeq atomic.Int32
	var modelsCalls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/plugin/session":
			require.Equal(t, "Bearer acct_secret", r.Header.Get("Authorization"))
			n := sessionSeq.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":     "sess_tok_" + string(rune('0'+n)),
				"expiresAt": time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case "/api/plugin/models":
			call := modelsCalls.Add(1)
			if call == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			require.Equal(t, "Bearer sess_tok_2", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"models": []map[string]any{
					{"id": "gpt-5", "name": "GPT-5", "routeCount": 2, "kind": "text", "supportsVision": true, "priceIn": 1.5, "priceOut": 6.0},
					{"id": "flux-img", "name": "Flux", "routeCount": 1, "kind": "image"},
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := NewClient(
		func() string { return "acct_secret" },
		WithBaseURL(func() string { return srv.URL }),
		WithLimitsStore(NewLimitsStore()),
	)

	models, err := client.FetchModels(context.Background())
	require.NoError(t, err)
	require.Equal(t, int32(2), sessionSeq.Load())
	require.Equal(t, int32(2), modelsCalls.Load())
	require.Len(t, models, 2)
	require.Equal(t, AutomaticModelID, models[0].ID)
	require.Equal(t, AutomaticModelName, models[0].Name)
	require.True(t, models[0].SupportsImages)
	require.Equal(t, "gpt-5", models[1].ID)
	require.Equal(t, 1.5, models[1].CostPer1MIn)
	require.Equal(t, 6.0, models[1].CostPer1MOut)
}

func TestFetchLimits_CacheAndListener(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/plugin/session":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":     "sess_lim",
				"expiresAt": time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case "/api/plugin/limits":
			require.Equal(t, "Bearer sess_lim", r.Header.Get("Authorization"))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"planName":           "Pro",
				"cacheWeightPercent": 25,
				"fiveHour": map[string]any{
					"usedTokens":     1200,
					"limitTokens":    10000,
					"usedRequests":   4,
					"limitRequests":  50,
					"percent":        12.5,
					"resetAt":        "2026-11-14T18:00:00Z",
					"resetInSeconds": 3600,
				},
				"weekly": map[string]any{
					"usedTokens":     5000,
					"limitTokens":    100000,
					"usedRequests":   20,
					"limitRequests":  500,
					"percent":        5.0,
					"resetAt":        1794668400000,
					"resetInSeconds": 86400,
				},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	store := NewLimitsStore()
	var notified Limits
	unsub := store.Subscribe(func(l Limits) { notified = l })
	defer unsub()

	client := NewClient(
		func() string { return "acct_lim" },
		WithBaseURL(func() string { return srv.URL }),
		WithLimitsStore(store),
	)

	limits, err := client.FetchLimits(context.Background())
	require.NoError(t, err)
	require.True(t, limits.Available)
	require.Equal(t, "Pro", limits.PlanName)
	require.Equal(t, 25, limits.CacheWeightPercent)
	require.NotNil(t, limits.FiveHour)
	require.Equal(t, 12.5, limits.FiveHour.Percent)
	require.Equal(t, int64(3600), limits.FiveHour.ResetInSeconds)
	require.NotNil(t, limits.Weekly)
	require.Equal(t, 5.0, limits.Weekly.Percent)
	require.NotEmpty(t, limits.Weekly.ResetAt)

	cached, ok := store.Cached()
	require.True(t, ok)
	require.Equal(t, "Pro", cached.PlanName)
	require.Equal(t, "Pro", notified.PlanName)
}

func TestStreamAndGenerate_SSEAnd401Retry(t *testing.T) {
	t.Parallel()

	var sessionSeq atomic.Int32
	var chatCalls atomic.Int32
	var capturedReq RouterCompletionRequest

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/plugin/session":
			n := sessionSeq.Add(1)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":     "sess_chat_" + string(rune('0'+n)),
				"expiresAt": time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case "/api/plugin/chat/completions":
			call := chatCalls.Add(1)
			if call == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			require.Equal(t, "Bearer sess_chat_2", r.Header.Get("Authorization"))
			require.Equal(t, "text/event-stream", r.Header.Get("Accept"))
			require.NoError(t, json.NewDecoder(r.Body).Decode(&capturedReq))

			w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
			flusher := w.(http.Flusher)
			writeEvent := func(name string, payload any) {
				b, _ := json.Marshal(payload)
				_, _ = w.Write([]byte("event: " + name + "\ndata: " + string(b) + "\n\n"))
				flusher.Flush()
			}
			writeEvent("route", map[string]any{"type": "route", "providerName": "upstream", "modelId": "gpt-5", "substituted": false})
			writeEvent("thinking", map[string]any{"type": "thinking", "delta": "plan"})
			writeEvent("text", map[string]any{"type": "text", "delta": "hello"})
			writeEvent("tool_call", map[string]any{"type": "tool_call", "call": map[string]any{"id": "tc_1", "name": "ls", "argumentsJson": `{"path":"."}`}})
			writeEvent("usage", map[string]any{"type": "usage", "inputTokens": 120, "outputTokens": 30, "cachedInputTokens": 40})
			writeEvent("limits", map[string]any{
				"type":     "limits",
				"planName": "Ultra",
				"fiveHour": map[string]any{"percent": 42.0, "resetInSeconds": 1800},
				"weekly":   map[string]any{"percent": 15.0, "resetInSeconds": 50000},
			})
			writeEvent("done", map[string]any{"type": "done"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	store := NewLimitsStore()
	prov, err := NewProvider(
		WithProviderBaseURL(srv.URL),
		WithProviderAccountToken(func() string { return "acct_chat" }),
		WithProviderLimitsStore(store),
	)
	require.NoError(t, err)
	require.Equal(t, Name, prov.Name())

	lm, err := prov.LanguageModel(context.Background(), AutomaticModelID)
	require.NoError(t, err)

	show := true
	temp := 0.2
	maxOut := int64(2048)
	resp, err := lm.Generate(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{
			fantasy.NewSystemMessage("be concise"),
			fantasy.NewUserMessage("list files"),
		},
		Tools: []fantasy.Tool{
			fantasy.FunctionTool{
				Name:        "ls",
				Description: "list directory",
				InputSchema: map[string]any{"type": "object"},
			},
		},
		Temperature:     &temp,
		MaxOutputTokens: &maxOut,
		ProviderOptions: fantasy.ProviderOptions{
			Name: &Options{ReasoningEffort: "high", ShowReasoning: &show},
		},
	})
	require.NoError(t, err)
	require.Equal(t, int32(2), chatCalls.Load())
	require.Equal(t, AutomaticModelID, capturedReq.Model)
	require.Equal(t, "be concise", capturedReq.SystemPrompt)
	require.Equal(t, "high", capturedReq.ReasoningEffort)
	require.NotNil(t, capturedReq.ShowReasoning)
	require.True(t, *capturedReq.ShowReasoning)
	require.Len(t, capturedReq.Tools, 1)

	require.Equal(t, fantasy.FinishReasonToolCalls, resp.FinishReason)
	require.Equal(t, "plan", resp.Content.ReasoningText())
	require.Equal(t, "hello", resp.Content.Text())
	require.Len(t, resp.Content.ToolCalls(), 1)
	require.Equal(t, "tc_1", resp.Content.ToolCalls()[0].ToolCallID)
	require.Equal(t, int64(120), resp.Usage.InputTokens)
	require.Equal(t, int64(30), resp.Usage.OutputTokens)
	require.Equal(t, int64(40), resp.Usage.CacheReadTokens)

	cached, ok := store.Cached()
	require.True(t, ok)
	require.Equal(t, "Ultra", cached.PlanName)
	require.Equal(t, 42.0, cached.FiveHour.Percent)

	emb := Embedded()
	require.Equal(t, catwalk.InferenceProvider(Name), emb.ID)
	require.NotEmpty(t, emb.Models)
	require.Equal(t, AutomaticModelID, emb.Models[0].ID)
}

func TestStream_LimitExceeded429UpdatesCache(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/plugin/session":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"token":     "sess_429",
				"expiresAt": time.Now().Add(10 * time.Minute).UnixMilli(),
			})
		case "/api/plugin/chat/completions":
			w.WriteHeader(http.StatusTooManyRequests)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":           "PLUGIN_LIMIT_EXCEEDED",
				"window":         "5h",
				"resetInSeconds": 900,
				"error":          "5h limit reached",
				"limits": map[string]any{
					"planName": "Free",
					"fiveHour": map[string]any{"percent": 100.0, "resetInSeconds": 900},
				},
			})
		}
	}))
	defer srv.Close()

	store := NewLimitsStore()
	prov, err := NewProvider(
		WithProviderBaseURL(srv.URL),
		WithProviderAccountToken(func() string { return "acct_429" }),
		WithProviderLimitsStore(store),
	)
	require.NoError(t, err)

	lm, err := prov.LanguageModel(context.Background(), AutomaticModelID)
	require.NoError(t, err)

	_, err = lm.Generate(context.Background(), fantasy.Call{
		Prompt: fantasy.Prompt{fantasy.NewUserMessage("hi")},
	})
	require.Error(t, err)
	var provErr *fantasy.ProviderError
	require.ErrorAs(t, err, &provErr)
	require.Equal(t, http.StatusTooManyRequests, provErr.StatusCode)
	require.Contains(t, provErr.Message, "5h limit reached")

	cached, ok := store.Cached()
	require.True(t, ok)
	require.Equal(t, 100.0, cached.FiveHour.Percent)
}
