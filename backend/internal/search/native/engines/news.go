package engines

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type gdeltEngine struct {
	fetcher *native.Fetcher
}

func NewGDELTEngine(fetcher *native.Fetcher) *gdeltEngine {
	return &gdeltEngine{fetcher: fetcherFor(fetcher)}
}

func (e *gdeltEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "gdelt",
		Name:     "GDELT",
		Priority: 66,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindNews},
			LanguageFilter:  true,
			CountryFilter:   true,
			Pagination:      true,
			TimeRangeFilter: true,
			MaxResults:      250,
		},
	}
}

func (e *gdeltEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	params := map[string]string{
		"query":      strings.TrimSpace(request.Query),
		"mode":       "artlist",
		"format":     "json",
		"maxrecords": fmt.Sprintf("%d", limit),
		"sort":       "hybridrel",
	}
	if request.Language != "" {
		params["query"] = strings.TrimSpace(params["query"] + " sourcelang:" + request.Language)
	}
	if request.Country != "" {
		params["query"] = strings.TrimSpace(params["query"] + " sourcecountry:" + request.Country)
	}
	if request.TimeRange != nil && request.TimeRange.From != nil {
		params["startdatetime"] = request.TimeRange.From.UTC().Format("20060102150405")
	}
	if request.TimeRange != nil && request.TimeRange.To != nil {
		params["enddatetime"] = request.TimeRange.To.UTC().Format("20060102150405")
	}
	rawURL := endpoint("https://api.gdeltproject.org/api/v2/doc/doc", params)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Articles []struct {
			URL           string `json:"url"`
			Title         string `json:"title"`
			SeenDate      string `json:"seendate"`
			Domain        string `json:"domain"`
			Language      string `json:"language"`
			SourceCountry string `json:"sourcecountry"`
			SocialImage   string `json:"socialimage"`
		} `json:"articles"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Articles))
	for _, item := range payload.Articles {
		if item.Title == "" || item.URL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.URL, "", len(results)+1)
		current.Language = item.Language
		current.Metadata.Type = "news"
		current.Metadata.ThumbnailURL = item.SocialImage
		if stamp := parseGDELTTime(item.SeenDate); stamp != nil {
			current.PublishedAt = stamp
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Articles) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

func parseGDELTTime(value string) *time.Time {
	for _, layout := range []string{"20060102T150405Z", "20060102150405"} {
		if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}

type spaceflightNewsEngine struct {
	fetcher *native.Fetcher
}

func NewSpaceflightNewsEngine(fetcher *native.Fetcher) *spaceflightNewsEngine {
	return &spaceflightNewsEngine{fetcher: fetcherFor(fetcher)}
}

func (e *spaceflightNewsEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "spaceflight_news",
		Name:     "Spaceflight News",
		Priority: 62,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindNews},
			Pagination:      true,
			TimeRangeFilter: true,
			MaxResults:      100,
		},
	}
}

func (e *spaceflightNewsEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"search": strings.TrimSpace(request.Query),
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", request.Offset),
	}
	if request.TimeRange != nil && request.TimeRange.From != nil {
		values["published_at_gte"] = request.TimeRange.From.UTC().Format("2006-01-02")
	}
	rawURL := endpoint("https://api.spaceflightnewsapi.net/v4/articles/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Results []struct {
			Title       string `json:"title"`
			URL         string `json:"url"`
			ImageURL    string `json:"image_url"`
			PublishedAt string `json:"published_at"`
			Summary     string `json:"summary"`
			NewsSite    string `json:"news_site"`
		} `json:"results"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		if item.Title == "" || item.URL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.URL, item.Summary, len(results)+1)
		current.Metadata.Type = "news"
		current.Metadata.ThumbnailURL = item.ImageURL
		current.Metadata.Merchant = item.NewsSite
		if parsed := parseTime(item.PublishedAt); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Results) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}
