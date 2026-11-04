package discover

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/catwalk/pkg/catwalk"
)

// httpClient is shared across all discovery and enrichment calls. It
// has a reasonable timeout so individual requests cannot block forever
// even if the caller forgets to set a context deadline.
var httpClient = &http.Client{
	Timeout: 10 * time.Second,
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

// stripV1Suffix removes a trailing /v1 from a base URL. Enricher
// endpoints (e.g. Ollama's /api/show, LM Studio's /api/v1/models) are
// served at the server root, not under the OpenAI-compatible /v1
// prefix. Since provider configs typically include /v1 in the base URL
// for chat completions, enrichers must strip it before constructing
// their own request paths.
func stripV1Suffix(baseURL string) string {
	return strings.TrimSuffix(strings.TrimRight(baseURL, "/"), "/v1")
}

// describeDiscoveryError wraps a discovery failure with a hint when the
// cause is local (DNS, proxy, timeout, TLS) rather than the provider.
func describeDiscoveryError(cfg Config, err error) error {
	hint := ""
	msg := err.Error()
	switch {
	case strings.Contains(msg, "deadline exceeded") || strings.Contains(msg, "timeout"):
		hint = "the request timed out: the network, DNS, or a proxy may be slow or blocking the connection"
	case strings.Contains(msg, "no such host") || strings.Contains(msg, "dns"):
		hint = "DNS resolution failed: check the host name and the system DNS settings"
	case strings.Contains(msg, "proxyconnect") || strings.Contains(msg, "proxy"):
		hint = "a local proxy (http_proxy/https_proxy) is interfering: unset it or add the host to no_proxy"
	case strings.Contains(msg, "certificate") || strings.Contains(msg, "tls"):
		hint = "TLS verification failed: check the system CA certificates and clock"
	}
	if hint == "" {
		return fmt.Errorf("discover models for provider %s: %w", cfg.ID, err)
	}
	return fmt.Errorf("discover models for provider %s: %w (%s)", cfg.ID, err, hint)
}

// doRequest builds and executes an authenticated HTTP request using the
// shared client. It resolves variable references in the base URL, API
// key, and extra headers via the provided Resolver. The path is joined
// to the base URL with proper slash handling.
func doRequest(ctx context.Context, method, baseURL, path, apiKey string, extraHeaders map[string]string, resolver Resolver, body any) (*http.Response, error) {
	resolvedBase, _ := resolver.ResolveValue(baseURL)
	resolvedKey, _ := resolver.ResolveValue(apiKey)

	url := strings.TrimRight(resolvedBase, "/") + "/" + strings.TrimLeft(path, "/")

	var reqBody *bytes.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	var req *http.Request
	var err error
	if reqBody != nil {
		req, err = http.NewRequestWithContext(ctx, method, url, reqBody)
	} else {
		req, err = http.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, err
	}

	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if resolvedKey != "" {
		req.Header.Set("Authorization", "Bearer "+resolvedKey)
	}
	for k, v := range extraHeaders {
		resolved, err := resolver.ResolveValue(v)
		if err != nil || resolved == "" {
			continue
		}
		req.Header.Set(k, resolved)
	}

	return httpClient.Do(req)
}

// ProviderDiscoveryTimeout bounds a single /models fetch from the UI.
const ProviderDiscoveryTimeout = 10 * time.Second

// Config holds the provider configuration needed for model discovery.
type Config struct {
	ID           string
	BaseURL      string
	APIKey       string
	ExtraHeaders map[string]string
	// Existing models from config — IDs present in this list are skipped
	// during discovery (user-specified models win).
	ExistingModels []catwalk.Model
}

// Resolver resolves variable references (e.g. $ENV_VAR) in config values.
type Resolver interface {
	ResolveValue(val string) (string, error)
}

type modelsResponse struct {
	Data []struct {
		ID                     string         `json:"id"`
		Object                 string         `json:"object"`
		Created                int64          `json:"created"`
		OwnedBy                string         `json:"owned_by"`
		ContextWindow          int64          `json:"context_window"`
		MaxContextLength       int64          `json:"max_context_length"`
		DefaultMaxTokens       int64          `json:"default_max_tokens"`
		SupportsAttachments    bool           `json:"supports_attachments"`
		SupportsVision         bool           `json:"supports_vision"`
		Vision                 bool           `json:"vision"`
		CostPer1MIn            float64        `json:"cost_per_1m_in"`
		CostPer1MOut           float64        `json:"cost_per_1m_out"`
		TokenCounting          string         `json:"token_counting"`
		ReasoningEfforts       []string       `json:"reasoning_efforts"`
		DefaultReasoningEffort string         `json:"default_reasoning_effort"`
		Gateway                map[string]any `json:"gateway"`
	} `json:"data"`
}

// DiscoverModels fetches available models from the provider's /models endpoint.
// It uses the provided context for cancellation and timeout; callers should set
// a deadline (e.g. context.WithTimeout) to avoid blocking indefinitely.
// Models whose IDs already appear in cfg.ExistingModels are skipped —
// user-specified models take precedence.
func DiscoverModels(ctx context.Context, cfg Config, resolver Resolver) ([]catwalk.Model, error) {
	resp, err := doRequest(ctx, http.MethodGet, cfg.BaseURL, "/models", cfg.APIKey, cfg.ExtraHeaders, resolver, nil)
	if err != nil && ctx.Err() == nil {
		resp, err = doRequest(ctx, http.MethodGet, cfg.BaseURL, "/models", cfg.APIKey, cfg.ExtraHeaders, resolver, nil)
	}
	if err != nil {
		return nil, describeDiscoveryError(cfg, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("discover models for provider %s: %s", cfg.ID, resp.Status)
	}

	var modelsResp modelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&modelsResp); err != nil {
		return nil, fmt.Errorf("discover models for provider %s: %w", cfg.ID, err)
	}

	// Start with user-specified models.
	advertised := make(map[string]bool, len(modelsResp.Data))
	codexCatalog := false
	for _, model := range modelsResp.Data {
		advertised[model.ID] = true
		codexCatalog = codexCatalog || model.Gateway["type"] == "codex"
	}
	result := make([]catwalk.Model, 0, len(cfg.ExistingModels))
	existing := make(map[string]int, len(cfg.ExistingModels))
	for _, model := range cfg.ExistingModels {
		gateway, _ := model.Options.ProviderOptions["gateway"].(map[string]any)
		if model.ID == "codex-default" && gateway["type"] == "codex" && codexCatalog && !advertised[model.ID] {
			continue
		}
		existing[model.ID] = len(result)
		result = append(result, model)
	}

	// Append discovered models not already in the list.
	for _, e := range modelsResp.Data {
		contextWindow := e.ContextWindow
		if contextWindow <= 0 {
			contextWindow = e.MaxContextLength
		}
		if index, ok := existing[e.ID]; ok {
			if e.Gateway["type"] == "codex" && result[index].ContextWindow <= 0 && contextWindow > 0 {
				result[index].ContextWindow = contextWindow
			}
			continue
		}
		model := catwalk.Model{
			ID:               e.ID,
			Name:             e.ID,
			ContextWindow:    contextWindow,
			DefaultMaxTokens: e.DefaultMaxTokens,
			SupportsImages:   e.SupportsAttachments || e.SupportsVision || e.Vision,
			CostPer1MIn:      e.CostPer1MIn,
			CostPer1MOut:     e.CostPer1MOut,
			TokenCounting:    e.TokenCounting,
		}
		if e.Gateway["type"] == "codex" {
			model.Options.ProviderOptions = map[string]any{"gateway": e.Gateway}
			model.CanReason = len(e.ReasoningEfforts) > 0
			model.ReasoningLevels = e.ReasoningEfforts
			model.DefaultReasoningEffort = e.DefaultReasoningEffort
		}
		result = append(result, model)
	}

	return result, nil
}
