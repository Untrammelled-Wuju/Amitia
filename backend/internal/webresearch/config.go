package webresearch

import "time"

type Mode string

const (
	ModeAuto         Mode = "auto"
	ModeFast         Mode = "fast"
	ModeResearch     Mode = "research"
	ModeDeepResearch Mode = "deep_research"
)

type Config struct {
	Enabled               bool
	DeepResearchEnabled   bool
	BrowserEnabled        bool
	PDFEnabled            bool
	PDFTextCommand        string
	PDFInfoCommand        string
	PDFTimeout            time.Duration
	MaxExecution          time.Duration
	MaxToolOutputChars    int
	MaxQueries            int
	MaxProvidersPerQuery  int
	MaxSearchResults      int
	MaxResultsPerDomain   int
	MaxOpenPages          int
	MaxDeepOpenPages      int
	MaxDeepRounds         int
	MaxSearchCalls        int
	MaxProviderCostUSD    float64
	MaxProviderCredits    float64
	MinDeepNewSources     int
	MaxParallelSearch     int
	MaxParallelFetch      int
	SearchTimeout         time.Duration
	FetchTimeout          time.Duration
	MaxFetchBytes         int64
	MaxPageChars          int
	MaxEvidencePerPage    int
	MaxEvidenceChars      int
	ReferenceTTL          time.Duration
	PageCacheTTL          time.Duration
	AutoBrowserEscalation bool
	MinStaticContentChars int
	MaxRedirects          int
	MaxBrowserPages       int
	MaxBrowserScrolls     int
	MaxBrowserTime        time.Duration
}

func DefaultConfig() Config {
	return Config{
		Enabled:               true,
		DeepResearchEnabled:   true,
		BrowserEnabled:        true,
		PDFEnabled:            true,
		PDFTextCommand:        "pdftotext",
		PDFInfoCommand:        "pdfinfo",
		PDFTimeout:            20 * time.Second,
		MaxExecution:          2 * time.Minute,
		MaxToolOutputChars:    120000,
		MaxQueries:            4,
		MaxProvidersPerQuery:  2,
		MaxSearchResults:      12,
		MaxResultsPerDomain:   3,
		MaxOpenPages:          4,
		MaxDeepOpenPages:      6,
		MaxDeepRounds:         3,
		MaxSearchCalls:        12,
		MaxProviderCostUSD:    1.0,
		MaxProviderCredits:    20,
		MinDeepNewSources:     2,
		MaxParallelSearch:     4,
		MaxParallelFetch:      3,
		SearchTimeout:         20 * time.Second,
		FetchTimeout:          20 * time.Second,
		MaxFetchBytes:         6 * 1024 * 1024,
		MaxPageChars:          60000,
		MaxEvidencePerPage:    12,
		MaxEvidenceChars:      3500,
		ReferenceTTL:          7 * 24 * time.Hour,
		PageCacheTTL:          10 * time.Minute,
		AutoBrowserEscalation: true,
		MinStaticContentChars: 240,
		MaxRedirects:          5,
		MaxBrowserPages:       2,
		MaxBrowserScrolls:     2,
		MaxBrowserTime:        30 * time.Second,
	}
}

func (c Config) normalize() Config {
	d := DefaultConfig()
	if c.PDFTextCommand == "" {
		c.PDFTextCommand = d.PDFTextCommand
	}
	if c.PDFInfoCommand == "" {
		c.PDFInfoCommand = d.PDFInfoCommand
	}
	if c.PDFTimeout <= 0 {
		c.PDFTimeout = d.PDFTimeout
	}
	if c.MaxExecution <= 0 {
		c.MaxExecution = d.MaxExecution
	}
	if c.MaxToolOutputChars <= 0 {
		c.MaxToolOutputChars = d.MaxToolOutputChars
	}
	if c.MaxQueries <= 0 {
		c.MaxQueries = d.MaxQueries
	}
	if c.MaxProvidersPerQuery <= 0 {
		c.MaxProvidersPerQuery = d.MaxProvidersPerQuery
	}
	if c.MaxSearchResults <= 0 {
		c.MaxSearchResults = d.MaxSearchResults
	}
	if c.MaxResultsPerDomain <= 0 {
		c.MaxResultsPerDomain = d.MaxResultsPerDomain
	}
	if c.MaxOpenPages <= 0 {
		c.MaxOpenPages = d.MaxOpenPages
	}
	if c.MaxDeepOpenPages <= 0 {
		c.MaxDeepOpenPages = d.MaxDeepOpenPages
	}
	if c.MaxDeepRounds <= 0 {
		c.MaxDeepRounds = d.MaxDeepRounds
	}
	if c.MaxSearchCalls <= 0 {
		c.MaxSearchCalls = d.MaxSearchCalls
	}
	if c.MaxProviderCostUSD <= 0 {
		c.MaxProviderCostUSD = d.MaxProviderCostUSD
	}
	if c.MaxProviderCredits <= 0 {
		c.MaxProviderCredits = d.MaxProviderCredits
	}
	if c.MinDeepNewSources <= 0 {
		c.MinDeepNewSources = d.MinDeepNewSources
	}
	if c.MaxParallelSearch <= 0 {
		c.MaxParallelSearch = d.MaxParallelSearch
	}
	if c.MaxParallelFetch <= 0 {
		c.MaxParallelFetch = d.MaxParallelFetch
	}
	if c.SearchTimeout <= 0 {
		c.SearchTimeout = d.SearchTimeout
	}
	if c.FetchTimeout <= 0 {
		c.FetchTimeout = d.FetchTimeout
	}
	if c.MaxFetchBytes <= 0 {
		c.MaxFetchBytes = d.MaxFetchBytes
	}
	if c.MaxPageChars <= 0 {
		c.MaxPageChars = d.MaxPageChars
	}
	if c.MaxEvidencePerPage <= 0 {
		c.MaxEvidencePerPage = d.MaxEvidencePerPage
	}
	if c.MaxEvidenceChars <= 0 {
		c.MaxEvidenceChars = d.MaxEvidenceChars
	}
	if c.ReferenceTTL <= 0 {
		c.ReferenceTTL = d.ReferenceTTL
	}
	if c.PageCacheTTL < 0 {
		c.PageCacheTTL = 0
	} else if c.PageCacheTTL == 0 {
		c.PageCacheTTL = d.PageCacheTTL
	}
	if c.MinStaticContentChars <= 0 {
		c.MinStaticContentChars = d.MinStaticContentChars
	}
	if c.MaxRedirects <= 0 {
		c.MaxRedirects = d.MaxRedirects
	}
	if c.MaxBrowserPages <= 0 {
		c.MaxBrowserPages = d.MaxBrowserPages
	}
	if c.MaxBrowserScrolls < 0 {
		c.MaxBrowserScrolls = 0
	} else if c.MaxBrowserScrolls == 0 {
		c.MaxBrowserScrolls = d.MaxBrowserScrolls
	}
	if c.MaxBrowserTime <= 0 {
		c.MaxBrowserTime = d.MaxBrowserTime
	}
	return c
}
