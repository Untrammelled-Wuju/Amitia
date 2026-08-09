package execution

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

type ExecutionPipeline struct {
	InvocationValidator *InvocationValidator
	InputValidator      *InputValidator
	AvailabilityGate    *AvailabilityGate
	ScopeGate           *ScopeGate
	PermissionGate      *PermissionGate
	ApprovalGate        *ApprovalGate
	ConcurrencyCtrl     *ConcurrencyController
	RateLimiter         *RateLimiter
	IdempotencyGuard    *IdempotencyGuard
	RetryCtrl           *RetryController
	TimeoutCtrl         *TimeoutController
	CancellationCtrl    *CancellationController
	DepthGuard          *DepthGuard
	Dispatcher          *RuntimeDispatcher
	ResultValidator     *ResultValidator
	Sanitizer           *Sanitizer
	SideEffectRec       *SideEffectRecorder
	AuditRec            *AuditRecorder
	MetricsRec          *MetricsRecorder
	CircuitBreaker      *CircuitBreakerCoordinator

	ToolResolver func(ctx context.Context, toolID string) (capability.ToolDefinition, error)
}

func (p *ExecutionPipeline) Execute(ctx context.Context, request ToolExecutionRequest) capability.UnifiedToolResult {
	result, _ := p.execute(ctx, request, nil)
	return result
}

func (p *ExecutionPipeline) ExecuteStream(ctx context.Context, request ToolExecutionRequest, sink capability.ToolStreamSink) (capability.UnifiedToolResult, error) {
	if sink == nil {
		return capability.NewToolFailureResult(request.Invocation.InvocationID, string(request.ToolID), &capability.ToolError{
			Code:     capability.ErrorCodeStreamProtocol,
			Category: capability.ToolErrorCategoryStream,
			Message:  "stream sink is required",
		}), fmt.Errorf("stream sink is nil")
	}

	toolID := string(request.ToolID)
	inv := normalizeExecutionInvocation(request.Invocation)

	policy := capability.ToolStreamingPolicy{}
	tool, err := p.resolveTool(ctx, toolID)
	if err == nil {
		policy = tool.ResultPolicy.Streaming
	}

	session := newToolStreamSession(inv.InvocationID, sink, policy, p.Sanitizer)
	result, streamErr := p.execute(ctx, request, session)

	terminalErr := session.Finish(ctx, result)
	return result, firstNonNil(streamErr, session.Err(), terminalErr)
}

func (p *ExecutionPipeline) execute(ctx context.Context, request ToolExecutionRequest, stream *toolStreamSession) (capability.UnifiedToolResult, error) {
	toolID := string(request.ToolID)
	inv := normalizeExecutionInvocation(request.Invocation)
	request.Invocation = inv

	if err := ctx.Err(); err != nil {
		result := capability.ResultFromContextError(inv.InvocationID, err)
		return p.finishEarlyResult(ctx, inv, toolID, result), nil
	}

	if p.InvocationValidator != nil {
		if err := p.InvocationValidator.Validate(ctx, request); err != nil {
			return p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeInvalidInput,
				Message: err.Error(),
			})), nil
		}
	}

	tool, err := p.resolveTool(ctx, toolID)
	if err != nil {
		return p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:    capability.ErrorCodeNotAvailable,
			Message: err.Error(),
		})), nil
	}

	if p.InputValidator != nil {
		if err := p.InputValidator.Validate(ctx, tool, request.Input); err != nil {
			return p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeInvalidInput,
				Message: "input validation failed",
			})), nil
		}
	}

	if p.AvailabilityGate != nil {
		avail := p.AvailabilityGate.Evaluate(ctx, tool, inv)
		if !avail.Executable {
			return p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeNotAvailable,
				Message: "tool not executable",
			})), nil
		}
	}

	if p.ScopeGate != nil {
		if err := p.ScopeGate.Evaluate(ctx, tool, inv); err != nil {
			return p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeScopeDenied,
				Message: err.Error(),
			})), nil
		}
	}

	if p.PermissionGate != nil {
		decision := p.PermissionGate.Evaluate(ctx, tool, inv)
		switch decision {
		case PermissionDeny:
			return p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodePermissionDenied,
				Message: "permission denied",
			})), nil
		case PermissionRequireApproval:
			if p.ApprovalGate != nil {
				approved, appErr := p.ApprovalGate.Evaluate(ctx, tool, inv, decision)
				if appErr != nil || !approved {
					return p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
						Code:    capability.ErrorCodePermissionDenied,
						Message: "approval denied",
					})), nil
				}
			}
		}
	}

	if p.DepthGuard != nil {
		if err := p.DepthGuard.Check(ctx, inv); err != nil {
			return p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    "max_depth_exceeded",
				Message: err.Error(),
			})), nil
		}
	}

	if p.RateLimiter != nil {
		if err := p.RateLimiter.Allow(ctx, tool); err != nil {
			return p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeRateLimited,
				Message: err.Error(),
			})), nil
		}
	}

	if p.ConcurrencyCtrl != nil {
		slot, slotErr := p.ConcurrencyCtrl.Acquire(ctx, tool, inv)
		if slotErr != nil {
			return p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeRateLimited,
				Message: slotErr.Error(),
			})), nil
		}
		defer p.ConcurrencyCtrl.Release(slot)
	}

	if p.IdempotencyGuard != nil && inv.IdempotencyKey != "" {
		if cached, found := p.IdempotencyGuard.Check(ctx, inv.IdempotencyKey, toolID); found {
			if stream != nil {
				_ = stream.Finish(ctx, cached)
			}
			return cached, nil
		}
	}

	if p.TimeoutCtrl != nil {
		timeoutCtx, cancel := p.TimeoutCtrl.WithTimeout(ctx, tool, inv)
		if cancel != nil {
			defer cancel()
		}
		ctx = timeoutCtx
	}

	if p.CancellationCtrl != nil {
		ctx = p.CancellationCtrl.Wrap(ctx, inv)
	}

	p.recordAuditStart(ctx, inv.InvocationID, toolID)
	startTime := time.Now()

	var result capability.UnifiedToolResult
	var deliveryErr error

	if p.Dispatcher != nil {
		if stream != nil && tool.ResultPolicy.Streaming.Enabled {
			result, _ = p.Dispatcher.DispatchStream(ctx, tool, inv, request.Input, stream)
		} else {
			result = p.Dispatcher.Dispatch(ctx, tool, inv, request.Input)
		}
	} else {
		result = capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:    capability.ErrorCodeInternalError,
			Message: "no dispatcher configured",
		})
	}

	streamRetryAllowed := stream == nil || (!stream.HasVisibleOutput() && stream.Err() == nil)

	if stream != nil && stream.Err() != nil && result.Status == capability.ToolResultStatusSuccess {
		result = capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:     capability.ErrorCodeStreamDeliveryFailed,
			Category: capability.ToolErrorCategoryStream,
			Message:  fmt.Sprintf("stream delivery failed: %s", stream.Err().Error()),
		})
	}

	if p.RetryCtrl != nil && result.Status != capability.ToolResultStatusSuccess && streamRetryAllowed {
		if result.Error != nil {
			switch result.Error.Code {
			case capability.ErrorCodePermissionDenied, capability.ErrorCodeScopeDenied, capability.ErrorCodeInvalidInput:
			default:
				if shouldRetry, _ := p.RetryCtrl.ShouldRetry(ctx, tool, result); shouldRetry {
					for attempt := 1; attempt <= tool.ExecutionPolicy.RetryPolicy.MaxRetries; attempt++ {
						time.Sleep(p.RetryCtrl.Backoff(attempt))
						if p.AuditRec != nil {
							p.AuditRec.RecordRetry(inv.InvocationID, attempt)
						}
						if stream != nil && tool.ResultPolicy.Streaming.Enabled {
							result, _ = p.Dispatcher.DispatchStream(ctx, tool, inv, request.Input, stream)
						} else {
							result = p.Dispatcher.Dispatch(ctx, tool, inv, request.Input)
						}
						if result.Status == capability.ToolResultStatusSuccess {
							break
						}
					}
				}
			}
		}
	}

	if p.ResultValidator != nil {
		result = p.ResultValidator.Validate(ctx, tool, inv, result)
	}

	result.DurationMS = time.Since(startTime).Milliseconds()

	if p.Sanitizer != nil {
		result = p.Sanitizer.Sanitize(ctx, result)
	}

	if p.IdempotencyGuard != nil && inv.IdempotencyKey != "" {
		cached := result.Clone()
		p.IdempotencyGuard.Record(ctx, inv.IdempotencyKey, toolID, &cached)
	}

	if p.SideEffectRec != nil {
		p.SideEffectRec.Record(ctx, inv.InvocationID, toolID, result.SideEffects)
	}

	p.recordAuditFinish(ctx, inv.InvocationID, toolID, string(result.Status), time.Since(startTime))

	if p.MetricsRec != nil {
		p.MetricsRec.Record(ctx, tool, result, time.Since(startTime))
	}

	if p.CircuitBreaker != nil {
		p.CircuitBreaker.RecordResult(ctx, tool, result)
	}

	return result, deliveryErr
}

func (p *ExecutionPipeline) resolveTool(ctx context.Context, toolID string) (capability.ToolDefinition, error) {
	if p.ToolResolver != nil {
		return p.ToolResolver(ctx, toolID)
	}
	return capability.ToolDefinition{}, fmt.Errorf("tool resolver not configured")
}

func (p *ExecutionPipeline) failWithAudit(ctx context.Context, inv capability.ToolInvocationContext, toolID string, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	if p.AuditRec != nil {
		p.AuditRec.RecordDenied(ctx, inv.InvocationID, toolID, result.Error.Code, result.Error.Message)
	}
	return result
}

func (p *ExecutionPipeline) finishEarlyResult(ctx context.Context, inv capability.ToolInvocationContext, toolID string, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	p.recordAuditFinish(ctx, inv.InvocationID, toolID, string(result.Status), 0)
	return result
}

func (p *ExecutionPipeline) recordAuditStart(ctx context.Context, invID, toolID string) {
	if p.AuditRec != nil {
		p.AuditRec.RecordStart(ctx, invID, toolID)
	}
}

func (p *ExecutionPipeline) recordAuditFinish(ctx context.Context, invID, toolID, status string, duration time.Duration) {
	if p.AuditRec != nil {
		p.AuditRec.RecordFinish(ctx, invID, toolID, status, duration)
	}
}

func normalizeExecutionInvocation(inv capability.ToolInvocationContext) capability.ToolInvocationContext {
	if inv.InvocationID == "" {
		inv.InvocationID = capability.NewInvocationID()
	}
	if inv.TraceID == "" {
		inv.TraceID = capability.NewTraceID()
	}
	if inv.OperationID == "" {
		inv.OperationID = capability.NewOperationID()
	}
	if inv.RootID == "" {
		if inv.ParentID != "" {
			inv.RootID = inv.ParentID
		} else {
			inv.RootID = inv.InvocationID
		}
	}
	return inv
}

func firstNonNil(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
