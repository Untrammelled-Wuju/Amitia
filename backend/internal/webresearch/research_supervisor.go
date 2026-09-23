package webresearch

import (
	"context"
	"sort"

	"github.com/u-ai/backend/internal/search"
)

// ResearchSupervisor owns one deep-research execution. It coordinates plan
// coverage, budgets, stopping conditions, worker rounds and final merge while
// leaving transport and model-visible protocol on the single web_run runtime.
type ResearchSupervisor struct {
	runtime *Runtime
	scope   Scope
	input   ToolInput
	output  *ToolOutput
	emit    ProgressFunc
}

func newResearchSupervisor(runtime *Runtime, scope Scope, input ToolInput, output *ToolOutput, emit ProgressFunc) *ResearchSupervisor {
	return &ResearchSupervisor{runtime: runtime, scope: scope, input: input, output: output, emit: emit}
}

// ResearchWorker executes bounded retrieval work for the supervisor. Workers do
// not synthesize a final answer; they only search, open sources and return
// evidence-bearing runtime artifacts.
type ResearchWorker struct {
	runtime *Runtime
}

func (w ResearchWorker) Search(ctx context.Context, scope Scope, queries []SearchQueryCommand, mode Mode, maxCalls int, emit ProgressFunc) ([]searchObservation, []string, int, search.ProviderUsage, bool, *Error) {
	if w.runtime == nil {
		return nil, nil, 0, search.ProviderUsage{}, false, newError(ErrNotConfigured, "research worker runtime unavailable", false, nil)
	}
	return w.runtime.searchQueries(ctx, scope, queries, mode, maxCalls, emit)
}

func (w ResearchWorker) OpenSources(ctx context.Context, scope Scope, hits []SearchHit, responseLength string, emit ProgressFunc) ([]Page, []Citation, int, int) {
	if w.runtime == nil {
		return nil, nil, 0, 0
	}
	return w.runtime.openTopSources(ctx, scope, hits, responseLength, emit)
}

func (s *ResearchSupervisor) Run(ctx context.Context) *Error {
	if s == nil || s.runtime == nil || s.output == nil {
		return newError(ErrNotConfigured, "research supervisor unavailable", false, nil)
	}
	r := s.runtime
	scope := s.scope
	input := s.input
	output := s.output
	emit := s.emit
	worker := ResearchWorker{runtime: r}

	plan := r.buildResearchPlan(input)
	initial := expandQueries(input.SearchQuery, input.FocusAreas, ModeResearch, r.config.MaxQueries)
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
	discoveredDomains := make(map[string]struct{})
	opened := make(map[string]struct{})
	pageSeen := make(map[string]struct{})
	citationSeen := make(map[string]struct{})
	roundSummaries := make([]ResearchRound, 0, r.config.MaxDeepRounds)
	current := initial
	stopReason := deepStopMaxRounds
	partial := false
	roundsCompleted := 0
	totalSearchCalls := 0
	remainingOpen := r.config.MaxDeepOpenPages
	lowGainRounds := 0

	for round := 1; round <= r.config.MaxDeepRounds; round++ {
		if len(current) == 0 {
			stopReason = deepStopNoNewQueries
			break
		}
		remainingCalls := r.config.MaxSearchCalls - totalSearchCalls
		if remainingCalls <= 0 {
			stopReason = deepStopMaxSearchCalls
			break
		}
		if r.config.MaxProviderCostUSD > 0 && output.Stats.ProviderCostUSD >= r.config.MaxProviderCostUSD {
			stopReason = deepStopMaxProviderCost
			break
		}
		if r.config.MaxProviderCredits > 0 && output.Stats.ProviderCredits >= r.config.MaxProviderCredits {
			stopReason = deepStopMaxProviderCredits
			break
		}
		for _, query := range current {
			key := researchQueryIdentity(query)
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

		beforeEvidence := len(output.Citations)
		beforeDomains := len(discoveredDomains)
		observations, providers, calls, usage, roundPartial, roundErr := worker.Search(ctx, scope, current, ModeDeepResearch, remainingCalls, emit)
		totalSearchCalls += calls
		output.Stats.SearchCalls += calls
		output.Stats.ProviderCostUSD += usage.CostUSD
		output.Stats.ProviderCredits += usage.Credits
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
		fused := fuseResults(allObservations, deepResultLimit, r.config.MaxResultsPerDomain)
		if used, rerankErr := r.semanticRerank(ctx, plan.Goal, fused); used {
			output.Stats.SemanticRerankCalls++
		} else if rerankErr != nil {
			output.Stats.SemanticRerankFailures++
		}
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
			if domain := domainOf(item.result.URL); domain != "" {
				discoveredDomains[domain] = struct{}{}
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
				pages, citations, fetchCalls, browserCalls := worker.OpenSources(ctx, scope, openBatch, input.ResponseLength, emit)
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

		newEvidence := maxInt(0, len(output.Citations)-beforeEvidence)
		newDomains := maxInt(0, len(discoveredDomains)-beforeDomains)
		gain := deepInformationGain(newSources, newEvidence, newDomains, r.config.MinDeepNewSources)
		roundSummaries = append(roundSummaries, ResearchRound{
			Round:           round,
			Queries:         len(current),
			NewSources:      newSources,
			NewEvidence:     newEvidence,
			NewDomains:      newDomains,
			InformationGain: gain,
		})
		if gain < deepInformationGainFloor {
			lowGainRounds++
		} else {
			lowGainRounds = 0
		}

		uniqueDomains := len(discoveredDomains)
		if researchPlanCoverageSatisfied(plan, output.Citations, uniqueDomains, round) {
			stopReason = deepStopCoverageSatisfied
			break
		}
		if r.config.MaxProviderCostUSD > 0 && output.Stats.ProviderCostUSD >= r.config.MaxProviderCostUSD {
			stopReason = deepStopMaxProviderCost
			break
		}
		if r.config.MaxProviderCredits > 0 && output.Stats.ProviderCredits >= r.config.MaxProviderCredits {
			stopReason = deepStopMaxProviderCredits
			break
		}
		if totalSearchCalls >= r.config.MaxSearchCalls {
			stopReason = deepStopMaxSearchCalls
			break
		}
		if round >= r.config.MaxDeepRounds {
			stopReason = deepStopMaxRounds
			break
		}
		if lowGainRounds >= deepLowGainRoundsBeforeStop {
			stopReason = deepStopLowInformationGain
			break
		}
		if round > 1 && newSources < r.config.MinDeepNewSources && newEvidence == 0 {
			stopReason = deepStopNoNewSources
			break
		}

		next := r.buildDeepFollowUps(input, plan, executed, output.Citations)
		if len(next) == 0 {
			stopReason = deepStopNoNewQueries
			break
		}
		for _, query := range next {
			followUpQueries = append(followUpQueries, query.Q)
			_ = emitProgress(emit, Progress{Phase: "research_followup", Query: query.Q, Message: "planning evidence gap follow-up", Indeterminate: true})
		}
		current = next
	}

	finalFused := fuseResults(allObservations, r.config.MaxSearchResults, r.config.MaxResultsPerDomain)
	if used, rerankErr := r.semanticRerank(ctx, plan.Goal, finalFused); used {
		output.Stats.SemanticRerankCalls++
	} else if rerankErr != nil {
		output.Stats.SemanticRerankFailures++
	}
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
	if uniqueDomains < len(discoveredDomains) {
		uniqueDomains = len(discoveredDomains)
	}
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
		Plan:            &plan,
		Unresolved:      unresolvedResearchQuestions(plan, output.Citations),
		Rounds:          roundSummaries,
		Findings:        mergeResearchFindings(plan, output.Citations),
		Cost: ResearchCost{
			SearchRequests:  output.Stats.SearchCalls,
			FetchRequests:   output.Stats.FetchCalls,
			BrowserCalls:    output.Stats.BrowserCalls,
			BrowserSeconds:  float64(output.Stats.BrowserDurationMs) / 1000.0,
			ProviderCostUSD: output.Stats.ProviderCostUSD,
			ProviderCredits: output.Stats.ProviderCredits,
		},
		Partial: partial,
	}
	_ = emitProgress(emit, Progress{Phase: "research_stop", Message: stopReason, Completed: roundsCompleted, Total: r.config.MaxDeepRounds, Fraction: 1})
	return nil
}
