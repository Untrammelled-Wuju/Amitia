package deepsearch

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/u-ai/backend/internal/search"
)

type Aggregator struct {
	policy    DeepSearchPolicy
	sources   map[string]*AggregatedSource
	domains   map[string]int
	seenKeys  map[string]struct{}
	focusHits map[string]int
}

func NewAggregator(policy DeepSearchPolicy) *Aggregator {
	return &Aggregator{
		policy:    policy,
		sources:   make(map[string]*AggregatedSource),
		domains:   make(map[string]int),
		seenKeys:  make(map[string]struct{}),
		focusHits: make(map[string]int),
	}
}

type ChildSearchResult struct {
	Rank         int
	Title        string
	URL          string
	Domain       string
	Snippet      string
	CanonicalURL string
	PublishedAt  *string
	FocusArea    string
	Round        int
	QueryIndex   int
	Provider     string
	RetrievedAt  *time.Time
	CitationID   string
}

func (a *Aggregator) AddResults(results []ChildSearchResult, round, queryIndex int, focusArea string) int {
	added := 0
	for _, r := range results {
		canon := r.CanonicalURL
		if canon == "" {
			canon = r.URL
		}
		if canon == "" {
			continue
		}

		observation := SourceObservation{
			Provider:    r.Provider,
			Round:       round,
			QueryIndex:  queryIndex,
		}
		if r.RetrievedAt != nil {
			observation.RetrievedAt = *r.RetrievedAt
		} else {
			observation.RetrievedAt = time.Now().UTC()
		}

		if _, exists := a.seenKeys[canon]; exists {
			if src, ok := a.sources[canon]; ok {
				src.SeenCount++
				if r.Rank < src.BestRank || src.BestRank == 0 {
					src.BestRank = r.Rank
				}
				src.QueryHits = append(src.QueryHits, QueryHit{
					Round:      round,
					QueryIndex: queryIndex,
					Rank:       r.Rank,
				})
				src.Observations = append(src.Observations, observation)
				if r.Provider != "" {
					src.Observations[len(src.Observations)-1].Provider = r.Provider
				}
			}
			continue
		}

		titleNorm := normalizeTitle(r.Title)
		isDup := false
		for key, src := range a.sources {
			if src.Domain == r.Domain && titleSimilarity(normalizeTitle(src.Title), titleNorm) >= 0.90 {
				isDup = true
				src.SeenCount++
				src.Observations = append(src.Observations, observation)
				a.seenKeys[key] = struct{}{}
				break
			}
		}
		if isDup {
			continue
		}

		citationID := r.CitationID
		if citationID == "" {
			citationID = citationIdentity(canon, r.Provider)
		}
		a.seenKeys[canon] = struct{}{}
		src := &AggregatedSource{
			CanonicalURL: canon,
			URL:          r.URL,
			Domain:       r.Domain,
			Title:        r.Title,
			Snippet:      truncateSnippet(r.Snippet, 2048),
			BestRank:     r.Rank,
			SeenCount:    1,
			CitationID:   citationID,
			QueryHits: []QueryHit{{
				Round:      round,
				QueryIndex: queryIndex,
				Rank:       r.Rank,
			}},
			Observations: []SourceObservation{observation},
		}
		if r.PublishedAt != nil {
			src.PublishedAt = parseTimePtr(*r.PublishedAt)
		}
		a.sources[canon] = src
		a.domains[r.Domain]++
		added++

		if focusArea != "" {
			a.focusHits[focusArea]++
		}
	}
	return added
}

func (a *Aggregator) SourceCount() int {
	return len(a.sources)
}

func (a *Aggregator) DomainCount() int {
	return len(a.domains)
}

func (a *Aggregator) FocusHitCount(focusArea string) int {
	return a.focusHits[focusArea]
}

func (a *Aggregator) BuildResult(query string, kind search.SearchKind, focusAreas []string, roundsCompleted, searchCalls int, stopReason string, completedAtEpoch int64) DeepSearchResult {
	var allSources []*AggregatedSource
	for _, src := range a.sources {
		rankScore := 1.0 / (1.0 + float64(src.BestRank))
		multiQuerySupport := float64(min(src.SeenCount, 3)) * 0.5
		src.Score = rankScore + multiQuerySupport
		allSources = append(allSources, src)
	}

	sort.Slice(allSources, func(i, j int) bool {
		if allSources[i].Score != allSources[j].Score {
			return allSources[i].Score > allSources[j].Score
		}
		if allSources[i].BestRank != allSources[j].BestRank {
			return allSources[i].BestRank < allSources[j].BestRank
		}
		return allSources[i].CanonicalURL < allSources[j].CanonicalURL
	})

	domainCount := make(map[string]int)
	var finalSources []AggregatedSource
	var overflow []AggregatedSource
	for _, src := range allSources {
		if domainCount[src.Domain] >= a.policy.MaxPerDomain {
			overflow = append(overflow, *src)
			continue
		}
		domainCount[src.Domain]++
		finalSources = append(finalSources, *src)
	}
	if len(finalSources) > a.policy.MaxSources {
		overflow = append(finalSources[a.policy.MaxSources:], overflow...)
		finalSources = finalSources[:a.policy.MaxSources]
	}
	_ = overflow

	for i := range finalSources {
		finalSources[i].CitationIndex = i + 1
	}

	focusCov := make([]FocusCoverage, 0, len(focusAreas))
	for _, fa := range focusAreas {
		hits := a.focusHits[fa]
		focusCov = append(focusCov, FocusCoverage{
			Name:        fa,
			HitCount:    hits,
			SourceCount: hits,
			Covered:     hits >= a.policy.FocusHitThreshold,
		})
	}

	citations := buildCitations(finalSources, kind)

	return DeepSearchResult{
		Query:           query,
		Kind:            kind,
		Status:          ResultStatusCompleted,
		RoundsCompleted: roundsCompleted,
		SearchCalls:     searchCalls,
		Coverage: CoverageState{
			FocusAreas:      focusCov,
			UniqueSources:   len(a.sources),
			UniqueDomains:   len(a.domains),
			SearchCalls:     searchCalls,
			RoundsCompleted: roundsCompleted,
		},
		Sources:        finalSources,
		StoppedReason:  stopReason,
		CompletedAt:    timeFromEpoch(completedAtEpoch),
		Citations:      citations,
	}
}

func buildCitations(sources []AggregatedSource, kind search.SearchKind) []search.Citation {
	citations := make([]search.Citation, 0, len(sources))
	for _, src := range sources {
		citations = append(citations, search.Citation{
			ID:           src.CitationID,
			Index:        src.CitationIndex,
			Title:        src.Title,
			URL:          src.URL,
			CanonicalURL: src.CanonicalURL,
			Domain:       src.Domain,
			Provider:     src.Provider(),
			PublishedAt:  src.PublishedAt,
			Snippet:      src.Snippet,
			Kind:         kind,
		})
	}
	return citations
}

func (a *AggregatedSource) Provider() string {
	if len(a.Observations) > 0 {
		return a.Observations[0].Provider
	}
	return ""
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func normalizeTitle(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastSpace = false
		} else if !lastSpace {
			b.WriteRune(' ')
			lastSpace = true
		}
	}
	return strings.TrimSpace(b.String())
}

func titleSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	if a == "" || b == "" {
		return 0.0
	}
	tokensA := strings.Fields(a)
	tokensB := strings.Fields(b)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0.0
	}
	setA := make(map[string]struct{}, len(tokensA))
	for _, t := range tokensA {
		setA[t] = struct{}{}
	}
	intersection := 0
	for _, t := range tokensB {
		if _, ok := setA[t]; ok {
			intersection++
		}
	}
	union := len(tokensA) + len(tokensB) - intersection
	if union == 0 {
		return 0.0
	}
	return float64(intersection) / float64(union)
}

func truncateSnippet(s string, maxRunes int) string {
	if utf8Len(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-3]) + "..."
}

func utf8Len(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

func parseTimePtr(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

func citationIdentity(canonicalURL, provider string) string {
	h := sha256.New()
	h.Write([]byte(strings.TrimSpace(canonicalURL)))
	h.Write([]byte("\n"))
	h.Write([]byte(strings.TrimSpace(provider)))
	sum := h.Sum(nil)
	return "cit_" + hex.EncodeToString(sum)[:12]
}

var timeFromEpoch = func(epoch int64) time.Time {
	return time.Unix(epoch, 0).UTC()
}
