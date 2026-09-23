package engines

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
	"golang.org/x/net/html"
)

type mdnEngine struct {
	fetcher *native.Fetcher
}

type pdbeEngine struct {
	fetcher *native.Fetcher
}

func NewPDBEEngine(fetcher *native.Fetcher) *pdbeEngine {
	return &pdbeEngine{fetcher: fetcherFor(fetcher)}
}

func (e *pdbeEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "pdbe",
		Name:     "Protein Data Bank in Europe",
		Priority: 52,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindAcademic},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *pdbeEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"q":     strings.TrimSpace(request.Query),
		"wt":    "json",
		"rows":  fmt.Sprintf("%d", limit),
		"start": fmt.Sprintf("%d", request.Offset),
	}
	rawURL := endpoint("https://www.ebi.ac.uk/pdbe/search/pdb/select", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Response struct {
			NumFound int `json:"numFound"`
			Docs     []struct {
				PDBID        string   `json:"pdb_id"`
				Title        string   `json:"title"`
				MoleculeName []string `json:"molecule_name"`
				ReleaseDate  string   `json:"release_date"`
				Resolution   float64  `json:"resolution"`
			} `json:"docs"`
		} `json:"response"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Response.Docs))
	for _, item := range payload.Response.Docs {
		if item.PDBID == "" || item.Title == "" {
			continue
		}
		snippet := strings.Join(item.MoleculeName, ", ")
		if item.Resolution > 0 {
			if snippet != "" {
				snippet += " | "
			}
			snippet += fmt.Sprintf("resolution %.2f A", item.Resolution)
		}
		current := result(e.Descriptor().ID, item.Title, "https://www.ebi.ac.uk/pdbe/entry/pdb/"+url.PathEscape(item.PDBID), snippet, len(results)+1)
		current.Metadata.Type = "protein"
		current.Metadata.ProductID = item.PDBID
		if parsed := parseTime(item.ReleaseDate); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Response.NumFound, HTTPStatus: status, RawBytes: len(body)}, nil
}

type hoogleEngine struct {
	fetcher *native.Fetcher
}

func NewHoogleEngine(fetcher *native.Fetcher) *hoogleEngine {
	return &hoogleEngine{fetcher: fetcherFor(fetcher)}
}

func (e *hoogleEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "hoogle",
		Name:     "Hoogle",
		Priority: 47,
		Weight:   0.6,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			MaxResults:  100,
		},
	}
}

func (e *hoogleEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	values := map[string]string{
		"hoogle": strings.TrimSpace(request.Query),
		"mode":   "json",
		"count":  fmt.Sprintf("%d", boundedLimit(request.Limit, 8, 100)),
	}
	rawURL := endpoint("https://hoogle.haskell.org/", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[[]struct {
		URL    string `json:"url"`
		Item   string `json:"item"`
		Module struct {
			Name string `json:"name"`
		} `json:"module"`
		Package struct {
			Name string `json:"name"`
		} `json:"package"`
		Docs string `json:"docs"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload))
	for _, item := range payload {
		if item.Item == "" || item.URL == "" {
			continue
		}
		current := result(e.Descriptor().ID, stripMarkup(item.Item), item.URL, stripMarkup(item.Docs), len(results)+1)
		current.Metadata.Repository = item.Package.Name
		current.Metadata.Path = item.Module.Name
		current.Metadata.Type = "documentation"
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: false, HTTPStatus: status, RawBytes: len(body)}, nil
}

func NewMDNEngine(fetcher *native.Fetcher) *mdnEngine {
	return &mdnEngine{fetcher: fetcherFor(fetcher)}
}

func (e *mdnEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "mdn",
		Name:     "MDN",
		Priority: 57,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  true,
			MaxResults:  30,
		},
	}
}

func (e *mdnEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 30)
	values := map[string]string{
		"q":      strings.TrimSpace(request.Query),
		"locale": firstNonEmpty(request.Language, "en-US"),
		"page":   fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://developer.mozilla.org/api/v1/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Documents []struct {
			Title      string  `json:"title"`
			Slug       string  `json:"slug"`
			Summary    string  `json:"summary"`
			Locale     string  `json:"locale"`
			Popularity float64 `json:"popularity"`
		} `json:"documents"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Documents))
	for _, item := range payload.Documents {
		if item.Title == "" || item.Slug == "" {
			continue
		}
		link := "https://developer.mozilla.org/" + strings.Trim(item.Locale, "/") + "/docs/" + strings.Trim(item.Slug, "/")
		current := result(e.Descriptor().ID, item.Title, link, item.Summary, len(results)+1)
		current.Language = item.Locale
		current.Metadata.Type = "documentation"
		current.Metadata.Rating = floatPointer(item.Popularity)
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Documents) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type pyPIEngine struct {
	fetcher *native.Fetcher
}

func NewPyPIEngine(fetcher *native.Fetcher) *pyPIEngine {
	return &pyPIEngine{fetcher: fetcherFor(fetcher)}
}

func (e *pyPIEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "pypi",
		Name:     "PyPI",
		Priority: 54,
		Weight:   0.8,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindCode, search.SearchKindSoftware},
			Pagination:  false,
			MaxResults:  20,
		},
	}
}

func (e *pyPIEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	rawURL := endpoint("https://pypi.org/search/", map[string]string{"q": strings.TrimSpace(request.Query)})
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	document, err := parseHTML(body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	limit := boundedLimit(request.Limit, 8, 20)
	results := make([]search.SearchResult, 0, limit)
	for _, snippet := range allByClass(document, "package-snippet") {
		if len(results) >= limit {
			break
		}
		nameNode := findFirst(snippet, func(node *html.Node) bool {
			return hasClass(node, "package-snippet__name")
		})
		descriptionNode := findFirst(snippet, func(node *html.Node) bool {
			return hasClass(node, "package-snippet__description")
		})
		linkNode := findFirst(snippet, func(node *html.Node) bool {
			return node.Data == "a" && attribute(node, "href") != ""
		})
		name := nodeText(nameNode)
		link := attribute(linkNode, "href")
		if name == "" || link == "" {
			continue
		}
		if strings.HasPrefix(link, "/") {
			link = "https://pypi.org" + link
		}
		current := result(e.Descriptor().ID, name, link, nodeText(descriptionNode), len(results)+1)
		current.Metadata.Repository = name
		current.Metadata.Type = "package"
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(results) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type vimeoEngine struct {
	fetcher *native.Fetcher
}

func NewVimeoEngine(fetcher *native.Fetcher) *vimeoEngine {
	return &vimeoEngine{fetcher: fetcherFor(fetcher)}
}

func (e *vimeoEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "vimeo",
		Name:     "Vimeo",
		Priority: 73,
		Weight:   1,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindVideo},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *vimeoEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"query":    strings.TrimSpace(request.Query),
		"per_page": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://api.vimeo.com/videos", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, http.Header{"Authorization": []string{"Bearer " + credential}})
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Total int `json:"total"`
		Data  []struct {
			Name        string  `json:"name"`
			Description string  `json:"description"`
			Link        string  `json:"link"`
			Duration    float64 `json:"duration"`
			CreatedTime string  `json:"created_time"`
			Pictures    struct {
				Sizes []struct {
					Link string `json:"link"`
				} `json:"sizes"`
			} `json:"pictures"`
			User struct {
				Name string `json:"name"`
			} `json:"user"`
		} `json:"data"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		if item.Name == "" || item.Link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Name, item.Link, item.Description, len(results)+1)
		current.Metadata.Type = "video"
		current.Metadata.DurationSeconds = item.Duration
		current.Metadata.Merchant = item.User.Name
		if len(item.Pictures.Sizes) > 0 {
			current.Metadata.ThumbnailURL = item.Pictures.Sizes[len(item.Pictures.Sizes)-1].Link
		}
		if parsed := parseTime(item.CreatedTime); parsed != nil {
			current.PublishedAt = parsed
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}

type pexelsEngine struct {
	fetcher *native.Fetcher
}

func NewPexelsEngine(fetcher *native.Fetcher) *pexelsEngine {
	return &pexelsEngine{fetcher: fetcherFor(fetcher)}
}

func (e *pexelsEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "pexels",
		Name:     "Pexels",
		Priority: 66,
		Weight:   0.9,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindImage},
			Pagination:  true,
			MaxResults:  80,
		},
	}
}

func (e *pexelsEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 80)
	values := map[string]string{
		"query":    strings.TrimSpace(request.Query),
		"per_page": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://api.pexels.com/v1/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, http.Header{"Authorization": []string{credential}})
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		TotalResults int `json:"total_results"`
		Photos       []struct {
			ID           int    `json:"id"`
			URL          string `json:"url"`
			Photographer string `json:"photographer"`
			Alt          string `json:"alt"`
			Src          struct {
				Large string `json:"large"`
				Small string `json:"small"`
			} `json:"src"`
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
		title := firstNonEmpty(item.Alt, item.Photographer, fmt.Sprintf("Pexels photo %d", item.ID))
		current := result(e.Descriptor().ID, title, item.URL, item.Photographer, len(results)+1)
		current.Metadata.Type = "image"
		current.Metadata.MediaURL = item.Src.Large
		current.Metadata.ThumbnailURL = item.Src.Small
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.TotalResults, HTTPStatus: status, RawBytes: len(body)}, nil
}

type unsplashEngine struct {
	fetcher *native.Fetcher
}

func NewUnsplashEngine(fetcher *native.Fetcher) *unsplashEngine {
	return &unsplashEngine{fetcher: fetcherFor(fetcher)}
}

func (e *unsplashEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "unsplash",
		Name:     "Unsplash",
		Priority: 68,
		Weight:   1,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindImage},
			Pagination:  true,
			MaxResults:  30,
		},
	}
}

func (e *unsplashEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	credential := search.EngineCredentialFromContext(ctx, e.Descriptor().ID)
	if credential == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_AUTH_FAILED, e.Descriptor().ID, false, nil)
	}
	limit := boundedLimit(request.Limit, 8, 30)
	values := map[string]string{
		"query":    strings.TrimSpace(request.Query),
		"per_page": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
	}
	rawURL := endpoint("https://api.unsplash.com/search/photos", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, http.Header{"Authorization": []string{"Client-ID " + credential}})
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Total   int `json:"total"`
		Results []struct {
			ID             string `json:"id"`
			Description    string `json:"description"`
			AltDescription string `json:"alt_description"`
			Links          struct {
				HTML string `json:"html"`
			} `json:"links"`
			URLs struct {
				Regular string `json:"regular"`
				Small   string `json:"small"`
			} `json:"urls"`
			User struct {
				Name string `json:"name"`
			} `json:"user"`
		} `json:"results"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		if item.Links.HTML == "" {
			continue
		}
		title := firstNonEmpty(item.Description, item.AltDescription, item.ID)
		current := result(e.Descriptor().ID, title, item.Links.HTML, item.User.Name, len(results)+1)
		current.Metadata.Type = "image"
		current.Metadata.MediaURL = item.URLs.Regular
		current.Metadata.ThumbnailURL = item.URLs.Small
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: request.Offset+len(results) < payload.Total, HTTPStatus: status, RawBytes: len(body)}, nil
}
