package engines

import (
	"context"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type openverseEngine struct {
	fetcher *native.Fetcher
}

func NewOpenverseEngine(fetcher *native.Fetcher) *openverseEngine {
	return &openverseEngine{fetcher: fetcherFor(fetcher)}
}

func (e *openverseEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "openverse",
		Name:     "Openverse",
		Priority: 51,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindImage},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *openverseEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":         strings.TrimSpace(request.Query),
		"page_size": fmt.Sprintf("%d", limit),
		"page":      fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://api.openverse.org/v1/images/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		ResultCount int `json:"result_count"`
		Results     []struct {
			ID                string `json:"id"`
			Title             string `json:"title"`
			URL               string `json:"url"`
			Thumbnail         string `json:"thumbnail"`
			Creator           string `json:"creator"`
			License           string `json:"license"`
			ForeignLandingURL string `json:"foreign_landing_url"`
		} `json:"results"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		link := firstNonEmpty(item.ForeignLandingURL, item.URL)
		if link == "" {
			continue
		}
		title := firstNonEmpty(item.Title, item.ID)
		current := result(e.Descriptor().ID, title, link, item.Creator, len(results)+1)
		current.Metadata.Type = "image"
		current.Metadata.MediaURL = item.URL
		current.Metadata.ThumbnailURL = firstNonEmpty(item.Thumbnail, item.URL)
		current.Metadata.License = item.License
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.ResultCount, HTTPStatus: status, RawBytes: len(body)}, nil
}

type radioBrowserEngine struct {
	fetcher *native.Fetcher
}

func NewRadioBrowserEngine(fetcher *native.Fetcher) *radioBrowserEngine {
	return &radioBrowserEngine{fetcher: fetcherFor(fetcher)}
}

func (e *radioBrowserEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "radio_browser",
		Name:     "Radio Browser",
		Priority: 50,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:    []search.SearchKind{search.SearchKindMusic},
			LanguageFilter: true,
			CountryFilter:  true,
			Pagination:     true,
			MaxResults:     100,
		},
	}
}

func (e *radioBrowserEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"name":       strings.TrimSpace(request.Query),
		"limit":      fmt.Sprintf("%d", limit),
		"offset":     fmt.Sprintf("%d", request.Offset),
		"hidebroken": "true",
		"order":      "votes",
		"reverse":    "true",
	}
	if request.Language != "" {
		values["language"] = request.Language
	}
	if request.Country != "" {
		values["countrycode"] = request.Country
	}
	rawURL := endpoint("https://de1.api.radio-browser.info/json/stations/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[[]struct {
		StationUUID string `json:"stationuuid"`
		Name        string `json:"name"`
		URL         string `json:"url"`
		Homepage    string `json:"homepage"`
		Favicon     string `json:"favicon"`
		Country     string `json:"country"`
		CountryCode string `json:"countrycode"`
		Language    string `json:"language"`
		Tags        string `json:"tags"`
		Votes       int    `json:"votes"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload))
	for _, item := range payload {
		link := firstNonEmpty(item.Homepage, item.URL)
		if item.Name == "" || link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Name, link, item.Tags, len(results)+1)
		current.Language = item.Language
		current.Metadata.Type = "radio"
		current.Metadata.MediaURL = item.URL
		current.Metadata.ThumbnailURL = item.Favicon
		current.Metadata.Address = item.Country
		current.Metadata.Rating = floatPointer(float64(item.Votes))
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

func floatPointer(value float64) *float64 {
	return &value
}
