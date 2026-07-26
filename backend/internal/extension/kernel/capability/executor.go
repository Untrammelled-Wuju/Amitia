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

	adapter, ok := e.AdapterRegistry.Resolve(tool.Runtime)
	if !ok {
		err := &ToolError{
			Code:        ErrorCodeRuntimeUnavailable,
			Message:     "no adapter found for runtime: " + string(tool.Runtime.RuntimeType),
			UserVisible: false,
		}
		return UnifiedToolResult{
			InvocationID: invocation.InvocationID,
			Status:       ToolResultStatusFailed,
			Error:        err,
		}
	}

	result := adapter.Execute(ctx, tool.Runtime, invocation, input)
	return result
}

type RuntimeAdapterRegistry struct {
	adapters map[RuntimeType]RuntimeAdapter
}

func NewRuntimeAdapterRegistry() *RuntimeAdapterRegistry {
	return &RuntimeAdapterRegistry{
		adapters: make(map[RuntimeType]RuntimeAdapter),
	}
}

func (r *RuntimeAdapterRegistry) Register(rt RuntimeType, adapter RuntimeAdapter) {
	r.adapters[rt] = adapter
}

func (r *RuntimeAdapterRegistry) Resolve(binding RuntimeBinding) (RuntimeAdapter, bool) {
	adapter, ok := r.adapters[binding.RuntimeType]
	return adapter, ok
}
