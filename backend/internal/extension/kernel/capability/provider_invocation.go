package capability

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/execution"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProviderInvocationRequest struct {
	CapabilityID CapabilityID

	Input json.RawMessage

	ExecContext *execution.ExecutionContext `json:"-"`

	PreferredPlacement ProviderPlacement
	RequiredPlacement  ProviderPlacement

	PreferredDeviceID runtimeidentity.DeviceID
	RequiredDeviceID  runtimeidentity.DeviceID

	PreferredProviderID ProviderID

	UserID runtimeidentity.UserID `json:"-"`

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
