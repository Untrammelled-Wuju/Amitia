package capability

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProviderInvocationRequest struct {
	CapabilityID CapabilityID

	Input json.RawMessage

	UserID runtimeidentity.UserID

	PreferredPlacement ProviderPlacement
	RequiredPlacement  ProviderPlacement

	PreferredDeviceID runtimeidentity.DeviceID
	RequiredDeviceID  runtimeidentity.DeviceID

	PreferredProviderID ProviderID

	ScopeSnapshotID      string
	PermissionSnapshotID string

	InvocationID string
	TraceID      string

	AllowCore   bool
	AllowDevice bool
}

type ProviderInvocationResult struct {
	CapabilityID CapabilityID

	ProviderID         ProviderID
	ProviderInstanceID ProviderInstanceID

	Output json.RawMessage

	ExecutionTarget InvocationExecutionTarget
}
