package task_runtime

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type CapabilityTaskPlacementResolver struct {
	resolver capability.CapabilityResolver
}

func NewCapabilityTaskPlacementResolver(resolver capability.CapabilityResolver) *CapabilityTaskPlacementResolver {
	return &CapabilityTaskPlacementResolver{resolver: resolver}
}

func (r *CapabilityTaskPlacementResolver) ResolveTaskPlacement(
	ctx context.Context,
	request TaskPlacementRequest,
) (TaskPlacementDecision, error) {
	if r.resolver == nil {
		placement := request.Requested.Normalize()
		if placement == "" {
			placement = TaskExecutionPlacementLocal
		}
		return TaskPlacementDecision{
			Placement: placement,
			Reason:    "capability resolver unavailable, fallback to requested placement",
			Resolved:  placement == TaskExecutionPlacementLocal,
		}, nil
	}

	req := capability.CapabilityResolutionRequest{
		ExtensionID: request.ExtensionID,
		ModuleID:    request.ModuleID,
		AllowCore:   true,
		AllowDevice: false,
	}

	if request.Requested == TaskExecutionPlacementDevice {
		req.AllowDevice = true
	}

	result, err := r.resolver.Resolve(req)
	if err != nil || !result.HasResult() {
		placement := request.Requested.Normalize()
		if placement == "" {
			placement = TaskExecutionPlacementLocal
		}
		return TaskPlacementDecision{
			Placement: placement,
			Reason:    "capability provider not registered or no available provider, fallback to requested placement",
			Resolved:  placement == TaskExecutionPlacementLocal,
		}, nil
	}

	placement := mapPlacement(result.ExecutionTarget.Placement)
	target := TaskExecutionTarget{
		ProviderID:         capability.ProviderID(result.ExecutionTarget.ProviderID),
		ProviderInstanceID: capability.ProviderInstanceID(result.ExecutionTarget.ProviderInstanceID),
		UserID:             result.ExecutionTarget.UserID,
		DeviceID:           result.ExecutionTarget.DeviceID,
		RuntimeID:          result.ExecutionTarget.RuntimeID,
		RuntimeSessionID:   result.ExecutionTarget.RuntimeSessionID,
	}

	return TaskPlacementDecision{
		Placement: placement,
		Target:    target,
		Reason:    "resolved via capability provider: " + result.ExecutionTarget.ProviderID,
		Resolved:  true,
	}, nil
}

func mapPlacement(providerPlacement string) TaskExecutionPlacement {
	switch providerPlacement {
	case string(capability.ProviderPlacementDevice):
		return TaskExecutionPlacementDevice
	case string(capability.ProviderPlacementCore):
		return TaskExecutionPlacementLocal
	default:
		return TaskExecutionPlacementLocal
	}
}
