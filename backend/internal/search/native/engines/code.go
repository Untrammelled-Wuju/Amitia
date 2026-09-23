package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type githubEngine struct {
	fetcher *native.Fetcher
}

func NewGitHubEngine(fetcher *native.Fetcher) *githubEngine {
	return &githubEngine{fetcher: fetcherFor(fetcher)}
}

func (e *githubEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "github",
		Name:     "GitHub",
		Priority: 74,
		Weight:   1.1,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *githubEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	query := strings.TrimSpace(request.Query)
	if request.Specialized.Code != nil {
		if language := strings.TrimSpace(request.Specialized.Code.Language); language != "" {
			query += " language:" + language
		}
		if repository := strings.TrimSpace(request.Specialized.Code.Repository); repository != "" {
			query += " repo:" + repository
		}
		if owner := strings.TrimSpace(request.Specialized.Code.Owner); owner != "" {
			query += " user:" + owner
		}
	}
	values := map[string]string{
		"q":        query,
		"per_page": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"sort":     "best-match",
	}
	rawURL := endpoint("https://api.github.com/search/repositories", values)
	headers := http.Header{
		"Accept":               []string{"application/vnd.github+json"},
		"X-GitHub-Api-Version": []string{"2022-11-28"},
	}
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, headers)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Items []struct {
			FullName        string   `json:"full_name"`
			HTMLURL         string   `json:"html_url"`
			Description     string   `json:"description"`
			Language        string   `json:"language"`
			StargazersCount int      `json:"stargazers_count"`
			Topics          []string `json:"topics"`
			License         *struct {
				SPDXID string `json:"spdx_id"`
			} `json:"license"`
		} `json:"items"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.FullName == "" || item.HTMLURL == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.FullName, item.HTMLURL, item.Description, len(results)+1)
		current.Metadata.Repository = item.FullName
		current.Metadata.Type = "repository"
		current.Language = item.Language
		current.Metadata.ReviewCount = item.StargazersCount
		if item.License != nil {
			current.Metadata.License = item.License.SPDXID
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Items) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type stackExchangeEngine struct {
	fetcher *native.Fetcher
}

func NewStackExchangeEngine(fetcher *native.Fetcher) *stackExchangeEngine {
	return &stackExchangeEngine{fetcher: fetcherFor(fetcher)}
}

func (e *stackExchangeEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "stackexchange",
		Name:     "Stack Exchange",
		Priority: 64,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *stackExchangeEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"site":     "stackoverflow",
		"q":        strings.TrimSpace(request.Query),
		"pagesize": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"order":    "desc",
		"sort":     "relevance",
		"filter":   "withbody",
	}
	rawURL := endpoint("https://api.stackexchange.com/2.3/search/advanced", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Items []struct {
			Title        string   `json:"title"`
			Link         string   `json:"link"`
			Excerpt      string   `json:"excerpt"`
			Body         string   `json:"body"`
			Tags         []string `json:"tags"`
			Score        int      `json:"score"`
			AnswerCount  int      `json:"answer_count"`
			CreationDate int64    `json:"creation_date"`
			AcceptedID   int64    `json:"accepted_answer_id"`
			IsAnswered   bool     `json:"is_answered"`
		} `json:"items"`
		HasMore bool `json:"has_more"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Items))
	for _, item := range payload.Items {
		if item.Title == "" || item.Link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.Link, firstNonEmpty(item.Excerpt, stripMarkup(item.Body)), len(results)+1)
		current.Metadata.Type = "question"
		current.Metadata.ReviewCount = item.AnswerCount
		if item.CreationDate > 0 {
			stamp := time.Unix(item.CreationDate, 0).UTC()
			current.PublishedAt = &stamp
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: payload.HasMore, HTTPStatus: status, RawBytes: len(body)}, nil
}

type gitLabEngine struct {
	fetcher *native.Fetcher
}

func NewGitLabEngine(fetcher *native.Fetcher) *gitLabEngine {
	return &gitLabEngine{fetcher: fetcherFor(fetcher)}
}

func (e *gitLabEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "gitlab",
		Name:     "GitLab",
		Priority: 60,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *gitLabEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"search":   strings.TrimSpace(request.Query),
		"per_page": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"order_by": "star_count",
		"sort":     "desc",
	}
	rawURL := endpoint("https://gitlab.com/api/v4/projects", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[[]struct {
		Name              string `json:"name"`
		PathWithNamespace string `json:"path_with_namespace"`
		WebURL            string `json:"web_url"`
		Description       string `json:"description"`
		StarCount         int    `json:"star_count"`
		DefaultBranch     string `json:"default_branch"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload))
	for _, item := range payload {
		if item.WebURL == "" {
			continue
		}
		title := firstNonEmpty(item.PathWithNamespace, item.Name)
		current := result(e.Descriptor().ID, title, item.WebURL, item.Description, len(results)+1)
		current.Metadata.Repository = item.PathWithNamespace
		current.Metadata.Type = "repository"
		current.Metadata.ReviewCount = item.StarCount
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type npmEngine struct {
	fetcher *native.Fetcher
}

func NewNPMEngine(fetcher *native.Fetcher) *npmEngine {
	return &npmEngine{fetcher: fetcherFor(fetcher)}
}

func (e *npmEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "npm",
		Name:     "npm",
		Priority: 56,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode},
			Pagination:  true,
			MaxResults:  250,
		},
	}
}

func (e *npmEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"text": strings.TrimSpace(request.Query),
		"size": fmt.Sprintf("%d", limit),
		"from": fmt.Sprintf("%d", request.Offset),
	}
	rawURL := endpoint("https://registry.npmjs.org/-/v1/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Objects []struct {
			Package struct {
				Name        string `json:"name"`
				Version     string `json:"version"`
				Description string `json:"description"`
				Links       struct {
					NPM        string `json:"npm"`
					Repository string `json:"repository"`
				} `json:"links"`
			} `json:"package"`
		} `json:"objects"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Objects))
	for _, item := range payload.Objects {
		link := firstNonEmpty(item.Package.Links.NPM, item.Package.Links.Repository)
		if item.Package.Name == "" || link == "" {
			continue
		}
		title := item.Package.Name
		if item.Package.Version != "" {
			title += "@" + item.Package.Version
		}
		current := result(e.Descriptor().ID, title, link, item.Package.Description, len(results)+1)
		current.Metadata.Repository = item.Package.Name
		current.Metadata.Type = "package"
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Objects) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type cratesEngine struct {
	fetcher *native.Fetcher
}

func NewCratesEngine(fetcher *native.Fetcher) *cratesEngine {
	return &cratesEngine{fetcher: fetcherFor(fetcher)}
}

func (e *cratesEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "crates",
		Name:     "crates.io",
		Priority: 54,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *cratesEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":        strings.TrimSpace(request.Query),
		"per_page": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://crates.io/api/v1/crates", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Crates []struct {
			Name          string `json:"name"`
			Description   string `json:"description"`
			Repository    string `json:"repository"`
			Documentation string `json:"documentation"`
			Downloads     int    `json:"downloads"`
		} `json:"crates"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Crates))
	for _, item := range payload.Crates {
		link := firstNonEmpty(item.Repository, item.Documentation)
		if link == "" && item.Name != "" {
			link = "https://crates.io/crates/" + url.PathEscape(item.Name)
		}
		if item.Name == "" || link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Name, link, item.Description, len(results)+1)
		current.Metadata.Repository = item.Name
		current.Metadata.Type = "package"
		current.Metadata.ReviewCount = item.Downloads
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Crates) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}
