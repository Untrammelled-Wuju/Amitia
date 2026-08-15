package acquisition

import (
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type DeploymentTarget struct {
	Placement        capability.ProviderPlacement `json:"placement"`
	UserID           runtimeidentity.UserID       `json:"userId,omitempty"`
	DeviceID         runtimeidentity.DeviceID     `json:"deviceId,omitempty"`
	RuntimeID        runtimeidentity.RuntimeID    `json:"runtimeId,omitempty"`
	RuntimeSessionID string                       `json:"runtimeSessionID,omitempty"`
}

type ApprovalRequirement struct {
	Reason    string   `json:"reason"`
	Scopes    []string `json:"scopes,omitempty"`
	RiskLevel string   `json:"riskLevel,omitempty"`
}

type PolicyDecision struct {
	Action              PolicyAction          `json:"action"`
	Reasons             []string              `json:"reasons,omitempty"`
	RequiredApprovals   []ApprovalRequirement `json:"requiredApprovals,omitempty"`
	RequiredPermissions []string              `json:"requiredPermissions,omitempty"`
}

type AcquisitionPlanStep struct {
	Order       int           `json:"order"`
	Action      string        `json:"action"`
	Description string        `json:"description,omitempty"`
	Kind        CandidateKind `json:"kind"`
	Completed   bool          `json:"completed"`
}

type AcquisitionPlan struct {
	Request             AcquisitionRequest    `json:"request"`
	Candidate           CapabilityCandidate   `json:"candidate"`
	Target              DeploymentTarget      `json:"target"`
	PolicyDecision      PolicyDecision        `json:"policyDecision"`
	Steps               []AcquisitionPlanStep `json:"steps"`
	RequiredPermissions []string              `json:"requiredPermissions,omitempty"`
	Warnings            []string              `json:"warnings,omitempty"`
}

func (p AcquisitionPlan) IsAutoAllowed() bool {
	return p.PolicyDecision.Action == ActionAllowAuto
}

func (p AcquisitionPlan) NeedsApproval() bool {
	return p.PolicyDecision.Action == ActionRequireApproval
}

func (p AcquisitionPlan) IsDenied() bool {
	return p.PolicyDecision.Action == ActionDeny
}
