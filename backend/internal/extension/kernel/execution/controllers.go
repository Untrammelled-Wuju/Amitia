package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type RetryReason string

const (
	RetryReasonRetryableRuntimeFailure RetryReason = "retryable_runtime_failure"
	RetryReasonNoBudget                RetryReason = "no_retry_budget"
	RetryReasonBudgetExhausted         RetryReason = "retry_budget_exhausted"
	RetryReasonDeadlineInsufficient    RetryReason = "deadline_budget_insufficient"
	RetryReasonNonRetryableError       RetryReason = "non_retryable_error"
	RetryReasonUnsafeSideEffect        RetryReason = "unsafe_side_effect"
	RetryReasonStreamVisible           RetryReason = "stream_visible"
	RetryReasonStreamFailure           RetryReason = "stream_failure"
	RetryReasonCancelled               RetryReason = "cancelled"
	RetryReasonTimedOut                RetryReason = "timed_out"
	RetryReasonHalfOpenProbe           RetryReason = "half_open_probe"
)

type RetryDecisionInput struct {
	Tool            capability.ToolDefinition
	Invocation      capability.ToolInvocationContext
	Result          capability.UnifiedToolResult
	RetryIndex      int
	AttemptNumber   int
	RemainingBudget time.Duration
	StreamVisible   bool
	StreamFailed    bool
	CircuitProbe    bool
}

type RetryDecisionResult struct {
	Retry             bool
	RetryIndex        int
	NextAttemptNumber int
	Delay             time.Duration
	Reason            RetryReason
}

type RetryController interface {
	Decide(ctx context.Context, input RetryDecisionInput) RetryDecisionResult
}

type DefaultRetryController struct{}

func NewRetryController() RetryController {
	return &DefaultRetryController{}
}

func (c *DefaultRetryController) Decide(ctx context.Context, input RetryDecisionInput) RetryDecisionResult {
	if ctx.Err() != nil {
		if errors.Is(ctx.Err(), context.Canceled) {
			return RetryDecisionResult{Retry: false, Reason: RetryReasonCancelled}
		}
		return RetryDecisionResult{Retry: false, Reason: RetryReasonTimedOut}
	}

	if input.Result.Status == capability.ToolResultStatusSuccess ||
		input.Result.Status == capability.ToolResultStatusCancelled ||
		input.Result.Status == capability.ToolResultStatusTimedOut {
		return RetryDecisionResult{Retry: false}
	}

	tool := input.Tool

	if input.CircuitProbe {
		return RetryDecisionResult{Retry: false, Reason: RetryReasonHalfOpenProbe}
	}

	if tool.ExecutionPolicy.RetryPolicy.MaxRetries <= 0 {
		return RetryDecisionResult{Retry: false, Reason: RetryReasonNoBudget}
	}

	if input.RetryIndex >= tool.ExecutionPolicy.RetryPolicy.MaxRetries {
		return RetryDecisionResult{Retry: false, Reason: RetryReasonBudgetExhausted}
	}

	if !isRetryableResult(input.Result) {
		return RetryDecisionResult{Retry: false, Reason: RetryReasonNonRetryableError}
	}

	if !isRetrySafe(tool) {
		return RetryDecisionResult{Retry: false, Reason: RetryReasonUnsafeSideEffect}
	}

	if input.StreamVisible {
		return RetryDecisionResult{Retry: false, Reason: RetryReasonStreamVisible}
	}
	if input.StreamFailed {
		return RetryDecisionResult{Retry: false, Reason: RetryReasonStreamFailure}
	}

	delay := ComputeRetryBackoff(tool.ExecutionPolicy.RetryPolicy, input.RetryIndex+1)

	if input.RemainingBudget > 0 && delay > input.RemainingBudget {
		return RetryDecisionResult{Retry: false, Reason: RetryReasonDeadlineInsufficient}
	}

	return RetryDecisionResult{
		Retry:             true,
		RetryIndex:        input.RetryIndex + 1,
		NextAttemptNumber: input.AttemptNumber + 1,
		Delay:             delay,
		Reason:            RetryReasonRetryableRuntimeFailure,
	}
}

func NewDepthGuard() *DepthGuard {
	return &DepthGuard{MaxDepth: 10}
}

type DepthGuard struct {
	MaxDepth int
}

func (g *DepthGuard) Check(ctx context.Context, inv capability.ToolInvocationContext) error {
	if inv.ParentID != "" {
		chain := g.getChain(ctx, inv)
		if len(chain) >= g.MaxDepth {
			return fmt.Errorf("max call depth %d exceeded", g.MaxDepth)
		}
	}
	return nil
}

func (g *DepthGuard) getChain(ctx context.Context, inv capability.ToolInvocationContext) []string {
	chain := []string{inv.InvocationID}
	if inv.ParentID != "" {
		chain = append([]string{inv.ParentID}, chain...)
	}
	return chain
}

func NewRuntimeDispatcher(adapterRegistry *capability.RuntimeAdapterRegistry, resolvers ...capability.RuntimeExecutionResolver) *RuntimeDispatcher {
	var resolver capability.RuntimeExecutionResolver
	if len(resolvers) > 0 && resolvers[0] != nil {
		resolver = resolvers[0]
	} else {
		resolver = &capability.LegacyRuntimeExecutionResolver{}
	}
	return &RuntimeDispatcher{
		adapterRegistry:   adapterRegistry,
		executionResolver: resolver,
	}
}

type RuntimeDispatcher struct {
	adapterRegistry   *capability.RuntimeAdapterRegistry
	executionResolver capability.RuntimeExecutionResolver
}

func (d *RuntimeDispatcher) SetExecutionResolver(resolver capability.RuntimeExecutionResolver) {
	if resolver != nil {
		d.executionResolver = resolver
	}
}

func (d *RuntimeDispatcher) Dispatch(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, input json.RawMessage) capability.UnifiedToolResult {
	if d.adapterRegistry == nil {
		return capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeRuntimeUnavailable,
				Message: "no adapter registry configured",
			},
		}
	}

	route, err := d.executionResolver.ResolveRuntimeExecution(ctx, tool, inv)
	if err != nil {
		return capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    mapDispatcherResolverError(err),
				Message: err.Error(),
				Details: map[string]any{
					"reason": mapDispatcherResolverReason(err),
				},
			},
		}
	}

	adapter, ok := d.adapterRegistry.ResolveRoute(route)
	if !ok {
		return capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeRuntimeUnavailable,
				Message: "no adapter for runtime: " + string(route.Binding.RuntimeType),
			},
		}
	}

	if routedAdapter, ok := adapter.(capability.RoutedRuntimeAdapter); ok {
		return routedAdapter.ExecuteRoute(ctx, route, inv, input)
	}

	return adapter.Execute(ctx, route.Binding, inv, input)
}

func (d *RuntimeDispatcher) DispatchStream(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, input json.RawMessage, emitter capability.ToolStreamEmitter) (capability.UnifiedToolResult, bool) {
	if d.adapterRegistry == nil {
		return capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeRuntimeUnavailable,
				Message: "no adapter registry configured",
			},
		}, false
	}

	route, err := d.executionResolver.ResolveRuntimeExecution(ctx, tool, inv)
	if err != nil {
		return capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    mapDispatcherResolverError(err),
				Message: err.Error(),
				Details: map[string]any{
					"reason": mapDispatcherResolverReason(err),
				},
			},
		}, false
	}

	adapter, ok := d.adapterRegistry.ResolveRoute(route)
	if !ok {
		return capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeRuntimeUnavailable,
				Message: "no adapter for runtime: " + string(route.Binding.RuntimeType),
			},
		}, false
	}

	if routedStreamingAdapter, ok := adapter.(capability.RoutedStreamingRuntimeAdapter); ok {
		result := routedStreamingAdapter.ExecuteStreamRoute(ctx, route, inv, input, emitter)
		return result, true
	}

	if streamingAdapter, ok := adapter.(capability.StreamingRuntimeAdapter); ok {
		result := streamingAdapter.ExecuteStream(ctx, route.Binding, inv, input, emitter)
		return result, true
	}

	return adapter.Execute(ctx, route.Binding, inv, input), false
}

func (d *RuntimeDispatcher) Cancel(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, reason capability.ToolCancellationReason) (bool, error) {
	if d.adapterRegistry == nil {
		return false, nil
	}

	route, err := d.executionResolver.ResolveRuntimeExecution(ctx, tool, inv)
	if err != nil {
		return false, nil
	}

	adapter, ok := d.adapterRegistry.ResolveRoute(route)
	if !ok {
		return false, nil
	}

	if routedCancellableAdapter, ok := adapter.(capability.RoutedCancellableRuntimeAdapter); ok {
		err := routedCancellableAdapter.CancelRoute(ctx, route, inv, reason)
		return true, err
	}

	cancellableAdapter, ok := adapter.(capability.CancellableRuntimeAdapter)
	if !ok {
		return false, capability.ErrRuntimeCancellationUnsupported{}
	}

	err = cancellableAdapter.Cancel(ctx, route.Binding, inv, reason)
	return true, err
}

func mapDispatcherResolverError(err error) string {
	if err == nil {
		return capability.ErrorCodeRuntimeUnavailable
	}
	if capability.IsProviderExecutionError(err) {
		switch err {
		case capability.ErrProviderExecutionProviderNotFound:
			return capability.ErrorCodeProviderNotFound
		case capability.ErrProviderExecutionInstanceNotFound:
			return capability.ErrorCodeProviderNotFound
		case capability.ErrProviderExecutionUnavailable:
			return capability.ErrorCodeProviderUnavailable
		case capability.ErrProviderExecutionBindingMismatch:
			return capability.ErrorCodeProviderUnavailable
		case capability.ErrProviderExecutionCapabilityMismatch:
			return capability.ErrorCodeProviderUnavailable
		case capability.ErrProviderExecutionPlacementMismatch:
			return capability.ErrorCodeProviderUnavailable
		case capability.ErrProviderRuntimeBindingInvalid:
			return capability.ErrorCodeRuntimeUnavailable
		}
	}
	return capability.ErrorCodeRuntimeUnavailable
}

func mapDispatcherResolverReason(err error) string {
	if err == nil {
		return "runtime_error"
	}
	if capability.IsProviderExecutionError(err) {
		return "provider_execution_error"
	}
	return "runtime_error"
}
