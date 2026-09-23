package engines

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type europePMCBatchEngine struct {
	fetcher *native.Fetcher
}

func NewEuropePMCBatchEngine(fetcher *native.Fetcher) *europePMCBatchEngine {
	return &europePMCBatchEngine{fetcher: fetcherFor(fetcher)}
}

func (e *europePMCBatchEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "europepmc",
		Name:     "Europe PMC",
		Priority: 63,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic},
			LanguageFilter:  false,
			Pagination:      true,
			TimeRangeFilter: false,
			MaxResults:      100,
		},
	}
}

func (e *europePMCBatchEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"query":    strings.TrimSpace(request.Query),
		"format":   "json",
		"pageSize": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://www.ebi.ac.uk/europepmc/webservices/rest/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		HitCount int `json:"hitCount"`
		Result   struct {
			Items []struct {
				ID           string `json:"id"`
				Source       string `json:"source"`
				Title        string `json:"title"`
				AuthorString string `json:"authorString"`
				JournalTitle string `json:"journalTitle"`
				PubYear      string `json:"pubYear"`
				DOI          string `json:"doi"`
				FirstPubDate string `json:"firstPublicationDate"`
				AbstractText string `json:"abstractText"`
			} `json:"result"`
		} `json:"resultList"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Result.Items))
	for _, item := range payload.Result.Items {
		if item.Title == "" {
			continue
		}
		link := ""
		if item.DOI != "" {
			link = "https://doi.org/" + item.DOI
		}
		if link == "" && item.ID != "" && item.Source != "" {
			link = "https://europepmc.org/article/" + url.PathEscape(item.Source) + "/" + url.PathEscape(item.ID)
		}
		if link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, link, item.AbstractText, len(results)+1)
		current.Language = request.Language
		current.Metadata.Type = "paper"
		current.Metadata.DOI = item.DOI
		current.Metadata.Journal = item.JournalTitle
		if item.AuthorString != "" {
			current.Metadata.Authors = []string{item.AuthorString}
		}
		if parsed := parseTime(item.FirstPubDate); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.HitCount, HTTPStatus: status, RawBytes: len(body)}, nil
}

type dataCiteEngine struct {
	fetcher *native.Fetcher
}

func NewDataCiteEngine(fetcher *native.Fetcher) *dataCiteEngine {
	return &dataCiteEngine{fetcher: fetcherFor(fetcher)}
}

func (e *dataCiteEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "datacite",
		Name:     "DataCite",
		Priority: 57,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic},
			Pagination:      true,
			TimeRangeFilter: false,
			MaxResults:      100,
		},
	}
}

func (e *dataCiteEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := url.Values{}
	values.Set("query", strings.TrimSpace(request.Query))
	values.Set("page[size]", fmt.Sprintf("%d", limit))
	values.Set("page[number]", fmt.Sprintf("%d", pageNumber(request.Offset, limit)))
	rawURL := "https://api.datacite.org/dois?" + values.Encode()
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				DOI    string `json:"doi"`
				URL    string `json:"url"`
				Titles []struct {
					Title string `json:"title"`
				} `json:"titles"`
				Descriptions []struct {
					Description string `json:"description"`
				} `json:"descriptions"`
				Published string `json:"published"`
				Creators  []struct {
					Name string `json:"name"`
				} `json:"creators"`
				Types struct {
					ResourceTypeGeneral string `json:"resourceTypeGeneral"`
				} `json:"types"`
			} `json:"attributes"`
		} `json:"data"`
		Meta struct {
			Total int `json:"total"`
		} `json:"meta"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		title := ""
		if len(item.Attributes.Titles) > 0 {
			title = item.Attributes.Titles[0].Title
		}
		link := firstNonEmpty(item.Attributes.URL, "https://doi.org/"+item.Attributes.DOI, "https://doi.org/"+item.ID)
		if title == "" || link == "" {
			continue
		}
		snippet := ""
		if len(item.Attributes.Descriptions) > 0 {
			snippet = item.Attributes.Descriptions[0].Description
		}
		current := result(e.Descriptor().ID, title, link, snippet, len(results)+1)
		current.Metadata.Type = firstNonEmpty(item.Attributes.Types.ResourceTypeGeneral, "dataset")
		current.Metadata.DOI = item.Attributes.DOI
		for _, creator := range item.Attributes.Creators {
			if name := strings.TrimSpace(creator.Name); name != "" {
				current.Metadata.Authors = append(current.Metadata.Authors, name)
			}
		}
		if parsed := parseTime(item.Attributes.Published); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Meta.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}

type zenodoEngine struct {
	fetcher *native.Fetcher
}

func NewZenodoEngine(fetcher *native.Fetcher) *zenodoEngine {
	return &zenodoEngine{fetcher: fetcherFor(fetcher)}
}

func (e *zenodoEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "zenodo",
		Name:     "Zenodo",
		Priority: 55,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic},
			Pagination:      true,
			TimeRangeFilter: false,
			MaxResults:      100,
		},
	}
}

func (e *zenodoEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":    strings.TrimSpace(request.Query),
		"size": fmt.Sprintf("%d", limit),
		"page": fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"sort": "bestmatch",
	}
	rawURL := endpoint("https://zenodo.org/api/records", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Hits struct {
			Total int `json:"total"`
			Hits  []struct {
				Metadata struct {
					Title           string `json:"title"`
					DOI             string `json:"doi"`
					PublicationDate string `json:"publication_date"`
					Description     string `json:"description"`
					ResourceType    struct {
						Type string `json:"type"`
					} `json:"resource_type"`
					Creators []struct {
						Name string `json:"name"`
					} `json:"creators"`
				} `json:"metadata"`
				Links struct {
					HTML string `json:"self_html"`
				} `json:"links"`
			} `json:"hits"`
		} `json:"hits"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Hits.Hits))
	for _, item := range payload.Hits.Hits {
		link := firstNonEmpty(item.Links.HTML, "https://doi.org/"+item.Metadata.DOI)
		if item.Metadata.Title == "" || link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Metadata.Title, link, stripMarkup(item.Metadata.Description), len(results)+1)
		current.Metadata.Type = firstNonEmpty(item.Metadata.ResourceType.Type, "publication")
		current.Metadata.DOI = item.Metadata.DOI
		for _, creator := range item.Metadata.Creators {
			if name := strings.TrimSpace(creator.Name); name != "" {
				current.Metadata.Authors = append(current.Metadata.Authors, name)
			}
		}
		if parsed := parseTime(item.Metadata.PublicationDate); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Hits.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}

type halEngine struct {
	fetcher *native.Fetcher
}

func NewHALEngine(fetcher *native.Fetcher) *halEngine {
	return &halEngine{fetcher: fetcherFor(fetcher)}
}

func (e *halEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "hal",
		Name:     "HAL",
		Priority: 54,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic},
			LanguageFilter:  true,
			Pagination:      true,
			TimeRangeFilter: false,
			MaxResults:      100,
		},
	}
}

func (e *halEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":     strings.TrimSpace(request.Query),
		"wt":    "json",
		"rows":  fmt.Sprintf("%d", limit),
		"start": fmt.Sprintf("%d", request.Offset),
		"fl":    "title_s,uri_s,abstract_s,authFullName_s,producedDate_s,doiId_s,docType_s",
	}
	if request.Language != "" {
		values["lang"] = request.Language
	}
	rawURL := endpoint("https://api.archives-ouvertes.fr/search/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Response struct {
			NumFound int `json:"numFound"`
			Docs     []struct {
				Title        string   `json:"title_s"`
				URI          string   `json:"uri_s"`
				Abstract     string   `json:"abstract_s"`
				Authors      []string `json:"authFullName_s"`
				ProducedDate string   `json:"producedDate_s"`
				DOI          string   `json:"doiId_s"`
				DocType      string   `json:"docType_s"`
			} `json:"docs"`
		} `json:"response"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Response.Docs))
	for _, item := range payload.Response.Docs {
		if item.Title == "" || item.URI == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, item.URI, stripMarkup(item.Abstract), len(results)+1)
		current.Language = request.Language
		current.Metadata.Type = firstNonEmpty(item.DocType, "paper")
		current.Metadata.DOI = item.DOI
		current.Metadata.Authors = append(current.Metadata.Authors, item.Authors...)
		if parsed := parseTime(item.ProducedDate); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Response.NumFound, HTTPStatus: status, RawBytes: len(body)}, nil
}

type openLibraryEngine struct {
	fetcher *native.Fetcher
}

func NewOpenLibraryEngine(fetcher *native.Fetcher) *openLibraryEngine {
	return &openLibraryEngine{fetcher: fetcherFor(fetcher)}
}

func (e *openLibraryEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "openlibrary",
		Name:     "Open Library",
		Priority: 50,
		Weight:   0.7,
		Capabilities: search.ProviderCapabilities{
			GeneralWeb:      true,
			SearchKinds:     []search.SearchKind{search.SearchKindWeb, search.SearchKindAcademic},
			LanguageFilter:  false,
			Pagination:      true,
			TimeRangeFilter: false,
			MaxResults:      100,
		},
	}
}

func (e *openLibraryEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":      strings.TrimSpace(request.Query),
		"limit":  fmt.Sprintf("%d", limit),
		"page":   fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"fields": "key,title,author_name,first_publish_year,edition_key,cover_i,ia,language",
	}
	rawURL := endpoint("https://openlibrary.org/search.json", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		NumFound int `json:"numFound"`
		Docs     []struct {
			Key              string   `json:"key"`
			Title            string   `json:"title"`
			AuthorName       []string `json:"author_name"`
			FirstPublishYear int      `json:"first_publish_year"`
			EditionKey       []string `json:"edition_key"`
			CoverID          int      `json:"cover_i"`
			IA               []string `json:"ia"`
			Language         []string `json:"language"`
		} `json:"docs"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Docs))
	for _, item := range payload.Docs {
		if item.Title == "" || item.Key == "" {
			continue
		}
		link := "https://openlibrary.org" + item.Key
		current := result(e.Descriptor().ID, item.Title, link, "", len(results)+1)
		current.Metadata.Type = "book"
		current.Metadata.Authors = append(current.Metadata.Authors, item.AuthorName...)
		current.Metadata.Year = item.FirstPublishYear
		if len(item.Language) > 0 {
			current.Language = item.Language[0]
		}
		if item.CoverID > 0 {
			current.Metadata.ThumbnailURL = fmt.Sprintf("https://covers.openlibrary.org/b/id/%d-M.jpg", item.CoverID)
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.NumFound, HTTPStatus: status, RawBytes: len(body)}, nil
}
