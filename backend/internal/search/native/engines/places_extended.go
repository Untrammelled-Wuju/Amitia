package engines

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type openMeteoGeocodingEngine struct {
	fetcher *native.Fetcher
}

func NewOpenMeteoGeocodingEngine(fetcher *native.Fetcher) *openMeteoGeocodingEngine {
	return &openMeteoGeocodingEngine{fetcher: fetcherFor(fetcher)}
}

func (e *openMeteoGeocodingEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "open_meteo_geocoding",
		Name:     "Open-Meteo Geocoding",
		Priority: 52,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:    []search.SearchKind{search.SearchKindPlaces},
			LanguageFilter: true,
			CountryFilter:  true,
			MaxResults:     100,
		},
	}
}

func (e *openMeteoGeocodingEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"name":     strings.TrimSpace(request.Query),
		"count":    fmt.Sprintf("%d", limit),
		"language": request.Language,
		"format":   "json",
	}
	rawURL := endpoint("https://geocoding-api.open-meteo.com/v1/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Results []struct {
			ID          int     `json:"id"`
			Name        string  `json:"name"`
			Latitude    float64 `json:"latitude"`
			Longitude   float64 `json:"longitude"`
			Elevation   float64 `json:"elevation"`
			FeatureCode string  `json:"feature_code"`
			CountryCode string  `json:"country_code"`
			Country     string  `json:"country"`
			Admin1      string  `json:"admin1"`
			Timezone    string  `json:"timezone"`
			Population  int     `json:"population"`
		} `json:"results"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		if item.Name == "" {
			continue
		}
		if request.Country != "" && !strings.EqualFold(request.Country, item.CountryCode) && !strings.EqualFold(request.Country, item.Country) {
			continue
		}
		link := "https://www.openstreetmap.org/search?query=" + url.QueryEscape(strings.TrimSpace(item.Name+", "+item.Country))
		current := result(e.Descriptor().ID, item.Name, link, placeAddress(photonProperties{
			City:    item.Admin1,
			Country: item.Country,
			Type:    item.FeatureCode,
		}), len(results)+1)
		current.Metadata.Type = firstNonEmpty(item.FeatureCode, "place")
		current.Metadata.Address = placeAddress(photonProperties{City: item.Admin1, Country: item.Country})
		current.Metadata.Latitude = &item.Latitude
		current.Metadata.Longitude = &item.Longitude
		if item.Population > 0 {
			rating := float64(item.Population)
			current.Metadata.Rating = &rating
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Results) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}
