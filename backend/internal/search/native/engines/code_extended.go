package engines

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type mavenCentralEngine struct {
	fetcher *native.Fetcher
}

func NewMavenCentralEngine(fetcher *native.Fetcher) *mavenCentralEngine {
	return &mavenCentralEngine{fetcher: fetcherFor(fetcher)}
}

func (e *mavenCentralEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "maven_central",
		Name:     "Maven Central",
		Priority: 57,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *mavenCentralEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":     strings.TrimSpace(request.Query),
		"rows":  fmt.Sprintf("%d", limit),
		"start": fmt.Sprintf("%d", request.Offset),
		"wt":    "json",
	}
	rawURL := endpoint("https://search.maven.org/solrsearch/select", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Response struct {
			NumFound int `json:"numFound"`
			Docs     []struct {
				ID            string `json:"id"`
				GroupID       string `json:"g"`
				ArtifactID    string `json:"a"`
				LatestVersion string `json:"latestVersion"`
				Timestamp     int64  `json:"timestamp"`
			} `json:"docs"`
		} `json:"response"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Response.Docs))
	for _, item := range payload.Response.Docs {
		if item.ID == "" {
			continue
		}
		link := "https://search.maven.org/artifact/" + url.PathEscape(item.GroupID) + "/" + url.PathEscape(item.ArtifactID)
		if item.LatestVersion != "" {
			link += "/" + url.PathEscape(item.LatestVersion) + "/jar"
		}
		current := result(e.Descriptor().ID, item.ID, link, "", len(results)+1)
		current.Metadata.Repository = item.ID
		current.Metadata.Type = "package"
		if item.Timestamp > 0 {
			stamp := time.UnixMilli(item.Timestamp).UTC()
			current.PublishedAt = &stamp
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Response.NumFound, HTTPStatus: status, RawBytes: len(body)}, nil
}

type dockerHubEngine struct {
	fetcher *native.Fetcher
}

func NewDockerHubEngine(fetcher *native.Fetcher) *dockerHubEngine {
	return &dockerHubEngine{fetcher: fetcherFor(fetcher)}
}

func (e *dockerHubEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "docker_hub",
		Name:     "Docker Hub",
		Priority: 53,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *dockerHubEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"query":     strings.TrimSpace(request.Query),
		"page_size": fmt.Sprintf("%d", limit),
		"page":      fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://hub.docker.com/v2/search/repositories/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Count   int `json:"count"`
		Results []struct {
			RepoName         string `json:"repo_name"`
			ShortDescription string `json:"short_description"`
			StarCount        int    `json:"star_count"`
			PullCount        int    `json:"pull_count"`
			IsOfficial       bool   `json:"is_official"`
			UpdatedAt        string `json:"updated_at"`
		} `json:"results"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		if item.RepoName == "" {
			continue
		}
		link := "https://hub.docker.com/r/" + item.RepoName
		current := result(e.Descriptor().ID, item.RepoName, link, item.ShortDescription, len(results)+1)
		current.Metadata.Repository = item.RepoName
		current.Metadata.Type = "container"
		current.Metadata.ReviewCount = item.PullCount
		current.Metadata.Rating = nil
		if item.IsOfficial {
			current.Metadata.License = "official"
		}
		if parsed := parseTime(item.UpdatedAt); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Count, HTTPStatus: status, RawBytes: len(body)}, nil
}

type huggingFaceEngine struct {
	fetcher *native.Fetcher
}

func NewHuggingFaceEngine(fetcher *native.Fetcher) *huggingFaceEngine {
	return &huggingFaceEngine{fetcher: fetcherFor(fetcher)}
}

func (e *huggingFaceEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "huggingface",
		Name:     "Hugging Face",
		Priority: 56,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  1000,
		},
	}
}

func (e *huggingFaceEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"search":    strings.TrimSpace(request.Query),
		"limit":     fmt.Sprintf("%d", limit),
		"skip":      fmt.Sprintf("%d", request.Offset),
		"sort":      "downloads",
		"direction": "-1",
	}
	rawURL := endpoint("https://huggingface.co/api/models", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[[]struct {
		ID           string `json:"id"`
		ModelID      string `json:"modelId"`
		PipelineTag  string `json:"pipeline_tag"`
		Downloads    int    `json:"downloads"`
		Likes        int    `json:"likes"`
		LastModified string `json:"lastModified"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload))
	for _, item := range payload {
		id := firstNonEmpty(item.ModelID, item.ID)
		if id == "" {
			continue
		}
		link := "https://huggingface.co/" + id
		current := result(e.Descriptor().ID, id, link, item.PipelineTag, len(results)+1)
		current.Metadata.Repository = id
		current.Metadata.Type = "model"
		current.Metadata.ReviewCount = item.Downloads
		if parsed := parseTime(item.LastModified); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}
