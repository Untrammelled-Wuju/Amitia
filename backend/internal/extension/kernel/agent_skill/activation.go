package agent_skill

import (
	"context"
	"sort"
	"strings"
)

type ActivationService struct {
	catalog *AgentSkillCatalog
	maxAutoActivatePerRound int
}

func NewActivationService(catalog *AgentSkillCatalog) *ActivationService {
	return &ActivationService{
		catalog:                 catalog,
		maxAutoActivatePerRound: 10,
	}
}

type ActivationCandidate struct {
	Definition AgentSkillDefinition
	Score      int
	MatchType  string
}

func (s *ActivationService) EvaluateAuto(ctx context.Context, userInput string, scope AgentSkillScope, scopeID string) []ActivationCandidate {
	all := s.catalog.List(CatalogFilter{
		Scope:   scope,
		Enabled: boolPtr(true),
	})

	var candidates []ActivationCandidate
	for _, def := range all {
		if def.Activation.Mode != ActivationAuto && def.Activation.Mode != ActivationManual {
			continue
		}
		if def.Scope == AgentSkillScopeCharacter && def.ScopeID != "" && def.ScopeID != scopeID {
			continue
		}

		score, matchType := s.matchScore(def, userInput)
		if score > 0 || def.Activation.Mode == ActivationAuto {
			candidates = append(candidates, ActivationCandidate{
				Definition: def,
				Score:      score,
				MatchType:   matchType,
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Definition.Activation.Priority != candidates[j].Definition.Activation.Priority {
			return candidates[i].Definition.Activation.Priority > candidates[j].Definition.Activation.Priority
		}
		return candidates[i].Score > candidates[j].Score
	})

	if len(candidates) > s.maxAutoActivatePerRound {
		candidates = candidates[:s.maxAutoActivatePerRound]
	}

	return candidates
}

func (s *ActivationService) EvaluateExplicit(ctx context.Context, skillID string) *ActivationCandidate {
	def, ok := s.catalog.Get(skillID)
	if !ok {
		return nil
	}
	if def.Activation.Mode == ActivationExplicit || def.Activation.Mode == ActivationManual {
		return &ActivationCandidate{
			Definition: def,
			Score:      100,
			MatchType:  "explicit",
		}
	}
	return nil
}

func (s *ActivationService) EvaluateSystem(ctx context.Context) []ActivationCandidate {
	all := s.catalog.List(CatalogFilter{
		Scope:   AgentSkillScopeGlobal,
		Enabled: boolPtr(true),
	})

	var candidates []ActivationCandidate
	for _, def := range all {
		if def.Activation.Mode == ActivationAuto || def.Activation.Mode == ActivationExplicit {
			candidates = append(candidates, ActivationCandidate{
				Definition: def,
				Score:      def.Activation.Priority,
				MatchType:  "system",
			})
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Definition.Activation.Priority > candidates[j].Definition.Activation.Priority
	})

	return candidates
}

func (s *ActivationService) matchScore(def AgentSkillDefinition, input string) (int, string) {
	if input == "" {
		if def.Activation.Mode == ActivationAuto {
			return def.Activation.Priority, "auto"
		}
		return 0, ""
	}

	lower := strings.ToLower(input)
	bestScore := 0
	matchType := ""

	for _, keyword := range def.Activation.Keywords {
		kw := strings.ToLower(keyword)
		if strings.Contains(lower, kw) {
			score := len(kw) * 10
			if score > bestScore {
				bestScore = score
				matchType = "keyword"
			}
		}
	}

	name := strings.ToLower(def.Name)
	if strings.Contains(lower, name) {
		score := len(name) * 5
		if score > bestScore {
			bestScore = score
			matchType = "name"
		}
	}

	if def.Activation.Mode == ActivationAuto {
		bestScore = max(bestScore, def.Activation.Priority)
	}

	return bestScore, matchType
}

type TokenBudgeter struct {
	poolSize int
}

func NewTokenBudgeter(poolSize int) *TokenBudgeter {
	if poolSize <= 0 {
		poolSize = 8000
	}
	return &TokenBudgeter{poolSize: poolSize}
}

type BudgetAllocation struct {
	SkillID      string `json:"skillId"`
	Instructions  int    `json:"instructions"`
	Resources     int    `json:"resources"`
	Total         int    `json:"total"`
	Truncated     bool   `json:"truncated,omitempty"`
}

type BudgetPlan struct {
	Allocations []BudgetAllocation `json:"allocations"`
	TotalUsed   int                `json:"totalUsed"`
	Remaining   int                `json:"remaining"`
}

func (b *TokenBudgeter) Allocate(candidates []ActivationCandidate) BudgetPlan {
	plan := BudgetPlan{Remaining: b.poolSize}

	for _, candidate := range candidates {
		def := candidate.Definition
		alloc := BudgetAllocation{
			SkillID:     def.ExtensionID,
			Instructions: def.Instructions.TokenCount,
		}

		for _, res := range def.Resources {
			alloc.Resources += res.TokenEstimate
		}

		budget := def.TokenPolicy.MaxInstructionTokens
		if budget <= 0 {
			budget = 2000
		}

		maxResources := def.TokenPolicy.MaxResourceTokensPerTurn
		if maxResources <= 0 {
			maxResources = 2000
		}

		if alloc.Instructions > budget {
			alloc.Instructions = budget
			alloc.Truncated = true
		}
		if alloc.Resources > maxResources {
			alloc.Resources = maxResources
			alloc.Truncated = true
		}

		alloc.Total = alloc.Instructions + alloc.Resources

		if alloc.Total > plan.Remaining {
			alloc.Total = plan.Remaining
			alloc.Truncated = true
		}

		plan.Allocations = append(plan.Allocations, alloc)
		plan.TotalUsed += alloc.Total
		plan.Remaining = b.poolSize - plan.TotalUsed

		if plan.Remaining <= 0 {
			break
		}
	}

	return plan
}
