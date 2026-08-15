package acquisition

import (
	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type AcquisitionContext struct {
	Source string `json:"source,omitempty"`
}

type AcquisitionRequest struct {
	CapabilityID capability.CapabilityID `json:"capabilityId"`

	UserID runtimeidentity.UserID `json:"userId"`

	ExecContext *execution.ExecutionContext `json:"-"`

	Description string `json:"description,omitempty"`
	Intent      string `json:"intent,omitempty"`

	PreferredKinds []CandidateKind `json:"preferredKinds,omitempty"`

	PreferredPlacement capability.ProviderPlacement `json:"preferredPlacement,omitempty"`
	RequiredPlacement  capability.ProviderPlacement `json:"requiredPlacement,omitempty"`

	PreferredDeviceID runtimeidentity.DeviceID `json:"preferredDeviceID,omitempty"`
	RequiredDeviceID  runtimeidentity.DeviceID `json:"requiredDeviceID,omitempty"`

	PreferredRuntimeID runtimeidentity.RuntimeID `json:"preferredRuntimeID,omitempty"`
	RequiredRuntimeID  runtimeidentity.RuntimeID `json:"requiredRuntimeID,omitempty"`

	AllowGeneratedSkill bool `json:"allowGeneratedSkill"`
	AutoInstallAllowed  bool `json:"autoInstallAllowed"`

	Context AcquisitionContext `json:"context,omitempty"`
}

func (r AcquisitionRequest) HasPlacementConstraint() bool {
	return r.RequiredPlacement != "" || r.RequiredDeviceID != "" || r.PreferredRuntimeID != ""
}

func (r AcquisitionRequest) WantsDevice() bool {
	return r.RequiredPlacement == capability.ProviderPlacementDevice ||
		r.RequiredDeviceID != ""
}
