package tools

import (
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/SpherePrime/CLI/internal/config"
	"github.com/SpherePrime/CLI/internal/permission"
	"github.com/SpherePrime/CLI/vendordeps/fantasy"
)

//go:embed web_search.md.tpl
var webSearchDescriptionTmpl []byte

var webSearchDescriptionTpl = template.Must(
	template.New("webSearchDescription").
		Parse(string(webSearchDescriptionTmpl)),
)

// ProviderWebSearchFunc returns the web search model of a connected provider.
type ProviderWebSearchFunc func() (config.ProviderWebSearchModel, bool)

// WebSearchOptions configures the web_search tool.
type WebSearchOptions struct {
	Permissions   permission.Service
	WorkingDir    string
	Resolver      config.VariableResolver
	ProviderModel ProviderWebSearchFunc
	SubAgent      bool
}

type webSearchDescriptionData struct {
	GhAvailable    bool
	ProviderSearch bool
	SubAgent       bool
}

func webSearchDescription(providerSearch, subAgent bool) string {
	return renderTemplate(webSearchDescriptionTpl, webSearchDescriptionData{
		GhAvailable:    ghAvailable,
		ProviderSearch: providerSearch,
		SubAgent:       subAgent,
	})
}

type webSearchTarget struct {
	baseURL string
	apiKey  string
	model   string
}

func resolveWebSearchTarget(resolver config.VariableResolver, providerModel ProviderWebSearchFunc) (webSearchTarget, bool, error) {
	if providerModel == nil {
		return webSearchTarget{}, false, nil
	}
	found, ok := providerModel()
	if !ok {
		return webSearchTarget{}, false, nil
	}
	baseURL := strings.TrimSpace(found.BaseURL)
	if baseURL != "" && resolver != nil {
		resolved, err := resolver.ResolveValue(baseURL)
		if err != nil {
			return webSearchTarget{}, false, fmt.Errorf("failed to resolve the base URL of provider %s: %w", found.ProviderID, err)
		}
		baseURL = strings.TrimSpace(resolved)
	}
	baseURL = strings.TrimRight(baseURL, "/")
	key := strings.TrimSpace(found.APIKey)
	if key != "" && resolver != nil {
		resolved, err := resolver.ResolveValue(key)
		if err != nil {
			return webSearchTarget{}, false, fmt.Errorf("failed to resolve the API key of provider %s: %w", found.ProviderID, err)
		}
		key = strings.TrimSpace(resolved)
	}
	return webSearchTarget{baseURL: baseURL, apiKey: key, model: found.Model}, true, nil
}

// NewWebSearchTool creates a web search tool, optionally backed by a connected
// provider's /web_search endpoint with DuckDuckGo fallback.
func NewWebSearchTool(client *http.Client, opts ...WebSearchOptions) fantasy.AgentTool {
	if client == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.MaxIdleConns = 100
		transport.MaxIdleConnsPerHost = 10
		transport.IdleConnTimeout = 90 * time.Second

		client = &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		}
	}

	var opt WebSearchOptions
	if len(opts) > 0 {
		opt = opts[0]
	}

	providerSearch := false
	if opt.ProviderModel != nil {
		_, providerSearch = opt.ProviderModel()
	}

	return fantasy.NewParallelAgentTool(
		WebSearchToolName,
		webSearchDescription(providerSearch, opt.SubAgent),
		func(ctx context.Context, params WebSearchParams, call fantasy.ToolCall) (fantasy.ToolResponse, error) {
			if strings.TrimSpace(params.Query) == "" {
				return fantasy.NewTextErrorResponse("query is required"), nil
			}

			maxResults := params.MaxResults
			if maxResults <= 0 {
				maxResults = 10
			}
			if maxResults > 20 {
				maxResults = 20
			}

			if opt.Permissions != nil {
				sessionID := GetSessionFromContext(ctx)
				if sessionID == "" {
					return fantasy.ToolResponse{}, fmt.Errorf("session ID is required for searching the web")
				}

				allowed, err := opt.Permissions.Request(
					ctx,
					permission.CreatePermissionRequest{
						SessionID:   sessionID,
						Path:        opt.WorkingDir,
						ToolCallID:  call.ID,
						ToolName:    WebSearchToolName,
						Action:      "fetch",
						Description: fmt.Sprintf("Search the web for: %s", params.Query),
						Params:      params,
					},
				)
				if err != nil {
					return fantasy.ToolResponse{}, err
				}
				if !allowed {
					return NewPermissionDeniedResponse(), nil
				}
			}

			target, configured, err := resolveWebSearchTarget(opt.Resolver, opt.ProviderModel)
			if err != nil {
				return fantasy.NewTextErrorResponse(err.Error()), nil
			}
			if configured {
				providerResp, providerErr := searchProvider(ctx, client, target, params.Query, maxResults)
				if providerErr == nil {
					slog.Debug("Provider web search completed", "query", params.Query, "model", target.model, "results", len(providerResp.Results))
					return fantasy.NewTextResponse(formatProviderSearchResponse(providerResp, maxResults)), nil
				}

				slog.Warn("Provider web search failed, falling back to DuckDuckGo", "query", params.Query, "model", target.model, "err", providerErr)
				maybeDelaySearch()
				results, ddgErr := searchDuckDuckGo(ctx, client, params.Query, maxResults)
				slog.Debug("Fallback web search completed", "query", params.Query, "results", len(results), "err", ddgErr)
				if ddgErr != nil {
					return fantasy.NewTextErrorResponse(fmt.Sprintf("Failed to search: provider error (%v); fallback error (%v)", providerErr, ddgErr)), nil
				}
				return fantasy.NewTextResponse(formatSearchResults(results)), nil
			}

			maybeDelaySearch()
			results, err := searchDuckDuckGo(ctx, client, params.Query, maxResults)
			slog.Debug("Web search completed", "query", params.Query, "results", len(results), "err", err)
			if err != nil {
				return fantasy.NewTextErrorResponse("Failed to search: " + err.Error()), nil
			}

			return fantasy.NewTextResponse(formatSearchResults(results)), nil
		},
	)
}
