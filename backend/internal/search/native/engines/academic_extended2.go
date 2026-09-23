package engines

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type plosEngine struct {
	fetcher *native.Fetcher
}

func NewPLOSEngine(fetcher *native.Fetcher) *plosEngine {
	return &plosEngine{fetcher: fetcherFor(fetcher)}
}

func (e *plosEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "plos",
		Name:     "PLOS",
		Priority: 56,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic},
			Pagination:      true,
			TimeRangeFilter: false,
			MaxResults:      100,
		},
	}
}

func (e *plosEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":     strings.TrimSpace(request.Query),
		"rows":  fmt.Sprintf("%d", limit),
		"start": fmt.Sprintf("%d", request.Offset),
		"wt":    "json",
		"fl":    "id,title_display,author_display,abstract,publication_date,journal,doi",
	}
	rawURL := endpoint("https://api.plos.org/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Response struct {
			NumFound int `json:"numFound"`
			Docs     []struct {
				ID              string   `json:"id"`
				Title           string   `json:"title_display"`
				Authors         []string `json:"author_display"`
				Abstract        []string `json:"abstract"`
				PublicationDate string   `json:"publication_date"`
				Journal         string   `json:"journal"`
				DOI             string   `json:"doi"`
			} `json:"docs"`
		} `json:"response"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Response.Docs))
	for _, item := range payload.Response.Docs {
		link := ""
		if item.DOI != "" {
			link = "https://doi.org/" + item.DOI
		}
		if link == "" && item.ID != "" {
			link = "https://journals.plos.org/plosone/article?id=" + url.QueryEscape(item.ID)
		}
		if item.Title == "" || link == "" {
			continue
		}
		snippet := ""
		if len(item.Abstract) > 0 {
			snippet = item.Abstract[0]
		}
		current := result(e.Descriptor().ID, item.Title, link, snippet, len(results)+1)
		current.Metadata.Type = "paper"
		current.Metadata.Journal = item.Journal
		current.Metadata.DOI = item.DOI
		current.Metadata.Authors = append(current.Metadata.Authors, item.Authors...)
		if parsed := parseTime(item.PublicationDate); parsed != nil {
			current.PublishedAt = parsed
			current.Metadata.Year = parsed.Year()
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Response.NumFound, HTTPStatus: status, RawBytes: len(body)}, nil
}

type osfEngine struct {
	fetcher *native.Fetcher
}

func NewOSFEngine(fetcher *native.Fetcher) *osfEngine {
	return &osfEngine{fetcher: fetcherFor(fetcher)}
}

func (e *osfEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "osf",
		Name:     "Open Science Framework",
		Priority: 52,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic, search.SearchKindFiles},
			Pagination:      true,
			TimeRangeFilter: false,
			MaxResults:      100,
		},
	}
}

func (e *osfEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":            strings.TrimSpace(request.Query),
		"page[size]":   fmt.Sprintf("%d", limit),
		"page[number]": fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://api.osf.io/v2/search/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Links struct {
			Next string `json:"next"`
		} `json:"links"`
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Title       string `json:"title"`
				Description string `json:"description"`
				DateCreated string `json:"date_created"`
				Public      bool   `json:"public"`
			} `json:"attributes"`
			Links struct {
				HTML string `json:"html"`
			} `json:"links"`
		} `json:"data"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.Attributes.Title == "" {
			continue
		}
		link := item.Links.HTML
		if link == "" && item.ID != "" {
			link = "https://osf.io/" + url.PathEscape(item.ID)
		}
		if link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Attributes.Title, link, item.Attributes.Description, len(results)+1)
		current.Metadata.Type = "research"
		if parsed := parseTime(item.Attributes.DateCreated); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: payload.Links.Next != "", HTTPStatus: status, RawBytes: len(body)}, nil
}

type figshareEngine struct {
	fetcher *native.Fetcher
}

func NewFigshareEngine(fetcher *native.Fetcher) *figshareEngine {
	return &figshareEngine{fetcher: fetcherFor(fetcher)}
}

func (e *figshareEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "figshare",
		Name:     "Figshare",
		Priority: 50,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic, search.SearchKindFiles},
			Pagination:      true,
			TimeRangeFilter: false,
			MaxResults:      100,
		},
	}
}

func (e *figshareEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	payload := map[string]any{
		"search_for":      strings.TrimSpace(request.Query),
		"page":            pageNumber(request.Offset, limit),
		"page_size":       limit,
		"order":           "published_date",
		"order_direction": "desc",
	}
	body, status, err := e.fetcher.PostJSON(ctx, e.Descriptor().ID, "https://api.figshare.com/v2/articles/search", nil, payload)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	items, err := decodeJSON[[]struct {
		ID              int    `json:"id"`
		Title           string `json:"title"`
		URLPublic       string `json:"url_public_api"`
		URLPrivate      string `json:"url"`
		DOI             string `json:"doi"`
		Description     string `json:"description"`
		PublishedDate   string `json:"published_date"`
		DefinedTypeName string `json:"defined_type_name"`
		Authors         []struct {
			FullName string `json:"full_name"`
		} `json:"authors"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(items))
	for _, item := range items {
		if item.Title == "" {
			continue
		}
		link := firstNonEmpty(item.URLPublic, item.URLPrivate, "https://doi.org/"+item.DOI)
		if link == "" && item.ID > 0 {
			link = fmt.Sprintf("https://figshare.com/articles/%d", item.ID)
		}
		if link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, link, item.Description, len(results)+1)
		current.Metadata.Type = firstNonEmpty(item.DefinedTypeName, "research")
		current.Metadata.DOI = item.DOI
		for _, author := range item.Authors {
			if name := strings.TrimSpace(author.FullName); name != "" {
				current.Metadata.Authors = append(current.Metadata.Authors, name)
			}
		}
		if parsed := parseTime(item.PublishedDate); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(items) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type doajEngine struct {
	fetcher *native.Fetcher
}

func NewDOAJEngine(fetcher *native.Fetcher) *doajEngine {
	return &doajEngine{fetcher: fetcherFor(fetcher)}
}

func (e *doajEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "doaj",
		Name:     "DOAJ",
		Priority: 55,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindAcademic},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *doajEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	query := url.PathEscape(strings.TrimSpace(request.Query))
	values := map[string]string{
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"pageSize": fmt.Sprintf("%d", limit),
	}
	rawURL := endpoint("https://doaj.org/api/v3/search/articles/"+query, values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Total   int `json:"total"`
		Results []struct {
			BibJSON struct {
				Title    string `json:"title"`
				Abstract string `json:"abstract"`
				Year     int    `json:"year"`
				Journal  struct {
					Title string `json:"title"`
				} `json:"journal"`
				Authors []struct {
					Name string `json:"name"`
				} `json:"author"`
				Identifiers []struct {
					Type string `json:"type"`
					ID   string `json:"id"`
				} `json:"identifier"`
				Links []struct {
					Type string `json:"type"`
					URL  string `json:"url"`
				} `json:"link"`
			} `json:"bibjson"`
		} `json:"results"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		if item.BibJSON.Title == "" {
			continue
		}
		link := ""
		doi := ""
		for _, identifier := range item.BibJSON.Identifiers {
			if strings.EqualFold(identifier.Type, "doi") {
				doi = identifier.ID
				link = "https://doi.org/" + identifier.ID
				break
			}
		}
		if link == "" {
			for _, itemLink := range item.BibJSON.Links {
				if itemLink.URL != "" {
					link = itemLink.URL
					break
				}
			}
		}
		if link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.BibJSON.Title, link, item.BibJSON.Abstract, len(results)+1)
		current.Metadata.Type = "paper"
		current.Metadata.Journal = item.BibJSON.Journal.Title
		current.Metadata.DOI = doi
		current.Metadata.Year = item.BibJSON.Year
		for _, author := range item.BibJSON.Authors {
			if name := strings.TrimSpace(author.Name); name != "" {
				current.Metadata.Authors = append(current.Metadata.Authors, name)
			}
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}
