package search

import (
	"encoding/json"
	"time"
)

type SafeSearchMode string

const (
	SafeSearchOff      SafeSearchMode = "off"
	SafeSearchModerate SafeSearchMode = "moderate"
	SafeSearchStrict   SafeSearchMode = "strict"
)

func (m SafeSearchMode) Valid() bool {
	switch m {
	case SafeSearchOff, SafeSearchModerate, SafeSearchStrict:
		return true
	}
	return false
}

type SearchKind string

const (
	SearchKindWeb      SearchKind = "web"
	SearchKindNews     SearchKind = "news"
	SearchKindAcademic SearchKind = "academic"
	SearchKindCode     SearchKind = "code"
	SearchKindImage    SearchKind = "image"
	SearchKindVideo    SearchKind = "video"
	SearchKindPlaces   SearchKind = "places"
	SearchKindProduct  SearchKind = "product"
)

func (k SearchKind) Valid() bool {
	switch k {
	case SearchKindWeb, SearchKindNews, SearchKindAcademic, SearchKindCode,
		SearchKindImage, SearchKindVideo, SearchKindPlaces, SearchKindProduct:
		return true
	}
	return false
}

type ProviderCapabilities struct {
	GeneralWeb     bool         `json:"generalWeb"`
	SearchKinds    []SearchKind `json:"searchKinds,omitempty"`
	LanguageFilter bool         `json:"languageFilter"`
	CountryFilter  bool         `json:"countryFilter"`
	SafeSearch     bool         `json:"safeSearch"`
	Pagination     bool         `json:"pagination"`
	TimeRangeFilter bool       `json:"timeRangeFilter,omitempty"`
	DomainFilter    bool        `json:"domainFilter,omitempty"`
	MaxResults     int          `json:"maxResults"`
}

type ProviderHealth string

const (
	ProviderHealthReady          ProviderHealth = "ready"
	ProviderHealthDisabled       ProviderHealth = "disabled"
	ProviderHealthMisconfigured  ProviderHealth = "misconfigured"
	ProviderHealthCredentialMiss ProviderHealth = "credential_missing"
	ProviderHealthNetworkDown    ProviderHealth = "network_unavailable"
	ProviderHealthDegraded       ProviderHealth = "degraded"
)

type TimeRangeFilter struct {
	From *time.Time `json:"from,omitempty"`
	To   *time.Time `json:"to,omitempty"`
}

type NewsSearchOptions struct {
	From     *time.Time `json:"from,omitempty"`
	To       *time.Time `json:"to,omitempty"`
	Freshness string    `json:"freshness,omitempty"`
}

type AcademicSearchOptions struct {
	FromYear       *int  `json:"fromYear,omitempty"`
	ToYear         *int  `json:"toYear,omitempty"`
	OpenAccessOnly bool  `json:"openAccessOnly,omitempty"`
}

type CodeSearchOptions struct {
	Language   string `json:"language,omitempty"`
	Repository string `json:"repository,omitempty"`
	Owner      string `json:"owner,omitempty"`
}

type ImageSearchOptions struct {
	Size        string `json:"size,omitempty"`
	AspectRatio string `json:"aspectRatio,omitempty"`
	ImageType   string `json:"imageType,omitempty"`
}

type VideoSearchOptions struct {
	Duration string `json:"duration,omitempty"`
}

type GeoPoint struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type PlacesSearchOptions struct {
	Location *GeoPoint `json:"location,omitempty"`
	RadiusM  int       `json:"radiusM,omitempty"`
}

type ProductSearchOptions struct {
	MinPrice *float64 `json:"minPrice,omitempty"`
	MaxPrice *float64 `json:"maxPrice,omitempty"`
	Currency string   `json:"currency,omitempty"`
}

type SpecializedSearchOptions struct {
	News     *NewsSearchOptions     `json:"news,omitempty"`
	Academic *AcademicSearchOptions `json:"academic,omitempty"`
	Code     *CodeSearchOptions     `json:"code,omitempty"`
	Image    *ImageSearchOptions    `json:"image,omitempty"`
	Video    *VideoSearchOptions    `json:"video,omitempty"`
	Places   *PlacesSearchOptions   `json:"places,omitempty"`
	Product  *ProductSearchOptions  `json:"product,omitempty"`
}

type GeneralSearchRequest struct {
	Query      string         `json:"query"`
	Limit      int            `json:"limit,omitempty"`
	Offset     int            `json:"offset,omitempty"`
	Language   string         `json:"language,omitempty"`
	Country    string         `json:"country,omitempty"`
	SafeSearch SafeSearchMode `json:"safeSearch,omitempty"`
}

type SearchRequest struct {
	Query      string         `json:"query"`
	Kind       SearchKind     `json:"kind,omitempty"`
	Limit      int            `json:"limit,omitempty"`
	Offset     int            `json:"offset,omitempty"`
	Language   string         `json:"language,omitempty"`
	Country    string         `json:"country,omitempty"`
	SafeSearch SafeSearchMode `json:"safeSearch,omitempty"`
	TimeRange  *TimeRangeFilter `json:"timeRange,omitempty"`
	Domains    []string       `json:"domains,omitempty"`
	Specialized SpecializedSearchOptions `json:"specialized,omitempty"`
}

type SearchSourceMetadata struct {
	Provider     string    `json:"provider"`
	ProviderRank int       `json:"providerRank"`
	OriginalURL  string    `json:"originalUrl"`
	CanonicalURL string    `json:"canonicalUrl"`
	RetrievedAt  time.Time `json:"retrievedAt"`
}

type SearchResultMetadata struct {
	Type     string `json:"type,omitempty"`
	Authors  []string `json:"authors,omitempty"`
	DOI      string `json:"doi,omitempty"`
	Journal  string `json:"journal,omitempty"`
	Year     int    `json:"year,omitempty"`
	Repository string `json:"repository,omitempty"`
	Path       string `json:"path,omitempty"`
	License    string `json:"license,omitempty"`
	ThumbnailURL string `json:"thumbnailUrl,omitempty"`
	MediaURL     string `json:"mediaUrl,omitempty"`
	Width        int     `json:"width,omitempty"`
	Height       int     `json:"height,omitempty"`
	DurationSeconds float64 `json:"durationSeconds,omitempty"`
	Address         string   `json:"address,omitempty"`
	Latitude        *float64 `json:"latitude,omitempty"`
	Longitude       *float64 `json:"longitude,omitempty"`
	Rating          *float64 `json:"rating,omitempty"`
	ReviewCount     int      `json:"reviewCount,omitempty"`
	Price           *float64 `json:"price,omitempty"`
	Currency        string   `json:"currency,omitempty"`
	Merchant        string   `json:"merchant,omitempty"`
	Availability    string   `json:"availability,omitempty"`
	ProductID       string   `json:"productId,omitempty"`
}

type CitationRef struct {
	ID    string `json:"id"`
	Index int    `json:"index"`
}

type SearchResult struct {
	Rank        int                 `json:"rank"`
	Title       string              `json:"title"`
	URL         string              `json:"url"`
	DisplayURL  string              `json:"displayUrl,omitempty"`
	Domain      string              `json:"domain"`
	Snippet     string              `json:"snippet"`
	PublishedAt *time.Time          `json:"publishedAt,omitempty"`
	Language    string              `json:"language,omitempty"`
	Source      SearchSourceMetadata `json:"source"`
	Metadata    SearchResultMetadata `json:"metadata,omitempty"`
	Citation    CitationRef          `json:"citation"`
}

type Citation struct {
	ID           string    `json:"id"`
	Index        int       `json:"index"`
	Title        string    `json:"title"`
	URL          string    `json:"url"`
	CanonicalURL string    `json:"canonicalUrl"`
	Domain       string    `json:"domain"`
	Provider     string    `json:"provider"`
	ProviderRank int       `json:"providerRank"`
	RetrievedAt  time.Time `json:"retrievedAt"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty"`
	Snippet      string    `json:"snippet,omitempty"`
	Kind         SearchKind `json:"kind,omitempty"`
	Metadata     SearchResultMetadata `json:"metadata,omitempty"`
}

type CitationSet struct {
	Citations []Citation `json:"citations"`
}

type GeneralSearchResponse struct {
	Query       string         `json:"query"`
	Provider    string         `json:"provider"`
	Results     []SearchResult `json:"results"`
	Returned    int            `json:"returned"`
	Offset      int            `json:"offset"`
	HasMore     bool           `json:"hasMore,omitempty"`
	RetrievedAt time.Time      `json:"retrievedAt"`
	DurationMs  int64          `json:"durationMs"`
}

type SearchResponse struct {
	Query       string         `json:"query"`
	Kind        SearchKind     `json:"kind"`
	Provider    string         `json:"provider"`
	Results     []SearchResult `json:"results"`
	Returned    int            `json:"returned"`
	Offset      int            `json:"offset"`
	HasMore     bool           `json:"hasMore,omitempty"`
	RetrievedAt time.Time      `json:"retrievedAt"`
	DurationMs  int64          `json:"durationMs"`
	CitationSet CitationSet    `json:"citationSet,omitempty"`
}

type ToolInput struct {
	Query       string         `json:"query"`
	Kind        SearchKind     `json:"kind,omitempty"`
	Limit       int            `json:"limit,omitempty"`
	Offset      int            `json:"offset,omitempty"`
	Language    string         `json:"language,omitempty"`
	Country     string         `json:"country,omitempty"`
	SafeSearch  SafeSearchMode `json:"safeSearch,omitempty"`
	Domains     []string       `json:"domains,omitempty"`
	Specialized SpecializedSearchOptions `json:"specialized,omitempty"`
}

func (in *ToolInput) ToRequest() SearchRequest {
	return SearchRequest{
		Query:       in.Query,
		Kind:        in.Kind,
		Limit:       in.Limit,
		Offset:      in.Offset,
		Language:    in.Language,
		Country:     in.Country,
		SafeSearch:  in.SafeSearch,
		Domains:     in.Domains,
		Specialized: in.Specialized,
	}
}

type ToolOutput struct {
	Query       string         `json:"query"`
	Kind        SearchKind     `json:"kind"`
	Provider    string         `json:"provider"`
	Results     []SearchResult `json:"results"`
	Returned    int            `json:"returned"`
	HasMore     bool           `json:"hasMore,omitempty"`
	RetrievedAt time.Time      `json:"retrievedAt"`
	CitationSet CitationSet    `json:"citationSet,omitempty"`
}

func ToolOutputFromResponse(resp *SearchResponse) ToolOutput {
	kind := resp.Kind
	return ToolOutput{
		Query:       resp.Query,
		Kind:        kind,
		Provider:    resp.Provider,
		Results:     resp.Results,
		Returned:    resp.Returned,
		HasMore:     resp.HasMore,
		RetrievedAt: resp.RetrievedAt,
		CitationSet: resp.CitationSet,
	}
}

func ToolOutputFromGeneralResponse(resp *GeneralSearchResponse) ToolOutput {
	return ToolOutput{
		Query:       resp.Query,
		Kind:        SearchKindWeb,
		Provider:    resp.Provider,
		Results:     resp.Results,
		Returned:    resp.Returned,
		HasMore:     resp.HasMore,
		RetrievedAt: resp.RetrievedAt,
	}
}

func (o ToolOutput) MarshalJSON() ([]byte, error) {
	type alias ToolOutput
	return json.Marshal(alias(o))
}
