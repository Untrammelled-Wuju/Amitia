package engines

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type nugetEngine struct {
	fetcher *native.Fetcher
}

func NewNuGetEngine(fetcher *native.Fetcher) *nugetEngine {
	return &nugetEngine{fetcher: fetcherFor(fetcher)}
}

func (e *nugetEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "nuget",
		Name:     "NuGet",
		Priority: 52,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *nugetEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":    strings.TrimSpace(request.Query),
		"skip": fmt.Sprintf("%d", request.Offset),
		"take": fmt.Sprintf("%d", limit),
	}
	rawURL := endpoint("https://azuresearch-usnc.nuget.org/query", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		TotalHits int `json:"totalHits"`
		Data      []struct {
			ID             string   `json:"id"`
			Version        string   `json:"version"`
			Description    string   `json:"description"`
			ProjectURL     string   `json:"projectUrl"`
			Tags           []string `json:"tags"`
			TotalDownloads int      `json:"totalDownloads"`
		} `json:"data"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.ID == "" {
			continue
		}
		link := firstNonEmpty(item.ProjectURL, "https://www.nuget.org/packages/"+url.PathEscape(item.ID))
		title := item.ID
		if item.Version != "" {
			title += " " + item.Version
		}
		current := result(e.Descriptor().ID, title, link, item.Description, len(results)+1)
		current.Metadata.Repository = item.ID
		current.Metadata.Type = "package"
		current.Metadata.ReviewCount = item.TotalDownloads
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.TotalHits, HTTPStatus: status, RawBytes: len(body)}, nil
}

type packagistEngine struct {
	fetcher *native.Fetcher
}

func NewPackagistEngine(fetcher *native.Fetcher) *packagistEngine {
	return &packagistEngine{fetcher: fetcherFor(fetcher)}
}

func (e *packagistEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "packagist",
		Name:     "Packagist",
		Priority: 50,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *packagistEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":        strings.TrimSpace(request.Query),
		"per_page": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://packagist.org/search.json", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Total   int `json:"total"`
		Results []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			URL         string `json:"url"`
			Repository  string `json:"repository"`
			Downloads   int    `json:"downloads"`
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
		link := firstNonEmpty(item.Repository, item.URL, "https://packagist.org/packages/"+url.PathEscape(item.Name))
		current := result(e.Descriptor().ID, item.Name, link, item.Description, len(results)+1)
		current.Metadata.Repository = item.Name
		current.Metadata.Type = "package"
		current.Metadata.ReviewCount = item.Downloads
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}

type rubyGemsEngine struct {
	fetcher *native.Fetcher
}

func NewRubyGemsEngine(fetcher *native.Fetcher) *rubyGemsEngine {
	return &rubyGemsEngine{fetcher: fetcherFor(fetcher)}
}

func (e *rubyGemsEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "rubygems",
		Name:     "RubyGems",
		Priority: 48,
		Weight:   0.6,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  30,
		},
	}
}

func (e *rubyGemsEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	rawURL := endpoint("https://rubygems.org/api/v1/search.json", map[string]string{"query": strings.TrimSpace(request.Query)})
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[[]struct {
		Name       string `json:"name"`
		Version    string `json:"version"`
		Info       string `json:"info"`
		ProjectURI string `json:"project_uri"`
		GemURI     string `json:"gem_uri"`
		Downloads  int    `json:"downloads"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	start := request.Offset
	if start > len(payload) {
		start = len(payload)
	}
	limit := boundedLimit(request.Limit, 8, 30)
	end := start + limit
	if end > len(payload) {
		end = len(payload)
	}
	results := make([]search.SearchResult, 0, end-start)
	for _, item := range payload[start:end] {
		if item.Name == "" {
			continue
		}
		link := firstNonEmpty(item.ProjectURI, item.GemURI, "https://rubygems.org/gems/"+url.PathEscape(item.Name))
		current := result(e.Descriptor().ID, item.Name, link, item.Info, len(results)+1)
		current.Metadata.Repository = item.Name
		current.Metadata.Type = "package"
		current.Metadata.ReviewCount = item.Downloads
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: end < len(payload), HTTPStatus: status, RawBytes: len(body)}, nil
}

type codebergEngine struct {
	fetcher *native.Fetcher
}

func NewCodebergEngine(fetcher *native.Fetcher) *codebergEngine {
	return &codebergEngine{fetcher: fetcherFor(fetcher)}
}

func (e *codebergEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "codeberg",
		Name:     "Codeberg",
		Priority: 49,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  50,
		},
	}
}

func (e *codebergEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 50)
	values := map[string]string{
		"q":     strings.TrimSpace(request.Query),
		"limit": fmt.Sprintf("%d", limit),
		"page":  fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://codeberg.org/api/v1/repos/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		OK   bool `json:"ok"`
		Data []struct {
			FullName    string `json:"full_name"`
			HTMLURL     string `json:"html_url"`
			Description string `json:"description"`
			Stars       int    `json:"stars_count"`
			Language    string `json:"language"`
		} `json:"data"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.FullName == "" || item.HTMLURL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.FullName, item.HTMLURL, item.Description, len(results)+1)
		current.Language = item.Language
		current.Metadata.Repository = item.FullName
		current.Metadata.Type = "repository"
		current.Metadata.ReviewCount = item.Stars
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Data) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type bitbucketEngine struct {
	fetcher *native.Fetcher
}

func NewBitbucketEngine(fetcher *native.Fetcher) *bitbucketEngine {
	return &bitbucketEngine{fetcher: fetcherFor(fetcher)}
}

func (e *bitbucketEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "bitbucket",
		Name:     "Bitbucket",
		Priority: 47,
		Weight:   0.6,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *bitbucketEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":       `name~"` + strings.ReplaceAll(strings.TrimSpace(request.Query), `"`, "") + `"`,
		"pagelen": fmt.Sprintf("%d", limit),
		"page":    fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://api.bitbucket.org/2.0/repositories", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Size   int `json:"size"`
		Values []struct {
			FullName string `json:"full_name"`
			Links    struct {
				HTML struct {
					Href string `json:"href"`
				} `json:"html"`
			} `json:"links"`
			Description string `json:"description"`
			Language    string `json:"language"`
		} `json:"values"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Values))
	for _, item := range payload.Values {
		if item.FullName == "" || item.Links.HTML.Href == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.FullName, item.Links.HTML.Href, item.Description, len(results)+1)
		current.Language = item.Language
		current.Metadata.Repository = item.FullName
		current.Metadata.Type = "repository"
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Size, HTTPStatus: status, RawBytes: len(body)}, nil
}

type softwareHeritageEngine struct {
	fetcher *native.Fetcher
}

func NewSoftwareHeritageEngine(fetcher *native.Fetcher) *softwareHeritageEngine {
	return &softwareHeritageEngine{fetcher: fetcherFor(fetcher)}
}

func (e *softwareHeritageEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "software_heritage",
		Name:     "Software Heritage",
		Priority: 46,
		Weight:   0.6,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindFiles, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *softwareHeritageEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	query := url.PathEscape(strings.TrimSpace(request.Query))
	values := map[string]string{
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", request.Offset),
	}
	rawURL := endpoint("https://archive.softwareheritage.org/api/1/origin/search/"+query+"/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[[]struct {
		URL        string   `json:"url"`
		VisitTypes []string `json:"visit_types"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload))
	for _, item := range payload {
		if item.URL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.URL, "https://archive.softwareheritage.org/browse/origin/?origin_url="+url.QueryEscape(item.URL), strings.Join(item.VisitTypes, ", "), len(results)+1)
		current.Metadata.Repository = item.URL
		current.Metadata.Type = "source"
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}
