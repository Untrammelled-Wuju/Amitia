package webresearch

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

const (
	deepStopCoverageSatisfied   = "coverage_satisfied"
	deepStopMaxRounds           = "max_rounds"
	deepStopMaxSearchCalls      = "max_search_calls"
	deepStopMaxProviderCost     = "max_provider_cost"
	deepStopMaxProviderCredits  = "max_provider_credits"
	deepStopNoNewQueries        = "no_new_queries"
	deepStopNoNewSources        = "no_new_sources"
	deepStopLowInformationGain  = "low_information_gain"
	deepStopBudgetSatisfied     = "budget_satisfied"
	deepInformationGainFloor    = 0.15
	deepLowGainRoundsBeforeStop = 2
)

func (r *Runtime) executeDeepResearch(ctx context.Context, scope Scope, input ToolInput, output *ToolOutput, emit ProgressFunc) *Error {
	return newResearchSupervisor(r, scope, input, output, emit).Run(ctx)
}

func (r *Runtime) buildResearchPlan(input ToolInput) ResearchPlan {
	goalParts := make([]string, 0, len(input.SearchQuery))
	questions := make([]ResearchQuestion, 0, len(input.SearchQuery)+len(input.FocusAreas))
	seen := make(map[string]string)
	addQuestion := func(text string, priority int, required bool, dependencies []string) string {
		text = strings.TrimSpace(text)
		key := normalizedQuery(text)
		if key == "" {
			return ""
		}
		if id, exists := seen[key]; exists {
			return id
		}
		id := fmt.Sprintf("q%d", len(questions)+1)
		seen[key] = id
		questions = append(questions, ResearchQuestion{
			ID:           id,
			Question:     text,
			Priority:     priority,
			Required:     required,
			Dependencies: dedupeStrings(dependencies),
			SuccessCriteria: []string{
				"at least one directly relevant evidence item",
			},
		})
		return id
	}

	// Broad user queries are roots in the research DAG. Focus questions depend
	// on those roots so later follow-up rounds first establish the requested
	// subject before spending budget on narrower branches. The first round may
	// still search the user's explicit focus areas directly; dependencies govern
	// dynamic follow-up scheduling, not whether user-supplied queries are valid.
	rootIDs := make([]string, 0, len(input.SearchQuery))
	for _, command := range input.SearchQuery {
		if q := strings.TrimSpace(command.Q); q != "" {
			goalParts = append(goalParts, q)
			if id := addQuestion(q, 100, len(input.FocusAreas) == 0, nil); id != "" {
				rootIDs = append(rootIDs, id)
			}
		}
	}
	rootIDs = dedupeStrings(rootIDs)
	for _, focus := range input.FocusAreas {
		addQuestion(focus, 120, true, rootIDs)
	}
	goal := strings.Join(dedupeStrings(goalParts), "; ")
	if goal == "" && len(questions) > 0 {
		goal = questions[0].Question
	}
	return ResearchPlan{
		Goal:      goal,
		Questions: questions,
		SuccessCriteria: []string{
			"cover every required research question",
			"prefer evidence from multiple independent domains when available",
			"stop when additional searches produce little new evidence",
		},
		Budget: ResearchBudget{
			MaxRounds:          r.config.MaxDeepRounds,
			MaxSearchCalls:     r.config.MaxSearchCalls,
			MaxOpenPages:       r.config.MaxDeepOpenPages,
			MaxProviderCostUSD: r.config.MaxProviderCostUSD,
			MaxProviderCredits: r.config.MaxProviderCredits,
		},
	}
}

func (r *Runtime) buildDeepFollowUps(input ToolInput, plan ResearchPlan, executed map[string]struct{}, citations []Citation) []SearchQueryCommand {
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
		value.Q = strings.TrimSpace(value.Q)
		key := researchQueryIdentity(value)
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

	questions := append([]ResearchQuestion(nil), plan.Questions...)
	sort.SliceStable(questions, func(i, j int) bool {
		if questions[i].Priority != questions[j].Priority {
			return questions[i].Priority > questions[j].Priority
		}
		return questions[i].ID < questions[j].ID
	})
	for _, question := range questions {
		if researchQuestionCovered(question, citations) {
			continue
		}
		if !researchQuestionDependenciesSatisfied(plan, question, citations) {
			continue
		}
		candidate := base
		if normalizedQuery(question.Question) == normalizedQuery(base.Q) {
			candidate.Q = question.Question
		} else {
			candidate.Q = strings.TrimSpace(base.Q + " " + question.Question)
		}
		add(candidate)
	}
	for _, command := range input.SearchQuery[1:] {
		add(command)
	}
	return out
}

func researchQuestionDependenciesSatisfied(plan ResearchPlan, question ResearchQuestion, citations []Citation) bool {
	if len(question.Dependencies) == 0 {
		return true
	}
	byID := make(map[string]ResearchQuestion, len(plan.Questions))
	for _, candidate := range plan.Questions {
		byID[candidate.ID] = candidate
	}
	for _, dependencyID := range question.Dependencies {
		dependency, ok := byID[dependencyID]
		if !ok || !researchQuestionCovered(dependency, citations) {
			return false
		}
	}
	return true
}

func researchPlanCoverageSatisfied(plan ResearchPlan, citations []Citation, uniqueDomains, round int) bool {
	if round < 2 || len(citations) < 4 || uniqueDomains < 2 {
		return false
	}
	for _, question := range plan.Questions {
		if question.Required && !researchQuestionCovered(question, citations) {
			return false
		}
	}
	if len(plan.Questions) == 0 {
		return len(citations) >= 6 && uniqueDomains >= 3
	}
	return true
}

func unresolvedResearchQuestions(plan ResearchPlan, citations []Citation) []string {
	out := make([]string, 0)
	for _, question := range plan.Questions {
		if question.Required && !researchQuestionCovered(question, citations) {
			out = append(out, question.Question)
		}
	}
	return out
}

func researchQuestionCovered(question ResearchQuestion, citations []Citation) bool {
	return focusCovered(question.Question, citations)
}

func researchQueryIdentity(command SearchQueryCommand) string {
	query := normalizedQuery(command.Q)
	if query == "" {
		return ""
	}
	kind := strings.ToLower(strings.TrimSpace(command.Kind))
	domains := normalizeDomains(command.Domains)
	sort.Strings(domains)
	return query + "\x00" + kind + "\x00" + strings.Join(domains, ",")
}

func deepInformationGain(newSources, newEvidence, newDomains, minNewSources int) float64 {
	if minNewSources <= 0 {
		minNewSources = 2
	}
	sourceGain := minFloat(1, float64(newSources)/float64(minNewSources))
	evidenceGain := minFloat(1, float64(newEvidence)/4.0)
	domainGain := minFloat(1, float64(newDomains)/2.0)
	return sourceGain*0.45 + evidenceGain*0.35 + domainGain*0.20
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
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
