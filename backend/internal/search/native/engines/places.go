package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type nominatimEngine struct {
	fetcher *native.Fetcher
}

func NewNominatimEngine(fetcher *native.Fetcher) *nominatimEngine {
	return &nominatimEngine{fetcher: fetcherFor(fetcher)}
}

func (e *nominatimEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "nominatim",
		Name:     "OpenStreetMap Nominatim",
		Priority: 60,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:    []search.SearchKind{search.SearchKindPlaces},
			LanguageFilter: true,
			CountryFilter:  true,
			MaxResults:     40,
		},
	}
}

func (e *nominatimEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 40)
	query := strings.TrimSpace(request.Query)
	if request.Country != "" {
		query = strings.TrimSpace(query + ", " + request.Country)
	}
	values := map[string]string{
		"q":               query,
		"format":          "jsonv2",
		"addressdetails":  "1",
		"limit":           fmt.Sprintf("%d", limit),
		"accept-language": request.Language,
	}
	rawURL := endpoint("https://nominatim.openstreetmap.org/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, http.Header{
		"Accept-Language": []string{request.Language},
	})
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[[]struct {
		DisplayName string            `json:"display_name"`
		Latitude    string            `json:"lat"`
		Longitude   string            `json:"lon"`
		Type        string            `json:"type"`
		Class       string            `json:"class"`
		Importance  float64           `json:"importance"`
		Address     map[string]string `json:"address"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload))
	for _, item := range payload {
		if item.DisplayName == "" {
			continue
		}
		link := "https://www.openstreetmap.org/search?query=" + url.QueryEscape(item.DisplayName)
		current := result(e.Descriptor().ID, item.DisplayName, link, item.DisplayName, len(results)+1)
		current.Metadata.Type = firstNonEmpty(item.Type, item.Class, "place")
		current.Metadata.Address = item.DisplayName
		if latitude, err := strconv.ParseFloat(item.Latitude, 64); err == nil {
			current.Metadata.Latitude = &latitude
		}
		if longitude, err := strconv.ParseFloat(item.Longitude, 64); err == nil {
			current.Metadata.Longitude = &longitude
		}
		rating := item.Importance
		current.Metadata.Rating = &rating
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type photonEngine struct {
	fetcher *native.Fetcher
}

func NewPhotonEngine(fetcher *native.Fetcher) *photonEngine {
	return &photonEngine{fetcher: fetcherFor(fetcher)}
}

func (e *photonEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "photon",
		Name:     "Photon",
		Priority: 62,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:    []search.SearchKind{search.SearchKindPlaces},
			LanguageFilter: true,
			MaxResults:     50,
		},
	}
}

func (e *photonEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 50)
	values := map[string]string{
		"q":     strings.TrimSpace(request.Query),
		"limit": fmt.Sprintf("%d", limit),
		"lang":  request.Language,
	}
	rawURL := endpoint("https://photon.komoot.io/api/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Features []struct {
			Properties photonProperties `json:"properties"`
			Geometry   struct {
				Coordinates []float64 `json:"coordinates"`
			} `json:"geometry"`
		} `json:"features"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Features))
	for _, item := range payload.Features {
		title := strings.TrimSpace(item.Properties.Name)
		if title == "" {
			title = firstNonEmpty(item.Properties.Street, item.Properties.City)
		}
		if title == "" {
			continue
		}
		link := "https://www.openstreetmap.org/search?query=" + url.QueryEscape(title)
		current := result(e.Descriptor().ID, title, link, placeAddress(item.Properties), len(results)+1)
		current.Metadata.Type = firstNonEmpty(item.Properties.Type, item.Properties.OSMValue, "place")
		current.Metadata.Address = placeAddress(item.Properties)
		if len(item.Geometry.Coordinates) >= 2 {
			longitude := item.Geometry.Coordinates[0]
			latitude := item.Geometry.Coordinates[1]
			current.Metadata.Longitude = &longitude
			current.Metadata.Latitude = &latitude
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Features) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type photonProperties struct {
	Name        string `json:"name"`
	Street      string `json:"street"`
	City        string `json:"city"`
	State       string `json:"state"`
	Country     string `json:"country"`
	CountryCode string `json:"countrycode"`
	Postcode    string `json:"postcode"`
	Type        string `json:"type"`
	OSMKey      string `json:"osm_key"`
	OSMValue    string `json:"osm_value"`
}

func placeAddress(properties photonProperties) string {
	parts := []string{properties.Street, properties.City, properties.State, properties.Postcode, properties.Country}
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			filtered = append(filtered, value)
		}
	}
	return strings.Join(filtered, ", ")
}
