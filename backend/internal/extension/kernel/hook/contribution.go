package hook

import (
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type RuntimeBinding struct {
	RuntimeType      string `json:"runtimeType"`
	ModuleID         string `json:"moduleId"`
	Entry            string `json:"entry"`
	InstanceStrategy string `json:"instanceStrategy,omitempty"`
}

type DependencyRequirement struct {
	Type       string `json:"type"`
	ID         string `json:"id"`
	MinVersion string `json:"minVersion,omitempty"`
	Optional   bool   `json:"optional,omitempty"`
}

type ScopeRule struct {
	ScopeType      string `json:"scopeType"`
	CharacterID    string `json:"characterId,omitempty"`
	ConversationID string `json:"conversationId,omitempty"`
	ExtensionID    string `json:"extensionId,omitempty"`
	ModuleID       string `json:"moduleId,omitempty"`
}

type PermissionRequirement = permission.PermissionRequirement

type HookContributionDefinition struct {
	ContributionID         string                  `json:"contributionId"`
	ExtensionID            string                  `json:"extensionId"`
	ModuleID               string                  `json:"moduleId"`
	HookPointID            string                  `json:"hookPointId"`
	ContractVersion        int                     `json:"contractVersion"`
	Phase                  HookPhase               `json:"phase"`
	Entry                  string                  `json:"entry"`
	Priority               int                     `json:"priority"`
	Before                 []string                `json:"before,omitempty"`
	After                  []string                `json:"after,omitempty"`
	Timeout                time.Duration           `json:"timeout"`
	FailurePolicy          *HookFailurePolicy      `json:"failurePolicy,omitempty"`
	MutationClaims         []string                `json:"mutationClaims,omitempty"`
	PermissionRequirements []PermissionRequirement `json:"permissionRequirements,omitempty"`
	ScopeRule              ScopeRule               `json:"scopeRule"`
	DependencyRequirements []DependencyRequirement `json:"dependencyRequirements,omitempty"`
	RuntimeBinding         RuntimeBinding          `json:"runtimeBinding"`
	DefinitionHash         string                  `json:"definitionHash"`
	Enabled                bool                    `json:"enabled"`
	SystemReserved         bool                    `json:"systemReserved"`
}

const (
	MinThirdPartyPriority = -100
	MaxThirdPartyPriority = 100
	SystemPriorityFloor   = -1000
	SystemPriorityCeiling = -101
)

func (d HookContributionDefinition) Validate(point HookPointDefinition) error {
	if d.ContributionID == "" {
		return NewHookError(ErrCodeHookResultInvalid, "contribution id required")
	}
	if d.ExtensionID == "" {
		return NewHookError(ErrCodeHookResultInvalid, "extension id required")
	}
	if d.HookPointID == "" {
		return NewHookError(ErrCodeHookResultInvalid, "hook point id required")
	}
	if d.HookPointID != point.HookPointID {
		return NewHookError(ErrCodeHookResultInvalid, "hook point id mismatch")
	}
	if d.ContractVersion != point.ContractVersion {
		return NewHookError(ErrCodeContractVersionMismatch, fmt.Sprintf("contribution version %d != point version %d", d.ContractVersion, point.ContractVersion))
	}
	if !d.Phase.Valid() {
		return NewHookError(ErrCodePhaseNotSupported, "invalid phase: "+string(d.Phase))
	}
	if !point.SupportsPhase(d.Phase) {
		return NewHookError(ErrCodePhaseNotSupported, "phase not supported by point: "+string(d.Phase))
	}
	if !d.SystemReserved {
		if d.Priority < MinThirdPartyPriority || d.Priority > MaxThirdPartyPriority {
			return NewHookError(ErrCodePriorityOutOfRange, fmt.Sprintf("priority %d out of range [%d, %d]", d.Priority, MinThirdPartyPriority, MaxThirdPartyPriority))
		}
	}
	if d.Timeout > point.MaxTimeout {
		return NewHookError(ErrCodeTimeoutExceedsMax, fmt.Sprintf("timeout %s exceeds max %s", d.Timeout, point.MaxTimeout))
	}
	for _, claim := range d.MutationClaims {
		if _, ok := point.FindMutationRule(claim); !ok {
			return NewHookError(ErrCodeMutationClaimDenied, "mutation claim not in allowed mutations: "+claim)
		}
	}
	if d.FailurePolicy != nil {
		if !d.FailurePolicy.IsStricterOrEqual(point.FailurePolicy) {
			return NewHookError(ErrCodeFailurePolicyTooLoose, "contribution failure policy is looser than point policy")
		}
	}
	return nil
}

func (d HookContributionDefinition) EffectiveFailurePolicy(point HookPointDefinition) HookFailurePolicy {
	if d.FailurePolicy != nil {
		return *d.FailurePolicy
	}
	return point.FailurePolicy
}

func (d HookContributionDefinition) ToPermissionRequirements() []permission.PermissionRequirement {
	return d.PermissionRequirements
}

func (d HookContributionDefinition) ToScopeEvaluationRequest() scope.ScopeEvaluationRequest {
	return scope.ScopeEvaluationRequest{
		CharacterID:    d.ScopeRule.CharacterID,
		ConversationID: d.ScopeRule.ConversationID,
		ExtensionID:    d.ScopeRule.ExtensionID,
		ModuleID:       d.ScopeRule.ModuleID,
	}
}
