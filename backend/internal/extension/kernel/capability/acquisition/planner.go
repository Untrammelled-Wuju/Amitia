package acquisition

import (
	"context"
	"errors"
)

// CandidateSearcher defines the interface used by the Planner to discover
// capability candidates from one or more sources (registries, local
// extensions, MCP servers, skill catalogs, etc.).
//
// Implementations are expected to return every candidate that could satisfy
// the requested capability, without ranking or filtering by policy.
type CandidateSearcher interface {
	Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error)
}

// SearchService is a stub implementation of CandidateSearcher.
// Replace with a concrete implementation that wires to registries, catalogs,
// and local providers.
type SearchService struct{}

// NewSearchService returns a SearchService stub. Replace with a concrete
// implementation that wires to registries, catalogs, and local providers.
func NewSearchService() *SearchService {
	return &SearchService{}
}

// Search returns candidate sources that may satisfy the acquisition request.
// The default implementation returns an empty slice; concrete implementations
// should query registries, local extensions, MCP servers, and skill catalogs.
func (s *SearchService) Search(ctx context.Context, request AcquisitionRequest) ([]CapabilityCandidate, error) {
	return nil, nil
}

// Planner orchestrates the full acquisition planning pipeline:
//  1. Search for candidate sources
//  2. Deduplicate candidates
//  3. Rank candidates
//  4. Plan deployment target for the top-ranked candidate
//  5. Evaluate policy for the top-ranked candidate
//  6. Construct ordered acquisition steps
type Planner struct {
	searchService CandidateSearcher
	policy        *PolicyEngine
	deployment    *DeploymentPlanner
	scorer        *CandidateScorer
}

// NewPlanner returns a Planner wired with the provided dependencies.
func NewPlanner(search CandidateSearcher, policy *PolicyEngine, deployment *DeploymentPlanner) *Planner {
	return &Planner{
		searchService: search,
		policy:        policy,
		deployment:    deployment,
		scorer:        NewCandidateScorer(),
	}
}

// Plan executes the acquisition planning pipeline and returns a complete
// AcquisitionPlan for the highest-ranked candidate that passes policy.
func (p *Planner) Plan(ctx context.Context, request AcquisitionRequest) (*AcquisitionPlan, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	candidates, err := p.searchService.Search(ctx, request)
	if err != nil {
		return nil, err
	}

	candidates = deduplicateCandidates(candidates)
	if len(candidates) == 0 {
		return nil, errors.Join(ErrNoCandidate, &AcquisitionError{
			Code:    "no_candidate",
			Message: "no candidate sources found for capability",
		})
	}

	ranked := p.rankedCandidates(request, candidates)
	if len(ranked) == 0 {
		return nil, ErrNoCompatibleCandidate
	}

	plan := p.buildPlan(ctx, request, ranked)
	return plan, nil
}

// rankedCandidates returns the candidates sorted by descending score.
func (p *Planner) rankedCandidates(request AcquisitionRequest, candidates []CapabilityCandidate) []CapabilityCandidate {
	return RankCandidates(candidates, request)
}

// buildPlan constructs an AcquisitionPlan from the ranked candidate list. It
// walks the ranked list until it finds a candidate whose policy decision is
// not ActionDeny, then plans the target and builds the step list.
func (p *Planner) buildPlan(ctx context.Context, request AcquisitionRequest, ranked []CapabilityCandidate) *AcquisitionPlan {
	for _, candidate := range ranked {
		decision := p.policy.Evaluate(candidate, request)
		if decision.Action == ActionDeny {
			continue
		}

		target, err := p.deployment.PlanTarget(ctx, candidate, request)
		if err != nil {
			continue
		}

		steps := buildSteps(candidate, decision)
		return &AcquisitionPlan{
			Request:             request,
			Candidate:           candidate,
			Target:              target,
			PolicyDecision:      decision,
			Steps:               steps,
			RequiredPermissions: decision.RequiredPermissions,
		}
	}

	// All candidates denied: surface the first candidate with its deny decision.
	candidate := ranked[0]
	decision := p.policy.Evaluate(candidate, request)
	target, _ := p.deployment.PlanTarget(ctx, candidate, request)
	return &AcquisitionPlan{
		Request:        request,
		Candidate:      candidate,
		Target:         target,
		PolicyDecision: decision,
		Steps:          nil,
		Warnings:       []string{"all candidates denied by policy"},
	}
}

// deduplicateCandidates removes duplicate candidates based on their ID.
// When duplicates exist, the first occurrence wins.
func deduplicateCandidates(candidates []CapabilityCandidate) []CapabilityCandidate {
	if len(candidates) <= 1 {
		return candidates
	}
	seen := make(map[string]bool, len(candidates))
	result := make([]CapabilityCandidate, 0, len(candidates))
	for _, c := range candidates {
		if c.ID == "" {
			result = append(result, c)
			continue
		}
		if seen[c.ID] {
			continue
		}
		seen[c.ID] = true
		result = append(result, c)
	}
	return result
}

// buildSteps constructs the ordered acquisition steps for a candidate based
// on its kind and the policy decision.
func buildSteps(candidate CapabilityCandidate, decision PolicyDecision) []AcquisitionPlanStep {
	var steps []AcquisitionPlanStep
	order := 0

	if decision.Action == ActionRequireApproval {
		order++
		steps = append(steps, AcquisitionPlanStep{
			Order:       order,
			Action:      "await_approval",
			Description: "Wait for user approval before proceeding",
			Kind:        candidate.Kind,
			Completed:   false,
		})
	}

	switch candidate.Install.Method {
	case InstallEnableExisting:
		order++
		steps = append(steps, AcquisitionPlanStep{
			Order:       order,
			Action:      "enable",
			Description: "Enable the already-installed provider",
			Kind:        candidate.Kind,
			Completed:   false,
		})
	default:
		order++
		steps = append(steps, AcquisitionPlanStep{
			Order:       order,
			Action:      "install",
			Description: "Install the capability provider",
			Kind:        candidate.Kind,
			Completed:   false,
		})
		order++
		steps = append(steps, AcquisitionPlanStep{
			Order:       order,
			Action:      "enable",
			Description: "Enable the installed provider",
			Kind:        candidate.Kind,
			Completed:   false,
		})
	}

	order++
	steps = append(steps, AcquisitionPlanStep{
		Order:       order,
		Action:      "reconcile",
		Description: "Reconcile provider instances and verify availability",
		Kind:        candidate.Kind,
		Completed:   false,
	})

	return steps
}
