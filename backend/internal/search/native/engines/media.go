package engines

import (
	"context"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type wikimediaCommonsEngine struct {
	fetcher *native.Fetcher
}

func NewWikimediaCommonsEngine(fetcher *native.Fetcher) *wikimediaCommonsEngine {
	return &wikimediaCommonsEngine{fetcher: fetcherFor(fetcher)}
}

func (e *wikimediaCommonsEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "wikimedia_commons",
		Name:     "Wikimedia Commons",
		Priority: 60,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindImage},
			Pagination:  true,
			MaxResults:  50,
		},
	}
}

func (e *wikimediaCommonsEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 50)
	values := map[string]string{
		"action":        "query",
		"generator":     "search",
		"gsrsearch":     strings.TrimSpace(request.Query),
		"gsrnamespace":  "6",
		"gsrlimit":      fmt.Sprintf("%d", limit),
		"gsroffset":     fmt.Sprintf("%d", request.Offset),
		"prop":          "imageinfo",
		"iiprop":        "url|extmetadata",
		"iiurlwidth":    "800",
		"format":        "json",
		"formatversion": "2",
	}
	rawURL := endpoint("https://commons.wikimedia.org/w/api.php", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Query struct {
			Pages []struct {
				Title     string `json:"title"`
				Index     int    `json:"index"`
				ImageInfo []struct {
					URL         string `json:"url"`
					Description string `json:"descriptionurl"`
					ThumbURL    string `json:"thumburl"`
					ExtMetadata map[string]struct {
						Value any `json:"value"`
					} `json:"extmetadata"`
				} `json:"imageinfo"`
			} `json:"pages"`
		} `json:"query"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	pages := payload.Query.Pages
	sort.SliceStable(pages, func(i, j int) bool {
		return pages[i].Index < pages[j].Index
	})
	results := make([]search.SearchResult, 0, len(pages))
	for _, page := range pages {
		if len(page.ImageInfo) == 0 {
			continue
		}
		info := page.ImageInfo[0]
		link := firstNonEmpty(info.Description, info.URL)
		if page.Title == "" || link == "" {
			continue
		}
		current := result(e.Descriptor().ID, strings.TrimPrefix(page.Title, "File:"), link, "", len(results)+1)
		current.Metadata.Type = "image"
		current.Metadata.MediaURL = info.URL
		current.Metadata.ThumbnailURL = firstNonEmpty(info.ThumbURL, info.URL)
		if metadata, ok := info.ExtMetadata["LicenseShortName"]; ok {
			current.Metadata.License = stripMarkup(anyString(metadata.Value))
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(pages) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type dailymotionEngine struct {
	fetcher *native.Fetcher
}

func NewDailymotionEngine(fetcher *native.Fetcher) *dailymotionEngine {
	return &dailymotionEngine{fetcher: fetcherFor(fetcher)}
}

func (e *dailymotionEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "dailymotion",
		Name:     "Dailymotion",
		Priority: 54,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindVideo},
			LanguageFilter:  true,
			Pagination:      true,
			TimeRangeFilter: false,
			MaxResults:      100,
		},
	}
}

func (e *dailymotionEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"search": strings.TrimSpace(request.Query),
		"limit":  fmt.Sprintf("%d", limit),
		"page":   fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"fields": "id,title,url,description,thumbnail_360_url,created_time,duration,language",
		"sort":   "relevance",
	}
	if request.Language != "" {
		values["language"] = request.Language
	}
	rawURL := endpoint("https://api.dailymotion.com/videos", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		List []struct {
			ID          string  `json:"id"`
			Title       string  `json:"title"`
			URL         string  `json:"url"`
			Description string  `json:"description"`
			Thumbnail   string  `json:"thumbnail_360_url"`
			CreatedTime int64   `json:"created_time"`
			Duration    float64 `json:"duration"`
			Language    string  `json:"language"`
		} `json:"list"`
		HasMore bool `json:"has_more"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.List))
	for _, item := range payload.List {
		link := item.URL
		if link == "" && item.ID != "" {
			link = "https://www.dailymotion.com/video/" + url.PathEscape(item.ID)
		}
		if item.Title == "" || link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, link, item.Description, len(results)+1)
		current.Language = item.Language
		current.Metadata.Type = "video"
		current.Metadata.ThumbnailURL = item.Thumbnail
		current.Metadata.DurationSeconds = item.Duration
		if item.CreatedTime > 0 {
			stamp := time.Unix(item.CreatedTime, 0).UTC()
			current.PublishedAt = &stamp
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: payload.HasMore, HTTPStatus: status, RawBytes: len(body)}, nil
}

type nasaMediaEngine struct {
	fetcher *native.Fetcher
}

func NewNASAMediaEngine(fetcher *native.Fetcher) *nasaMediaEngine {
	return &nasaMediaEngine{fetcher: fetcherFor(fetcher)}
}

func (e *nasaMediaEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "nasa_media",
		Name:     "NASA Image and Video Library",
		Priority: 58,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindImage, search.SearchKindVideo},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *nasaMediaEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	mediaType := "image"
	if request.Kind == search.SearchKindVideo {
		mediaType = "video"
	}
	values := map[string]string{
		"q":          strings.TrimSpace(request.Query),
		"media_type": mediaType,
		"page":       fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"page_size":  fmt.Sprintf("%d", limit),
	}
	rawURL := endpoint("https://images-api.nasa.gov/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Collection struct {
			Items []struct {
				Data []struct {
					NASAID      string `json:"nasa_id"`
					Title       string `json:"title"`
					Description string `json:"description"`
					DateCreated string `json:"date_created"`
					MediaType   string `json:"media_type"`
				} `json:"data"`
				Links []struct {
					Href string `json:"href"`
				} `json:"links"`
			} `json:"items"`
		} `json:"collection"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Collection.Items))
	for _, item := range payload.Collection.Items {
		if len(item.Data) == 0 || len(item.Links) == 0 {
			continue
		}
		data := item.Data[0]
		if data.Title == "" {
			continue
		}
		link := "https://images.nasa.gov/details/" + url.PathEscape(data.NASAID)
		current := result(e.Descriptor().ID, data.Title, link, data.Description, len(results)+1)
		if data.MediaType == "video" {
			current.Metadata.Type = "video"
		} else {
			current.Metadata.Type = "image"
		}
		current.Metadata.MediaURL = item.Links[0].Href
		current.Metadata.ThumbnailURL = item.Links[0].Href
		if parsed := parseTime(data.DateCreated); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Collection.Items) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type peerTubeEngine struct {
	fetcher *native.Fetcher
}

func NewPeerTubeEngine(fetcher *native.Fetcher) *peerTubeEngine {
	return &peerTubeEngine{fetcher: fetcherFor(fetcher)}
}

func (e *peerTubeEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "peertube",
		Name:     "PeerTube",
		Priority: 56,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindVideo},
			LanguageFilter:  true,
			Pagination:      true,
			TimeRangeFilter: false,
			MaxResults:      100,
		},
	}
}

func (e *peerTubeEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"search": strings.TrimSpace(request.Query),
		"count":  fmt.Sprintf("%d", limit),
		"start":  fmt.Sprintf("%d", request.Offset),
		"sort":   "-match",
	}
	if request.Language != "" {
		values["languageOneOf"] = request.Language
	}
	rawURL := endpoint("https://sepiasearch.org/api/v1/search/videos", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Total int `json:"total"`
		Data  []struct {
			Name         string  `json:"name"`
			URL          string  `json:"url"`
			Description  string  `json:"description"`
			ThumbnailURL string  `json:"thumbnailUrl"`
			Duration     float64 `json:"duration"`
			PublishedAt  string  `json:"publishedAt"`
			Language     struct {
				ID string `json:"id"`
			} `json:"language"`
		} `json:"data"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.Name == "" || item.URL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Name, item.URL, item.Description, len(results)+1)
		current.Language = item.Language.ID
		current.Metadata.Type = "video"
		current.Metadata.ThumbnailURL = item.ThumbnailURL
		current.Metadata.DurationSeconds = item.Duration
		if parsed := parseTime(item.PublishedAt); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}
