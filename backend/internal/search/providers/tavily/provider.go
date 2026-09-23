package tavily

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
)

const (
	providerID      = "tavily"
	defaultEndpoint = "https://api.tavily.com/search"
	maxResults      = 20
	userAgent       = "Amitia/0.1.0"
)

type Provider struct {
	endpoint      string
	credential    string
	credentialRef string
	enabled       bool
	transport     *search.SecureTransport
	maxBytes      int64
}

type requestPayload struct {
	Query                string   `json:"query"`
	SearchDepth          string   `json:"search_depth"`
	ChunksPerSource      int      `json:"chunks_per_source"`
	MaxResults           int      `json:"max_results"`
	Topic                string   `json:"topic"`
	StartDate            string   `json:"start_date,omitempty"`
	EndDate              string   `json:"end_date,omitempty"`
	IncludePublishedDate bool     `json:"include_published_date"`
	IncludeAnswer        bool     `json:"include_answer"`
	IncludeRawContent    bool     `json:"include_raw_content"`
	IncludeImages        bool     `json:"include_images"`
	IncludeDomains       []string `json:"include_domains,omitempty"`
	ExcludeDomains       []string `json:"exclude_domains,omitempty"`
	Language             string   `json:"language,omitempty"`
	FilterByLanguage     bool     `json:"filter_by_language"`
	AutoParameters       bool     `json:"auto_parameters"`
	SafeSearch           bool     `json:"safe_search"`
	IncludeUsage         bool     `json:"include_usage"`
}

type responseResult struct {
	Title         string  `json:"title"`
	URL           string  `json:"url"`
	Content       string  `json:"content"`
	Score         float64 `json:"score"`
	PublishedDate string  `json:"published_date"`
	Favicon       string  `json:"favicon"`
}

type responsePayload struct {
	Results []responseResult `json:"results"`
	Usage   struct {
		Credits float64 `json:"credits"`
	} `json:"usage"`
}

type errorPayload struct {
	Detail json.RawMessage `json:"detail"`
}

func NewProvider(credential, credentialRef, endpoint string, enabled bool) *Provider {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &Provider{
		endpoint:      endpoint,
		credential:    credential,
		credentialRef: credentialRef,
		enabled:       enabled,
		transport:     search.NewSecureTransport(),
		maxBytes:      2 * 1024 * 1024,
	}
}

func (p *Provider) ID() string                       { return providerID }
func (p *Provider) SetCredential(credential string)  { p.credential = credential }
func (p *Provider) WithCredential(credential string) { p.credential = credential }

func (p *Provider) SetMaxResponseBytes(limit int64) {
	if limit > 0 {
		p.maxBytes = limit
	}
}

func (p *Provider) Capabilities() search.ProviderCapabilities {
	return search.ProviderCapabilities{
		GeneralWeb:          true,
		SearchKinds:         []search.SearchKind{search.SearchKindWeb, search.SearchKindNews},
		LanguageFilter:      true,
		CountryFilter:       false,
		SafeSearch:          true,
		Pagination:          false,
		TimeRangeFilter:     true,
		DomainFilter:        true,
		ExcludeDomainFilter: true,
		MaxResults:          maxResults,
	}
}

func (p *Provider) Health(ctx context.Context) search.ProviderHealth {
	if !p.enabled {
		return search.ProviderHealthDisabled
	}
	if strings.TrimSpace(p.endpoint) == "" {
		return search.ProviderHealthMisconfigured
	}
	if _, err := p.transport.ValidateEndpoint(ctx, p.endpoint); err != nil {
		return search.ProviderHealthMisconfigured
	}
	if strings.TrimSpace(p.credential) == "" && strings.TrimSpace(p.credentialRef) == "" {
		return search.ProviderHealthReady
	}
	return search.ProviderHealthReady
}

func (p *Provider) Search(ctx context.Context, req search.SearchRequest) (search.ProviderSearchResponse, error) {
	if !p.enabled {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_DISABLED, providerID, false, nil)
	}
	credential := search.ProviderCredentialFromContext(ctx)
	if credential == "" {
		credential = search.EngineCredentialFromContext(ctx, providerID)
	}
	if credential == "" {
		credential = p.credential
	}
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, providerID, false, nil)
	}
	validated, err := p.transport.ValidateEndpoint(ctx, p.endpoint)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_BLOCKED_BY_NETWORK, providerID, false, err)
	}
	httpReq, err := p.buildRequest(ctx, req, credential)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	client := p.transport.PinHTTPClient(validated, 12*time.Second)
	resp, err := client.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil {
			return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_CANCELLED, providerID, false, ctx.Err())
		}
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_TIMEOUT, providerID, true, err)
	}
	defer resp.Body.Close()
	body, err := p.readBody(resp.Body)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	result, mappedErr := p.handleResponse(resp.StatusCode, body, req)
	if searchErr, ok := mappedErr.(*search.Error); ok && searchErr.Code == search.SEARCH_PROVIDER_RATE_LIMITED {
		searchErr.RetryAfter = search.ParseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
	}
	return result, mappedErr
}

func (p *Provider) buildRequest(ctx context.Context, req search.SearchRequest, credential string) (*http.Request, error) {
	limit := req.Limit
	if limit <= 0 {
		limit = search.DefaultLimit
	}
	if limit > maxResults {
		limit = maxResults
	}
	payload := requestPayload{
		Query:                strings.TrimSpace(req.Query),
		SearchDepth:          "basic",
		ChunksPerSource:      3,
		MaxResults:           limit,
		Topic:                mapTopic(req.Kind),
		IncludePublishedDate: true,
		IncludeAnswer:        false,
		IncludeRawContent:    false,
		IncludeImages:        false,
		IncludeDomains:       compactDomains(req.Domains),
		ExcludeDomains:       compactDomains(req.ExcludeDomains),
		Language:             strings.TrimSpace(req.Language),
		FilterByLanguage:     strings.TrimSpace(req.Language) != "",
		AutoParameters:       false,
		SafeSearch:           req.SafeSearch != search.SafeSearchOff,
		IncludeUsage:         true,
	}
	if req.TimeRange != nil {
		if req.TimeRange.From != nil {
			payload.StartDate = req.TimeRange.From.UTC().Format("2006-01-02")
		}
		if req.TimeRange.To != nil {
			payload.EndDate = req.TimeRange.To.UTC().Format("2006-01-02")
		}
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, search.NewError(search.SEARCH_PROVIDER_REQUEST_FAILED, providerID, false, err)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, search.NewError(search.SEARCH_PROVIDER_REQUEST_FAILED, providerID, false, err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+credential)
	httpReq.Header.Set("User-Agent", userAgent)
	return httpReq, nil
}

func mapTopic(kind search.SearchKind) string {
	if kind == search.SearchKindNews {
		return "news"
	}
	return "general"
}

func compactDomains(values []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		value = strings.TrimPrefix(value, "https://")
		value = strings.TrimPrefix(value, "http://")
		value = strings.TrimSuffix(value, "/")
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func (p *Provider) readBody(body io.Reader) ([]byte, error) {
	limit := p.maxBytes
	if limit <= 0 {
		limit = 2 * 1024 * 1024
	}
	data, err := io.ReadAll(io.LimitReader(body, limit+1))
	if err != nil {
		return nil, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, providerID, false, err)
	}
	if int64(len(data)) > limit {
		return nil, search.NewError(search.SEARCH_PROVIDER_RESPONSE_TOO_LARGE, providerID, false, nil)
	}
	return data, nil
}

func (p *Provider) handleResponse(status int, body []byte, req search.SearchRequest) (search.ProviderSearchResponse, error) {
	if status >= 400 {
		return p.mapHTTPError(status, body)
	}
	var payload responsePayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, providerID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for i, item := range payload.Results {
		meta := search.SearchResultMetadata{ThumbnailURL: strings.TrimSpace(item.Favicon)}
		results = append(results, search.SearchResult{
			Rank:        i + 1,
			Title:       item.Title,
			URL:         item.URL,
			Snippet:     item.Content,
			PublishedAt: parsePublishedDate(item.PublishedDate),
			Language:    strings.TrimSpace(req.Language),
			Source: search.SearchSourceMetadata{
				Provider:     providerID,
				ProviderRank: i + 1,
				OriginalURL:  item.URL,
			},
			Metadata: meta,
		})
	}
	return search.ProviderSearchResponse{
		Results:    results,
		HasMore:    false,
		HTTPStatus: status,
		RawBytes:   len(body),
		Usage:      search.ProviderUsage{Credits: payload.Usage.Credits},
	}, nil
}

func (p *Provider) mapHTTPError(status int, body []byte) (search.ProviderSearchResponse, error) {
	cause := parseError(body)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_AUTH_FAILED, providerID, status, cause)
	case status == http.StatusTooManyRequests:
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_RATE_LIMITED, providerID, status, cause)
	case status >= 500:
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_REQUEST_FAILED, providerID, status, cause)
	default:
		// Tavily 432/433 are account/plan limit responses. They are not transient
		// transport failures and therefore must not spin the retry/circuit loop.
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_REQUEST_REJECTED, providerID, status, cause)
	}
}

func parsePublishedDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC1123, time.RFC1123Z, time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}

type providerMessageError string

func (e providerMessageError) Error() string { return string(e) }

func parseError(body []byte) error {
	if len(body) == 0 {
		return nil
	}
	var payload errorPayload
	if err := json.Unmarshal(body, &payload); err == nil && len(payload.Detail) > 0 {
		var message string
		if json.Unmarshal(payload.Detail, &message) == nil && strings.TrimSpace(message) != "" {
			return providerMessageError(strings.TrimSpace(message))
		}
		var object struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(payload.Detail, &object) == nil && strings.TrimSpace(object.Error) != "" {
			return providerMessageError(strings.TrimSpace(object.Error))
		}
	}
	return nil
}
