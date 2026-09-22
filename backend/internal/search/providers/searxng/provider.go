package searxng

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
)

const providerID = "searxng"

const defaultMaxResults = 20

type resultItem struct {
	URL       string  `json:"url"`
	Title     string  `json:"title"`
	Content   string  `json:"content"`
	Engine    string  `json:"engine"`
	Score     float64 `json:"score"`
	Published string  `json:"publishedDate"`
}

type response struct {
	Results []resultItem `json:"results"`
}

type Provider struct {
	endpoint  string
	enabled   bool
	transport *search.SecureTransport
	maxBytes  int64
}

func NewProvider(endpoint string, enabled, allowHTTP, allowPrivate bool) *Provider {
	ep := strings.TrimSpace(endpoint)
	return &Provider{
		endpoint:  ep,
		enabled:   enabled,
		transport: search.NewConfiguredTransport(allowHTTP, allowPrivate, true),
		maxBytes:  2 * 1024 * 1024,
	}
}

func (p *Provider) ID() string {
	return providerID
}

func (p *Provider) SetMaxResponseBytes(limit int64) {
	if limit > 0 {
		p.maxBytes = limit
	}
}

func (p *Provider) Capabilities() search.ProviderCapabilities {
	return search.ProviderCapabilities{
		GeneralWeb:      true,
		SearchKinds:     []search.SearchKind{search.SearchKindWeb, search.SearchKindNews, search.SearchKindAcademic, search.SearchKindCode, search.SearchKindImage, search.SearchKindVideo},
		LanguageFilter:  true,
		CountryFilter:   false,
		SafeSearch:      true,
		Pagination:      true,
		TimeRangeFilter: true,
		DomainFilter:    true,
		MaxResults:      defaultMaxResults,
	}
}

func (p *Provider) Search(ctx context.Context, req search.SearchRequest) (search.ProviderSearchResponse, error) {
	if !p.enabled {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_DISABLED, providerID, false, nil)
	}
	if p.endpoint == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_NOT_CONFIGURED, providerID, false, nil)
	}
	validated, err := p.transport.ValidateEndpoint(ctx, p.endpoint)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_BLOCKED_BY_NETWORK, providerID, false, err)
	}
	client := p.transport.PinHTTPClient(validated, 12*time.Second)
	httpReq, err := p.buildRequest(ctx, req)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_CANCELLED, providerID, false, ctx.Err())
		}
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_TIMEOUT, providerID, true, err)
	}
	defer httpResp.Body.Close()
	body, err := p.readBody(httpResp.Body)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	if httpResp.StatusCode >= 400 {
		return search.ProviderSearchResponse{}, p.httpError(httpResp.StatusCode)
	}
	var payload response
	if err := json.Unmarshal(body, &payload); err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, providerID, false, err)
	}
	limit := req.Limit
	if limit <= 0 || limit > defaultMaxResults {
		limit = defaultMaxResults
	}
	if len(payload.Results) > limit {
		payload.Results = payload.Results[:limit]
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for i, item := range payload.Results {
		published := parsePublished(item.Published)
		results = append(results, search.SearchResult{
			Rank:        i + 1,
			Title:       strings.TrimSpace(item.Title),
			URL:         strings.TrimSpace(item.URL),
			Snippet:     strings.TrimSpace(item.Content),
			PublishedAt: published,
			Source: search.SearchSourceMetadata{
				Provider:     providerID,
				ProviderRank: i + 1,
				OriginalURL:  strings.TrimSpace(item.URL),
			},
		})
	}
	return search.ProviderSearchResponse{Results: results, HasMore: len(payload.Results) >= limit, HTTPStatus: httpResp.StatusCode, RawBytes: len(body)}, nil
}

func (p *Provider) Health(ctx context.Context) search.ProviderHealth {
	if !p.enabled {
		return search.ProviderHealthDisabled
	}
	if p.endpoint == "" {
		return search.ProviderHealthMisconfigured
	}
	if _, err := p.transport.ValidateEndpoint(ctx, p.endpoint); err != nil {
		return search.ProviderHealthMisconfigured
	}
	return search.ProviderHealthReady
}

func (p *Provider) buildRequest(ctx context.Context, req search.SearchRequest) (*http.Request, error) {
	u, err := url.Parse(p.endpoint)
	if err != nil {
		return nil, search.NewError(search.SEARCH_PROVIDER_REQUEST_FAILED, providerID, false, err)
	}
	path := strings.TrimRight(u.Path, "/")
	if !strings.HasSuffix(path, "/search") {
		path += "/search"
	}
	u.Path = path
	q := u.Query()
	q.Set("q", queryWithDomains(req.Query, req.Domains))
	q.Set("format", "json")
	if req.Language != "" {
		q.Set("language", req.Language)
	}
	if req.SafeSearch != "" {
		safe := 1
		if req.SafeSearch == search.SafeSearchOff {
			safe = 0
		} else if req.SafeSearch == search.SafeSearchStrict {
			safe = 2
		}
		q.Set("safesearch", strconv.Itoa(safe))
	}
	if req.Offset > 0 {
		limit := req.Limit
		if limit <= 0 {
			limit = defaultMaxResults
		}
		q.Set("pageno", strconv.Itoa(req.Offset/limit+1))
	}
	if req.TimeRange != nil {
		q.Set("time_range", mapTimeRange(req.TimeRange))
	}
	if category := mapCategory(req.Kind); category != "" {
		q.Set("categories", category)
	}
	u.RawQuery = q.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, search.NewError(search.SEARCH_PROVIDER_REQUEST_FAILED, providerID, false, err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("User-Agent", "Amitia/1.0")
	return httpReq, nil
}

func queryWithDomains(query string, domains []string) string {
	query = strings.TrimSpace(query)
	if len(domains) == 0 {
		return query
	}
	parts := make([]string, 0, len(domains))
	for _, domain := range domains {
		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}
		parts = append(parts, "site:"+domain)
	}
	if len(parts) == 0 {
		return query
	}
	if len(parts) == 1 {
		return strings.TrimSpace(query + " " + parts[0])
	}
	return strings.TrimSpace(query + " (" + strings.Join(parts, " OR ") + ")")
}

func mapTimeRange(filter *search.TimeRangeFilter) string {
	if filter == nil {
		return ""
	}
	now := time.Now().UTC()
	if filter.From != nil {
		age := now.Sub(filter.From.UTC())
		if age <= 48*time.Hour {
			return "day"
		}
		if age <= 10*24*time.Hour {
			return "week"
		}
		if age <= 45*24*time.Hour {
			return "month"
		}
		return "year"
	}
	return ""
}

func (p *Provider) readBody(body io.Reader) ([]byte, error) {
	limit := p.maxBytes
	if limit <= 0 {
		limit = 2 * 1024 * 1024
	}
	limited := io.LimitReader(body, limit+1)
	payload, err := io.ReadAll(limited)
	if err != nil {
		return nil, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, providerID, false, err)
	}
	if int64(len(payload)) > limit {
		return nil, search.NewError(search.SEARCH_PROVIDER_RESPONSE_TOO_LARGE, providerID, false, nil)
	}
	return payload, nil
}

func (p *Provider) httpError(status int) error {
	if search.IsAuthError(status) {
		return search.WrapHTTPError(search.SEARCH_PROVIDER_AUTH_FAILED, providerID, status, nil)
	}
	if search.IsRateLimited(status) {
		return search.WrapHTTPError(search.SEARCH_PROVIDER_RATE_LIMITED, providerID, status, nil)
	}
	if search.IsServerError(status) {
		return search.WrapHTTPError(search.SEARCH_PROVIDER_REQUEST_FAILED, providerID, status, nil)
	}
	return search.WrapHTTPError(search.SEARCH_PROVIDER_REQUEST_REJECTED, providerID, status, nil)
}

func mapCategory(kind search.SearchKind) string {
	switch search.NormalizeKind(kind) {
	case search.SearchKindNews:
		return "news"
	case search.SearchKindImage:
		return "images"
	case search.SearchKindVideo:
		return "videos"
	case search.SearchKindAcademic:
		return "science"
	default:
		return "general"
	}
}

func parsePublished(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}
