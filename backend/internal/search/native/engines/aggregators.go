package engines

import (
	"context"
	"net/http"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type serperEngine struct {
	fetcher *native.Fetcher
}

func NewSerperEngine(fetcher *native.Fetcher) *serperEngine {
	return &serperEngine{fetcher: fetcherFor(fetcher)}
}

func (e *serperEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "serper",
		Name:     "Serper",
		Priority: 92,
		Weight:   1.4,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:      true,
			SearchKinds:     []search.SearchKind{search.SearchKindWeb, search.SearchKindNews, search.SearchKindImage},
			LanguageFilter:  true,
			CountryFilter:   true,
			SafeSearch:      true,
			Pagination:      true,
			TimeRangeFilter: true,
			DomainFilter:    true,
			MaxResults:      100,
		},
	}
}

func (e *serperEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	path := "https://google.serper.dev/search"
	switch request.Kind {
	case search.SearchKindNews:
		path = "https://google.serper.dev/news"
	case search.SearchKindImage:
		path = "https://google.serper.dev/images"
	}
	payload := map[string]any{
		"q":   queryWithDomains(request.Query, request.Domains),
		"num": boundedLimit(request.Limit, 8, 100),
	}
	if request.Language != "" {
		payload["hl"] = request.Language
	}
	if request.Country != "" {
		payload["gl"] = request.Country
	}
	body, status, err := e.fetcher.PostJSON(ctx, e.Descriptor().ID, path, http.Header{
		"X-API-KEY": []string{credential},
	}, payload)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payloadData, err := decodeJSON[struct {
		Organic []struct {
			Title    string `json:"title"`
			Link     string `json:"link"`
			Snippet  string `json:"snippet"`
			Date     string `json:"date"`
			ImageURL string `json:"imageUrl"`
			Source   string `json:"source"`
		} `json:"organic"`
		News []struct {
			Title    string `json:"title"`
			Link     string `json:"link"`
			Snippet  string `json:"snippet"`
			Date     string `json:"date"`
			ImageURL string `json:"imageUrl"`
			Source   string `json:"source"`
		} `json:"news"`
		Images []struct {
			Title    string `json:"title"`
			Link     string `json:"link"`
			ImageURL string `json:"imageUrl"`
			Source   string `json:"source"`
		} `json:"images"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payloadData.Organic)+len(payloadData.News)+len(payloadData.Images))
	for _, item := range payloadData.Organic {
		if item.Title == "" || item.Link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.Link, item.Snippet, len(results)+1)
		current.Metadata.Type = "web"
		current.Metadata.Merchant = item.Source
		if parsed := parseTime(item.Date); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	for _, item := range payloadData.News {
		if item.Title == "" || item.Link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.Link, item.Snippet, len(results)+1)
		current.Metadata.Type = "news"
		current.Metadata.Merchant = item.Source
		current.Metadata.ThumbnailURL = item.ImageURL
		if parsed := parseTime(item.Date); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	for _, item := range payloadData.Images {
		if item.Title == "" || item.Link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.Link, item.Source, len(results)+1)
		current.Metadata.Type = "image"
		current.Metadata.MediaURL = item.ImageURL
		current.Metadata.ThumbnailURL = item.ImageURL
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(results) >= boundedLimit(request.Limit, 8, 100), HTTPStatus: status, RawBytes: len(body)}, nil
}
