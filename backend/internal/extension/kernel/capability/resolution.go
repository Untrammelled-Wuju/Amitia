package capability

import (
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type CapabilityResolutionRequest struct {
	CapabilityID CapabilityID

	UserID runtimeidentity.UserID

	PreferredPlacement ProviderPlacement
	RequiredPlacement  ProviderPlacement

	PreferredDeviceID runtimeidentity.DeviceID
	RequiredDeviceID  runtimeidentity.DeviceID

	PreferredRuntimeID runtimeidentity.RuntimeID

	Platform runtimeidentity.Platform

	ExtensionID string
	ModuleID    string

	AllowCore   bool
	AllowDevice bool

	Metadata map[string]any
}

type CapabilityResolution struct {
	CapabilityID CapabilityID

	Provider         CapabilityProviderDefinition
	ProviderInstance CapabilityProviderInstance

	ExecutionTarget InvocationExecutionTarget

	CandidateCount int
	RejectedCount  int
	ReasonCodes    []string

	Decision       RoutingDecision
	Trace          *RoutingTrace
}

func (r CapabilityResolution) HasResult() bool {
	return r.Provider.ID != "" && r.ProviderInstance.ID != ""
}
