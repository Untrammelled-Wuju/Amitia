package capability

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProviderOperation struct {
	CapabilityID CapabilityID
	Action       string
	Input        json.RawMessage
}

type ProviderOperationResult struct {
	CapabilityID       CapabilityID
	ProviderID         ProviderID
	ProviderInstanceID ProviderInstanceID
	Output             json.RawMessage
	ExecutionTarget    InvocationExecutionTarget
}

type ProviderInvocationService struct {
	capabilityService *CapabilityService
	adapterRegistry   *RuntimeAdapterRegistry
}

func NewProviderInvocationService(
	capabilityService *CapabilityService,
	adapterRegistry *RuntimeAdapterRegistry,
) *ProviderInvocationService {
	return &ProviderInvocationService{
		capabilityService: capabilityService,
		adapterRegistry:   adapterRegistry,
	}
}

func (s *ProviderInvocationService) Invoke(
	ctx context.Context,
	request ProviderInvocationRequest,
) (ProviderInvocationResult, error) {
	if s.capabilityService == nil {
		return ProviderInvocationResult{}, fmt.Errorf("provider invocation service: capability service not configured")
	}

	resolveReq := CapabilityResolutionRequest{
		CapabilityID:       request.CapabilityID,
		UserID:             request.UserID,
		PreferredPlacement: request.PreferredPlacement,
		RequiredPlacement:  request.RequiredPlacement,
		PreferredDeviceID:  request.PreferredDeviceID,
		RequiredDeviceID:   request.RequiredDeviceID,
		AllowCore:          request.AllowCore,
		AllowDevice:        request.AllowDevice,
	}

	resolution, err := s.capabilityService.Resolve(resolveReq)
	if err != nil {
		return ProviderInvocationResult{CapabilityID: request.CapabilityID}, fmt.Errorf("resolve capability %s: %w", request.CapabilityID, err)
	}

	result := ProviderInvocationResult{
		CapabilityID:       request.CapabilityID,
		ProviderID:         resolution.Provider.ID,
		ProviderInstanceID: resolution.ProviderInstance.ID,
		ExecutionTarget:    resolution.ExecutionTarget,
	}

	if !resolution.HasResult() {
		return result, fmt.Errorf("provider invocation: no executable provider for capability %s", request.CapabilityID)
	}

	if s.adapterRegistry == nil {
		return result, fmt.Errorf("provider invocation: runtime adapter registry unavailable for capability %s (provider %s, instance %s)",
			request.CapabilityID, resolution.Provider.ID, resolution.ProviderInstance.ID)
	}

	route := RuntimeExecutionRoute{
		Binding:                   resolution.Provider.Runtime,
		Placement:                 resolution.Provider.Placement,
		ProviderID:                resolution.Provider.ID,
		ProviderInstanceID:        resolution.ProviderInstance.ID,
		ProviderRuntimeInstanceID: resolution.ProviderInstance.RuntimeInstanceID,
		UserID:                    request.UserID,
		DeviceID:                  resolution.ProviderInstance.DeviceID,
		RuntimeID:                 resolution.ProviderInstance.RuntimeID,
	}

	adapter, ok := s.adapterRegistry.ResolveRoute(route)
	if !ok {
		return result, fmt.Errorf("provider invocation: no runtime adapter for provider %s (placement %s, binding %s, device %s, runtime %s)",
			resolution.Provider.ID, resolution.Provider.Placement, resolution.Provider.Runtime.RuntimeType,
			resolution.ProviderInstance.DeviceID, resolution.ProviderInstance.RuntimeID)
	}

	invocation := NewToolInvocationContext(ToolInvocationOptions{
		Source:          InvocationSourceUser,
		UserID:          string(request.UserID),
		ExecutionTarget: resolution.ExecutionTarget,
	})

	var execResult UnifiedToolResult
	if routedAdapter, ok := adapter.(RoutedRuntimeAdapter); ok {
		execResult = routedAdapter.ExecuteRoute(ctx, route, invocation, request.Input)
	} else {
		execResult = adapter.Execute(ctx, route.Binding, invocation, request.Input)
	}

	switch execResult.Status {
	case ToolResultStatusSuccess:
		result.Output = execResult.Structured
		return result, nil
	case ToolResultStatusFailed:
		return result, fmt.Errorf("provider invocation: execution failed for capability %s: %s", request.CapabilityID, execResult.Error.Message)
	case ToolResultStatusCancelled:
		return result, fmt.Errorf("provider invocation: execution cancelled for capability %s", request.CapabilityID)
	case ToolResultStatusTimedOut:
		return result, fmt.Errorf("provider invocation: execution timed out for capability %s", request.CapabilityID)
	default:
		if execResult.Error != nil {
			return result, fmt.Errorf("provider invocation: execution error for capability %s: %s", request.CapabilityID, execResult.Error.Message)
		}
		result.Output = execResult.Structured
		return result, nil
	}
}

func (s *ProviderInvocationService) InvokeLocal(
	ctx context.Context,
	op ProviderOperation,
	userID runtimeidentity.UserID,
) (ProviderOperationResult, error) {
	req := ProviderInvocationRequest{
		CapabilityID:       op.CapabilityID,
		Input:              op.Input,
		UserID:             userID,
		PreferredPlacement: ProviderPlacementCore,
		AllowCore:          true,
	}

	result, err := s.Invoke(ctx, req)
	if err != nil {
		return ProviderOperationResult{CapabilityID: op.CapabilityID}, err
	}

	return ProviderOperationResult{
		CapabilityID:       result.CapabilityID,
		ProviderID:         result.ProviderID,
		ProviderInstanceID: result.ProviderInstanceID,
		Output:             result.Output,
		ExecutionTarget:    result.ExecutionTarget,
	}, nil
}
