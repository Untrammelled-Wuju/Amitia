package deepsearch

import (
	"time"
)

const (
	StopReasonMaxRounds      = "max_rounds"
	StopReasonMaxSearchCalls = "max_search_calls"
	StopReasonMaxSources     = "max_sources"
	StopReasonNoNewQueries   = "no_new_queries"
	StopReasonNoNewSources   = "no_new_sources"
	StopReasonCoverageOK     = "coverage_satisfied"
	StopReasonCancelled      = "cancelled"
	StopReasonDeadline       = "deadline_reached"
	StopReasonNoResults      = "no_results"
)

const (
	ResultStatusCompleted = "completed"
	ResultStatusPartial   = "partial"
)

type DeepSearchRequest struct {
	Query              string   `json:"query"`
	FocusAreas         []string `json:"focusAreas,omitempty"`
	MaxRounds          int      `json:"maxRounds,omitempty"`
	MaxQueriesPerRound int      `json:"maxQueriesPerRound,omitempty"`
	ResultsPerQuery    int      `json:"resultsPerQuery,omitempty"`
	MaxSources         int      `json:"maxSources,omitempty"`
	Language           string   `json:"language,omitempty"`
	Country            string   `json:"country,omitempty"`
	SafeSearch         string   `json:"safeSearch,omitempty"`
}

type SearchQueryPlan struct {
	Query     string `json:"query"`
	Round     int    `json:"round"`
	Reason    string `json:"reason,omitempty"`
	FocusArea string `json:"focusArea,omitempty"`
}

type ExecutedQuery struct {
	Round       int    `json:"round"`
	QueryIndex  int    `json:"queryIndex"`
	Query       string `json:"query"`
	Reason      string `json:"reason,omitempty"`
	FocusArea   string `json:"focusArea,omitempty"`
	ResultCount int    `json:"resultCount"`
	Error       string `json:"error,omitempty"`
}

type QueryHit struct {
	Round      int `json:"round"`
	QueryIndex int `json:"queryIndex"`
	Rank       int `json:"rank"`
}

type AggregatedSource struct {
	CanonicalURL string     `json:"canonicalUrl"`
	URL          string     `json:"url"`
	Domain       string     `json:"domain"`
	Title        string     `json:"title"`
	Snippet      string     `json:"snippet"`
	PublishedAt  *time.Time `json:"publishedAt,omitempty"`
	BestRank     int        `json:"bestRank"`
	SeenCount    int        `json:"seenCount"`
	QueryHits    []QueryHit `json:"queryHits"`
	Score        float64    `json:"score"`
}

type FocusCoverage struct {
	Name        string `json:"name"`
	HitCount    int    `json:"hitCount"`
	SourceCount int    `json:"sourceCount"`
	Covered     bool   `json:"covered"`
}

type CoverageState struct {
	FocusAreas      []FocusCoverage `json:"focusAreas"`
	UniqueSources   int             `json:"uniqueSources"`
	UniqueDomains   int             `json:"uniqueDomains"`
	SearchCalls     int             `json:"searchCalls"`
	RoundsCompleted int             `json:"roundsCompleted"`
}

type DeepSearchResult struct {
	Query           string             `json:"query"`
	Status          string             `json:"status"`
	RoundsCompleted int                `json:"roundsCompleted"`
	SearchCalls     int                `json:"searchCalls"`
	Queries         []ExecutedQuery    `json:"queries"`
	Sources         []AggregatedSource `json:"sources"`
	Coverage        CoverageState      `json:"coverage"`
	StoppedReason   string             `json:"stoppedReason"`
	CompletedAt     time.Time          `json:"completedAt"`
}

type DeepSearchCheckpoint struct {
	Version          int                `json:"version"`
	Round            int                `json:"round"`
	QueryIndex       int                `json:"queryIndex"`
	SearchCalls      int                `json:"searchCalls"`
	SourcePool       []AggregatedSource `json:"sourcePool"`
	FocusAreas       []string           `json:"focusAreas"`
	ExecutedQueries  []SearchQueryPlan  `json:"executedQueries"`
	CompletedQueries []bool             `json:"completedQueries"`
}
