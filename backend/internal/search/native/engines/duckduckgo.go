package engines

import (
	"context"
	"net/url"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
	"golang.org/x/net/html"
)

type duckDuckGoEngine struct {
	fetcher *native.Fetcher
}

func NewDuckDuckGoEngine(fetcher *native.Fetcher) *duckDuckGoEngine {
	return &duckDuckGoEngine{fetcher: fetcherFor(fetcher)}
}

func (e *duckDuckGoEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "duckduckgo_html",
		Name:     "DuckDuckGo HTML",
		Priority: 82,
		Weight:   1.25,
		Group:    "general",
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:          true,
			SearchKinds:         []search.SearchKind{search.SearchKindWeb},
			LanguageFilter:      true,
			SafeSearch:          true,
			Pagination:          true,
			TimeRangeFilter:     true,
			DomainFilter:        true,
			ExcludeDomainFilter: true,
			MaxResults:          30,
		},
	}
}

func (e *duckDuckGoEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 30)
	query := queryWithDomainFilters(request.Query, request.Domains, request.ExcludeDomains)
	values := map[string]string{"q": query}
	if request.Language != "" {
		// DuckDuckGo HTML accepts kl region/language values but does not expose
		// a stable language-only contract. Passing the normalized locale is a
		// useful hint while keeping result parsing deterministic.
		values["kl"] = strings.ToLower(strings.ReplaceAll(request.Language, "_", "-"))
	}
	if request.SafeSearch == search.SafeSearchOff {
		values["kp"] = "-2"
	} else if request.SafeSearch == search.SafeSearchStrict {
		values["kp"] = "1"
	}
	if request.Offset > 0 {
		values["s"] = stringInt(request.Offset)
	}
	if request.TimeRange != nil && request.TimeRange.From != nil {
		age := time.Since(request.TimeRange.From.UTC())
		switch {
		case age <= 48*time.Hour:
			values["df"] = "d"
		case age <= 10*24*time.Hour:
			values["df"] = "w"
		case age <= 45*24*time.Hour:
			values["df"] = "m"
		default:
			values["df"] = "y"
		}
	}
	rawURL := endpoint("https://html.duckduckgo.com/html/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	root, err := parseHTML(body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, limit)
	for _, block := range findAll(root, func(node *html.Node) bool {
		return hasClass(node, "result") || hasClass(node, "web-result")
	}) {
		anchor := findFirst(block, func(node *html.Node) bool {
			return node.Data == "a" && (hasClass(node, "result__a") || hasClass(node, "result-link"))
		})
		if anchor == nil {
			continue
		}
		title := nodeText(anchor)
		link := duckDuckGoResultURL(attribute(anchor, "href"))
		if title == "" || link == "" {
			continue
		}
		snippetNode := findFirst(block, func(node *html.Node) bool {
			return hasClass(node, "result__snippet") || hasClass(node, "result-snippet")
		})
		current := result(e.Descriptor().ID, title, link, nodeText(snippetNode), len(results)+1)
		current.Language = request.Language
		results = append(results, current)
		if len(results) >= limit {
			break
		}
	}
	return search.ProviderSearchResponse{
		Results:    withProviders(e.Descriptor().ID, results),
		HasMore:    len(results) >= limit,
		HTTPStatus: status,
		RawBytes:   len(body),
	}, nil
}

func duckDuckGoResultURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	if strings.Contains(strings.ToLower(parsed.Hostname()), "duckduckgo.com") {
		if target := strings.TrimSpace(parsed.Query().Get("uddg")); target != "" {
			if decoded, err := url.QueryUnescape(target); err == nil {
				return decoded
			}
			return target
		}
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	return parsed.String()
}

func queryWithDomainFilters(query string, include, exclude []string) string {
	query = queryWithDomains(query, include)
	for _, domain := range exclude {
		domain = strings.TrimSpace(domain)
		if domain != "" {
			query += " -site:" + domain
		}
	}
	return strings.TrimSpace(query)
}

func stringInt(value int) string {
	if value <= 0 {
		return "0"
	}
	const digits = "0123456789"
	var buf [20]byte
	pos := len(buf)
	for value > 0 {
		pos--
		buf[pos] = digits[value%10]
		value /= 10
	}
	return string(buf[pos:])
}
