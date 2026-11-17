package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/internal/pubsub"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
	"github.com/SpherePrime/CLI/vendordeps/stretchr/testify/require"
)

// serveSearchStub points the DuckDuckGo endpoint at a local server
// returning the given status and body, and restores it on cleanup.
func serveSearchStub(t *testing.T, status int, body string) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	orig := ddgLiteEndpoint
	ddgLiteEndpoint = srv.URL + "/lite/?q="
	t.Cleanup(func() { ddgLiteEndpoint = orig })
}

// loadAnomalyPage reads the real bot-check payload captured from
// lite.duckduckgo.com on 2026-07-29 (HTTP 202, captcha modal).
func loadAnomalyPage(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("testdata/ddg_anomaly_202.html")
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	return string(data)
}

// TestSearchRateLimitedOn202 verifies the 202 anomaly-challenge
// interstitial is reported as throttling, not parsed into an empty
// result set.
func TestSearchRateLimitedOn202(t *testing.T) {
	serveSearchStub(t, http.StatusAccepted, loadAnomalyPage(t))
	_, err := searchDuckDuckGo(context.Background(), http.DefaultClient, "anything", 10)
	if !errors.Is(err, errSearchRateLimited) {
		t.Fatalf("expected errSearchRateLimited, got %v", err)
	}
}

// TestSearchRateLimitedOnAnomalyPage verifies the captcha modal is
// detected by content as well: DuckDuckGo also serves the same page
// with HTTP 200 once a client is flagged.
func TestSearchRateLimitedOnAnomalyPage(t *testing.T) {
	serveSearchStub(t, http.StatusOK, loadAnomalyPage(t))
	_, err := searchDuckDuckGo(context.Background(), http.DefaultClient, "anything", 10)
	if !errors.Is(err, errSearchRateLimited) {
		t.Fatalf("expected errSearchRateLimited, got %v", err)
	}
}

// TestSearchParsesNormalResults guards against false positives: a
// genuine results page still parses.
func TestSearchParsesNormalResults(t *testing.T) {
	page := `<html><body><table>
<tr><td><a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fpost">Example Post</a></td></tr>
<tr><td class="result-snippet">A snippet about the example post.</td></tr>
</table></body></html>`
	serveSearchStub(t, http.StatusOK, page)
	results, err := searchDuckDuckGo(context.Background(), http.DefaultClient, "example", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 || results[0].Link != "https://example.com/post" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

type webSearchPermissionService struct {
	*pubsub.Broker[permission.PermissionRequest]
	allow    bool
	err      error
	requests []permission.CreatePermissionRequest
}

func (r *webSearchPermissionService) Request(_ context.Context, req permission.CreatePermissionRequest) (bool, error) {
	r.requests = append(r.requests, req)
	return r.allow, r.err
}

func (r *webSearchPermissionService) Grant(permission.PermissionRequest) bool           { return true }
func (r *webSearchPermissionService) Deny(permission.PermissionRequest) bool            { return true }
func (r *webSearchPermissionService) GrantPersistent(permission.PermissionRequest) bool { return true }
func (r *webSearchPermissionService) AutoApproveSession(string)                         {}
func (r *webSearchPermissionService) SetSkipRequests(bool)                              {}
func (r *webSearchPermissionService) SkipRequests() bool                                { return false }
func (r *webSearchPermissionService) SubscribeNotifications(context.Context) <-chan pubsub.Event[permission.PermissionNotification] {
	return make(<-chan pubsub.Event[permission.PermissionNotification])
}

type stubResolver map[string]string

func (s stubResolver) ResolveValue(val string) (string, error) {
	if v, ok := s[val]; ok {
		return v, nil
	}
	return val, nil
}

func runWebSearchTool(t *testing.T, ctx context.Context, tool fantasy.AgentTool, params WebSearchParams) (fantasy.ToolResponse, error) {
	t.Helper()
	input, err := json.Marshal(params)
	require.NoError(t, err)
	return tool.Run(ctx, fantasy.ToolCall{
		ID:    "ws-call-1",
		Name:  WebSearchToolName,
		Input: string(input),
	})
}

func TestWebSearchTool_ProviderSearchAndDescription(t *testing.T) {
	var gotAuth string
	var gotBody providerWebSearchRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/v1/web_search", r.URL.Path)
		gotAuth = r.Header.Get("Authorization")
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		require.NoError(t, json.NewEncoder(w).Encode(providerWebSearchResponse{
			Query:   gotBody.Query,
			Queries: []string{"golang generics", "go 1.24 release"},
			Answer:  "Go supports type parameters since Go 1.18.",
			Results: []providerWebSearchResultItem{
				{Title: "Go Generics", URL: "https://go.dev/doc/tutorial/generics", Snippet: "Tutorial on generics in Go."},
				{Title: "Release Notes", URL: "https://go.dev/doc/devel/release", Snippet: "Go release history."},
			},
		}))
	}))
	t.Cleanup(srv.Close)

	perms := &webSearchPermissionService{
		Broker: pubsub.NewBroker[permission.PermissionRequest](),
		allow:  true,
	}
	workingDir := t.TempDir()
	tool := NewWebSearchTool(srv.Client(), WebSearchOptions{
		Permissions: perms,
		WorkingDir:  workingDir,
		Resolver: stubResolver{
			"$SEARCH_KEY": "resolved-secret",
			"$SEARCH_URL": srv.URL + "/v1",
		},
		ProviderModel: func() (config.ProviderWebSearchModel, bool) {
			return config.ProviderWebSearchModel{
				ProviderID: "router",
				BaseURL:    "$SEARCH_URL",
				APIKey:     "$SEARCH_KEY",
				Model:      "gemini-2.5-flash",
			}, true
		},
		SubAgent: false,
	})

	require.Contains(t, tool.Info().Description, "configured provider's search engine")
	require.Contains(t, tool.Info().Description, "Follow up with fetch")

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "sess-123")
	resp, err := runWebSearchTool(t, ctx, tool, WebSearchParams{Query: "golang generics", MaxResults: 5})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)

	require.Equal(t, "Bearer resolved-secret", gotAuth)
	require.Equal(t, providerWebSearchRequest{
		Model:      "gemini-2.5-flash",
		Query:      "golang generics",
		MaxResults: 5,
	}, gotBody)

	require.Len(t, perms.requests, 1)
	require.Equal(t, WebSearchToolName, perms.requests[0].ToolName)
	require.Equal(t, "fetch", perms.requests[0].Action)
	require.Equal(t, workingDir, perms.requests[0].Path)
	require.Equal(t, "Search the web for: golang generics", perms.requests[0].Description)

	require.Contains(t, resp.Content, "Summary:\nGo supports type parameters since Go 1.18.")
	require.Contains(t, resp.Content, "Search queries: golang generics, go 1.24 release")
	require.Contains(t, resp.Content, "1. Go Generics")
	require.Contains(t, resp.Content, "URL: https://go.dev/doc/tutorial/generics")
	require.Contains(t, resp.Content, "Summary: Tutorial on generics in Go.")
	require.Contains(t, resp.Content, "2. Release Notes")
}

func TestWebSearchTool_SubAgentDescription(t *testing.T) {
	providerTool := NewWebSearchTool(nil, WebSearchOptions{
		ProviderModel: func() (config.ProviderWebSearchModel, bool) {
			return config.ProviderWebSearchModel{BaseURL: "https://example.com/v1", Model: "m"}, true
		},
		SubAgent: true,
	})
	require.Contains(t, providerTool.Info().Description, "configured provider's search engine")
	require.Contains(t, providerTool.Info().Description, "Follow up with web_fetch")

	ddgTool := NewWebSearchTool(nil, WebSearchOptions{SubAgent: true})
	require.Contains(t, ddgTool.Info().Description, "DuckDuckGo")
	require.Contains(t, ddgTool.Info().Description, "Follow up with web_fetch")
}

func TestWebSearchTool_PermissionDenied(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("provider endpoint must not be called when permission is denied")
	}))
	t.Cleanup(srv.Close)

	perms := &webSearchPermissionService{
		Broker: pubsub.NewBroker[permission.PermissionRequest](),
		allow:  false,
	}
	tool := NewWebSearchTool(srv.Client(), WebSearchOptions{
		Permissions: perms,
		WorkingDir:  t.TempDir(),
		ProviderModel: func() (config.ProviderWebSearchModel, bool) {
			return config.ProviderWebSearchModel{BaseURL: srv.URL, Model: "m"}, true
		},
	})

	ctx := context.WithValue(context.Background(), SessionIDContextKey, "sess-123")
	resp, err := runWebSearchTool(t, ctx, tool, WebSearchParams{Query: "test"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.True(t, resp.StopTurn)
	require.Len(t, perms.requests, 1)
}

func TestWebSearchTool_FallbackToDuckDuckGoOnProviderFailure(t *testing.T) {
	lastSearchMu.Lock()
	lastSearchTime = time.Time{}
	lastSearchMu.Unlock()

	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`{"error": "upstream down"}`))
	}))
	t.Cleanup(providerSrv.Close)

	page := `<html><body><table>
<tr><td><a class="result-link" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Ffallback.example%2Fdoc">Fallback Doc</a></td></tr>
<tr><td class="result-snippet">Fallback snippet text.</td></tr>
</table></body></html>`
	serveSearchStub(t, http.StatusOK, page)

	tool := NewWebSearchTool(providerSrv.Client(), WebSearchOptions{
		ProviderModel: func() (config.ProviderWebSearchModel, bool) {
			return config.ProviderWebSearchModel{BaseURL: providerSrv.URL, Model: "m"}, true
		},
		SubAgent: true,
	})

	resp, err := runWebSearchTool(t, context.Background(), tool, WebSearchParams{Query: "fallback query"})
	require.NoError(t, err)
	require.False(t, resp.IsError, resp.Content)
	require.Contains(t, resp.Content, "Fallback Doc")
	require.Contains(t, resp.Content, "https://fallback.example/doc")
	require.Contains(t, resp.Content, "Fallback snippet text.")
}

func TestWebSearchTool_ReturnsCombinedErrorWhenProviderAndFallbackFail(t *testing.T) {
	lastSearchMu.Lock()
	lastSearchTime = time.Time{}
	lastSearchMu.Unlock()

	providerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	}))
	t.Cleanup(providerSrv.Close)

	serveSearchStub(t, http.StatusServiceUnavailable, "unavailable")

	tool := NewWebSearchTool(providerSrv.Client(), WebSearchOptions{
		ProviderModel: func() (config.ProviderWebSearchModel, bool) {
			return config.ProviderWebSearchModel{BaseURL: providerSrv.URL, Model: "m"}, true
		},
		SubAgent: true,
	})

	resp, err := runWebSearchTool(t, context.Background(), tool, WebSearchParams{Query: "broken query"})
	require.NoError(t, err)
	require.True(t, resp.IsError)
	require.Contains(t, resp.Content, "provider error")
	require.Contains(t, resp.Content, "fallback error")
}
