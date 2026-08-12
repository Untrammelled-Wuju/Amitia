package deepsearch

import (
	"strings"

	"github.com/u-ai/backend/internal/search"
)

type RoundPlanner interface {
	PlanInitial(req DeepSearchRequest) []SearchQueryPlan
	PlanNext(req DeepSearchRequest, state *SearchState) []SearchQueryPlan
}

type SearchState struct {
	CurrentRound  int
	Coverage      map[string]int
	ExecutedPlans []SearchQueryPlan
	searchCalls   int
}

func NewSearchState() *SearchState {
	return &SearchState{
		Coverage: make(map[string]int),
	}
}

func (s *SearchState) RecordExecuted(plan SearchQueryPlan, resultCount int) {
	s.ExecutedPlans = append(s.ExecutedPlans, plan)
	s.searchCalls++
	if plan.FocusArea != "" {
		s.Coverage[plan.FocusArea] += resultCount
	}
}

type DeterministicRoundPlanner struct {
	policy DeepSearchPolicy
}

func NewDeterministicRoundPlanner(policy DeepSearchPolicy) *DeterministicRoundPlanner {
	return &DeterministicRoundPlanner{policy: policy}
}

func (p *DeterministicRoundPlanner) PlanInitial(req DeepSearchRequest) []SearchQueryPlan {
	kind := search.NormalizeKind(req.Kind)
	var plans []SearchQueryPlan
	plans = append(plans, SearchQueryPlan{
		Query:  req.Query,
		Kind:   kind,
		Round:  1,
		Reason: "seed",
	})

	seen := map[string]struct{}{normalize(req.Query): {}}

	perRound := p.policy.MaxQueriesPerRound
	if len(req.FocusAreas)+1 < perRound {
		perRound = len(req.FocusAreas) + 1
	}

	for _, fa := range req.FocusAreas {
		if len(plans) >= perRound {
			break
		}
		candidate := req.Query + " " + fa
		key := normalize(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		plans = append(plans, SearchQueryPlan{
			Query:     candidate,
			Kind:      kind,
			Round:     1,
			Reason:    "focus_gap",
			FocusArea: fa,
		})
	}

	return plans
}

func (p *DeterministicRoundPlanner) PlanNext(req DeepSearchRequest, state *SearchState) []SearchQueryPlan {
	state.CurrentRound++
	kind := search.NormalizeKind(req.Kind)

	if state.CurrentRound >= p.policy.MaxRounds {
		return nil
	}

	var plans []SearchQueryPlan
	perRound := p.policy.MaxQueriesPerRound
	seen := map[string]struct{}{}

	for _, ep := range state.ExecutedPlans {
		seen[normalize(ep.Query)] = struct{}{}
	}

	for _, fa := range req.FocusAreas {
		if len(plans) >= perRound-1 {
			break
		}
		hits := state.Coverage[fa]
		if hits >= p.policy.FocusHitThreshold {
			continue
		}
		candidate := req.Query + " " + fa
		key := normalize(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		plans = append(plans, SearchQueryPlan{
			Query:     candidate,
			Kind:      kind,
			Round:     state.CurrentRound,
			Reason:    "focus_gap",
			FocusArea: fa,
		})
	}

	if len(plans) == 0 && len(req.FocusAreas) == 0 {
		clarify := req.Query + " guide"
		key := normalize(clarify)
		if _, ok := seen[key]; !ok {
			plans = append(plans, SearchQueryPlan{
				Query:  clarify,
				Kind:   kind,
				Round:  state.CurrentRound,
				Reason: "coverage_gap",
			})
		}
	}

	return plans
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	fields := strings.Fields(s)
	return strings.Join(fields, " ")
}
