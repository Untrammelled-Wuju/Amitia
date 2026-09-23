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

type googleCSEEngine struct {
	fetcher *native.Fetcher
}

func NewGoogleCSEEngine(fetcher *native.Fetcher) *googleCSEEngine {
	return &googleCSEEngine{fetcher: fetcherFor(fetcher)}
}

func (e *googleCSEEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "google_cse",
		Name:     "Google Programmable Search",
		Priority: 95,
		Weight:   1.5,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:      true,
			SearchKinds:     []search.SearchKind{search.SearchKindWeb, search.SearchKindImage},
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

func (e *googleCSEEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	var config struct {
		APIKey string `json:"apiKey"`
		CX     string `json:"cx"`
	}
	if err := json.Unmarshal([]byte(credential), &config); err != nil || strings.TrimSpace(config.APIKey) == "" || strings.TrimSpace(config.CX) == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, err)
	}
	limit := boundedLimit(request.Limit, 8, 10)
	values := map[string]string{
		"key":   strings.TrimSpace(config.APIKey),
		"cx":    strings.TrimSpace(config.CX),
		"q":     queryWithDomains(request.Query, request.Domains),
		"num":   fmt.Sprintf("%d", limit),
		"start": fmt.Sprintf("%d", request.Offset+1),
	}
	if request.Kind == search.SearchKindImage {
		values["searchType"] = "image"
	}
	if request.Language != "" {
		values["hl"] = request.Language
	}
	if request.Country != "" {
		values["gl"] = request.Country
	}
	if request.SafeSearch != "" {
		values["safe"] = string(request.SafeSearch)
	}
	rawURL := endpoint("https://www.googleapis.com/customsearch/v1", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Items []struct {
			Title   string `json:"title"`
			Link    string `json:"link"`
			Snippet string `json:"snippet"`
			Image   *struct {
				ContextLink   string `json:"contextLink"`
				ThumbnailLink string `json:"thumbnailLink"`
			} `json:"image"`
		} `json:"items"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.Title == "" || item.Link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.Link, item.Snippet, len(results)+1)
		if item.Image != nil {
			current.Metadata.Type = "image"
			current.Metadata.MediaURL = item.Link
			current.Metadata.ThumbnailURL = item.Image.ThumbnailLink
			if item.Image.ContextLink != "" {
				current.Metadata.Address = item.Image.ContextLink
			}
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Items) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type ebayBrowseEngine struct {
	fetcher *native.Fetcher
}

type googleBooksEngine struct {
	fetcher *native.Fetcher
}

func NewGoogleBooksEngine(fetcher *native.Fetcher) *googleBooksEngine {
	return &googleBooksEngine{fetcher: fetcherFor(fetcher)}
}

func (e *googleBooksEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "google_books",
		Name:     "Google Books",
		Priority: 58,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:     true,
			SearchKinds:    []search.SearchKind{search.SearchKindWeb, search.SearchKindAcademic},
			LanguageFilter: true,
			Pagination:     true,
			MaxResults:     40,
		},
	}
}

func (e *googleBooksEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 40)
	values := map[string]string{
		"q":            strings.TrimSpace(request.Query),
		"maxResults":   fmt.Sprintf("%d", limit),
		"startIndex":   fmt.Sprintf("%d", request.Offset),
		"langRestrict": request.Language,
	}
	if credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID); credential != "" {
		values["key"] = credential
	}
	rawURL := endpoint("https://www.googleapis.com/books/v1/volumes", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		TotalItems int `json:"totalItems"`
		Items      []struct {
			ID         string `json:"id"`
			VolumeInfo struct {
				Title         string   `json:"title"`
				Authors       []string `json:"authors"`
				Description   string   `json:"description"`
				PublishedDate string   `json:"publishedDate"`
				PreviewLink   string   `json:"previewLink"`
				InfoLink      string   `json:"infoLink"`
				Language      string   `json:"language"`
				ImageLinks    struct {
					Thumbnail string `json:"thumbnail"`
				} `json:"imageLinks"`
			} `json:"volumeInfo"`
		} `json:"items"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.VolumeInfo.Title == "" {
			continue
		}
		link := firstNonEmpty(item.VolumeInfo.InfoLink, item.VolumeInfo.PreviewLink, "https://books.google.com/books?id="+url.PathEscape(item.ID))
		if link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.VolumeInfo.Title, link, item.VolumeInfo.Description, len(results)+1)
		current.Language = item.VolumeInfo.Language
		current.Metadata.Type = "book"
		current.Metadata.Authors = append(current.Metadata.Authors, item.VolumeInfo.Authors...)
		current.Metadata.ThumbnailURL = item.VolumeInfo.ImageLinks.Thumbnail
		if parsed := parseTime(item.VolumeInfo.PublishedDate); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.TotalItems, HTTPStatus: status, RawBytes: len(body)}, nil
}

type youTubeEngine struct {
	fetcher *native.Fetcher
}

func NewYouTubeEngine(fetcher *native.Fetcher) *youTubeEngine {
	return &youTubeEngine{fetcher: fetcherFor(fetcher)}
}

func (e *youTubeEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "youtube",
		Name:     "YouTube",
		Priority: 78,
		Weight:   1.2,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindVideo},
			LanguageFilter:  true,
			CountryFilter:   true,
			SafeSearch:      true,
			TimeRangeFilter: true,
			MaxResults:      50,
		},
	}
}

func (e *youTubeEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 50)
	values := map[string]string{
		"part":       "snippet",
		"q":          strings.TrimSpace(request.Query),
		"type":       "video",
		"maxResults": fmt.Sprintf("%d", limit),
		"key":        credential,
	}
	if request.Language != "" {
		values["relevanceLanguage"] = request.Language
	}
	if request.Country != "" {
		values["regionCode"] = request.Country
	}
	if request.SafeSearch != "" {
		values["safeSearch"] = string(request.SafeSearch)
	}
	rawURL := endpoint("https://www.googleapis.com/youtube/v3/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Items []struct {
			ID struct {
				VideoID string `json:"videoId"`
			} `json:"id"`
			Snippet struct {
				Title        string `json:"title"`
				Description  string `json:"description"`
				PublishedAt  string `json:"publishedAt"`
				ChannelTitle string `json:"channelTitle"`
				Thumbnails   struct {
					High struct {
						URL string `json:"url"`
					} `json:"high"`
				} `json:"thumbnails"`
			} `json:"snippet"`
		} `json:"items"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.ID.VideoID == "" || item.Snippet.Title == "" {
			continue
		}
		link := "https://www.youtube.com/watch?v=" + url.QueryEscape(item.ID.VideoID)
		current := result(e.Descriptor().ID, item.Snippet.Title, link, item.Snippet.Description, len(results)+1)
		current.Language = request.Language
		current.Metadata.Type = "video"
		current.Metadata.ThumbnailURL = item.Snippet.Thumbnails.High.URL
		current.Metadata.Merchant = item.Snippet.ChannelTitle
		if parsed := parseTime(item.Snippet.PublishedAt); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Items) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

func NewEbayBrowseEngine(fetcher *native.Fetcher) *ebayBrowseEngine {
	return &ebayBrowseEngine{fetcher: fetcherFor(fetcher)}
}

func (e *ebayBrowseEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "ebay_browse",
		Name:     "eBay Browse",
		Priority: 70,
		Weight:   1,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:    []search.SearchKind{search.SearchKindProduct},
			LanguageFilter: false,
			CountryFilter:  true,
			Pagination:     true,
			DomainFilter:   false,
			MaxResults:     200,
		},
	}
}

func (e *ebayBrowseEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":      strings.TrimSpace(request.Query),
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", request.Offset),
	}
	rawURL := endpoint("https://api.ebay.com/buy/browse/v1/item_summary/search", values)
	headers := http.Header{
		"Authorization":           []string{"Bearer " + credential},
		"X-EBAY-C-MARKETPLACE-ID": []string{firstNonEmpty(request.Country, "EBAY_US")},
	}
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, headers)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Total         int `json:"total"`
		ItemSummaries []struct {
			ItemID           string `json:"itemId"`
			Title            string `json:"title"`
			ItemWebURL       string `json:"itemWebUrl"`
			ShortDescription string `json:"shortDescription"`
			Image            struct {
				ImageURL string `json:"imageUrl"`
			} `json:"image"`
			Price struct {
				Value    string `json:"value"`
				Currency string `json:"currency"`
			} `json:"price"`
			Seller struct {
				Username string `json:"username"`
			} `json:"seller"`
		} `json:"itemSummaries"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.ItemSummaries))
	for _, item := range payload.ItemSummaries {
		if item.Title == "" || item.ItemWebURL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.ItemWebURL, item.ShortDescription, len(results)+1)
		current.Metadata.Type = "product"
		current.Metadata.ProductID = item.ItemID
		current.Metadata.ThumbnailURL = item.Image.ImageURL
		current.Metadata.Merchant = item.Seller.Username
		current.Metadata.Currency = item.Price.Currency
		if item.Price.Value != "" {
			if price, parseErr := parseFloat(item.Price.Value); parseErr == nil {
				current.Metadata.Price = &price
			}
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}

func parseFloat(value string) (float64, error) {
	var parsed float64
	_, err := fmt.Sscanf(strings.TrimSpace(value), "%f", &parsed)
	return parsed, err
}
