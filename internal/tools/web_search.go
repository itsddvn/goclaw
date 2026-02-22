package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Matching TS src/agents/tools/web-search.ts constants.
const (
	defaultSearchCount   = 5
	maxSearchCount       = 10
	searchTimeoutSeconds = 30
	braveSearchEndpoint  = "https://api.search.brave.com/res/v1/web/search"
	webSearchUserAgent   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_7_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

// SearchProvider abstracts a web search backend.
type SearchProvider interface {
	Search(ctx context.Context, params searchParams) ([]searchResult, error)
	Name() string
}

type searchParams struct {
	Query      string
	Count      int
	Country    string
	SearchLang string
	UILang     string
	Freshness  string
}

type searchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
}

// --- Brave Search Provider ---

type braveSearchProvider struct {
	apiKey string
	client *http.Client
}

func newBraveSearchProvider(apiKey string) *braveSearchProvider {
	return &braveSearchProvider{
		apiKey: apiKey,
		client: &http.Client{Timeout: time.Duration(searchTimeoutSeconds) * time.Second},
	}
}

func (p *braveSearchProvider) Name() string { return "brave" }

func (p *braveSearchProvider) Search(ctx context.Context, params searchParams) ([]searchResult, error) {
	q := url.Values{}
	q.Set("q", params.Query)
	q.Set("count", fmt.Sprintf("%d", params.Count))

	if params.Country != "" {
		q.Set("country", params.Country)
	}
	if params.SearchLang != "" {
		q.Set("search_lang", params.SearchLang)
	}
	if params.UILang != "" {
		q.Set("ui_lang", params.UILang)
	}
	if f := normalizeFreshness(params.Freshness); f != "" {
		q.Set("freshness", f)
	}

	reqURL := braveSearchEndpoint + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Subscription-Token", p.apiKey)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("brave API returned %d: %s", resp.StatusCode, truncateStr(string(body), 200))
	}

	var braveResp struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
			} `json:"results"`
		} `json:"web"`
	}

	if err := json.Unmarshal(body, &braveResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	results := make([]searchResult, 0, len(braveResp.Web.Results))
	for _, r := range braveResp.Web.Results {
		results = append(results, searchResult{
			Title:       r.Title,
			URL:         r.URL,
			Description: r.Description,
		})
	}
	return results, nil
}

// --- DuckDuckGo Search Provider ---

type duckDuckGoSearchProvider struct {
	client *http.Client
}

func newDuckDuckGoSearchProvider() *duckDuckGoSearchProvider {
	return &duckDuckGoSearchProvider{
		client: &http.Client{Timeout: time.Duration(searchTimeoutSeconds) * time.Second},
	}
}

func (p *duckDuckGoSearchProvider) Name() string { return "duckduckgo" }

func (p *duckDuckGoSearchProvider) Search(ctx context.Context, params searchParams) ([]searchResult, error) {
	searchURL := fmt.Sprintf("https://html.duckduckgo.com/html/?q=%s", url.QueryEscape(params.Query))

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("User-Agent", webSearchUserAgent)

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	return extractDDGResults(string(body), params.Count)
}

var (
	ddgLinkRe    = regexp.MustCompile(`<a[^>]*class="[^"]*result__a[^"]*"[^>]*href="([^"]+)"[^>]*>([\s\S]*?)</a>`)
	ddgSnippetRe = regexp.MustCompile(`<a class="result__snippet[^"]*".*?>([\s\S]*?)</a>`)
	htmlTagRe    = regexp.MustCompile(`<[^>]+>`)
)

func extractDDGResults(html string, count int) ([]searchResult, error) {
	linkMatches := ddgLinkRe.FindAllStringSubmatch(html, count+5)
	if len(linkMatches) == 0 {
		return nil, nil
	}

	snippetMatches := ddgSnippetRe.FindAllStringSubmatch(html, count+5)

	var results []searchResult
	for i := 0; i < len(linkMatches) && i < count; i++ {
		rawURL := linkMatches[i][1]
		title := strings.TrimSpace(htmlTagRe.ReplaceAllString(linkMatches[i][2], ""))

		// DDG wraps URLs with redirect — extract real URL from uddg= param
		if strings.Contains(rawURL, "uddg=") {
			if u, err := url.QueryUnescape(rawURL); err == nil {
				if idx := strings.Index(u, "uddg="); idx != -1 {
					extracted := u[idx+5:]
					// uddg value may have trailing &params
					if ampIdx := strings.Index(extracted, "&"); ampIdx != -1 {
						extracted = extracted[:ampIdx]
					}
					rawURL = extracted
				}
			}
		}

		desc := ""
		if i < len(snippetMatches) {
			desc = strings.TrimSpace(htmlTagRe.ReplaceAllString(snippetMatches[i][1], ""))
		}

		results = append(results, searchResult{
			Title:       title,
			URL:         rawURL,
			Description: desc,
		})
	}

	return results, nil
}

// --- Freshness validation (matching TS) ---

var (
	freshnessShortcuts = map[string]bool{"pd": true, "pw": true, "pm": true, "py": true}
	freshnessRangeRe   = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})to(\d{4}-\d{2}-\d{2})$`)
)

func normalizeFreshness(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if v == "" {
		return ""
	}
	if freshnessShortcuts[v] {
		return v
	}
	if m := freshnessRangeRe.FindStringSubmatch(v); len(m) == 3 {
		start, errS := time.Parse("2006-01-02", m[1])
		end, errE := time.Parse("2006-01-02", m[2])
		if errS == nil && errE == nil && !start.After(end) {
			return v
		}
	}
	return ""
}

// --- WebSearchTool ---

// WebSearchTool implements the web_search tool matching TS src/agents/tools/web-search.ts.
type WebSearchTool struct {
	providers []SearchProvider
	cache     *webCache
}

// WebSearchConfig holds configuration for the web search tool.
type WebSearchConfig struct {
	BraveAPIKey    string
	BraveEnabled   bool
	BraveMaxResults int
	DDGEnabled     bool
	DDGMaxResults  int
	CacheTTL       time.Duration
}

func NewWebSearchTool(cfg WebSearchConfig) *WebSearchTool {
	var providers []SearchProvider

	// Priority: Brave > DuckDuckGo (matching TS)
	if cfg.BraveEnabled && cfg.BraveAPIKey != "" {
		providers = append(providers, newBraveSearchProvider(cfg.BraveAPIKey))
	}
	if cfg.DDGEnabled {
		providers = append(providers, newDuckDuckGoSearchProvider())
	}

	if len(providers) == 0 {
		return nil
	}

	ttl := cfg.CacheTTL
	if ttl <= 0 {
		ttl = defaultCacheTTL
	}

	return &WebSearchTool{
		providers: providers,
		cache:     newWebCache(defaultCacheMaxEntries, ttl),
	}
}

func (t *WebSearchTool) Name() string { return "web_search" }

func (t *WebSearchTool) Description() string {
	return "Search the web for current information. Returns titles, URLs, and snippets from search results."
}

func (t *WebSearchTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "Search query string.",
			},
			"count": map[string]interface{}{
				"type":        "number",
				"description": "Number of results to return (1-10).",
				"minimum":     1.0,
				"maximum":     float64(maxSearchCount),
			},
			"country": map[string]interface{}{
				"type":        "string",
				"description": "2-letter country code for region-specific results (e.g., 'DE', 'US', 'ALL'). Default: 'US'.",
			},
			"search_lang": map[string]interface{}{
				"type":        "string",
				"description": "ISO language code for search results (e.g., 'de', 'en', 'fr').",
			},
			"ui_lang": map[string]interface{}{
				"type":        "string",
				"description": "ISO language code for UI elements.",
			},
			"freshness": map[string]interface{}{
				"type":        "string",
				"description": "Filter results by discovery time. Supports 'pd' (past day), 'pw' (past week), 'pm' (past month), 'py' (past year), and date range 'YYYY-MM-DDtoYYYY-MM-DD'.",
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) Execute(ctx context.Context, args map[string]interface{}) *Result {
	query, _ := args["query"].(string)
	if query == "" {
		return ErrorResult("query is required")
	}

	count := defaultSearchCount
	if c, ok := args["count"].(float64); ok && int(c) >= 1 && int(c) <= maxSearchCount {
		count = int(c)
	}

	country, _ := args["country"].(string)
	searchLang, _ := args["search_lang"].(string)
	uiLang, _ := args["ui_lang"].(string)
	freshness, _ := args["freshness"].(string)

	params := searchParams{
		Query:      query,
		Count:      count,
		Country:    country,
		SearchLang: searchLang,
		UILang:     uiLang,
		Freshness:  freshness,
	}

	// Check cache
	cacheKey := buildSearchCacheKey(params)
	if cached, ok := t.cache.get(cacheKey); ok {
		slog.Debug("web_search cache hit", "query", query)
		return NewResult(cached)
	}

	// Try providers in order (first success wins)
	var lastErr error
	for _, provider := range t.providers {
		results, err := provider.Search(ctx, params)
		if err != nil {
			slog.Warn("web_search provider failed", "provider", provider.Name(), "error", err)
			lastErr = err
			continue
		}

		formatted := formatSearchResults(query, results, provider.Name())
		wrapped := wrapExternalContent(formatted, "Web Search", false)

		t.cache.set(cacheKey, wrapped)
		return NewResult(wrapped)
	}

	if lastErr != nil {
		return ErrorResult(fmt.Sprintf("all search providers failed: %v", lastErr))
	}
	return ErrorResult("no search providers configured")
}

func buildSearchCacheKey(p searchParams) string {
	parts := []string{
		p.Query,
		fmt.Sprintf("%d", p.Count),
		orDefault(p.Country, "default"),
		orDefault(p.SearchLang, "default"),
		orDefault(p.UILang, "default"),
		orDefault(p.Freshness, "default"),
	}
	return strings.Join(parts, ":")
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func formatSearchResults(query string, results []searchResult, provider string) string {
	if len(results) == 0 {
		return fmt.Sprintf("No results found for: %s", query)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Search results for: %s (via %s)\n\n", query, provider))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("%d. %s\n   %s\n", i+1, r.Title, r.URL))
		if r.Description != "" {
			sb.WriteString(fmt.Sprintf("   %s\n", r.Description))
		}
		sb.WriteByte('\n')
	}
	return sb.String()
}

func truncateStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
