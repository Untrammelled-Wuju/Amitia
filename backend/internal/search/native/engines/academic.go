package engines

import (
	"context"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/search"
	"github.com/u-ai/backend/internal/search/native"
)

type arXivEngine struct {
	fetcher *native.Fetcher
}

func NewarXivEngine(fetcher *native.Fetcher) *arXivEngine {
	return &arXivEngine{fetcher: fetcherFor(fetcher)}
}

func (e *arXivEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "arxiv",
		Name:     "arXiv",
		Priority: 72,
		Weight:   1,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic},
			Pagination:      true,
			TimeRangeFilter: true,
			MaxResults:      50,
		},
	}
}

func (e *arXivEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 50)
	query := strings.TrimSpace(request.Query)
	if query == "" {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_INVALID_QUERY, e.Descriptor().ID, false, nil)
	}
	values := map[string]string{
		"search_query": "all:\"" + query + "\"",
		"start":        fmt.Sprintf("%d", request.Offset),
		"max_results":  fmt.Sprintf("%d", limit),
		"sortBy":       "relevance",
		"sortOrder":    "descending",
	}
	if request.TimeRange != nil {
		if request.TimeRange.From != nil {
			values["start_date"] = request.TimeRange.From.UTC().Format("200601021504")
		}
		if request.TimeRange.To != nil {
			values["end_date"] = request.TimeRange.To.UTC().Format("200601021504")
		}
	}
	rawURL := endpoint("https://export.arxiv.org/api/query", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, http.Header{"Accept": []string{"application/atom+xml"}})
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	var feed struct {
		Entries []struct {
			Title     string `xml:"title"`
			Summary   string `xml:"summary"`
			Published string `xml:"published"`
			Updated   string `xml:"updated"`
			Links     []struct {
				Href string `xml:"href,attr"`
				Rel  string `xml:"rel,attr"`
			} `xml:"link"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal(body, &feed); err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		link := ""
		for _, candidate := range entry.Links {
			if candidate.Rel == "alternate" {
				link = candidate.Href
				break
			}
		}
		if link == "" {
			continue
		}
		item := result(e.Descriptor().ID, entry.Title, link, entry.Summary, len(results)+1)
		if parsed := parseTime(entry.Published, entry.Updated); parsed != nil {
			item.PublishedAt = parsed
		}
		item.Metadata.Type = "paper"
		results = append(results, item)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(feed.Entries) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type crossrefEngine struct {
	fetcher *native.Fetcher
}

func NewCrossrefEngine(fetcher *native.Fetcher) *crossrefEngine {
	return &crossrefEngine{fetcher: fetcherFor(fetcher)}
}

func (e *crossrefEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "crossref",
		Name:     "Crossref",
		Priority: 70,
		Weight:   1,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic},
			Pagination:      true,
			TimeRangeFilter: true,
			MaxResults:      100,
		},
	}
}

func (e *crossrefEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"query":  strings.TrimSpace(request.Query),
		"rows":   fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", request.Offset),
		"select": "DOI,title,URL,author,published,container-title,abstract,type",
	}
	if request.TimeRange != nil {
		if request.TimeRange.From != nil {
			values["from-pub-date"] = request.TimeRange.From.UTC().Format("2006-01-02")
		}
		if request.TimeRange.To != nil {
			values["until-pub-date"] = request.TimeRange.To.UTC().Format("2006-01-02")
		}
	}
	rawURL := endpoint("https://api.crossref.org/works", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Message struct {
			Items []struct {
				DOI       string   `json:"DOI"`
				Title     []string `json:"title"`
				URL       string   `json:"URL"`
				Abstract  string   `json:"abstract"`
				Type      string   `json:"type"`
				Container []string `json:"container-title"`
				Author    []struct {
					Given  string `json:"given"`
					Family string `json:"family"`
				} `json:"author"`
				Published struct {
					DateParts [][]int `json:"date-parts"`
				} `json:"published"`
			} `json:"items"`
		} `json:"message"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Message.Items))
	for _, item := range payload.Message.Items {
		title := firstString(item.Title)
		link := strings.TrimSpace(item.URL)
		if link == "" && item.DOI != "" {
			link = "https://doi.org/" + strings.TrimPrefix(item.DOI, "https://doi.org/")
		}
		if title == "" || link == "" {
			continue
		}
		current := result(e.Descriptor().ID, title, link, stripMarkup(item.Abstract), len(results)+1)
		current.Metadata.DOI = item.DOI
		current.Metadata.Type = item.Type
		current.Metadata.Journal = firstString(item.Container)
		for _, author := range item.Author {
			name := strings.TrimSpace(strings.TrimSpace(author.Given + " " + author.Family))
			if name != "" {
				current.Metadata.Authors = append(current.Metadata.Authors, name)
			}
		}
		if len(item.Published.DateParts) > 0 && len(item.Published.DateParts[0]) > 0 {
			parts := item.Published.DateParts[0]
			year := parts[0]
			month := 1
			day := 1
			if len(parts) > 1 {
				month = parts[1]
			}
			if len(parts) > 2 {
				day = parts[2]
			}
			stamp := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
			current.PublishedAt = &stamp
			current.Metadata.Year = year
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(results) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type openAlexEngine struct {
	fetcher *native.Fetcher
}

func NewOpenAlexEngine(fetcher *native.Fetcher) *openAlexEngine {
	return &openAlexEngine{fetcher: fetcherFor(fetcher)}
}

func (e *openAlexEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "openalex",
		Name:     "OpenAlex",
		Priority: 68,
		Weight:   1,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic},
			Pagination:      true,
			TimeRangeFilter: true,
			MaxResults:      200,
		},
	}
}

func (e *openAlexEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"search":   strings.TrimSpace(request.Query),
		"per-page": fmt.Sprintf("%d", limit),
		"page":     fmt.Sprintf("%d", pageNumber(request.Offset, limit)),
		"sort":     "relevance_score:desc",
	}
	var filters []string
	if request.TimeRange != nil {
		if request.TimeRange.From != nil {
			filters = append(filters, "from_publication_date:"+request.TimeRange.From.UTC().Format("2006-01-02"))
		}
		if request.TimeRange.To != nil {
			filters = append(filters, "to_publication_date:"+request.TimeRange.To.UTC().Format("2006-01-02"))
		}
	}
	if len(filters) > 0 {
		values["filter"] = strings.Join(filters, ",")
	}
	rawURL := endpoint("https://api.openalex.org/works", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Results []struct {
			Title           string `json:"display_name"`
			DOI             string `json:"doi"`
			PublicationYear int    `json:"publication_year"`
			PublicationDate string `json:"publication_date"`
			Authorships     []struct {
				Author struct {
					DisplayName string `json:"display_name"`
				} `json:"author"`
			} `json:"authorships"`
			PrimaryLocation struct {
				LandingPageURL string `json:"landing_page_url"`
				PDFURL         string `json:"pdf_url"`
			} `json:"primary_location"`
			BestOA *struct {
				LandingPageURL string `json:"landing_page_url"`
				PDFURL         string `json:"pdf_url"`
			} `json:"best_oa_location"`
			AbstractInvertedIndex map[string][]int `json:"abstract_inverted_index"`
		} `json:"results"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Results))
	for _, item := range payload.Results {
		link := firstNonEmpty(item.PrimaryLocation.LandingPageURL, item.DOI)
		if item.BestOA != nil {
			link = firstNonEmpty(item.BestOA.LandingPageURL, item.PrimaryLocation.LandingPageURL, item.DOI)
		}
		if item.Title == "" || link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, link, rebuildAbstract(item.AbstractInvertedIndex), len(results)+1)
		current.Metadata.DOI = strings.TrimPrefix(item.DOI, "https://doi.org/")
		current.Metadata.Year = item.PublicationYear
		current.Metadata.Type = "paper"
		for _, authorship := range item.Authorships {
			if name := strings.TrimSpace(authorship.Author.DisplayName); name != "" {
				current.Metadata.Authors = append(current.Metadata.Authors, name)
			}
		}
		if item.PublicationDate != "" {
			if parsed, parseErr := time.Parse("2006-01-02", item.PublicationDate); parseErr == nil {
				current.PublishedAt = &parsed
			}
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Results) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type semanticScholarEngine struct {
	fetcher *native.Fetcher
}

func NewSemanticScholarEngine(fetcher *native.Fetcher) *semanticScholarEngine {
	return &semanticScholarEngine{fetcher: fetcherFor(fetcher)}
}

func (e *semanticScholarEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "semantic_scholar",
		Name:     "Semantic Scholar",
		Priority: 66,
		Weight:   1,
		Capabilities: search.ProviderCapabilities{
			SearchKinds: []search.SearchKind{search.SearchKindAcademic},
			Pagination:  true,
			MaxResults:  100,
		},
	}
}

func (e *semanticScholarEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	values := map[string]string{
		"query":  strings.TrimSpace(request.Query),
		"limit":  fmt.Sprintf("%d", limit),
		"offset": fmt.Sprintf("%d", request.Offset),
		"fields": "title,url,abstract,year,authors,externalIds,openAccessPdf,venue,publicationDate",
	}
	rawURL := endpoint("https://api.semanticscholar.org/graph/v1/paper/search", values)
	body, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, rawURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	payload, err := decodeJSON[struct {
		Data []struct {
			Title           string `json:"title"`
			URL             string `json:"url"`
			Abstract        string `json:"abstract"`
			Year            int    `json:"year"`
			Venue           string `json:"venue"`
			PublicationDate string `json:"publicationDate"`
			Authors         []struct {
				Name string `json:"name"`
			} `json:"authors"`
			ExternalIDs struct {
				DOI string `json:"DOI"`
			} `json:"externalIds"`
			OpenAccessPDF *struct {
				URL string `json:"url"`
			} `json:"openAccessPdf"`
		} `json:"data"`
	}](body)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(payload.Data))
	for _, item := range payload.Data {
		link := item.URL
		if item.OpenAccessPDF != nil && item.OpenAccessPDF.URL != "" {
			link = item.OpenAccessPDF.URL
		}
		if link == "" && item.ExternalIDs.DOI != "" {
			link = "https://doi.org/" + item.ExternalIDs.DOI
		}
		if item.Title == "" || link == "" {
			continue
		}
		current := result(e.Descriptor().ID, item.Title, link, item.Abstract, len(results)+1)
		current.Metadata.Year = item.Year
		current.Metadata.Journal = item.Venue
		current.Metadata.DOI = item.ExternalIDs.DOI
		current.Metadata.Type = "paper"
		for _, author := range item.Authors {
			if strings.TrimSpace(author.Name) != "" {
				current.Metadata.Authors = append(current.Metadata.Authors, strings.TrimSpace(author.Name))
			}
		}
		if item.PublicationDate != "" {
			if parsed, parseErr := time.Parse("2006-01-02", item.PublicationDate); parseErr == nil {
				current.PublishedAt = &parsed
			}
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(payload.Data) >= limit, HTTPStatus: status, RawBytes: len(body)}, nil
}

type pubmedEngine struct {
	fetcher *native.Fetcher
}

func NewPubMedEngine(fetcher *native.Fetcher) *pubmedEngine {
	return &pubmedEngine{fetcher: fetcherFor(fetcher)}
}

func (e *pubmedEngine) Descriptor() native.EngineDescriptor {
	return native.EngineDescriptor{
		ID:       "pubmed",
		Name:     "PubMed",
		Priority: 64,
		Weight:   1,
		Capabilities: search.ProviderCapabilities{
			SearchKinds:     []search.SearchKind{search.SearchKindAcademic},
			Pagination:      true,
			TimeRangeFilter: true,
			MaxResults:      100,
		},
	}
}

func (e *pubmedEngine) Search(ctx context.Context, request search.SearchRequest) (search.ProviderSearchResponse, error) {
	limit := boundedLimit(request.Limit, 8, 100)
	query := strings.TrimSpace(request.Query)
	if request.TimeRange != nil && request.TimeRange.From != nil {
		query = fmt.Sprintf("(%s) AND (\"%s\"[Date - Publication] : \"3000\"[Date - Publication])", query, request.TimeRange.From.UTC().Format("2006/01/02"))
	}
	searchURL := endpoint("https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esearch.fcgi", map[string]string{
		"db":       "pubmed",
		"term":     query,
		"retmode":  "json",
		"retmax":   fmt.Sprintf("%d", limit),
		"retstart": fmt.Sprintf("%d", request.Offset),
		"sort":     "relevance",
	})
	searchBody, searchStatus, err := e.fetcher.Get(ctx, e.Descriptor().ID, searchURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	searchPayload, err := decodeJSON[struct {
		Result struct {
			IDs []string `json:"idlist"`
		} `json:"esearchresult"`
	}](searchBody)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	if len(searchPayload.Result.IDs) == 0 {
		return search.ProviderSearchResponse{Results: []search.SearchResult{}, HTTPStatus: searchStatus, RawBytes: len(searchBody)}, nil
	}
	summaryURL := endpoint("https://eutils.ncbi.nlm.nih.gov/entrez/eutils/esummary.fcgi", map[string]string{
		"db":      "pubmed",
		"id":      strings.Join(searchPayload.Result.IDs, ","),
		"retmode": "json",
	})
	summaryBody, status, err := e.fetcher.Get(ctx, e.Descriptor().ID, summaryURL, nil)
	if err != nil {
		return search.ProviderSearchResponse{}, err
	}
	summaryPayload, err := decodeJSON[struct {
		Result map[string]jsonObject `json:"result"`
	}](summaryBody)
	if err != nil {
		return search.ProviderSearchResponse{}, search.NewError(search.SEARCH_PROVIDER_INVALID_RESPONSE, e.Descriptor().ID, false, err)
	}
	results := make([]search.SearchResult, 0, len(searchPayload.Result.IDs))
	for _, id := range searchPayload.Result.IDs {
		item := summaryPayload.Result[id]
		title := item.String("title")
		if title == "" {
			continue
		}
		link := "https://pubmed.ncbi.nlm.nih.gov/" + url.PathEscape(id) + "/"
		current := result(e.Descriptor().ID, title, link, "", len(results)+1)
		current.Metadata.Type = "paper"
		current.Metadata.Journal = item.String("fulljournalname")
		current.Metadata.DOI = item.String("elocationid")
		current.Metadata.Year = item.Int("pubdate")
		for _, author := range item.Strings("authors") {
			if strings.TrimSpace(author) != "" {
				current.Metadata.Authors = append(current.Metadata.Authors, strings.TrimSpace(author))
			}
		}
		if date := parsePubMedDate(item.String("pubdate")); date != nil {
			current.PublishedAt = date
			current.Metadata.Year = date.Year()
		}
		results = append(results, current)
	}
	return search.ProviderSearchResponse{Results: withProviders(e.Descriptor().ID, results), HasMore: len(searchPayload.Result.IDs) >= limit, HTTPStatus: status, RawBytes: len(searchBody) + len(summaryBody)}, nil
}

type jsonObject map[string]any

func (o jsonObject) String(key string) string {
	value, _ := o[key].(string)
	return strings.TrimSpace(value)
}

func (o jsonObject) Int(key string) int {
	switch value := o[key].(type) {
	case float64:
		return int(value)
	case int:
		return value
	case string:
		return parsePositiveInt(value)
	}
	return 0
}

func (o jsonObject) Strings(key string) []string {
	items, _ := o[key].([]any)
	result := make([]string, 0, len(items))
	for _, item := range items {
		object, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := object["name"].(string)
		result = append(result, name)
	}
	return result
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func stripMarkup(value string) string {
	value = strings.ReplaceAll(value, "<jats:p>", "")
	value = strings.ReplaceAll(value, "</jats:p>", " ")
	value = strings.ReplaceAll(value, "<p>", "")
	value = strings.ReplaceAll(value, "</p>", " ")
	return strings.Join(strings.Fields(value), " ")
}

func rebuildAbstract(index map[string][]int) string {
	if len(index) == 0 {
		return ""
	}
	type token struct {
		position int
		value    string
	}
	tokens := make([]token, 0)
	for value, positions := range index {
		for _, position := range positions {
			tokens = append(tokens, token{position: position, value: value})
		}
	}
	sort.Slice(tokens, func(i, j int) bool {
		return tokens[i].position < tokens[j].position
	})
	values := make([]string, 0, len(tokens))
	for _, item := range tokens {
		values = append(values, item.value)
	}
	return strings.Join(values, " ")
}

func parseTime(values ...string) *time.Time {
	for _, value := range values {
		for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z", "2006-01-02"} {
			if parsed, err := time.Parse(layout, strings.TrimSpace(value)); err == nil {
				parsed = parsed.UTC()
				return &parsed
			}
		}
	}
	return nil
}

func parsePubMedDate(value string) *time.Time {
	value = strings.TrimSpace(value)
	for _, layout := range []string{"2006 Jan 2", "2006 Jan", "2006", "Jan 2, 2006"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			parsed = parsed.UTC()
			return &parsed
		}
	}
	return nil
}
