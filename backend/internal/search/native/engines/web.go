package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

var languageCodePattern = regexp.MustCompile(`^[a-z]{2,3}(?:-[a-z0-9]{2,8})?$`)

type wikipediaEngine struct {
	fetcher *native.Fetcher
}

func NewWikipediaEngine(fetcher *native.Fetcher) *wikipediaEngine {
	return &wikipediaEngine{fetcher: fetcherFor(fetcher)}
}

func (e *wikipediaEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "wikipedia",
		Name:     "Wikipedia",
		Priority: 62,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:     true,
			SearchKinds:    []search.SearchKind{search.SearchKindWeb},
			LanguageFilter: true,
			Pagination:     true,
			MaxResults:     50,
		},
	}
}

func (e *wikipediaEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	language := strings.ToLower(strings.TrimSpace(request.Language))
	if language == "" {
		language = "en"
	}
	language = strings.Split(language, "-")[0]
	if !languageCodePattern.MatchString(language) {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_INVALID_LANGUAGE, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 50)
	values := map[string]string{
		"action":   "query",
		"list":     "search",
		"srsearch": queryWithDomains(request.Query, request.Domains),
		"srlimit":  fmt.Sprintf("%d", limit),
		"sroffset": fmt.Sprintf("%d", request.Offset),
		"format":   "json",
		"utf8":     "1",
	}
	rawURL := endpoint("https://"+language+".wikipedia.org/w/api.php", values)
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
		link := "https://" + language + ".wikipedia.org/wiki/" + url.PathEscape(strings.ReplaceAll(item.Title, " ", "_"))
		current := result(e.Descriptor().ID, item.Title, link, stripMarkup(item.Snippet), len(results)+1)
		current.Language = language
		if parsed := parseTime(item.Timestamp); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Query.Search) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type hackerNewsEngine struct {
	fetcher *native.Fetcher
}

func NewHackerNewsEngine(fetcher *native.Fetcher) *hackerNewsEngine {
	return &hackerNewsEngine{fetcher: fetcherFor(fetcher)}
}

func (e *hackerNewsEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "hackernews",
		Name:     "Hacker News",
		Priority: 58,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:      true,
			SearchKinds:     []search.SearchKind{search.SearchKindWeb, search.SearchKindCode, search.SearchKindSocial},
			Pagination:      true,
			TimeRangeFilter: true,
			DomainFilter:    true,
			MaxResults:      100,
		},
	}
}

func (e *hackerNewsEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"query":                        queryWithDomains(request.Query, request.Domains),
		"hitsPerPage":                  fmt.Sprintf("%d", limit),
		"page":                         fmt.Sprintf("%d", request.Offset/limit),
		"tags":                         "story",
		"restrictSearchableAttributes": "title,url,story_text",
	}
	if request.TimeRange != nil && request.TimeRange.From != nil {
		values["numericFilters"] = fmt.Sprintf("created_at_i>%d", request.TimeRange.From.Unix())
	}
	rawURL := endpoint("https://hn.algolia.com/api/v1/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Hits []struct {
			Title       string `json:"title"`
			StoryTitle  string `json:"story_title"`
			URL         string `json:"url"`
			StoryURL    string `json:"story_url"`
			StoryText   string `json:"story_text"`
			CommentText string `json:"comment_text"`
			CreatedAt   string `json:"created_at"`
			ObjectID    string `json:"objectID"`
		} `json:"hits"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Hits))
	for _, item := range payload.Hits {
		title := firstNonEmpty(item.Title, item.StoryTitle)
		link := firstNonEmpty(item.URL, item.StoryURL)
		if link == "" && item.ObjectID != "" {
			link = "https://news.ycombinator.com/item?id=" + url.QueryEscape(item.ObjectID)
		}
		if title == "" || link == "" {
			continue
		}
		current := result(e.Descriptor().ID, title, link, firstNonEmpty(item.StoryText, item.CommentText), len(results)+1)
		if parsed := parseTime(item.CreatedAt); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Hits) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type internetArchiveEngine struct {
	fetcher *native.Fetcher
}

func NewInternetArchiveEngine(fetcher *native.Fetcher) *internetArchiveEngine {
	return &internetArchiveEngine{fetcher: fetcherFor(fetcher)}
}

func (e *internetArchiveEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "internet_archive",
		Name:     "Internet Archive",
		Priority: 52,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:  true,
			SearchKinds: []search.SearchKind{search.SearchKindWeb, search.SearchKindFiles},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *internetArchiveEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := url.Values{}
	values.Set("q", strings.TrimSpace(request.Query))
	values.Set("rows", fmt.Sprintf("%d", limit))
	values.Set("page", fmt.Sprintf("%d", pageNumber(request.Offset, limit)))
	values.Set("output", "json")
	values.Add("fl[]", "identifier")
	values.Add("fl[]", "title")
	values.Add("fl[]", "description")
	values.Add("fl[]", "date")
	rawURL := "https://archive.org/advancedsearch.php?" + values.Encode()
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Response struct {
			Docs []struct {
				Identifier  string `json:"identifier"`
				Title       any    `json:"title"`
				Description any    `json:"description"`
				Date        string `json:"date"`
			} `json:"docs"`
		} `json:"response"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Response.Docs))
	for _, item := range payload.Response.Docs {
		title := anyString(item.Title)
		if title == "" {
			title = item.Identifier
		}
		if title == "" || item.Identifier == "" {
			continue
		}
		link := "https://archive.org/details/" + url.PathEscape(item.Identifier)
		current := result(e.Descriptor().ID, title, link, anyString(item.Description), len(results)+1)
		if parsed := parseTime(item.Date); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Response.Docs) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type braveAPIEngine struct {
	fetcher *native.Fetcher
}

func NewBraveAPIEngine(fetcher *native.Fetcher) *braveAPIEngine {
	return &braveAPIEngine{fetcher: fetcherFor(fetcher)}
}

func (e *braveAPIEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "brave_api",
		Name:     "Brave Search",
		Priority: 90,
		Weight:   1.4,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:      true,
			SearchKinds:     []search.SearchKind{search.SearchKindWeb, search.SearchKindNews, search.SearchKindImage, search.SearchKindVideo},
			LanguageFilter:  true,
			CountryFilter:   true,
			SafeSearch:      true,
			Pagination:      true,
			TimeRangeFilter: true,
			DomainFilter:    true,
			MaxResults:      20,
		},
	}
}

func (e *braveAPIEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, "brave_api")
	if credential == "" {
		credential = search.ProviderCredentialFromContext(ctx)
	}
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 20)
	endpointPath := "https://api.search.brave.com/res/v1/web/search"
	if request.Kind == search.SearchKindImage {
		endpointPath = "https://api.search.brave.com/res/v1/images/search"
	} else if request.Kind == search.SearchKindVideo {
		endpointPath = "https://api.search.brave.com/res/v1/videos/search"
	}
	values := map[string]string{
		"q":                queryWithDomains(request.Query, request.Domains),
		"count":            fmt.Sprintf("%d", limit),
		"offset":           fmt.Sprintf("%d", request.Offset),
		"search_lang":      request.Language,
		"country":          request.Country,
		"safesearch":       string(request.SafeSearch),
		"text_decorations": "0",
	}
	if request.Kind == search.SearchKindNews {
		values["result_filter"] = "news"
	}
	if request.TimeRange != nil && request.TimeRange.From != nil {
		values["freshness"] = freshness(request.TimeRange.From.String())
	}
	rawURL := endpoint(endpointPath, values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, http.Header{
		"X-Subscription-Token": []string{credential},
	})
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Age         string `json:"age"`
			} `json:"results"`
		} `json:"web"`
		News struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Age         string `json:"age"`
			} `json:"results"`
		} `json:"news"`
		Images struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Thumbnail   struct {
					Src string `json:"src"`
				} `json:"thumbnail"`
			} `json:"results"`
		} `json:"images"`
		Videos struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Thumbnail   struct {
					Src string `json:"src"`
				} `json:"thumbnail"`
				Video struct {
					Duration string `json:"duration"`
				} `json:"video"`
			} `json:"results"`
		} `json:"videos"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	items := payload.Web.Results
	if request.Kind == search.SearchKindNews && len(payload.News.Results) > 0 {
		items = payload.News.Results
	}
	results := make([]search.SearchResult, 0, len(items))
	for _, item := range items {
		if item.Title == "" || item.URL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.URL, item.Description, len(results)+1)
		current.Language = request.Language
		results = append(results, current)
	}
	if request.Kind == search.SearchKindImage {
		results := make([]search.SearchResult, 0, len(payload.Images.Results))
		for _, item := range payload.Images.Results {
			if item.Title == "" || item.URL == "" {
				continue
			}
			current := result(e.Descriptor().ID, item.Title, item.URL, item.Description, len(results)+1)
			current.Metadata.Type = "image"
			current.Metadata.MediaURL = item.URL
			current.Metadata.ThumbnailURL = item.Thumbnail.Src
			results = append(results, current)
		}
		return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Images.Results) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
	}
	if request.Kind == search.SearchKindVideo {
		results := make([]search.SearchResult, 0, len(payload.Videos.Results))
		for _, item := range payload.Videos.Results {
			if item.Title == "" || item.URL == "" {
				continue
			}
			current := result(e.Descriptor().ID, item.Title, item.URL, item.Description, len(results)+1)
			current.Metadata.Type = "video"
			current.Metadata.ThumbnailURL = item.Thumbnail.Src
			results = append(results, current)
		}
		return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Videos.Results) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(items) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

func queryWithDomains(query string, domains []string) string {
	query = strings.TrimSpace(query)
	if len(domains) == 0 {
		return query
	}
	parts := make([]string, 0, len(domains))
	for _, domain := range domains {
		if value := strings.TrimSpace(domain); value != "" {
			parts = append(parts, "site:"+value)
		}
	}
	if len(parts) == 0 {
		return query
	}
	if len(parts) == 1 {
		return strings.TrimSpace(query + " " + parts[0])
	}
	return strings.TrimSpace(query + " (" + strings.Join(parts, " OR ") + ")")
}

func freshness(value string) string {
	parsed := parseTime(value)
	if parsed == nil {
		return ""
	}
	age := time.Since(*parsed)
	switch {
	case age <= 48*time.Hour:
		return "pd"
	case age <= 10*24*time.Hour:
		return "pw"
	case age <= 45*24*time.Hour:
		return "pm"
	default:
		return "py"
	}
}

func anyString(value any) string {
	switch current := value.(type) {
	case string:
		return strings.TrimSpace(current)
	case float64:
		return strconv.FormatFloat(current, 'f', -1, 64)
	case bool:
		if current {
			return "true"
		}
		return "false"
	case map[string]any:
		return anyString(current["value"])
	case []any:
		values := make([]string, 0, len(current))
		for _, item := range current {
			if text := anyString(item); text != "" {
				values = append(values, text)
			}
		}
		return strings.Join(values, " ")
	default:
		return ""
	}
}
