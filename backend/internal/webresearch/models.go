package webresearch

import "time"

type Scope struct {
	ConversationID string
	TurnID         string
	InvocationID   string
}

type SearchQueryCommand struct {
	Q           string   `json:"q"`
	Kind        string   `json:"kind,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	RecencyDays int      `json:"recency_days,omitempty"`
	Domains     []string `json:"domains,omitempty"`
	Language    string   `json:"language,omitempty"`
	Country     string   `json:"country,omitempty"`
	SafeSearch  string   `json:"safe_search,omitempty"`
}

type OpenCommand struct {
	RefID string `json:"ref_id,omitempty"`
	URL   string `json:"url,omitempty"`
	Line  int    `json:"line,omitempty"`
}

type FindCommand struct {
	RefID   string `json:"ref_id"`
	Pattern string `json:"pattern"`
}

type ClickCommand struct {
	RefID  string `json:"ref_id"`
	LinkID int    `json:"link_id"`
}

type ScreenshotCommand struct {
	RefID    string `json:"ref_id,omitempty"`
	URL      string `json:"url,omitempty"`
	Page     *int   `json:"page,omitempty"`
	FullPage bool   `json:"full_page,omitempty"`
}

type ToolInput struct {
	Mode           Mode                 `json:"mode,omitempty"`
	SearchQuery    []SearchQueryCommand `json:"search_query,omitempty"`
	Open           []OpenCommand        `json:"open,omitempty"`
	Find           []FindCommand        `json:"find,omitempty"`
	Click          []ClickCommand       `json:"click,omitempty"`
	Screenshot     []ScreenshotCommand  `json:"screenshot,omitempty"`
	FocusAreas     []string             `json:"focus_areas,omitempty"`
	ResponseLength string               `json:"response_length,omitempty"`
}

type Reference struct {
	RefID          string     `json:"ref_id"`
	ConversationID string     `json:"-"`
	TurnID         string     `json:"-"`
	InvocationID   string     `json:"-"`
	Kind           string     `json:"kind"`
	URL            string     `json:"url"`
	CanonicalURL   string     `json:"canonical_url"`
	Title          string     `json:"title,omitempty"`
	Snippet        string     `json:"snippet,omitempty"`
	Provider       string     `json:"provider,omitempty"`
	Query          string     `json:"query,omitempty"`
	Rank           int        `json:"rank,omitempty"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	ExpiresAt      time.Time  `json:"expires_at"`
}

type SearchHit struct {
	RefID       string     `json:"ref_id"`
	Rank        int        `json:"rank"`
	Title       string     `json:"title"`
	URL         string     `json:"url"`
	Domain      string     `json:"domain"`
	Snippet     string     `json:"snippet"`
	Provider    string     `json:"provider"`
	PublishedAt *time.Time `json:"published_at,omitempty"`
	RetrievedAt time.Time  `json:"retrieved_at"`
	Score       float64    `json:"score,omitempty"`
	SeenCount   int        `json:"seen_count,omitempty"`
}

type Link struct {
	ID   int    `json:"id"`
	Text string `json:"text,omitempty"`
	URL  string `json:"url"`
}

type Page struct {
	RefID          string    `json:"ref_id"`
	SourceRefID    string    `json:"source_ref_id,omitempty"`
	ConversationID string    `json:"-"`
	URL            string    `json:"url"`
	CanonicalURL   string    `json:"canonical_url"`
	Title          string    `json:"title,omitempty"`
	ContentType    string    `json:"content_type"`
	Content        string    `json:"content"`
	ContentHash    string    `json:"content_hash"`
	Links          []Link    `json:"links,omitempty"`
	Truncated      bool      `json:"truncated"`
	Dynamic        bool      `json:"dynamic,omitempty"`
	FetchedAt      time.Time `json:"fetched_at"`
}

type EvidenceLocator struct {
	Kind       string   `json:"kind"`
	BlockIndex int      `json:"block_index,omitempty"`
	Page       int      `json:"page,omitempty"`
	Headings   []string `json:"headings,omitempty"`
}

type Evidence struct {
	ID             string          `json:"id"`
	ConversationID string          `json:"-"`
	PageRefID      string          `json:"page_ref_id"`
	Query          string          `json:"query,omitempty"`
	Text           string          `json:"text"`
	TextHash       string          `json:"text_hash"`
	Locator        EvidenceLocator `json:"locator"`
	Relevance      float64         `json:"relevance"`
	CreatedAt      time.Time       `json:"created_at"`
}

type Citation struct {
	Index      int     `json:"index"`
	EvidenceID string  `json:"evidence_id"`
	RefID      string  `json:"ref_id"`
	Title      string  `json:"title,omitempty"`
	URL        string  `json:"url"`
	Text       string  `json:"text"`
	Relevance  float64 `json:"relevance"`
}

type FindMatch struct {
	RefID      string `json:"ref_id"`
	BlockIndex int    `json:"block_index"`
	Text       string `json:"text"`
}

type ScreenshotResult struct {
	RefID       string `json:"ref_id"`
	Page        *int   `json:"page,omitempty"`
	ResourceURI string `json:"resource_uri"`
	Width       int    `json:"width,omitempty"`
	Height      int    `json:"height,omitempty"`
	Format      string `json:"format,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
}

type ResearchSummary struct {
	Mode            Mode     `json:"mode"`
	Queries         []string `json:"queries"`
	FollowUpQueries []string `json:"follow_up_queries,omitempty"`
	Providers       []string `json:"providers"`
	RoundsCompleted int      `json:"rounds_completed"`
	OpenedPages     int      `json:"opened_pages"`
	EvidenceCount   int      `json:"evidence_count"`
	UniqueDomains   int      `json:"unique_domains"`
	StopReason      string   `json:"stop_reason"`
	Partial         bool     `json:"partial,omitempty"`
}

type Stats struct {
	SearchCalls   int   `json:"search_calls"`
	FetchCalls    int   `json:"fetch_calls"`
	BrowserCalls  int   `json:"browser_calls"`
	CacheHits     int   `json:"cache_hits"`
	PageCacheHits int   `json:"page_cache_hits"`
	DurationMs    int64 `json:"duration_ms"`
}

type SecuritySummary struct {
	UntrustedExternalContent bool     `json:"untrusted_external_content"`
	PotentialPromptInjection bool     `json:"potential_prompt_injection,omitempty"`
	SignalCount              int      `json:"signal_count,omitempty"`
	Warnings                 []string `json:"warnings,omitempty"`
}

type ToolOutput struct {
	Operation                string             `json:"operation"`
	Mode                     Mode               `json:"mode"`
	Search                   []SearchHit        `json:"search,omitempty"`
	Pages                    []Page             `json:"pages,omitempty"`
	Matches                  []FindMatch        `json:"matches,omitempty"`
	Screenshots              []ScreenshotResult `json:"screenshots,omitempty"`
	Citations                []Citation         `json:"citations,omitempty"`
	Research                 *ResearchSummary   `json:"research,omitempty"`
	Stats                    Stats              `json:"stats"`
	Security                 SecuritySummary    `json:"security"`
	UntrustedExternalContent bool               `json:"untrusted_external_content"`
}

type Progress struct {
	Phase         string  `json:"phase"`
	Message       string  `json:"message,omitempty"`
	Query         string  `json:"query,omitempty"`
	RefID         string  `json:"ref_id,omitempty"`
	Title         string  `json:"title,omitempty"`
	Completed     int     `json:"completed,omitempty"`
	Total         int     `json:"total,omitempty"`
	Fraction      float64 `json:"fraction,omitempty"`
	Indeterminate bool    `json:"indeterminate,omitempty"`
}

type ProgressFunc func(Progress) error
