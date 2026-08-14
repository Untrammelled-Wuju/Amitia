package capability

import (
	"context"
	"encoding/json"
)

type DeviceRuntimeInvocationRequest struct {
	Route      RuntimeExecutionRoute
	Binding    RuntimeBinding
	Invocation ToolInvocationContext
	Input      json.RawMessage
}

type DeviceRuntimeInvocationPort interface {
	Execute(
		ctx context.Context,
		request DeviceRuntimeInvocationRequest,
	) UnifiedToolResult

	Health(
		ctx context.Context,
		route RuntimeExecutionRoute,
	) HealthStatus

	Cancel(
		ctx context.Context,
		request DeviceRuntimeInvocationRequest,
		reason ToolCancellationReason,
	) error
}

type DeviceRuntimeAdapter struct {
	port DeviceRuntimeInvocationPort
}

func NewDeviceRuntimeAdapter(port DeviceRuntimeInvocationPort) *DeviceRuntimeAdapter {
	return &DeviceRuntimeAdapter{port: port}
}

func (a *DeviceRuntimeAdapter) Supports(binding RuntimeBinding) bool {
	return binding.RuntimeType != ""
}

func (a *DeviceRuntimeAdapter) Execute(ctx context.Context, binding RuntimeBinding, invocation ToolInvocationContext, input json.RawMessage) UnifiedToolResult {
	route := RuntimeExecutionRoute{
		Binding:      binding,
		RemoteDevice: true,
	}
	return a.ExecuteRoute(ctx, route, invocation, input)
}

func (a *DeviceRuntimeAdapter) Health(ctx context.Context, binding RuntimeBinding) HealthStatus {
	if a.port == nil {
		return HealthUnknown
	}
	return HealthUnknown
}

func (a *DeviceRuntimeAdapter) ExecuteRoute(ctx context.Context, route RuntimeExecutionRoute, invocation ToolInvocationContext, input json.RawMessage) UnifiedToolResult {
	if a.port == nil {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:      ErrorCodeRuntimeUnavailable,
				Message:   "device runtime port is not configured",
				Retryable: true,
			},
		}
	}

	if route.Placement != ProviderPlacementDevice {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:      ErrorCodeRuntimeUnavailable,
				Message:   "device runtime adapter requires device placement",
				Retryable: false,
			},
		}
	}

	request := DeviceRuntimeInvocationRequest{
		Route:      route,
		Binding:    route.Binding,
		Invocation: invocation,
		Input:      input,
	}
	return a.port.Execute(ctx, request)
}

func (a *DeviceRuntimeAdapter) HealthRoute(ctx context.Context, route RuntimeExecutionRoute) HealthStatus {
	if a.port == nil {
		return HealthUnhealthy
	}
	return a.port.Health(ctx, route)
}

func (a *DeviceRuntimeAdapter) CancelRoute(ctx context.Context, route RuntimeExecutionRoute, invocation ToolInvocationContext, reason ToolCancellationReason) error {
	if a.port == nil {
		return ErrRuntimeCancellationUnsupported{}
	}
	request := DeviceRuntimeInvocationRequest{
		Route:      route,
		Binding:    route.Binding,
		Invocation: invocation,
	}
	return a.port.Cancel(ctx, request, reason)
}
