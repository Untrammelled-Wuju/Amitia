package webresearch

import "time"

type Scope struct {
	ConversationID string
	TurnID         string
	InvocationID   string
}

type SearchQueryCommand struct {
	Q              string   `json:"q"`
	Kind           string   `json:"kind,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	RecencyDays    int      `json:"recency_days,omitempty"`
	Domains        []string `json:"domains,omitempty"`
	ExcludeDomains []string `json:"exclude_domains,omitempty"`
	Language       string   `json:"language,omitempty"`
	Country        string   `json:"country,omitempty"`
	SafeSearch     string   `json:"safe_search,omitempty"`
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

// PageVersion is an immutable-by-content-hash page snapshot used to audit
// historical evidence after a source changes. web_pages remains the latest
// read-through view; versions are retained separately.
type PageVersion struct {
	RefID       string    `json:"ref_id"`
	ContentHash string    `json:"content_hash"`
	Title       string    `json:"title,omitempty"`
	ContentType string    `json:"content_type"`
	Content     string    `json:"content"`
	Links       []Link    `json:"links,omitempty"`
	Truncated   bool      `json:"truncated,omitempty"`
	Dynamic     bool      `json:"dynamic,omitempty"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

type Evidence struct {
	ID              string          `json:"id"`
	ConversationID  string          `json:"-"`
	PageRefID       string          `json:"page_ref_id"`
	PageContentHash string          `json:"page_content_hash,omitempty"`
	Query           string          `json:"query,omitempty"`
	Text            string          `json:"text"`
	TextHash        string          `json:"text_hash"`
	Locator         EvidenceLocator `json:"locator"`
	Relevance       float64         `json:"relevance"`
	CreatedAt       time.Time       `json:"created_at"`
}

type Citation struct {
	Index      int             `json:"index"`
	EvidenceID string          `json:"evidence_id"`
	RefID      string          `json:"ref_id"`
	Title      string          `json:"title,omitempty"`
	URL        string          `json:"url"`
	Text       string          `json:"text"`
	Locator    EvidenceLocator `json:"locator"`
	Relevance  float64         `json:"relevance"`
}

// TurnCitationBinding is the durable citation-number registry for one turn.
// The number is stable across reconnects/restarts and must never be inferred
// from frontend memory.
type TurnCitationBinding struct {
	TurnID         string    `json:"turn_id"`
	RefID          string    `json:"ref_id"`
	CitationNumber int       `json:"citation_number"`
	CreatedAt      time.Time `json:"created_at"`
}

// TurnCitationEvidenceBinding preserves the exact evidence fragments represented
// by a stable turn-level citation number. A single source citation can reference
// multiple evidence excerpts without creating duplicate UI citation numbers.
type TurnCitationEvidenceBinding struct {
	TurnID         string    `json:"turn_id"`
	CitationNumber int       `json:"citation_number"`
	RefID          string    `json:"ref_id"`
	EvidenceID     string    `json:"evidence_id"`
	CreatedAt      time.Time `json:"created_at"`
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

type ResearchBudget struct {
	MaxRounds          int     `json:"max_rounds"`
	MaxSearchCalls     int     `json:"max_search_calls"`
	MaxOpenPages       int     `json:"max_open_pages"`
	MaxProviderCostUSD float64 `json:"max_provider_cost_usd,omitempty"`
	MaxProviderCredits float64 `json:"max_provider_credits,omitempty"`
}

type ResearchQuestion struct {
	ID              string   `json:"id"`
	Question        string   `json:"question"`
	Priority        int      `json:"priority"`
	Required        bool     `json:"required"`
	Dependencies    []string `json:"dependencies,omitempty"`
	SuccessCriteria []string `json:"success_criteria,omitempty"`
}

type ResearchPlan struct {
	Goal            string             `json:"goal"`
	Questions       []ResearchQuestion `json:"questions"`
	SuccessCriteria []string           `json:"success_criteria,omitempty"`
	Budget          ResearchBudget     `json:"budget"`
}

type ResearchRound struct {
	Round           int     `json:"round"`
	Queries         int     `json:"queries"`
	NewSources      int     `json:"new_sources"`
	NewEvidence     int     `json:"new_evidence"`
	NewDomains      int     `json:"new_domains"`
	InformationGain float64 `json:"information_gain"`
}

type ResearchCost struct {
	SearchRequests  int     `json:"search_requests"`
	FetchRequests   int     `json:"fetch_requests"`
	BrowserCalls    int     `json:"browser_calls"`
	BrowserSeconds  float64 `json:"browser_seconds,omitempty"`
	ProviderCostUSD float64 `json:"provider_cost_usd,omitempty"`
	ProviderCredits float64 `json:"provider_credits,omitempty"`
	DurationMs      int64   `json:"duration_ms,omitempty"`
}

type ResearchFinding struct {
	QuestionID  string   `json:"question_id"`
	Question    string   `json:"question"`
	Status      string   `json:"status"`
	Summary     string   `json:"summary,omitempty"`
	EvidenceIDs []string `json:"evidence_ids,omitempty"`
	SourceRefs  []string `json:"source_refs,omitempty"`
}

type EvidenceScope struct {
	Platforms []string `json:"platforms,omitempty"`
	Regions   []string `json:"regions,omitempty"`
	Years     []string `json:"years,omitempty"`
}

type EvidenceConflict struct {
	Kind            string        `json:"kind"`
	LeftEvidenceID  string        `json:"left_evidence_id"`
	RightEvidenceID string        `json:"right_evidence_id"`
	LeftRefID       string        `json:"left_ref_id"`
	RightRefID      string        `json:"right_ref_id"`
	Reason          string        `json:"reason"`
	Confidence      float64       `json:"confidence"`
	LeftScope       EvidenceScope `json:"left_scope,omitempty"`
	RightScope      EvidenceScope `json:"right_scope,omitempty"`
	ResolutionHint  string        `json:"resolution_hint,omitempty"`
}

type EvidenceClaim struct {
	ID              string   `json:"id"`
	Text            string   `json:"text"`
	Status          string   `json:"status"`
	EvidenceIDs     []string `json:"evidence_ids,omitempty"`
	CitationIndexes []int    `json:"citation_indexes,omitempty"`
}

type EvidenceGraphSummary struct {
	SourceCount   int                `json:"source_count"`
	PageCount     int                `json:"page_count"`
	EvidenceCount int                `json:"evidence_count"`
	ClaimCount    int                `json:"claim_count"`
	CitationCount int                `json:"citation_count"`
	ConflictCount int                `json:"conflict_count"`
	Claims        []EvidenceClaim    `json:"claims,omitempty"`
	Conflicts     []EvidenceConflict `json:"conflicts,omitempty"`
}

type ResearchSummary struct {
	Mode            Mode              `json:"mode"`
	Queries         []string          `json:"queries"`
	FollowUpQueries []string          `json:"follow_up_queries,omitempty"`
	Providers       []string          `json:"providers"`
	RoundsCompleted int               `json:"rounds_completed"`
	OpenedPages     int               `json:"opened_pages"`
	EvidenceCount   int               `json:"evidence_count"`
	UniqueDomains   int               `json:"unique_domains"`
	StopReason      string            `json:"stop_reason"`
	Plan            *ResearchPlan     `json:"plan,omitempty"`
	Unresolved      []string          `json:"unresolved_questions,omitempty"`
	Rounds          []ResearchRound   `json:"rounds,omitempty"`
	Findings        []ResearchFinding `json:"findings,omitempty"`
	Cost            ResearchCost      `json:"cost"`
	Partial         bool              `json:"partial,omitempty"`
}

type Stats struct {
	SearchCalls            int     `json:"search_calls"`
	FetchCalls             int     `json:"fetch_calls"`
	BrowserCalls           int     `json:"browser_calls"`
	BrowserDurationMs      int64   `json:"browser_duration_ms,omitempty"`
	CacheHits              int     `json:"cache_hits"`
	PageCacheHits          int     `json:"page_cache_hits"`
	ProviderCostUSD        float64 `json:"provider_cost_usd,omitempty"`
	ProviderCredits        float64 `json:"provider_credits,omitempty"`
	SemanticRerankCalls    int     `json:"semantic_rerank_calls,omitempty"`
	SemanticRerankFailures int     `json:"semantic_rerank_failures,omitempty"`
	DurationMs             int64   `json:"duration_ms"`
}

const ExternalContentTrustLabel = "UNTRUSTED_EXTERNAL_CONTENT"

type SecuritySummary struct {
	TrustLabel               string   `json:"trust_label"`
	UntrustedExternalContent bool     `json:"untrusted_external_content"`
	PotentialPromptInjection bool     `json:"potential_prompt_injection,omitempty"`
	SignalCount              int      `json:"signal_count,omitempty"`
	Warnings                 []string `json:"warnings,omitempty"`
}

type ToolOutput struct {
	Operation                string                `json:"operation"`
	Mode                     Mode                  `json:"mode"`
	Search                   []SearchHit           `json:"search,omitempty"`
	Pages                    []Page                `json:"pages,omitempty"`
	Matches                  []FindMatch           `json:"matches,omitempty"`
	Screenshots              []ScreenshotResult    `json:"screenshots,omitempty"`
	Citations                []Citation            `json:"citations,omitempty"`
	Research                 *ResearchSummary      `json:"research,omitempty"`
	EvidenceGraph            *EvidenceGraphSummary `json:"evidence_graph,omitempty"`
	Stats                    Stats                 `json:"stats"`
	Security                 SecuritySummary       `json:"security"`
	ContentTrust             string                `json:"content_trust"`
	UntrustedExternalContent bool                  `json:"untrusted_external_content"`
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
