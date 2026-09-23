package engines

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type redditEngine struct {
	fetcher *native.Fetcher
}

func NewRedditEngine(fetcher *native.Fetcher) *redditEngine {
	return &redditEngine{fetcher: fetcherFor(fetcher)}
}

func (e *redditEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "reddit",
		Name:     "Reddit",
		Priority: 48,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:      true,
			SearchKinds:     []search.SearchKind{search.SearchKindWeb, search.SearchKindNews, search.SearchKindSocial},
			Pagination:      false,
			TimeRangeFilter: true,
			MaxResults:      100,
		},
	}
}

func (e *redditEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":        strings.TrimSpace(request.Query),
		"limit":    fmt.Sprintf("%d", limit),
		"sort":     "relevance",
		"raw_json": "1",
	}
	rawURL := endpoint("https://www.reddit.com/search.json", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Data struct {
			After    string `json:"after"`
			Children []struct {
				Data struct {
					Title      string  `json:"title"`
					URL        string  `json:"url"`
					Permalink  string  `json:"permalink"`
					Subreddit  string  `json:"subreddit"`
					Selftext   string  `json:"selftext"`
					CreatedUTC float64 `json:"created_utc"`
					Score      int     `json:"score"`
					Thumbnail  string  `json:"thumbnail"`
				} `json:"data"`
			} `json:"children"`
		} `json:"data"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data.Children))
	start := request.Offset
	if start > len(payload.Data.Children) {
		start = len(payload.Data.Children)
	}
	for _, item := range payload.Data.Children[start:] {
		if len(results) >= limit {
			break
		}
		post := item.Data
		if post.Title == "" {
			continue
		}
		link := post.URL
		if link == "" && post.Permalink != "" {
			link = "https://www.reddit.com" + post.Permalink
		}
		if link == "" {
			continue
		}
		current := result(e.Descriptor().ID, post.Title, link, post.Selftext, len(results)+1)
		current.Language = request.Language
		current.Metadata.Type = "social"
		current.Metadata.Address = post.Subreddit
		current.Metadata.ReviewCount = post.Score
		if strings.HasPrefix(post.Thumbnail, "http") {
			current.Metadata.ThumbnailURL = post.Thumbnail
		}
		if post.CreatedUTC > 0 {
			stamp := time.Unix(int64(post.CreatedUTC), 0).UTC()
			current.PublishedAt = &stamp
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: payload.Data.After != "", HTTPStatus: status, RawBytes: len(body)}, nil
}

type lemmyEngine struct {
	fetcher *native.Fetcher
}

func NewLemmyEngine(fetcher *native.Fetcher) *lemmyEngine {
	return &lemmyEngine{fetcher: fetcherFor(fetcher)}
}

func (e *lemmyEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "lemmy",
		Name:     "Lemmy",
		Priority: 45,
		Weight:   0.6,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindSocial},
			Pagination:  true,
			MaxResults:  50,
		},
	}
}

func (e *lemmyEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 50)
	values := map[string]string{
		"q":     strings.TrimSpace(request.Query),
		"type_": "Posts",
		"limit": fmt.Sprintf("%d", limit),
		"page":  fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"sort":  "TopAll",
	}
	rawURL := endpoint("https://lemmy.world/api/v3/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Posts []struct {
			Post struct {
				ID        int    `json:"id"`
				Name      string `json:"name"`
				URL       string `json:"url"`
				Body      string `json:"body"`
				Published string `json:"published"`
			} `json:"post"`
			Creator struct {
				Name string `json:"name"`
			} `json:"creator"`
			Community struct {
				Name    string `json:"name"`
				ActorID string `json:"actor_id"`
			} `json:"community"`
			Counts struct {
				Score    int `json:"score"`
				Comments int `json:"comments"`
			} `json:"counts"`
		} `json:"posts"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Posts))
	for _, item := range payload.Posts {
		if item.Post.Name == "" {
			continue
		}
		link := item.Post.URL
		if link == "" && item.Post.ID > 0 {
			link = fmt.Sprintf("https://lemmy.world/post/%d", item.Post.ID)
		}
		if link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Post.Name, link, item.Post.Body, len(results)+1)
		current.Metadata.Type = "social"
		current.Metadata.Address = item.Community.Name
		current.Metadata.Merchant = item.Creator.Name
		current.Metadata.ReviewCount = item.Counts.Comments
		current.Metadata.Rating = floatPointer(float64(item.Counts.Score))
		if parsed := parseTime(item.Post.Published); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Posts) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type searchcodeEngine struct {
	fetcher *native.Fetcher
}

func NewSearchcodeEngine(fetcher *native.Fetcher) *searchcodeEngine {
	return &searchcodeEngine{fetcher: fetcherFor(fetcher)}
}

func (e *searchcodeEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "searchcode",
		Name:     "searchcode",
		Priority: 44,
		Weight:   0.6,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *searchcodeEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":        strings.TrimSpace(request.Query),
		"per_page": fmt.Sprintf("%d", limit),
		"p":        fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://searchcode.com/api/codesearch_I/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Total   int `json:"total"`
		Results []struct {
			Repo     string            `json:"repo"`
			Filename string            `json:"filename"`
			URL      string            `json:"url"`
			Lines    map[string]string `json:"lines"`
		} `json:"results"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		if item.URL == "" {
			continue
		}
		title := firstNonEmpty(item.Filename, item.Repo)
		snippet := ""
		for _, line := range item.Lines {
			if snippet == "" && strings.TrimSpace(line) != "" {
				snippet = line
				break
			}
		}
		current := result(e.Descriptor().ID, title, item.URL, snippet, len(results)+1)
		current.Metadata.Repository = item.Repo
		current.Metadata.Path = item.Filename
		current.Metadata.Type = "code"
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}
