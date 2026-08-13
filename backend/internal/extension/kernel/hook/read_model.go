package hook

import (
	"context"
	"time"
)

type HookPointSummary struct {
	ID                string        `json:"id"`
	ContractVersion   int           `json:"contractVersion"`
	Description       string        `json:"description"`
	RiskLevel         RiskLevel     `json:"riskLevel"`
	ThirdPartyAllowed bool          `json:"thirdPartyAllowed"`
	SupportedPhases   []HookPhase   `json:"supportedPhases"`
	MaxHandlers       int           `json:"maxHandlers"`
	DefaultTimeout    time.Duration `json:"defaultTimeout"`
	MaxTimeout        time.Duration `json:"maxTimeout"`
}

type HookContributionSummary struct {
	ContributionID         string                  `json:"contributionId"`
	ExtensionID            string                  `json:"extensionId"`
	HookPointID            string                  `json:"hookPointId"`
	Phase                  HookPhase               `json:"phase"`
	Priority               int                     `json:"priority"`
	Enabled                bool                    `json:"enabled"`
	CircuitState           CircuitState            `json:"circuitState"`
	EffectiveState         string                  `json:"effectiveState"`
	EffectiveReason        string                  `json:"effectiveReason,omitempty"`
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

type HookInvocationSummary struct {
	InvocationID   string       `json:"invocationId"`
	ContributionID string       `json:"contributionId"`
	HookPointID    string       `json:"hookPointId"`
	Phase          HookPhase    `json:"phase"`
	Sequence       int          `json:"sequence"`
	Status         string       `json:"status"`
	Decision       HookDecision `json:"decision"`
	DurationMs     int64        `json:"durationMs"`
	StartedAt      string       `json:"startedAt"`
	ErrorCode      string       `json:"errorCode,omitempty"`
	ErrorMessage   string       `json:"errorMessage,omitempty"`
	MutationCount  int          `json:"mutationCount"`
	InputHash      string       `json:"inputHash,omitempty"`
	ResultHash     string       `json:"resultHash,omitempty"`
}

type HookMutationSummary struct {
	InvocationID string `json:"invocationId"`
	Path         string `json:"path"`
	Operation    string `json:"operation"`
	BeforeHash   string `json:"beforeHash,omitempty"`
	AfterHash    string `json:"afterHash,omitempty"`
	Applied      bool   `json:"applied"`
	Conflict     bool   `json:"conflict"`
}

type HookCircuitSummary struct {
	ContributionID   string       `json:"contributionId"`
	State            CircuitState `json:"state"`
	ConsecutiveFails int          `json:"consecutiveFails"`
	TotalFails       int64        `json:"totalFails"`
	TotalSuccess     int64        `json:"totalSuccess"`
	LastFailCode     string       `json:"lastFailCode,omitempty"`
	OpenedAt         time.Time    `json:"openedAt,omitempty"`
}

type HookReadModel struct {
	Pipeline *Pipeline
	Registry HookPointRegistry
}

func (rm *HookReadModel) circuitStateOf(contributionID string) CircuitState {
	if rm.Pipeline == nil || rm.Pipeline.Circuit == nil {
		return CircuitClosed
	}
	return rm.Pipeline.Circuit.State(contributionID)
}

func (rm *HookReadModel) GetSummary(contrib HookContributionDefinition) HookContributionSummary {
	return rm.toContributionSummary(contrib)
}

func (rm *HookReadModel) toContributionSummary(contrib HookContributionDefinition) HookContributionSummary {
	circuitState := rm.circuitStateOf(contrib.ContributionID)
	effectiveResult := rm.computeEffectiveState(contrib, circuitState)
	return HookContributionSummary{
		ContributionID:         contrib.ContributionID,
		ExtensionID:            contrib.ExtensionID,
		HookPointID:            contrib.HookPointID,
		Phase:                  contrib.Phase,
		Priority:               contrib.Priority,
		Enabled:                contrib.Enabled,
		CircuitState:           circuitState,
		EffectiveState:         string(effectiveResult.State),
		EffectiveReason:        effectiveResult.Reason,
		Before:                 contrib.Before,
		After:                  contrib.After,
		Timeout:                contrib.Timeout,
		FailurePolicy:          contrib.FailurePolicy,
		MutationClaims:         contrib.MutationClaims,
		PermissionRequirements: contrib.PermissionRequirements,
		ScopeRule:              contrib.ScopeRule,
		RuntimeBinding:         contrib.RuntimeBinding,
		DefinitionHash:         contrib.DefinitionHash,
	}
}

func (rm *HookReadModel) computeEffectiveState(contrib HookContributionDefinition, circuitState CircuitState) EffectiveStateResult {
	point, err := rm.Registry.GetPoint(nil, contrib.HookPointID)
	if err != nil {
		return ComputeEffectiveState(EffectiveStateInput{
			Contribution:    contrib,
			Point:           nil,
			CircuitState:    circuitState,
			ExtensionActive: true,
			RuntimeReady:    true,
			PermissionOK:    true,
			ScopeOK:         true,
		})
	}
	return ComputeEffectiveState(EffectiveStateInput{
		Contribution:    contrib,
		Point:           &point,
		CircuitState:    circuitState,
		ExtensionActive: true,
		RuntimeReady:    rm.Pipeline != nil && rm.Pipeline.RuntimeBridge.IsReady(nil, contrib),
		PermissionOK:    true,
		ScopeOK:         true,
		PlanExists:      rm.Pipeline != nil && rm.Pipeline.PlanCache != nil,
	})
}

func (rm *HookReadModel) GetEffectiveState(ctx context.Context, contributionID string) (EffectiveStateResult, error) {
	contrib, err := rm.Pipeline.ContribStore.Get(ctx, contributionID)
	if err != nil {
		return EffectiveStateResult{State: StateDisabled, Reason: "contribution not found"}, err
	}
	circuitState := rm.circuitStateOf(contributionID)
	return rm.computeEffectiveState(contrib, circuitState), nil
}

func (rm *HookReadModel) GetPlan(ctx context.Context, hookPointID string) (*CompiledHookPlan, error) {
	if rm.Pipeline == nil || rm.Pipeline.PlanCache == nil {
		return nil, nil
	}
	if plan, ok := rm.Pipeline.PlanCache.Get(hookPointID); ok {
		return plan, nil
	}
	plan := rm.Pipeline.RebuildPlan(ctx, hookPointID)
	return plan, nil
}

func (rm *HookReadModel) ListHookPoints(ctx context.Context) ([]HookPointSummary, error) {
	points, err := rm.Registry.ListPoints(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]HookPointSummary, 0, len(points))
	for _, p := range points {
		out = append(out, HookPointSummary{
			ID:                p.HookPointID,
			ContractVersion:   p.ContractVersion,
			Description:       p.Description,
			RiskLevel:         p.RiskLevel,
			ThirdPartyAllowed: p.ThirdPartyAllowed,
			SupportedPhases:   p.SupportedPhases,
			MaxHandlers:       p.MaxHandlers,
			DefaultTimeout:    p.DefaultTimeout,
			MaxTimeout:        p.MaxTimeout,
		})
	}
	return out, nil
}

func (rm *HookReadModel) ListContributions(ctx context.Context, extensionID string) ([]HookContributionSummary, error) {
	contribs, err := rm.Pipeline.ContribStore.ListByExtension(ctx, extensionID)
	if err != nil {
		return nil, err
	}
	out := make([]HookContributionSummary, 0, len(contribs))
	for _, c := range contribs {
		out = append(out, rm.toContributionSummary(c))
	}
	return out, nil
}

func (rm *HookReadModel) GetContribution(ctx context.Context, contributionID string) (HookContributionSummary, error) {
	contrib, err := rm.Pipeline.ContribStore.Get(ctx, contributionID)
	if err != nil {
		return HookContributionSummary{}, err
	}
	return rm.toContributionSummary(contrib), nil
}

func (rm *HookReadModel) ListContributionsByPoint(ctx context.Context, hookPointID string) ([]HookContributionSummary, error) {
	contribs, err := rm.Pipeline.ContribStore.ListByHookPoint(ctx, hookPointID)
	if err != nil {
		return nil, err
	}
	out := make([]HookContributionSummary, 0, len(contribs))
	for _, c := range contribs {
		out = append(out, rm.toContributionSummary(c))
	}
	return out, nil
}

func (rm *HookReadModel) GetCircuitStats(ctx context.Context, contributionID string) (HookCircuitSummary, error) {
	if rm.Pipeline == nil || rm.Pipeline.Circuit == nil {
		return HookCircuitSummary{ContributionID: contributionID, State: CircuitClosed}, nil
	}
	stats := rm.Pipeline.Circuit.GetStats(contributionID)
	return HookCircuitSummary{
		ContributionID:   contributionID,
		State:            stats.State,
		ConsecutiveFails: stats.ConsecutiveFails,
		TotalFails:       stats.TotalFails,
		TotalSuccess:     stats.TotalSuccess,
		LastFailCode:     stats.LastFailCode,
		OpenedAt:         stats.OpenedAt,
	}, nil
}

func (rm *HookReadModel) EnableContribution(ctx context.Context, contributionID string) error {
	return rm.Pipeline.ContribStore.SetEnabled(ctx, contributionID, true)
}

func (rm *HookReadModel) DisableContribution(ctx context.Context, contributionID string) error {
	return rm.Pipeline.ContribStore.SetEnabled(ctx, contributionID, false)
}

func (rm *HookReadModel) ResetCircuit(ctx context.Context, contributionID string) error {
	if rm.Pipeline == nil || rm.Pipeline.Circuit == nil {
		return nil
	}
	rm.Pipeline.Circuit.Reset(contributionID)
	return nil
}
