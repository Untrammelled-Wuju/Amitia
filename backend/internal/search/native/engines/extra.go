package engines

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type openAIREEngine struct {
	fetcher *native.Fetcher
}

func NewOpenAIREEngine(fetcher *native.Fetcher) *openAIREEngine {
	return &openAIREEngine{fetcher: fetcherFor(fetcher)}
}

func (e *openAIREEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "openaire",
		Name:     "OpenAIRE",
		Priority: 54,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindAcademic},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *openAIREEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"title":  strings.TrimSpace(request.Query),
		"size":   fmt.Sprintf("%d", limit),
		"page":   fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"format": "json",
	}
	rawURL := endpoint("https://api.openaire.eu/search/publications", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	var root map[string]any
	if err := json.Unmarshal(body, &root); err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	response := mapValue(root["response"])
	header := mapValue(response["header"])
	total := intValue(mapValue(header["total"])["$"])
	resultsContainer := mapValue(response["results"])
	rawResults := collectionValues(resultsContainer["result"])
	results := make([]search.SearchResult, 0, len(rawResults))
	for _, value := range rawResults {
		resultEnvelope := mapValue(value)
		metadata := mapValue(resultEnvelope["metadata"])
		entity := mapValue(metadata["oaf:entity"])
		resultMap := mapValue(entity["oaf:result"])
		if len(resultMap) == 0 {
			resultMap = entity
		}
		title := firstDollarString(resultMap["title"])
		if title == "" {
			continue
		}
		description := firstDollarString(resultMap["description"])
		doi := doiValue(resultMap["pid"])
		link := ""
		if doi != "" {
			link = "https://doi.org/" + doi
		}
		if link == "" {
			link = "https://www.openaire.eu/search/publication?articleId=" + url.QueryEscape(stringValue(resultEnvelope["header"], "dri:objIdentifier"))
		}
		if strings.HasSuffix(link, "articleId=") {
			continue
		}
		current := result(e.Descriptor().ID, title, link, description, len(results)+1)
		current.Metadata.Type = "paper"
		current.Metadata.DOI = doi
		current.Metadata.Authors = dollarStringList(resultMap["creator"])
		if publisher := firstDollarString(resultMap["publisher"]); publisher != "" {
			current.Metadata.Journal = publisher
		}
		if date := firstDollarString(resultMap["dateofacceptance"]); date != "" {
			if parsed := parseTime(date); parsed != nil {
				current.PublishedAt = parsed
			}
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < total, HTTPStatus: status, RawBytes: len(body)}, nil
}

type fiveHundredPXEngine struct {
	fetcher *native.Fetcher
}

func New500pxEngine(fetcher *native.Fetcher) *fiveHundredPXEngine {
	return &fiveHundredPXEngine{fetcher: fetcherFor(fetcher)}
}

func (e *fiveHundredPXEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "500px",
		Name:     "500px",
		Priority: 64,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindImage},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *fiveHundredPXEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"term":         strings.TrimSpace(request.Query),
		"consumer_key": credential,
		"page":         fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"rpp":          fmt.Sprintf("%d", limit),
		"image_size":   "4",
	}
	rawURL := endpoint("https://api.500px.com/v1/photos/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		TotalPages int `json:"total_pages"`
		Photos     []struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
			URL         string `json:"url"`
			ImageURL    string `json:"image_url"`
			CreatedAt   string `json:"created_at"`
			User        struct {
				FullName string `json:"fullname"`
			} `json:"user"`
		} `json:"photos"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Photos))
	for _, item := range payload.Photos {
		if item.URL == "" {
			continue
		}
		current := result(e.Descriptor().ID, firstNonEmpty(item.Name, fmt.Sprintf("500px photo %d", item.ID)), item.URL, item.Description, len(results)+1)
		current.Metadata.Type = "image"
		current.Metadata.MediaURL = item.ImageURL
		current.Metadata.ThumbnailURL = item.ImageURL
		current.Metadata.Merchant = item.User.FullName
		if parsed := parseTime(item.CreatedAt); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: pageNumber(request.Offset, limit) < payload.TotalPages, HTTPStatus: status, RawBytes: len(body)}, nil
}

type geniusEngine struct {
	fetcher *native.Fetcher
}

func NewGeniusEngine(fetcher *native.Fetcher) *geniusEngine {
	return &geniusEngine{fetcher: fetcherFor(fetcher)}
}

func (e *geniusEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "genius",
		Name:     "Genius",
		Priority: 61,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindMusic},
			Pagination:  true,
			MaxResults:  20,
		},
	}
}

func (e *geniusEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 20)
	values := map[string]string{
		"q":        strings.TrimSpace(request.Query),
		"per_page": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://api.genius.com/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, http.Header{"Authorization": []string{"Bearer " + credential}})
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Response struct {
			Hits []struct {
				Result struct {
					Title       string `json:"title"`
					URL         string `json:"url"`
					Artist      string `json:"artist_names"`
					Thumbnail   string `json:"header_image_thumbnail_url"`
					ReleaseDate string `json:"release_date_for_display"`
				} `json:"result"`
			} `json:"hits"`
		} `json:"response"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Response.Hits))
	for _, item := range payload.Response.Hits {
		if item.Result.Title == "" || item.Result.URL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Result.Title, item.Result.URL, item.Result.Artist, len(results)+1)
		current.Metadata.Type = "music"
		current.Metadata.Merchant = item.Result.Artist
		current.Metadata.ThumbnailURL = item.Result.Thumbnail
		if parsed := parseTime(item.Result.ReleaseDate); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Response.Hits) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type soundCloudEngine struct {
	fetcher *native.Fetcher
}

func NewSoundCloudEngine(fetcher *native.Fetcher) *soundCloudEngine {
	return &soundCloudEngine{fetcher: fetcherFor(fetcher)}
}

func (e *soundCloudEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "soundcloud",
		Name:     "SoundCloud",
		Priority: 60,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindMusic},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *soundCloudEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":         strings.TrimSpace(request.Query),
		"client_id": credential,
		"limit":     fmt.Sprintf("%d", limit),
		"offset":    fmt.Sprintf("%d", request.Offset),
	}
	rawURL := endpoint("https://api-v2.soundcloud.com/search/tracks", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		TotalResults int `json:"total_results"`
		Collection   []struct {
			Title       string `json:"title"`
			Permalink   string `json:"permalink_url"`
			Description string `json:"description"`
			Duration    int    `json:"duration"`
			CreatedAt   string `json:"created_at"`
			ArtworkURL  string `json:"artwork_url"`
			User        struct {
				Username string `json:"username"`
			} `json:"user"`
		} `json:"collection"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Collection))
	for _, item := range payload.Collection {
		if item.Title == "" || item.Permalink == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.Permalink, item.Description, len(results)+1)
		current.Metadata.Type = "music"
		current.Metadata.Merchant = item.User.Username
		current.Metadata.DurationSeconds = float64(item.Duration) / 1000
		current.Metadata.ThumbnailURL = item.ArtworkURL
		if parsed := parseTime(item.CreatedAt); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.TotalResults, HTTPStatus: status, RawBytes: len(body)}, nil
}

type mixcloudEngine struct {
	fetcher *native.Fetcher
}

func NewMixcloudEngine(fetcher *native.Fetcher) *mixcloudEngine {
	return &mixcloudEngine{fetcher: fetcherFor(fetcher)}
}

func (e *mixcloudEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "mixcloud",
		Name:     "Mixcloud",
		Priority: 53,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindMusic},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *mixcloudEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":      strings.TrimSpace(request.Query),
		"type":   "cloudcast",
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", request.Offset),
	}
	rawURL := endpoint("https://api.mixcloud.com/search/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Data []struct {
			Name        string `json:"name"`
			URL         string `json:"url"`
			Key         string `json:"key"`
			CreatedTime string `json:"created_time"`
			User        struct {
				Name string `json:"name"`
			} `json:"user"`
			Pictures struct {
				Large  string `json:"large"`
				Medium string `json:"medium"`
			} `json:"pictures"`
		} `json:"data"`
		Paging struct {
			Next string `json:"next"`
		} `json:"paging"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.Name == "" || item.URL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Name, item.URL, item.User.Name, len(results)+1)
		current.Metadata.Type = "music"
		current.Metadata.Merchant = item.User.Name
		current.Metadata.ThumbnailURL = firstNonEmpty(item.Pictures.Large, item.Pictures.Medium)
		if parsed := parseTime(item.CreatedTime); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: payload.Paging.Next != "", HTTPStatus: status, RawBytes: len(body)}, nil
}

func mapValue(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

func collectionValues(value any) []any {
	if values, ok := value.([]any); ok {
		return values
	}
	if value != nil {
		return []any{value}
	}
	return nil
}

func firstDollarString(value any) string {
	for _, item := range collectionValues(value) {
		if text := anyString(mapValue(item)["$"]); text != "" {
			return text
		}
		if text := anyString(item); text != "" {
			return text
		}
	}
	return ""
}

func dollarStringList(value any) []string {
	result := make([]string, 0)
	for _, item := range collectionValues(value) {
		if text := firstDollarString(item); text != "" {
			result = append(result, text)
		}
	}
	return result
}

func doiValue(value any) string {
	for _, item := range collectionValues(value) {
		current := mapValue(item)
		if strings.EqualFold(anyString(current["@classid"]), "doi") {
			return anyString(current["$"])
		}
	}
	return ""
}

func stringValue(value any, key string) string {
	return anyString(mapValue(value)[key])
}

func intValue(value any) int {
	switch current := value.(type) {
	case float64:
		return int(current)
	case string:
		return parsePositiveInt(current)
	default:
		return 0
	}
}
