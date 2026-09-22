package webresearch

import (
	"context"
	"sort"
	"strings"
)

const (
	deepStopCoverageSatisfied = "coverage_satisfied"
	deepStopMaxRounds         = "max_rounds"
	deepStopMaxSearchCalls    = "max_search_calls"
	deepStopNoNewQueries      = "no_new_queries"
	deepStopNoNewSources      = "no_new_sources"
	deepStopBudgetSatisfied   = "budget_satisfied"
)

func (r *Runtime) executeDeepResearch(ctx context.Context, scope Scope, input ToolInput, output *ToolOutput, emit ProgressFunc) *Error {
	initial := expandQueries(input.SearchQuery, input.FocusAreas, ModeDeepResearch, r.config.MaxQueries)
	if len(initial) == 0 {
		return newError(ErrInvalidInput, "no valid search queries", false, nil)
	}

	executed := make(map[string]struct{})
	allQueries := make([]string, 0, r.config.MaxDeepRounds*r.config.MaxQueries)
	followUpQueries := make([]string, 0)
	allObservations := make([]searchObservation, 0)
	providerSet := make(map[string]struct{})
	referenceByCanonical := make(map[string]Reference)
	discovered := make(map[string]struct{})
	opened := make(map[string]struct{})
	pageSeen := make(map[string]struct{})
	citationSeen := make(map[string]struct{})
	current := initial
	stopReason := deepStopMaxRounds
	partial := false
	roundsCompleted := 0
	totalSearchCalls := 0
	remainingOpen := r.config.MaxDeepOpenPages

	for round := 1; round <= r.config.MaxDeepRounds; round++ {
		if len(current) == 0 {
			stopReason = deepStopNoNewQueries
			break
		}
		remainingCalls := r.config.MaxDeepSearchCalls - totalSearchCalls
		if remainingCalls <= 0 {
			stopReason = deepStopMaxSearchCalls
			break
		}
		for _, query := range current {
			key := normalizedQuery(query.Q)
			if key != "" {
				executed[key] = struct{}{}
				allQueries = append(allQueries, query.Q)
			}
		}
		_ = emitProgress(emit, Progress{
			Phase:         "research_round",
			Message:       "running deep research round",
			Completed:     round,
			Total:         r.config.MaxDeepRounds,
			Fraction:      float64(round-1) / float64(maxInt(1, r.config.MaxDeepRounds)),
			Indeterminate: false,
		})

		observations, providers, calls, roundPartial, roundErr := r.searchQueries(ctx, scope, current, ModeDeepResearch, remainingCalls, emit)
		totalSearchCalls += calls
		output.Stats.SearchCalls += calls
		output.Stats.CacheHits += countCacheHitCalls(observations)
		for _, provider := range providers {
			providerSet[provider] = struct{}{}
		}
		if roundPartial || roundErr != nil {
			partial = true
		}
		if roundErr != nil && len(allObservations) == 0 && len(observations) == 0 {
			return roundErr
		}
		if ctx.Err() != nil {
			return newError(ErrCancelled, "deep research cancelled", false, ctx.Err())
		}
		allObservations = append(allObservations, observations...)
		roundsCompleted = round

		deepResultLimit := maxInt(r.config.MaxSearchResults, r.config.MaxDeepOpenPages*3)
		fused := fuseResults(allObservations, deepResultLimit)
		newSources := 0
		for _, item := range fused {
			canonical := fusedCanonical(item)
			if canonical == "" {
				continue
			}
			if _, exists := discovered[canonical]; !exists {
				discovered[canonical] = struct{}{}
				newSources++
			}
		}

		if remainingOpen > 0 {
			openBatch := make([]SearchHit, 0, remainingOpen)
			for index, item := range fused {
				if len(openBatch) >= remainingOpen {
					break
				}
				canonical := fusedCanonical(item)
				if canonical == "" {
					continue
				}
				if _, exists := opened[canonical]; exists {
					continue
				}
				ref, refErr := r.ensureSearchReference(ctx, scope, item, index+1, referenceByCanonical)
				if refErr != nil {
					return refErr
				}
				opened[canonical] = struct{}{}
				openBatch = append(openBatch, searchHitFromReference(ref, item, index+1))
			}
			if len(openBatch) > 0 {
				pages, citations, fetchCalls, browserCalls := r.openTopSources(ctx, scope, openBatch, input.ResponseLength, emit)
				output.Stats.FetchCalls += fetchCalls
				output.Stats.BrowserCalls += browserCalls
				output.Stats.PageCacheHits += maxInt(0, len(pages)-fetchCalls)
				if ctx.Err() != nil {
					return newError(ErrCancelled, "deep research cancelled", false, ctx.Err())
				}
				if len(pages) < len(openBatch) {
					partial = true
				}
				appendUniquePages(output, pages, pageSeen)
				appendUniqueCitations(output, citations, citationSeen)
				remainingOpen -= len(openBatch)
				if remainingOpen < 0 {
					remainingOpen = 0
				}
			}
		}

		uniqueDomains := countFusedDomains(fused)
		if deepCoverageSatisfied(input.FocusAreas, output.Citations, uniqueDomains, round) {
			stopReason = deepStopCoverageSatisfied
			break
		}
		if totalSearchCalls >= r.config.MaxDeepSearchCalls {
			stopReason = deepStopMaxSearchCalls
			break
		}
		if round >= r.config.MaxDeepRounds {
			stopReason = deepStopMaxRounds
			break
		}
		if round > 1 && newSources < r.config.MinDeepNewSources {
			stopReason = deepStopNoNewSources
			break
		}

		next := r.buildDeepFollowUps(input, executed, output.Citations, round+1)
		if len(next) == 0 {
			stopReason = deepStopNoNewQueries
			break
		}
		for _, query := range next {
			followUpQueries = append(followUpQueries, query.Q)
			_ = emitProgress(emit, Progress{Phase: "research_followup", Query: query.Q, Message: "planning follow-up search", Indeterminate: true})
		}
		current = next
	}

	finalFused := fuseResults(allObservations, r.config.MaxSearchResults)
	for index, item := range finalFused {
		ref, refErr := r.ensureSearchReference(ctx, scope, item, index+1, referenceByCanonical)
		if refErr != nil {
			return refErr
		}
		output.Search = append(output.Search, searchHitFromReference(ref, item, index+1))
	}

	providers := make([]string, 0, len(providerSet))
	for provider := range providerSet {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	uniqueDomains := countSearchHitDomains(output.Search)
	if roundsCompleted == 0 && len(output.Search) > 0 {
		roundsCompleted = 1
	}
	if stopReason == "" {
		stopReason = deepStopBudgetSatisfied
	}
	output.Research = &ResearchSummary{
		Mode:            ModeDeepResearch,
		Queries:         dedupeStrings(allQueries),
		FollowUpQueries: dedupeStrings(followUpQueries),
		Providers:       providers,
		RoundsCompleted: roundsCompleted,
		OpenedPages:     len(output.Pages),
		EvidenceCount:   len(output.Citations),
		UniqueDomains:   uniqueDomains,
		StopReason:      stopReason,
		Partial:         partial,
	}
	_ = emitProgress(emit, Progress{Phase: "research_stop", Message: stopReason, Completed: roundsCompleted, Total: r.config.MaxDeepRounds, Fraction: 1})
	return nil
}

func (r *Runtime) buildDeepFollowUps(input ToolInput, executed map[string]struct{}, citations []Citation, round int) []SearchQueryCommand {
	if len(input.SearchQuery) == 0 {
		return nil
	}
	base := input.SearchQuery[0]
	maxQueries := r.config.MaxQueries
	if maxQueries <= 0 {
		maxQueries = 4
	}
	out := make([]SearchQueryCommand, 0, maxQueries)
	local := make(map[string]struct{})
	add := func(value SearchQueryCommand) {
		if len(out) >= maxQueries {
			return
		}
		key := normalizedQuery(value.Q)
		if key == "" {
			return
		}
		if _, exists := executed[key]; exists {
			return
		}
		if _, exists := local[key]; exists {
			return
		}
		local[key] = struct{}{}
		out = append(out, value)
	}

	for _, area := range input.FocusAreas {
		area = strings.TrimSpace(area)
		if area == "" || focusCovered(area, citations) {
			continue
		}
		candidate := base
		candidate.Q = strings.TrimSpace(base.Q + " " + area)
		add(candidate)
	}

	var suffixes []string
	switch round {
	case 2:
		suffixes = []string{"official documentation", "architecture implementation", "source code"}
	default:
		suffixes = []string{"limitations issues", "benchmark comparison", "recent changes"}
	}
	for _, suffix := range suffixes {
		candidate := base
		candidate.Q = strings.TrimSpace(base.Q + " " + suffix)
		add(candidate)
	}
	return out
}

func deepCoverageSatisfied(focusAreas []string, citations []Citation, uniqueDomains, round int) bool {
	if round < 2 {
		return false
	}
	if len(citations) < 4 || uniqueDomains < 2 {
		return false
	}
	for _, area := range focusAreas {
		if strings.TrimSpace(area) != "" && !focusCovered(area, citations) {
			return false
		}
	}
	if len(focusAreas) == 0 {
		return len(citations) >= 6 && uniqueDomains >= 3
	}
	return true
}

func focusCovered(focus string, citations []Citation) bool {
	focus = strings.TrimSpace(focus)
	if focus == "" {
		return true
	}
	focusTokens := tokenize(focus)
	if len(focusTokens) == 0 {
		return false
	}
	needle := strings.ToLower(focus)
	for _, citation := range citations {
		text := strings.ToLower(citation.Title + "\n" + citation.Text)
		if strings.Contains(text, needle) {
			return true
		}
		if overlapScore(focusTokens, tokenize(text)) >= 0.45 {
			return true
		}
	}
	return false
}

func (r *Runtime) ensureSearchReference(ctx context.Context, scope Scope, item fusedResult, rank int, refs map[string]Reference) (Reference, *Error) {
	canonical := fusedCanonical(item)
	if canonical != "" {
		if ref, exists := refs[canonical]; exists {
			ref.Rank = rank
			if err := r.store.PutReference(ctx, ref); err != nil {
				return Reference{}, newError(ErrNotConfigured, "failed to update search reference", true, err)
			}
			return ref, nil
		}
	}
	ref, err := r.referenceFromSearch(ctx, scope, item, rank)
	if err != nil {
		return Reference{}, err
	}
	if canonical != "" {
		refs[canonical] = ref
	}
	return ref, nil
}

func fusedCanonical(item fusedResult) string {
	canonical := strings.TrimSpace(item.result.Source.CanonicalURL)
	if canonical == "" {
		canonical = canonicalizeURL(item.result.URL)
	}
	return canonical
}

func searchHitFromReference(ref Reference, item fusedResult, rank int) SearchHit {
	return SearchHit{
		RefID:       ref.RefID,
		Rank:        rank,
		Title:       ref.Title,
		URL:         ref.URL,
		Domain:      domainOf(ref.URL),
		Snippet:     ref.Snippet,
		Provider:    ref.Provider,
		PublishedAt: ref.PublishedAt,
		RetrievedAt: item.retrievedAt,
		Score:       item.score,
		SeenCount:   item.seenCount,
	}
}

func appendUniquePages(output *ToolOutput, pages []Page, seen map[string]struct{}) {
	for _, page := range pages {
		key := page.CanonicalURL
		if key == "" {
			key = page.RefID
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		output.Pages = append(output.Pages, page)
	}
}

func appendUniqueCitations(output *ToolOutput, citations []Citation, seen map[string]struct{}) {
	for _, citation := range citations {
		key := citation.EvidenceID
		if key == "" {
			key = citation.RefID + "\x00" + citation.Text
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		output.Citations = append(output.Citations, citation)
	}
}

func countFusedDomains(items []fusedResult) int {
	seen := make(map[string]struct{})
	for _, item := range items {
		if domain := domainOf(item.result.URL); domain != "" {
			seen[domain] = struct{}{}
		}
	}
	return len(seen)
}

func countSearchHitDomains(items []SearchHit) int {
	seen := make(map[string]struct{})
	for _, item := range items {
		if item.Domain != "" {
			seen[item.Domain] = struct{}{}
		}
	}
	return len(seen)
}

func normalizedQuery(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func dedupeStrings(values []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		key := normalizedQuery(value)
		if key == "" {
			continue
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
	}
	return out
}
