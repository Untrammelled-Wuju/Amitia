package engines

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type mediaWikiEngine struct {
	fetcher  *native.Fetcher
	id       string
	name     string
	apiBase  string
	pageBase string
	priority int
	kinds    []search.SearchKind
}

func NewWikidataEngine(fetcher *native.Fetcher) *mediaWikiEngine {
	return &mediaWikiEngine{fetcher: fetcherFor(fetcher), id: "wikidata", name: "Wikidata", apiBase: "https://www.wikidata.org/w/api.php", pageBase: "https://www.wikidata.org/wiki/", priority: 60, kinds: []search.SearchKind{search.SearchKindWeb}}
}

func NewWiktionaryEngine(fetcher *native.Fetcher) *mediaWikiEngine {
	return &mediaWikiEngine{fetcher: fetcherFor(fetcher), id: "wiktionary", name: "Wiktionary", apiBase: "https://en.wiktionary.org/w/api.php", pageBase: "https://en.wiktionary.org/wiki/", priority: 52, kinds: []search.SearchKind{search.SearchKindWeb}}
}

func NewWikinewsEngine(fetcher *native.Fetcher) *mediaWikiEngine {
	return &mediaWikiEngine{fetcher: fetcherFor(fetcher), id: "wikinews", name: "Wikinews", apiBase: "https://en.wikinews.org/w/api.php", pageBase: "https://en.wikinews.org/wiki/", priority: 52, kinds: []search.SearchKind{search.SearchKindNews, search.SearchKindWeb}}
}

func NewArchWikiEngine(fetcher *native.Fetcher) *mediaWikiEngine {
	return &mediaWikiEngine{fetcher: fetcherFor(fetcher), id: "archwiki", name: "Arch Wiki", apiBase: "https://wiki.archlinux.org/api.php", pageBase: "https://wiki.archlinux.org/title/", priority: 49, kinds: []search.SearchKind{search.SearchKindWeb, search.SearchKindSoftware}}
}

func NewGentooWikiEngine(fetcher *native.Fetcher) *mediaWikiEngine {
	return &mediaWikiEngine{fetcher: fetcherFor(fetcher), id: "gentoo_wiki", name: "Gentoo Wiki", apiBase: "https://wiki.gentoo.org/api.php", pageBase: "https://wiki.gentoo.org/wiki/", priority: 48, kinds: []search.SearchKind{search.SearchKindWeb, search.SearchKindSoftware}}
}

func (e *mediaWikiEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       e.id,
		Name:     e.name,
		Priority: e.priority,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:  containsSearchKind(e.kinds, search.SearchKindWeb),
			SearchKinds: append([]search.SearchKind(nil), e.kinds...),
			Pagination:  true,
			MaxResults:  50,
		},
	}
}

func (e *mediaWikiEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 50)
	values := map[string]string{
		"action":   "query",
		"list":     "search",
		"srsearch": strings.TrimSpace(request.Query),
		"srlimit":  fmt.Sprintf("%d", limit),
		"sroffset": fmt.Sprintf("%d", request.Offset),
		"format":   "json",
		"utf8":     "1",
	}
	rawURL := endpoint(e.apiBase, values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Query struct {
			Search []struct {
				Title     string `json:"title"`
				Snippet   string `json:"snippet"`
				Timestamp string `json:"timestamp"`
			} `json:"search"`
		} `json:"query"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Query.Search))
	for _, item := range payload.Query.Search {
		if item.Title == "" {
			continue
		}
		link := e.pageBase + url.PathEscape(strings.ReplaceAll(item.Title, " ", "_"))
		current := result(e.Descriptor().ID, item.Title, link, stripMarkup(item.Snippet), len(results)+1)
		current.Metadata.Type = "wiki"
		if parsed := parseTime(item.Timestamp); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Query.Search) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

func containsSearchKind(values []search.SearchKind, target search.SearchKind) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
