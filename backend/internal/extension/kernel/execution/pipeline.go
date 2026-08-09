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
		result := p.classifyContextError(inv, err)
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

	if p.TimeoutCtrl == nil {
		return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:    capability.ErrorCodeInternalError,
			Message: "timeout controller not configured",
		}))), nil
	}

	acceptedAt := p.TimeoutCtrl.Now()

	tool, err := p.resolveTool(ctx, toolID)
	if err != nil {
		return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:    capability.ErrorCodeNotAvailable,
			Message: err.Error(),
		}))), nil
	}

	budget, err := p.TimeoutCtrl.ResolveBudget(ctx, acceptedAt, inv, tool)
	if err != nil {
		return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:    capability.ErrorCodeInternalError,
			Message: "resolve timeout budget failed",
		}))), nil
	}

	if budget.Expired(acceptedAt) {
		result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
		return p.finalizeTimeout(ctx, inv, result, budget), nil
	}

	timeoutCtx, timeoutCancel, err := p.TimeoutCtrl.Wrap(ctx, budget)
	if err != nil {
		return p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:    capability.ErrorCodeInternalError,
			Message: "wrap timeout context failed",
		}))), nil
	}
	defer timeoutCancel()

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	if p.InputValidator != nil {
		if err := p.InputValidator.Validate(timeoutCtx, tool, request.Input); err != nil {
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeInvalidInput,
				Message: "input validation failed",
			}))), nil
		}
	}

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	if p.AvailabilityGate != nil {
		avail := p.AvailabilityGate.Evaluate(timeoutCtx, tool, inv)
		if !avail.Executable {
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeNotAvailable,
				Message: "tool not executable",
			}))), nil
		}
	}

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	if p.ScopeGate != nil {
		if err := p.ScopeGate.Evaluate(timeoutCtx, tool, inv); err != nil {
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeScopeDenied,
				Message: err.Error(),
			}))), nil
		}
	}

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	if p.PermissionGate != nil {
		decision := p.PermissionGate.Evaluate(timeoutCtx, tool, inv)
		switch decision {
		case PermissionDeny:
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodePermissionDenied,
				Message: "permission denied",
			}))), nil
		case PermissionRequireApproval:
			if p.ApprovalGate != nil {
				approved, appErr := p.ApprovalGate.Evaluate(timeoutCtx, tool, inv, decision)
				if appErr != nil || !approved {
					if approvalTimeout(appErr) || errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
						result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
						return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
					}
					return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
						Code:    capability.ErrorCodePermissionDenied,
						Message: "approval denied",
					}))), nil
				}
			}
		}
	}

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	if p.DepthGuard != nil {
		if err := p.DepthGuard.Check(timeoutCtx, inv); err != nil {
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    "max_depth_exceeded",
				Message: err.Error(),
			}))), nil
		}
	}

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	if p.RateLimiter != nil {
		if err := p.RateLimiter.Allow(timeoutCtx, tool); err != nil {
			if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
				result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
				return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
			}
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:    capability.ErrorCodeRateLimited,
				Message: err.Error(),
			}))), nil
		}
	}

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	if p.ConcurrencyCtrl != nil {
		slot, slotErr := p.ConcurrencyCtrl.Acquire(timeoutCtx, tool, inv)
		if slotErr != nil {
			if errors.Is(slotErr, context.DeadlineExceeded) {
				result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
				return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
			}
			if errors.Is(slotErr, context.Canceled) {
				result := capability.NewToolCancelledResult(inv.InvocationID, toolID)
				return p.finalizeCancellation(timeoutCtx, inv, result), nil
			}
			result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
			return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
		}
		defer p.ConcurrencyCtrl.Release(slot)
	}

	if p.IdempotencyGuard != nil && inv.IdempotencyKey != "" {
		if budget.Expired(p.TimeoutCtrl.Now()) {
			result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
			return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
		}
		if cached, found := p.IdempotencyGuard.Check(timeoutCtx, inv.IdempotencyKey, toolID); found {
			if cached.Status == capability.ToolResultStatusCancelled {
				p.IdempotencyGuard.Remove(timeoutCtx, inv.IdempotencyKey)
			} else {
				if stream != nil {
					_ = stream.Finish(timeoutCtx, cached)
				}
				return p.finalizeCancellation(timeoutCtx, inv, cached), nil
			}
		}
	}

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	p.registerDeadlineAbortHook(timeoutCtx, inv, budget)
	p.attachRuntimeCanceller(timeoutCtx, tool, inv)

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhaseRuntime); cancelled {
		return result, nil
	}

	p.recordAuditStart(timeoutCtx, inv.InvocationID, toolID)
	startTime := p.TimeoutCtrl.Now()

	var result capability.UnifiedToolResult
	var deliveryErr error

	if p.Dispatcher != nil {
		if stream != nil && tool.ResultPolicy.Streaming.Enabled {
			result, _ = p.Dispatcher.DispatchStream(timeoutCtx, tool, inv, request.Input, stream)
		} else {
			result = p.Dispatcher.Dispatch(timeoutCtx, tool, inv, request.Input)
		}
	} else {
		result = capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:    capability.ErrorCodeInternalError,
			Message: "no dispatcher configured",
		})
	}

	result = p.checkLateResult(timeoutCtx, inv, toolID, result, budget)

	if result, cancelled := p.checkTimeoutAfterDispatch(timeoutCtx, inv, result, budget, TimeoutPhaseRuntime); cancelled {
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
			p.AuditRec.RecordCancelled(timeoutCtx, inv.InvocationID, toolID, string(getCancelReasonCode(timeoutCtx, inv)))
		}
	}

	if p.RetryCtrl != nil && result.Status != capability.ToolResultStatusSuccess && result.Status != capability.ToolResultStatusCancelled && result.Status != capability.ToolResultStatusTimedOut && streamRetryAllowed {
		if result.Error != nil {
			switch result.Error.Code {
			case capability.ErrorCodePermissionDenied, capability.ErrorCodeScopeDenied, capability.ErrorCodeInvalidInput:
			default:
				if shouldRetry, _ := p.RetryCtrl.ShouldRetry(timeoutCtx, tool, result); shouldRetry {
					for attempt := 1; attempt <= tool.ExecutionPolicy.RetryPolicy.MaxRetries; attempt++ {
						if budget.Expired(p.TimeoutCtrl.Now()) {
							result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
							return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
						}
						if err := timeoutCtx.Err(); err != nil {
							result := p.classifyContextError(inv, err)
							return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
						}
						if result, cancelled := p.runRetryAttempt(timeoutCtx, tool, inv, request, stream, attempt, result, budget); cancelled {
							return result, nil
						}
						if result.Status == capability.ToolResultStatusSuccess {
							break
						}
						if result.Status == capability.ToolResultStatusTimedOut || result.Status == capability.ToolResultStatusCancelled {
							break
						}
					}
				}
			}
		}
	}

	if p.ResultValidator != nil {
		result = p.ResultValidator.Validate(timeoutCtx, tool, inv, result)
	}

	result.DurationMS = p.TimeoutCtrl.Now().Sub(startTime).Milliseconds()

	if p.Sanitizer != nil {
		result = p.Sanitizer.Sanitize(timeoutCtx, result)
	}

	if p.IdempotencyGuard != nil && inv.IdempotencyKey != "" {
		if result.Status != capability.ToolResultStatusCancelled && result.Status != capability.ToolResultStatusTimedOut {
			cached := result.Clone()
			p.IdempotencyGuard.Record(timeoutCtx, inv.IdempotencyKey, toolID, &cached)
		}
	}

	if p.SideEffectRec != nil {
		p.SideEffectRec.Record(timeoutCtx, inv.InvocationID, toolID, result.SideEffects)
	}

	p.recordAuditFinish(timeoutCtx, inv.InvocationID, toolID, string(result.Status), p.TimeoutCtrl.Now().Sub(startTime))

	if p.MetricsRec != nil {
		p.MetricsRec.Record(timeoutCtx, tool, result, p.TimeoutCtrl.Now().Sub(startTime))
	}

	if p.CircuitBreaker != nil {
		if wasDispatched(tool, startTime, result) {
			p.CircuitBreaker.RecordResult(timeoutCtx, tool, result)
		}
	}

	return p.finalizeCancellation(timeoutCtx, inv, result), deliveryErr
}

func (p *ExecutionPipeline) runRetryAttempt(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, request ToolExecutionRequest, stream *toolStreamSession, attempt int, currentResult capability.UnifiedToolResult, budget TimeoutBudget) (capability.UnifiedToolResult, bool) {
	delay := p.RetryCtrl.Backoff(attempt)
	if delay > 0 {
		if !respectBackoff(ctx, delay) {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				result := capability.NewToolTimedOutResult(inv.InvocationID, string(request.ToolID))
				return p.finalizeTimeout(ctx, inv, result, budget), true
			}
			result := capability.NewToolCancelledResult(inv.InvocationID, string(request.ToolID))
			return p.finalizeCancellation(ctx, inv, result), true
		}
	}

	if budget.Expired(p.TimeoutCtrl.Now()) {
		result := capability.NewToolTimedOutResult(inv.InvocationID, string(request.ToolID))
		return p.finalizeTimeout(ctx, inv, result, budget), true
	}

	if err := ctx.Err(); err != nil {
		result := p.classifyContextError(inv, err)
		return p.finalizeTimeout(ctx, inv, result, budget), true
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

	result = p.checkLateResult(ctx, inv, string(request.ToolID), result, budget)

	if result, cancelled := p.checkTimeoutAfterDispatch(ctx, inv, result, budget, TimeoutPhaseRetryBackoff); cancelled {
		return result, true
	}

	return result, false
}

func (p *ExecutionPipeline) checkTimeout(ctx context.Context, inv capability.ToolInvocationContext, toolID string, budget TimeoutBudget, phase TimeoutPhase) (capability.UnifiedToolResult, bool) {
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
			return p.finalizeTimeout(ctx, inv, result, budget), true
		}
		if errors.Is(err, context.Canceled) {
			result := capability.NewToolCancelledResult(inv.InvocationID, toolID)
			return p.finalizeCancellation(ctx, inv, result), true
		}
	}
	if budget.Expired(p.TimeoutCtrl.Now()) {
		result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
		return p.finalizeTimeout(ctx, inv, result, budget), true
	}
	return capability.UnifiedToolResult{}, false
}

func (p *ExecutionPipeline) checkTimeoutAfterDispatch(ctx context.Context, inv capability.ToolInvocationContext, result capability.UnifiedToolResult, budget TimeoutBudget, phase TimeoutPhase) (capability.UnifiedToolResult, bool) {
	if result.Status == capability.ToolResultStatusCancelled {
		return result, false
	}

	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			timedOut := capability.NewToolTimedOutResult(inv.InvocationID, result.ToolID)
			return p.finalizeTimeout(ctx, inv, timedOut, budget), true
		}
	}

	return p.checkTimeout(ctx, inv, result.ToolID, budget, phase)
}

func (p *ExecutionPipeline) registerDeadlineAbortHook(ctx context.Context, inv capability.ToolInvocationContext, budget TimeoutBudget) {
	if p.CancellationCtrl == nil {
		return
	}

	context.AfterFunc(ctx, func() {
		if !errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return
		}
		deadlineReason := capability.ToolCancellationReason{
			Code: capability.CancellationReasonDeadlineExceeded,
		}
		abortCtx, abortCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer abortCancel()
		_, _ = p.CancellationCtrl.RequestRuntimeAbort(abortCtx, inv.InvocationID, deadlineReason)
	})
}

func (p *ExecutionPipeline) classifyContextError(inv capability.ToolInvocationContext, err error) capability.UnifiedToolResult {
	if errors.Is(err, context.DeadlineExceeded) {
		return capability.NewToolTimedOutResult(inv.InvocationID, "")
	}
	if errors.Is(err, context.Canceled) {
		return capability.NewToolCancelledResult(inv.InvocationID, "")
	}
	return capability.NewToolFailureResult(inv.InvocationID, "", &capability.ToolError{
		Code:    capability.ErrorCodeInternalError,
		Message: "execution context error",
	})
}

func (p *ExecutionPipeline) finalizeTimeout(ctx context.Context, inv capability.ToolInvocationContext, result capability.UnifiedToolResult, budget TimeoutBudget) capability.UnifiedToolResult {
	if result.Status != capability.ToolResultStatusTimedOut {
		result = capability.NewToolTimedOutResult(inv.InvocationID, result.ToolID)
	}
	if p.CircuitBreaker != nil && !wasDispatchedFromBudget(budget) {
		return result
	}
	return p.finalizeCancellation(ctx, inv, result)
}

func (p *ExecutionPipeline) checkLateResult(ctx context.Context, inv capability.ToolInvocationContext, toolID string, result capability.UnifiedToolResult, budget TimeoutBudget) capability.UnifiedToolResult {
	if result.Status == capability.ToolResultStatusCancelled || result.Status == capability.ToolResultStatusTimedOut {
		return result
	}
	if budget.Expired(p.TimeoutCtrl.Now()) {
		timedOut := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
		return timedOut
	}
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			timedOut := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
			return timedOut
		}
	}
	return result
}

func (p *ExecutionPipeline) checkCancellation(ctx context.Context, inv capability.ToolInvocationContext) (capability.UnifiedToolResult, bool) {
	if p.CancellationCtrl == nil {
		return capability.UnifiedToolResult{}, false
	}
	return p.CancellationCtrl.ResolveCancellation(ctx, inv)
}

func (p *ExecutionPipeline) finalizeCancellation(ctx context.Context, inv capability.ToolInvocationContext, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	if p.CancellationCtrl == nil {
		return result
	}
	return p.CancellationCtrl.Finalize(ctx, inv, result)
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

func approvalTimeout(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, ErrToolDeadlineExceeded)
}

func wasDispatched(tool capability.ToolDefinition, startTime time.Time, result capability.UnifiedToolResult) bool {
	return !startTime.IsZero() && result.Status != capability.ToolResultStatusTimedOut
}

func wasDispatchedFromBudget(budget TimeoutBudget) bool {
	return !budget.Deadline.IsZero() && budget.Source != ""
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
