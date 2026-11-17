package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/SpherePrime/CLI/vendordeps/x/net/html"
)

// SearchResult represents a single search result from DuckDuckGo.
type SearchResult struct {
	Title    string
	Link     string
	Snippet  string
	Position int
}

var userAgents = []string{
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/129.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:133.0) Gecko/20100101 Firefox/133.0",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:132.0) Gecko/20100101 Firefox/132.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:133.0) Gecko/20100101 Firefox/133.0",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15",
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.6 Safari/605.1.15",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36 Edg/131.0.0.0",
}

var acceptLanguages = []string{
	"en-US,en;q=0.9",
	"en-US,en;q=0.9,es;q=0.8",
	"en-GB,en;q=0.9,en-US;q=0.8",
	"en-US,en;q=0.5",
	"en-CA,en;q=0.9,en-US;q=0.8",
}

// errSearchRateLimited reports that DuckDuckGo served a bot-check page
// instead of results.
var errSearchRateLimited = errors.New(
	"DuckDuckGo is rate-limiting this machine. " +
		"Do not retry or rephrase; wait a few minutes or fetch known URLs directly",
)

// ddgAnomalyMarkers are substrings of the bot-detection page DuckDuckGo
// Lite serves (with HTTP 200) instead of results once a client trips its
// rate limiter.  Parsing that page yields zero results, which would
// otherwise masquerade as "your query found nothing".
var ddgAnomalyMarkers = []string{
	"anomaly-modal",
	"/anomaly.js",
	"Unfortunately, bots use DuckDuckGo too",
}

// ddgLiteEndpoint is a package var so tests can point the search at a
// local httptest server.
var ddgLiteEndpoint = "https://lite.duckduckgo.com/lite/?q="

func searchDuckDuckGo(ctx context.Context, client *http.Client, query string, maxResults int) ([]SearchResult, error) {
	if maxResults <= 0 {
		maxResults = 10
	}

	searchURL := ddgLiteEndpoint + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	setRandomizedHeaders(req)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute search: %w", err)
	}
	defer resp.Body.Close()

	// A 202 from DuckDuckGo is the anomaly-challenge interstitial, not a
	// result page; report throttling rather than parsing it into an
	// empty result set.
	if resp.StatusCode == http.StatusAccepted {
		return nil, errSearchRateLimited
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("search failed with status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	content := string(body)
	for _, marker := range ddgAnomalyMarkers {
		if strings.Contains(content, marker) {
			return nil, errSearchRateLimited
		}
	}

	return parseLiteSearchResults(content, maxResults)
}

func setRandomizedHeaders(req *http.Request) {
	req.Header.Set("User-Agent", userAgents[rand.IntN(len(userAgents))])
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", acceptLanguages[rand.IntN(len(acceptLanguages))])
	req.Header.Set("Accept-Encoding", "identity")
	req.Header.Set("Connection", "keep-alive")
	req.Header.Set("Upgrade-Insecure-Requests", "1")
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "none")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Cache-Control", "max-age=0")
	if rand.IntN(2) == 0 {
		req.Header.Set("DNT", "1")
	}
}

func parseLiteSearchResults(htmlContent string, maxResults int) ([]SearchResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	var results []SearchResult
	var currentResult *SearchResult

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.ElementNode {
			if n.Data == "a" && hasClass(n, "result-link") {
				if currentResult != nil && currentResult.Link != "" {
					currentResult.Position = len(results) + 1
					results = append(results, *currentResult)
					if len(results) >= maxResults {
						return
					}
				}
				currentResult = &SearchResult{Title: getTextContent(n)}
				for _, attr := range n.Attr {
					if attr.Key == "href" {
						currentResult.Link = cleanDuckDuckGoURL(attr.Val)
						break
					}
				}
			}
			if n.Data == "td" && hasClass(n, "result-snippet") && currentResult != nil {
				currentResult.Snippet = getTextContent(n)
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if len(results) >= maxResults {
				return
			}
			traverse(c)
		}
	}

	traverse(doc)

	if currentResult != nil && currentResult.Link != "" && len(results) < maxResults {
		currentResult.Position = len(results) + 1
		results = append(results, *currentResult)
	}

	return results, nil
}

func hasClass(n *html.Node, class string) bool {
	for _, attr := range n.Attr {
		if attr.Key == "class" {
			if slices.Contains(strings.Fields(attr.Val), class) {
				return true
			}
		}
	}
	return false
}

func getTextContent(n *html.Node) string {
	var text strings.Builder
	var traverse func(*html.Node)
	traverse = func(node *html.Node) {
		if node.Type == html.TextNode {
			text.WriteString(node.Data)
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}
	}
	traverse(n)
	return strings.TrimSpace(text.String())
}

func cleanDuckDuckGoURL(rawURL string) string {
	if strings.HasPrefix(rawURL, "//duckduckgo.com/l/?uddg=") {
		if _, after, ok := strings.Cut(rawURL, "uddg="); ok {
			encoded := after
			if ampIdx := strings.Index(encoded, "&"); ampIdx != -1 {
				encoded = encoded[:ampIdx]
			}
			if decoded, err := url.QueryUnescape(encoded); err == nil {
				return decoded
			}
		}
	}
	return rawURL
}

func formatSearchResults(results []SearchResult) string {
	if len(results) == 0 {
		return "No results found. Try rephrasing your search."
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "Found %d search results:\n\n", len(results))
	for _, result := range results {
		fmt.Fprintf(&sb, "%d. %s\n", result.Position, result.Title)
		fmt.Fprintf(&sb, "   URL: %s\n", result.Link)
		fmt.Fprintf(&sb, "   Summary: %s\n\n", result.Snippet)
	}
	return sb.String()
}

var (
	lastSearchMu   sync.Mutex
	lastSearchTime time.Time
)

// maybeDelaySearch adds a random delay if the last search was recent.
func maybeDelaySearch() {
	lastSearchMu.Lock()
	defer lastSearchMu.Unlock()

	minGap := time.Duration(500+rand.IntN(1500)) * time.Millisecond
	elapsed := time.Since(lastSearchTime)
	if elapsed < minGap {
		time.Sleep(minGap - elapsed)
	}
	lastSearchTime = time.Now()
}

type providerWebSearchRequest struct {
	Model      string `json:"model"`
	Query      string `json:"query"`
	MaxResults int    `json:"max_results"`
}

type providerWebSearchResultItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
}

type providerWebSearchResponse struct {
	Query   string                        `json:"query"`
	Queries []string                      `json:"queries"`
	Answer  string                        `json:"answer"`
	Results []providerWebSearchResultItem `json:"results"`
}

func searchProvider(ctx context.Context, client *http.Client, target webSearchTarget, query string, maxResults int) (*providerWebSearchResponse, error) {
	payload, err := json.Marshal(providerWebSearchRequest{
		Model:      target.model,
		Query:      query,
		MaxResults: maxResults,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal search request: %w", err)
	}

	endpoint := strings.TrimRight(target.baseURL, "/") + "/web_search"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("create search request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if target.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+target.apiKey)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("provider search request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 2*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("read provider search response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("provider search failed with status %d: %s", resp.StatusCode, summarizeErrorBody(string(body)))
	}

	var parsed providerWebSearchResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("decode provider search response: %w", err)
	}
	return &parsed, nil
}

func formatProviderSearchResponse(resp *providerWebSearchResponse, maxResults int) string {
	if resp == nil {
		return "No results found. Try rephrasing your search."
	}

	results := resp.Results
	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}

	answer := strings.TrimSpace(resp.Answer)
	var queries []string
	for _, q := range resp.Queries {
		if trimmed := strings.TrimSpace(q); trimmed != "" {
			queries = append(queries, trimmed)
		}
	}

	if answer == "" && len(results) == 0 {
		return "No results found. Try rephrasing your search."
	}

	var sb strings.Builder
	if answer != "" {
		sb.WriteString("Summary:\n")
		sb.WriteString(answer)
		sb.WriteString("\n\n")
	}
	if len(queries) > 0 {
		fmt.Fprintf(&sb, "Search queries: %s\n\n", strings.Join(queries, ", "))
	}
	if len(results) > 0 {
		fmt.Fprintf(&sb, "Found %d search results:\n\n", len(results))
		for i, r := range results {
			title := strings.TrimSpace(r.Title)
			if title == "" {
				title = strings.TrimSpace(r.URL)
			}
			fmt.Fprintf(&sb, "%d. %s\n", i+1, title)
			if u := strings.TrimSpace(r.URL); u != "" {
				fmt.Fprintf(&sb, "   URL: %s\n", u)
			}
			if snippet := strings.TrimSpace(r.Snippet); snippet != "" {
				fmt.Fprintf(&sb, "   Summary: %s\n", snippet)
			}
			sb.WriteString("\n")
		}
	}
	return strings.TrimSpace(sb.String()) + "\n"
}
