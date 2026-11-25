package capability

import (
	"encoding/json"
	"strings"
	"time"

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

	SpaceID runtimeidentity.SpaceID `json:"-"`

	Trace          execution.InvocationTrace
	SideEffect     execution.SideEffectClass
	IdempotencyKey string
	Deadline       time.Time

	AllowCore   bool
	AllowDevice bool
}

func (r ProviderInvocationRequest) InvocationTrace() execution.InvocationTrace {
	trace := r.Trace
	trace.RootExecutionID = strings.TrimSpace(trace.RootExecutionID)
	trace.TraceID = strings.TrimSpace(trace.TraceID)
	trace.InvocationID = strings.TrimSpace(trace.InvocationID)
	trace.ParentInvocationID = strings.TrimSpace(trace.ParentInvocationID)
	return trace
}

func (r ProviderInvocationRequest) InvocationContract() (execution.InvocationContract, error) {
	return execution.NewInvocationContract(r.InvocationTrace(), string(r.CapabilityID), r.SideEffect, r.IdempotencyKey)
}

type ProviderInvocationResult struct {
	CapabilityID CapabilityID

	ProviderID         ProviderID
	ProviderInstanceID ProviderInstanceID

	Output json.RawMessage

	ExecutionTarget InvocationExecutionTarget
}
