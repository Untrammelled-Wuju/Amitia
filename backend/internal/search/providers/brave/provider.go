package brave

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

const (
	providerID      = "brave"
	defaultEndpoint = "https://api.search.brave.com/res/v1/web/search"
	maxResults      = 10
	userAgent       = "Amitia/0.1.0"
)

type braveWebResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Age         string `json:"age"`
	DisplayURL  string `json:"display_url"`
}

type braveWebResponse struct {
	Type    string             `json:"type"`
	Results []braveWebResult   `json:"results"`
}

type braveResponse struct {
	Type    string            `json:"type"`
	Query   json.RawMessage   `json:"query"`
	Web     *braveWebResponse `json:"web"`
	Message string            `json:"message"`
	Mixed   json.RawMessage   `json:"mixed"`
}

type braveErrorDetail struct {
	Code    string `json:"code"`
	Detail  string `json:"detail"`
	Message string `json:"message"`
}

type braveErrorResponse struct {
	Type  string           `json:"type"`
	Error braveErrorDetail `json:"error"`
}

type safeSearchMode string

const (
	safeSearchOff      safeSearchMode = "off"
	safeSearchModerate safeSearchMode = "moderate"
	safeSearchStrict   safeSearchMode = "strict"
)

type Provider struct {
	endpoint      string
	credential    string
	credentialRef string
	enabled       bool
	transport     *search.SecureTransport
	normalizer    *search.Normalizer
	maxBytes      int64
}

func NewProvider(credential, credentialRef, endpoint string, enabled bool) *Provider {
	ep := strings.TrimSpace(endpoint)
	if ep == "" {
		ep = defaultEndpoint
	}
	return &Provider{
		endpoint:      ep,
		credential:    credential,
		credentialRef: credentialRef,
		enabled:       enabled,
		transport:     search.NewSecureTransport(),
		normalizer:    search.NewNormalizer(),
		maxBytes:      2 * 1024 * 1024,
	}
}

func (p *Provider) WithCredential(cred string) {
	p.credential = cred
}

func (p *Provider) SetCredential(cred string) {
	p.credential = cred
}

func (p *Provider) ID() string {
	return providerID
}

func (p *Provider) Capabilities() search.ProviderCapabilities {
	return search.ProviderCapabilities{
		GeneralWeb:     true,
		LanguageFilter: true,
		CountryFilter:  true,
		SafeSearch:     true,
		Pagination:     true,
		MaxResults:     maxResults,
	}
}

func (p *Provider) Search(ctx context.Context, req search.GeneralSearchRequest) (search.ProviderSearchResponse, error) {
	if !p.enabled {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_DISABLED, providerID, false, nil)
	}
	if p.credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, providerID, false, nil)
	}
	validated, terr := p.transport.ValidateEndpoint(ctx, p.endpoint)
	if terr != nil {
		var sErr *search.Error
		if AsSearchError(terr, &sErr) {
			return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_BLOCKED_BY_NETWORK, providerID, false, sErr)
		}
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_BLOCKED_BY_NETWORK, providerID, false, terr)
	}
	client := p.transport.PinHTTPClient(validated, 10*time.Second)
	httpReq, err := p.buildRequest(ctx, req)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_TIMEOUT, providerID, true, err)
	}
	defer resp.Body.Close()
	body, readErr := p.readBody(resp.Body)
	if readErr != nil {
		return search.ProviderSearchResponse{}, readErr
	}
	return p.handleResponse(resp.StatusCode, body, req)
}

func AsSearchError(err error, target **search.Error) bool {
	if err == nil {
		return false
	}
	if se, ok := err.(*search.Error); ok {
		*target = se
		return true
	}
	return false
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
	if p.credential == "" && p.credentialRef == "" {
		return search.ProviderHealthCredentialMiss
	}
	return search.ProviderHealthReady
}

func (p *Provider) buildRequest(ctx context.Context, req search.GeneralSearchRequest) (*http.Request, error) {
	u, err := url.Parse(p.endpoint)
	if err != nil {
		return nil, search.NewError(search.SEARCH_PROVIDER_REQUEST_FAILED, providerID, false, err)
	}
	q := u.Query()
	q.Set("q", req.Query)
	limit := req.Limit
	if limit < 1 {
		limit = search.DefaultLimit
	}
	if limit > maxResults {
		limit = maxResults
	}
	q.Set("count", strconv.Itoa(limit))
	if req.Offset > 0 {
		q.Set("offset", strconv.Itoa(req.Offset))
	}
	if req.Language != "" {
		q.Set("search_lang", req.Language)
	}
	if req.Country != "" {
		q.Set("country", req.Country)
		q.Set("ui_lang", req.Country)
	}
	q.Set("safesearch", mapSafeSearch(req.SafeSearch))
	q.Set("text_decorations", "0")
	q.Set("spellcheck", "1")
	u.RawQuery = q.Encode()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, search.NewError(search.SEARCH_PROVIDER_REQUEST_FAILED, providerID, false, err)
	}
	httpReq.Header.Set("Accept", "application/json")
	httpReq.Header.Set("Accept-Encoding", "gzip")
	httpReq.Header.Set("User-Agent", userAgent)
	httpReq.Header.Set("X-Subscription-Token", p.credential)
	return httpReq, nil
}

func mapSafeSearch(mode search.SafeSearchMode) string {
	switch mode {
	case search.SafeSearchOff:
		return string(safeSearchOff)
	case search.SafeSearchStrict:
		return string(safeSearchStrict)
	case "":
		return string(safeSearchModerate)
	default:
		return string(mode)
	}
}

func (p *Provider) readBody(body io.Reader) ([]byte, error) {
	limit := p.maxBytes
	if limit <= 0 {
		limit = 2 * 1024 * 1024
	}
	buf := make([]byte, 0, 4096)
	r := io.LimitReader(body, limit+1)
	tmp := make([]byte, 4096)
	for {
		n, rerr := r.Read(tmp)
		if n > 0 {
			if int64(len(buf)+n) > limit {
				return nil, search.NewError(search.SEARCH_PROVIDER_RESPONSE_TOO_LARGE, providerID, false, nil)
			}
			buf = append(buf, tmp[:n]...)
		}
		if rerr != nil {
			if rerr == io.EOF {
				break
			}
			return nil, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, providerID, false, rerr)
		}
	}
	return buf, nil
}

func (p *Provider) handleResponse(status int, body []byte, req search.GeneralSearchRequest) (search.ProviderSearchResponse, error) {
	if status >= 400 {
		var bErr braveErrorResponse
		if len(body) > 0 {
			_ = json.Unmarshal(body, &bErr)
		}
		return p.mapHTTPError(status, bErr)
	}
	return p.mapSuccess(body, status, req)
}

func (p *Provider) mapSuccess(body []byte, status int, req search.GeneralSearchRequest) (search.ProviderSearchResponse, error) {
	var resp braveResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, providerID, false, err)
	}
	if resp.Web == nil || len(resp.Web.Results) == 0 {
		return search.ProviderSearchResponse{
			Results:    []search.SearchResult{},
			HasMore:    false,
			HTTPStatus: status,
			RawBytes:   len(body),
		}, nil
	}
	results := make([]search.SearchResult, 0, len(resp.Web.Results))
	for i, r := range resp.Web.Results {
		results = append(results, search.SearchResult{
			Rank:       i + 1,
			Title:      r.Title,
			URL:        r.URL,
			DisplayURL: r.DisplayURL,
			Snippet:    r.Description,
			Language:   r.Language,
			Source: search.SearchSourceMetadata{
				Provider:     providerID,
				ProviderRank: i + 1,
				OriginalURL:  r.URL,
			},
		})
	}
	return search.ProviderSearchResponse{
		Results:    results,
		HasMore:    len(resp.Web.Results) >= req.Limit && req.Limit < maxResults,
		HTTPStatus: status,
		RawBytes:   len(body),
	}, nil
}

func (p *Provider) mapHTTPError(status int, bErr braveErrorResponse) (search.ProviderSearchResponse, error) {
	if search.IsAuthError(status) {
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_AUTH_FAILED, providerID, status, nil)
	}
	if search.IsRateLimited(status) {
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_RATE_LIMITED, providerID, status, nil)
	}
	if search.IsServerError(status) {
		return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_REQUEST_FAILED, providerID, status, nil)
	}
	return search.ProviderSearchResponse{}, search.WrapHTTPError(search.SEARCH_PROVIDER_REQUEST_REJECTED, providerID, status, nil)
}
