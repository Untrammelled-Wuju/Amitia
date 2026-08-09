package execution

import (
	"context"
	"errors"
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

	var cleanup func()
	if p.CancellationCtrl != nil {
		ctx, cleanup, _ = p.CancellationCtrl.Register(ctx, inv)
		if cleanup != nil {
			defer cleanup()
		}
	}

	session := newToolStreamSession(inv.InvocationID, sink, policy, p.Sanitizer, p.CancellationCtrl)
	result, streamErr := p.execute(ctx, request, session)

	terminalErr := session.Finish(ctx, result)
	return result, firstNonNil(streamErr, session.Err(), terminalErr)
}

func (p *ExecutionPipeline) execute(ctx context.Context, request ToolExecutionRequest, stream *toolStreamSession) (capability.UnifiedToolResult, error) {
	toolID := string(request.ToolID)
	inv := normalizeExecutionInvocation(request.Invocation)
	request.Invocation = inv

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, nil
	}

	if err := ctx.Err(); err != nil {
		result := capability.ResultFromContextError(inv.InvocationID, err)
		return p.finalizeCancellation(ctx, inv, result), nil
	}

	if p.InvocationValidator != nil {
		if err := p.InvocationValidator.Validate(ctx, request); err != nil {
			return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeInvalidInput,
				Message: err.Error(),
			}))), nil
		}
	}

	var cancelCleanup func()
	if p.CancellationCtrl != nil {
		ctx, cancelCleanup, _ = p.CancellationCtrl.Register(ctx, inv)
		if cancelCleanup != nil {
			defer cancelCleanup()
		}
	}

	tool, err := p.resolveTool(ctx, toolID)
	if err != nil {
		return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:    capability.ErrorCodeNotAvailable,
			Message: err.Error(),
		}))), nil
	}

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, nil
	}

	if p.InputValidator != nil {
		if err := p.InputValidator.Validate(ctx, tool, request.Input); err != nil {
			return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeInvalidInput,
				Message: "input validation failed",
			}))), nil
		}
	}

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, nil
	}

	if p.AvailabilityGate != nil {
		avail := p.AvailabilityGate.Evaluate(ctx, tool, inv)
		if !avail.Executable {
			return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeNotAvailable,
				Message: "tool not executable",
			}))), nil
		}
	}

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, nil
	}

	if p.ScopeGate != nil {
		if err := p.ScopeGate.Evaluate(ctx, tool, inv); err != nil {
			return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeScopeDenied,
				Message: err.Error(),
			}))), nil
		}
	}

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, nil
	}

	if p.PermissionGate != nil {
		decision := p.PermissionGate.Evaluate(ctx, tool, inv)
		switch decision {
		case PermissionDeny:
			return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodePermissionDenied,
				Message: "permission denied",
			}))), nil
		case PermissionRequireApproval:
			if p.ApprovalGate != nil {
				approved, appErr := p.ApprovalGate.Evaluate(ctx, tool, inv, decision)
				if appErr != nil || !approved {
					return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
						Code:    capability.ErrorCodePermissionDenied,
						Message: "approval denied",
					}))), nil
				}
			}
		}
	}

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, nil
	}

	if p.DepthGuard != nil {
		if err := p.DepthGuard.Check(ctx, inv); err != nil {
			return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    "max_depth_exceeded",
				Message: err.Error(),
			}))), nil
		}
	}

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, nil
	}

	if p.RateLimiter != nil {
		if err := p.RateLimiter.Allow(ctx, tool); err != nil {
			return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeRateLimited,
				Message: err.Error(),
			}))), nil
		}
	}

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, nil
	}

	if p.ConcurrencyCtrl != nil {
		slot, slotErr := p.ConcurrencyCtrl.Acquire(ctx, tool, inv)
		if slotErr != nil {
			if errors.Is(slotErr, context.Canceled) {
				result := capability.NewToolCancelledResult(inv.InvocationID, toolID)
				return p.finalizeCancellation(ctx, inv, result), nil
			}
			return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeRateLimited,
				Message: slotErr.Error(),
			}))), nil
		}
		defer p.ConcurrencyCtrl.Release(slot)
	}

	if p.IdempotencyGuard != nil && inv.IdempotencyKey != "" {
		if cached, found := p.IdempotencyGuard.Check(ctx, inv.IdempotencyKey, toolID); found {
			if cached.Status == capability.ToolResultStatusCancelled {
				p.IdempotencyGuard.Remove(ctx, inv.IdempotencyKey)
			} else {
				if stream != nil {
					_ = stream.Finish(ctx, cached)
				}
				return p.finalizeCancellation(ctx, inv, cached), nil
			}
		}
	}

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, nil
	}

	if p.TimeoutCtrl != nil {
		timeoutCtx, cancel := p.TimeoutCtrl.WithTimeout(ctx, tool, inv)
		if cancel != nil {
			defer cancel()
		}
		ctx = timeoutCtx
	}

	p.attachRuntimeCanceller(ctx, tool, inv)

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, nil
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

	if result, cancelled := p.checkCancellationAfterDispatch(ctx, inv, result); cancelled {
		return result, nil
	}

	streamRetryAllowed := stream == nil || (!stream.HasVisibleOutput() && stream.Err() == nil)

	if stream != nil && stream.Err() != nil && result.Status == capability.ToolResultStatusSuccess {
		result = capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:     capability.ErrorCodeStreamDeliveryFailed,
			Category: capability.ToolErrorCategoryStream,
			Message:  fmt.Sprintf("stream delivery failed: %s", stream.Err().Error()),
		})
	}

	if p.AuditRec != nil {
		if result.Status == capability.ToolResultStatusCancelled {
			p.AuditRec.RecordCancelled(ctx, inv.InvocationID, toolID, string(getCancelReasonCode(ctx, inv)))
		}
	}

	if p.RetryCtrl != nil && result.Status != capability.ToolResultStatusSuccess && result.Status != capability.ToolResultStatusCancelled && streamRetryAllowed {
		if result.Error != nil {
			switch result.Error.Code {
			case capability.ErrorCodePermissionDenied, capability.ErrorCodeScopeDenied, capability.ErrorCodeInvalidInput:
			default:
				if shouldRetry, _ := p.RetryCtrl.ShouldRetry(ctx, tool, result); shouldRetry {
					for attempt := 1; attempt <= tool.ExecutionPolicy.RetryPolicy.MaxRetries; attempt++ {
						if result, cancelled := p.runRetryAttempt(ctx, tool, inv, request, stream, attempt, result); cancelled {
							return result, nil
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
		if result.Status != capability.ToolResultStatusCancelled {
			cached := result.Clone()
			p.IdempotencyGuard.Record(ctx, inv.IdempotencyKey, toolID, &cached)
		}
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

	return p.finalizeCancellation(ctx, inv, result), deliveryErr
}

func (p *ExecutionPipeline) runRetryAttempt(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, request ToolExecutionRequest, stream *toolStreamSession, attempt int, currentResult capability.UnifiedToolResult) (capability.UnifiedToolResult, bool) {
	delay := p.RetryCtrl.Backoff(attempt)
	if delay > 0 {
		if !respectBackoff(ctx, delay) {
			result := capability.NewToolCancelledResult(inv.InvocationID, string(request.ToolID))
			return p.finalizeCancellation(ctx, inv, result), true
		}
	}

	if result, cancelled := p.checkCancellation(ctx, inv); cancelled {
		return result, true
	}

	if p.AuditRec != nil {
		p.AuditRec.RecordRetry(inv.InvocationID, attempt)
	}

	var result capability.UnifiedToolResult
	if stream != nil && tool.ResultPolicy.Streaming.Enabled {
		result, _ = p.Dispatcher.DispatchStream(ctx, tool, inv, request.Input, stream)
	} else {
		result = p.Dispatcher.Dispatch(ctx, tool, inv, request.Input)
	}

	if result, cancelled := p.checkCancellationAfterDispatch(ctx, inv, result); cancelled {
		return result, true
	}

	return result, false
}

func (p *ExecutionPipeline) attachRuntimeCanceller(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) {
	if p.Dispatcher == nil || p.CancellationCtrl == nil {
		return
	}

	toolCopy := tool
	invCopy := inv

	_ = p.CancellationCtrl.AttachRuntimeCanceller(inv.InvocationID, func() {
		reason := capability.ToolCancellationReason{Code: capability.CancellationReasonRuntimeRequested}
		_, _ = p.Dispatcher.Cancel(ctx, toolCopy, invCopy, reason)
	})
}

func (p *ExecutionPipeline) checkCancellation(ctx context.Context, inv capability.ToolInvocationContext) (capability.UnifiedToolResult, bool) {
	if p.CancellationCtrl == nil {
		return capability.UnifiedToolResult{}, false
	}
	return p.CancellationCtrl.ResolveCancellation(ctx, inv)
}

func (p *ExecutionPipeline) checkCancellationAfterDispatch(ctx context.Context, inv capability.ToolInvocationContext, result capability.UnifiedToolResult) (capability.UnifiedToolResult, bool) {
	if p.CancellationCtrl == nil {
		return capability.UnifiedToolResult{}, false
	}

	if result.Status == capability.ToolResultStatusCancelled {
		return result, false
	}

	return p.CancellationCtrl.ResolveCancellation(ctx, inv)
}

func (p *ExecutionPipeline) finalizeCancellation(ctx context.Context, inv capability.ToolInvocationContext, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	if p.CancellationCtrl == nil {
		return result
	}
	return p.CancellationCtrl.Finalize(ctx, inv, result)
}

func getCancelReasonCode(ctx context.Context, inv capability.ToolInvocationContext) string {
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.Canceled) {
			causeErr := context.Cause(ctx)
			var cancelledErr *invocationCancelledError
			if errors.As(causeErr, &cancelledErr) {
				return string(cancelledErr.reason.Code)
			}
			return string(capability.CancellationReasonCallerContext)
		}
	}
	return string(capability.CancellationReasonCallerContext)
}

func respectBackoff(ctx context.Context, delay time.Duration) bool {
	if delay <= 0 {
		return true
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
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

func (p *ExecutionPipeline) CancelInvocation(ctx context.Context, invocationID string, reason capability.ToolCancellationReason) CancellationResult {
	if p.CancellationCtrl == nil {
		return CancellationResult{Requested: false, TargetInvocationID: invocationID}
	}
	return p.CancellationCtrl.CancelInvocation(ctx, invocationID, reason)
}

func (p *ExecutionPipeline) CancelRoot(ctx context.Context, rootID string, reason capability.ToolCancellationReason) CancellationResult {
	if p.CancellationCtrl == nil {
		return CancellationResult{Requested: false}
	}
	return p.CancellationCtrl.CancelRoot(ctx, rootID, reason)
}

func (p *ExecutionPipeline) CancelExternalCall(ctx context.Context, scope capability.CancellationExternalScope, externalCallID string, reason capability.ToolCancellationReason) CancellationResult {
	if p.CancellationCtrl == nil {
		return CancellationResult{Requested: false}
	}
	return p.CancellationCtrl.CancelExternalCall(ctx, scope, externalCallID, reason)
}
