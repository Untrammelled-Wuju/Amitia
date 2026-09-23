package exa

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
	providerID      = "exa"
	defaultEndpoint = "https://api.exa.ai/search"
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
	Query              string   `json:"query"`
	NumResults         int      `json:"numResults"`
	IncludeDomains     []string `json:"includeDomains,omitempty"`
	ExcludeDomains     []string `json:"excludeDomains,omitempty"`
	StartPublishedDate string   `json:"startPublishedDate,omitempty"`
	EndPublishedDate   string   `json:"endPublishedDate,omitempty"`
	Moderation         bool     `json:"moderation"`
	Type               string   `json:"type,omitempty"`
}

type responseResult struct {
	Title         string   `json:"title"`
	URL           string   `json:"url"`
	PublishedDate string   `json:"publishedDate"`
	Author        string   `json:"author"`
	Text          string   `json:"text"`
	Highlights    []string `json:"highlights"`
	Summary       string   `json:"summary"`
}

type responsePayload struct {
	Results     []responseResult `json:"results"`
	CostDollars struct {
		Total float64 `json:"total"`
	} `json:"costDollars"`
}

type errorPayload struct {
	Error string `json:"error"`
	Tag   string `json:"tag"`
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

func (p *Provider) ID() string { return providerID }

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
		SearchKinds:         []search.SearchKind{search.SearchKindWeb},
		LanguageFilter:      false,
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
		Query:          strings.TrimSpace(req.Query),
		NumResults:     limit,
		IncludeDomains: compactDomains(req.Domains),
		ExcludeDomains: compactDomains(req.ExcludeDomains),
		Moderation:     req.SafeSearch != search.SafeSearchOff,
		Type:           "auto",
	}
	if req.TimeRange != nil {
		if req.TimeRange.From != nil {
			payload.StartPublishedDate = req.TimeRange.From.UTC().Format(time.RFC3339Nano)
		}
		if req.TimeRange.To != nil {
			payload.EndPublishedDate = req.TimeRange.To.UTC().Format(time.RFC3339Nano)
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
	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.Header.Set("x-api-key", credential)
	return httpReq, nil
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
		snippet := strings.TrimSpace(item.Summary)
		if snippet == "" && len(item.Highlights) > 0 {
			snippet = strings.Join(item.Highlights, " … ")
		}
		if snippet == "" {
			snippet = strings.TrimSpace(item.Text)
		}
		publishedAt := parsePublishedDate(item.PublishedDate)
		meta := search.SearchResultMetadata{}
		if author := strings.TrimSpace(item.Author); author != "" {
			meta.Authors = []string{author}
		}
		results = append(results, search.SearchResult{
			Rank:        i + 1,
			Title:       item.Title,
			URL:         item.URL,
			Snippet:     snippet,
			PublishedAt: publishedAt,
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
		Usage:      search.ProviderUsage{CostUSD: payload.CostDollars.Total},
	}, nil
}

func (p *Provider) mapHTTPError(status int, body []byte) (search.ProviderSearchResponse, error) {
	var payload errorPayload
	_ = json.Unmarshal(body, &payload)
	cause := errorFromMessage(payload.Error)
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_AUTH_FAILED, providerID, status, cause)
	case status == http.StatusTooManyRequests:
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_RATE_LIMITED, providerID, status, cause)
	case status >= 500:
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_REQUEST_FAILED, providerID, status, cause)
	default:
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_REQUEST_REJECTED, providerID, status, cause)
	}
}

func parsePublishedDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339, "2006-01-02"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}

type providerMessageError string

func (e providerMessageError) Error() string { return string(e) }

func errorFromMessage(message string) error {
	message = strings.TrimSpace(message)
	if message == "" {
		return nil
	}
	return providerMessageError(message)
}
