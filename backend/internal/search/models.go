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

type ProviderCapabilities struct {
	GeneralWeb     bool `json:"generalWeb"`
	LanguageFilter bool `json:"languageFilter"`
	CountryFilter  bool `json:"countryFilter"`
	SafeSearch     bool `json:"safeSearch"`
	Pagination     bool `json:"pagination"`
	MaxResults     int  `json:"maxResults"`
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

type GeneralSearchRequest struct {
	Query      string         `json:"query"`
	Limit      int            `json:"limit,omitempty"`
	Offset     int            `json:"offset,omitempty"`
	Language   string         `json:"language,omitempty"`
	Country    string         `json:"country,omitempty"`
	SafeSearch SafeSearchMode `json:"safeSearch,omitempty"`
}

type SearchSourceMetadata struct {
	Provider    string    `json:"provider"`
	ProviderRank int      `json:"providerRank"`
	OriginalURL string    `json:"originalUrl"`
	CanonicalURL string   `json:"canonicalUrl"`
	RetrievedAt time.Time `json:"retrievedAt"`
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

type ToolInput struct {
	Query      string         `json:"query"`
	Limit      int            `json:"limit,omitempty"`
	Offset     int            `json:"offset,omitempty"`
	Language   string         `json:"language,omitempty"`
	Country    string         `json:"country,omitempty"`
	SafeSearch SafeSearchMode `json:"safeSearch,omitempty"`
}

func (in *ToolInput) ToRequest() GeneralSearchRequest {
	return GeneralSearchRequest{
		Query:      in.Query,
		Limit:      in.Limit,
		Offset:     in.Offset,
		Language:   in.Language,
		Country:    in.Country,
		SafeSearch: in.SafeSearch,
	}
}

type ToolOutput struct {
	Query       string         `json:"query"`
	Provider    string         `json:"provider"`
	Results     []SearchResult `json:"results"`
	Returned    int            `json:"returned"`
	HasMore     bool           `json:"hasMore,omitempty"`
	RetrievedAt time.Time      `json:"retrievedAt"`
}

func ToolOutputFromResponse(resp *GeneralSearchResponse) ToolOutput {
	return ToolOutput{
		Query:       resp.Query,
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
