package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func NewIdempotencyGuard() *IdempotencyGuard {
	return &IdempotencyGuard{
		store: make(map[string]capability.UnifiedToolResult),
	}
}

type IdempotencyGuard struct {
	store map[string]capability.UnifiedToolResult
	mu    sync.RWMutex
}

func (g *IdempotencyGuard) Check(ctx context.Context, key, toolID string) (capability.UnifiedToolResult, bool) {
	g.mu.RLock()
	defer g.mu.RUnlock()
	result, ok := g.store[key]
	return result, ok
}

func (g *IdempotencyGuard) Record(ctx context.Context, key, toolID string, result *capability.UnifiedToolResult) {
	if result == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, exists := g.store[key]; !exists {
		g.store[key] = *result
	}
}

func (g *IdempotencyGuard) Remove(_ context.Context, key string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.store, key)
}

type RetryReason string

const (
	RetryReasonRetryableRuntimeFailure RetryReason = "retryable_runtime_failure"
	RetryReasonNoBudget               RetryReason = "no_retry_budget"
	RetryReasonBudgetExhausted        RetryReason = "retry_budget_exhausted"
	RetryReasonDeadlineInsufficient   RetryReason = "deadline_budget_insufficient"
	RetryReasonNonRetryableError      RetryReason = "non_retryable_error"
	RetryReasonUnsafeSideEffect       RetryReason = "unsafe_side_effect"
	RetryReasonStreamVisible          RetryReason = "stream_visible"
	RetryReasonStreamFailure          RetryReason = "stream_failure"
	RetryReasonCancelled              RetryReason = "cancelled"
	RetryReasonTimedOut               RetryReason = "timed_out"
	RetryReasonHalfOpenProbe          RetryReason = "half_open_probe"
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

func NewRuntimeDispatcher(adapterRegistry *capability.RuntimeAdapterRegistry) *RuntimeDispatcher {
	return &RuntimeDispatcher{adapterRegistry: adapterRegistry}
}

type RuntimeDispatcher struct {
	adapterRegistry *capability.RuntimeAdapterRegistry
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

	adapter, ok := d.adapterRegistry.Resolve(tool.Runtime)
	if !ok {
		return capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeRuntimeUnavailable,
				Message: "no adapter for runtime: " + string(tool.Runtime.RuntimeType),
			},
		}
	}

	return adapter.Execute(ctx, tool.Runtime, inv, input)
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

	adapter, ok := d.adapterRegistry.Resolve(tool.Runtime)
	if !ok {
		return capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeRuntimeUnavailable,
				Message: "no adapter for runtime: " + string(tool.Runtime.RuntimeType),
			},
		}, false
	}

	streamingAdapter, ok := adapter.(capability.StreamingRuntimeAdapter)
	if !ok {
		return adapter.Execute(ctx, tool.Runtime, inv, input), false
	}

	result := streamingAdapter.ExecuteStream(ctx, tool.Runtime, inv, input, emitter)
	return result, true
}

func (d *RuntimeDispatcher) Cancel(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, reason capability.ToolCancellationReason) (bool, error) {
	if d.adapterRegistry == nil {
		return false, nil
	}

	adapter, ok := d.adapterRegistry.Resolve(tool.Runtime)
	if !ok {
		return false, nil
	}

	cancellableAdapter, ok := adapter.(capability.CancellableRuntimeAdapter)
	if !ok {
		return false, capability.ErrRuntimeCancellationUnsupported{}
	}

	err := cancellableAdapter.Cancel(ctx, tool.Runtime, inv, reason)
	return true, err
}
