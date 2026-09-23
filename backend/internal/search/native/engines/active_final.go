package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type wttrEngine struct {
	fetcher *native.Fetcher
}

func NewWttrEngine(fetcher *native.Fetcher) *wttrEngine {
	return &wttrEngine{fetcher: fetcherFor(fetcher)}
}

func (e *wttrEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "wttr",
		Name:     "wttr.in",
		Priority: 49,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:    []search.SearchKind{search.SearchKindWeather},
			LanguageFilter: true,
			MaxResults:     1,
		},
	}
}

func (e *wttrEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	location := weatherLocation(request.Query)
	if location == "" {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: 200}, nil
	}
	rawURL := "https://wttr.in/" + url.PathEscape(location) + "?format=j1"
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Current []struct {
			TempC         string `json:"temp_C"`
			FeelsLikeC    string `json:"FeelsLikeC"`
			Humidity      string `json:"humidity"`
			WindSpeedKmph string `json:"windspeedKmph"`
			WeatherDesc   []struct {
				Value string `json:"value"`
			} `json:"weatherDesc"`
		} `json:"current_condition"`
		Nearest []struct {
			AreaName []struct {
				Value string `json:"value"`
			} `json:"areaName"`
			Country []struct {
				Value string `json:"value"`
			} `json:"country"`
		} `json:"nearest_area"`
	}](body)
	if err != nil || len(payload.Current) == 0 {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: status, RawBytes: len(body)}, nil
	}
	currentWeather := payload.Current[0]
	description := ""
	if len(currentWeather.WeatherDesc) > 0 {
		description = currentWeather.WeatherDesc[0].Value
	}
	titleLocation := location
	if len(payload.Nearest) > 0 {
		if len(payload.Nearest[0].AreaName) > 0 {
			titleLocation = payload.Nearest[0].AreaName[0].Value
		}
	}
	title := titleLocation + " weather"
	snippet := fmt.Sprintf("%s，温度 %s°C，体感 %s°C，湿度 %s%%，风速 %s km/h", description, currentWeather.TempC, currentWeather.FeelsLikeC, currentWeather.Humidity, currentWeather.WindSpeedKmph)
	current := result(e.Descriptor().ID, title, "https://wttr.in/"+url.PathEscape(titleLocation), snippet, 1)
	current.Language = request.Language
	current.Metadata.Type = "weather"
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, []search.SearchResult{current}), HTTPStatus: status, RawBytes: len(body)}, nil
}

type wordnikEngine struct {
	fetcher *native.Fetcher
}

func NewWordnikEngine(fetcher *native.Fetcher) *wordnikEngine {
	return &wordnikEngine{fetcher: fetcherFor(fetcher)}
}

func (e *wordnikEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "wordnik",
		Name:     "Wordnik",
		Priority: 50,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindDictionary},
			MaxResults:  1,
		},
	}
}

func (e *wordnikEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	word := dictionaryWord(request.Query)
	if word == "" {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: 200}, nil
	}
	rawURL := endpoint("https://api.wordnik.com/v4/word.json/"+url.PathEscape(word)+"/definitions", map[string]string{
		"api_key": credential,
		"limit":   "5",
	})
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[[]struct {
		Text         string `json:"text"`
		PartOfSpeech string `json:"partOfSpeech"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	if len(payload) == 0 {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: status, RawBytes: len(body)}, nil
	}
	definitions := make([]string, 0, len(payload))
	for _, item := range payload {
		value := strings.TrimSpace(item.Text)
		if item.PartOfSpeech != "" {
			value = item.PartOfSpeech + ": " + value
		}
		if value != "" {
			definitions = append(definitions, value)
		}
	}
	current := result(e.Descriptor().ID, word, "https://www.wordnik.com/words/"+url.PathEscape(word), strings.Join(definitions, " | "), 1)
	current.Metadata.Type = "dictionary"
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, []search.SearchResult{current}), HTTPStatus: status, RawBytes: len(body)}, nil
}

type pinterestEngine struct {
	fetcher *native.Fetcher
}

func NewPinterestEngine(fetcher *native.Fetcher) *pinterestEngine {
	return &pinterestEngine{fetcher: fetcherFor(fetcher)}
}

func (e *pinterestEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "pinterest",
		Name:     "Pinterest",
		Priority: 62,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindImage},
			Pagination:  true,
			MaxResults:  50,
		},
	}
}

func (e *pinterestEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 50)
	rawURL := endpoint("https://api.pinterest.com/v5/search/pins", map[string]string{
		"query":     strings.TrimSpace(request.Query),
		"page_size": fmt.Sprintf("%d", limit),
	})
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, http.Header{"Authorization": []string{"Bearer " + credential}})
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Items []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			Link        string `json:"link"`
			Media       struct {
				Images map[string]struct {
					URL string `json:"url"`
				} `json:"images"`
			} `json:"media"`
		} `json:"items"`
		Bookmark string `json:"bookmark"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Items))
	for _, item := range payload.Items {
		link := firstNonEmpty(item.Link, item.Media.Images["1200x"].URL, "https://www.pinterest.com/pin/"+url.PathEscape(item.ID)+"/")
		if link == "" {
			continue
		}
		title := firstNonEmpty(item.Title, item.Description, item.ID)
		current := result(e.Descriptor().ID, title, link, item.Description, len(results)+1)
		current.Metadata.Type = "image"
		current.Metadata.MediaURL = firstNonEmpty(item.Media.Images["1200x"].URL, item.Media.Images["600x"].URL)
		current.Metadata.ThumbnailURL = firstNonEmpty(item.Media.Images["400x300"].URL, item.Media.Images["600x"].URL)
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: payload.Bookmark != "", HTTPStatus: status, RawBytes: len(body)}, nil
}
