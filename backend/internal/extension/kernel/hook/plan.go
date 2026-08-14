package hook

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"time"
)

type CompiledHookContribution struct {
	ContributionID         string                  `json:"contributionId"`
	ExtensionID            string                  `json:"extensionId"`
	HookPointID            string                  `json:"hookPointId"`
	Phase                  HookPhase               `json:"phase"`
	Priority               int                     `json:"priority"`
	Before                 []string                `json:"before,omitempty"`
	After                  []string                `json:"after,omitempty"`
	Timeout                time.Duration           `json:"timeout"`
	FailurePolicy          *HookFailurePolicy      `json:"failurePolicy,omitempty"`
	MutationClaims         []string                `json:"mutationClaims,omitempty"`
	PermissionRequirements []PermissionRequirement `json:"permissionRequirements,omitempty"`
	ScopeRule              ScopeRule               `json:"scopeRule"`
	RuntimeBinding         RuntimeBinding          `json:"runtimeBinding"`
	DefinitionHash         string                  `json:"definitionHash"`
}

type CompiledHookPlan struct {
	HookPointID         string                     `json:"hookPointId"`
	HookContractVersion uint32                     `json:"hookContractVersion"`
	PlanGeneration      uint64                     `json:"planGeneration"`
	Ordered             []CompiledHookContribution `json:"ordered"`
	DefinitionHash      string                     `json:"definitionHash"`
	CompiledAt          time.Time                  `json:"compiledAt"`
	CycleDetected       bool                       `json:"cycleDetected"`
	Warnings            []string                   `json:"warnings,omitempty"`
}

type PlanCache struct {
	mu         sync.RWMutex
	plans      map[string]*CompiledHookPlan
	generation uint64
}

func NewPlanCache() *PlanCache {
	return &PlanCache{
		plans: make(map[string]*CompiledHookPlan),
	}
}

func (c *PlanCache) Get(hookPointID string) (*CompiledHookPlan, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	plan, ok := c.plans[hookPointID]
	return plan, ok
}

func (c *PlanCache) BuildOrReplace(point HookPointDefinition, contribs []HookContributionDefinition, circuitStates map[string]CircuitStats) *CompiledHookPlan {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.generation++
	plan := compilePlan(point, contribs, c.generation, circuitStates)
	c.plans[point.HookPointID] = plan
	return plan
}

func (c *PlanCache) Invalidate(hookPointID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.plans, hookPointID)
}

func (c *PlanCache) InvalidateAll() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.plans = make(map[string]*CompiledHookPlan)
}

func (c *PlanCache) Generation() uint64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.generation
}

func compilePlan(point HookPointDefinition, contribs []HookContributionDefinition, generation uint64, circuitStates map[string]CircuitStats) *CompiledHookPlan {
	enabled := make([]HookContributionDefinition, 0, len(contribs))
	for _, c := range contribs {
		if !c.Enabled {
			continue
		}
		stats, hasStats := circuitStates[c.ContributionID]
		if hasStats && stats.State == CircuitOpen {
			continue
		}
		if !c.SystemReserved && (c.Priority < MinThirdPartyPriority || c.Priority > MaxThirdPartyPriority) {
			continue
		}
		enabled = append(enabled, c)
	}

	orderResult := OrderHooks(OrderingInput{Contributions: enabled})

	ordered := make([]CompiledHookContribution, 0, len(orderResult.Ordered))
	for _, c := range orderResult.Ordered {
		ordered = append(ordered, CompiledHookContribution{
			ContributionID:         c.ContributionID,
			ExtensionID:            c.ExtensionID,
			HookPointID:            c.HookPointID,
			Phase:                  c.Phase,
			Priority:               c.Priority,
			Before:                 c.Before,
			After:                  c.After,
			Timeout:                c.Timeout,
			FailurePolicy:          c.FailurePolicy,
			MutationClaims:         c.MutationClaims,
			PermissionRequirements: c.PermissionRequirements,
			ScopeRule:              c.ScopeRule,
			RuntimeBinding:         c.RuntimeBinding,
			DefinitionHash:         c.DefinitionHash,
		})
	}

	defHash := computePlanDefinitionHash(point.HookPointID, ordered)

	return &CompiledHookPlan{
		HookPointID:         point.HookPointID,
		HookContractVersion: uint32(point.ContractVersion),
		PlanGeneration:      generation,
		Ordered:             ordered,
		DefinitionHash:      defHash,
		CompiledAt:          time.Now().UTC(),
		CycleDetected:       orderResult.CycleDetected,
		Warnings:            orderResult.Warnings,
	}
}

func computePlanDefinitionHash(hookPointID string, ordered []CompiledHookContribution) string {
	data, _ := json.Marshal(struct {
		HookPointID string                     `json:"hookPointId"`
		Ordered     []CompiledHookContribution `json:"ordered"`
	}{
		HookPointID: hookPointID,
		Ordered:     ordered,
	})
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:16])
}

func (plan *CompiledHookPlan) IsStale(currentGen uint64) bool {
	return plan.PlanGeneration != currentGen
}

func (plan *CompiledHookPlan) LookupByPhase(phase HookPhase) []CompiledHookContribution {
	var result []CompiledHookContribution
	for _, c := range plan.Ordered {
		if c.Phase == phase {
			result = append(result, c)
		}
	}
	return result
}

func (plan *CompiledHookPlan) FindContribution(contributionID string) (*CompiledHookContribution, bool) {
	for i := range plan.Ordered {
		if plan.Ordered[i].ContributionID == contributionID {
			return &plan.Ordered[i], true
		}
	}
	return nil, false
}
