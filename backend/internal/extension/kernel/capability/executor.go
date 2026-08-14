package capability

import (
	"context"
	"encoding/json"
)

type ToolExecutor interface {
	Execute(
		ctx context.Context,
		tool ToolDefinition,
		invocation ToolInvocationContext,
		input json.RawMessage,
	) UnifiedToolResult
}

type DefaultToolExecutor struct {
	Registry          *ToolRegistry
	AvailabilityEval  AvailabilityEvaluator
	AdapterRegistry   *RuntimeAdapterRegistry
	ExecutionResolver RuntimeExecutionResolver
}

func (e *DefaultToolExecutor) Execute(
	ctx context.Context,
	tool ToolDefinition,
	invocation ToolInvocationContext,
	input json.RawMessage,
) UnifiedToolResult {
	if e.AvailabilityEval != nil {
		avail := e.AvailabilityEval.Evaluate(ctx, tool, invocation)
		if !avail.Executable {
			err := &ToolError{
				Code:        ErrorCodeNotAvailable,
				Message:     "tool is not executable",
				UserVisible: true,
				Details:     map[string]any{"reasons": avail.Reasons},
			}
			return UnifiedToolResult{
				InvocationID: invocation.InvocationID,
				Status:       ToolResultStatusFailed,
				Error:        err,
			}
		}
	}

	if e.AdapterRegistry == nil {
		err := &ToolError{
			Code:        ErrorCodeRuntimeUnavailable,
			Message:     "no adapter registry configured",
			UserVisible: false,
		}
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error:        err,
		}
	}

	resolver := e.ExecutionResolver
	if resolver == nil {
		resolver = &LegacyRuntimeExecutionResolver{}
	}

	route, err := resolver.ResolveRuntimeExecution(ctx, tool, invocation)
	if err != nil {
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error: &ToolError{
				Code:    mapResolverErrorToErrorCode(err),
				Message: err.Error(),
				Details: map[string]any{
					"reason": mapResolverErrorToReason(err),
				},
			},
		}
	}

	adapter, ok := e.AdapterRegistry.ResolveRoute(route)
	if !ok {
		err := &ToolError{
			Code:        ErrorCodeRuntimeUnavailable,
			Message:     "no adapter found for runtime: " + string(route.Binding.RuntimeType),
			UserVisible: false,
		}
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error:        err,
		}
	}

	if routedAdapter, ok := adapter.(RoutedRuntimeAdapter); ok {
		return routedAdapter.ExecuteRoute(ctx, route, invocation, input)
	}

	return adapter.Execute(ctx, route.Binding, invocation, input)
}

func mapResolverErrorToErrorCode(err error) string {
	switch {
	case IsProviderExecutionError(err):
		return ErrorCodeRuntimeUnavailable
	default:
		return ErrorCodeRuntimeUnavailable
	}
}

func mapResolverErrorToReason(err error) string {
	switch {
	case IsProviderExecutionError(err):
		return "provider_execution_error"
	default:
		return "runtime_error"
	}
}

func IsProviderExecutionError(err error) bool {
	if err == nil {
		return false
	}
	switch err.(type) {
	case interface{ IsProviderExecutionError() bool }:
		return true
	}
	return false
}
