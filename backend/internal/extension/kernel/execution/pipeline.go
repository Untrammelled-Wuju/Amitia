package execution

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/observability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
	"github.com/u-ai/backend/internal/extension/kernel/secret"
)

type ExecutionPipeline struct {
	InvocationValidator *InvocationValidator
	InputValidator      *InputValidator
	AvailabilityGate    *AvailabilityGate
	ScopeGate           *ScopeGate
	PermissionGate      *PermissionGate
	ApprovalGate        *ApprovalGate
	ResourceQuotaCtrl   *ResourceQuotaController
	RateLimiter         *RateLimiter
	IdempotencyGuard    *IdempotencyGuard
	ConcurrencyCtrl     *ConcurrencyController
	TimeoutCtrl         *TimeoutController
	CancellationCtrl    *CancellationController
	DepthGuard          *DepthGuard
	Dispatcher          *RuntimeDispatcher
	ResultValidator     *ResultValidator
	Sanitizer           *Sanitizer
	SideEffectRec       *SideEffectRecorder
	AuditSink           observability.ExecutionRecorder
	MetricsRec          *MetricsRecorder
	CircuitBreaker      *CircuitBreakerCoordinator
	RetryCtrl           RetryController
	circuitClassifier   CircuitResultClassifier

	ToolResolver            func(ctx context.Context, toolID string) (capability.ToolDefinition, error)
	ScopeStore              scope.ScopeStore
	PermissionSnapshotStore permission.PermissionSnapshotStore
	SecretBroker            *secret.Broker
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

	if p.AuditSink != nil {
		inputJSON, _ := json.Marshal(request.Input)
		_ = p.AuditSink.BeginInvocation(ctx, inv, toolID, inputJSON, p.TimeoutCtrl.Now())
	}

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
				Code:     capability.ErrorCodeScopeDenied,
				Category: capability.ToolErrorCategoryPermission,
				Message:  err.Error(),
			}))), nil
		}
	}

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	if p.ScopeGate != nil {
		if scopeManager, ok := p.ScopeGate.ScopeManager.(scope.ScopeManager); ok && scopeManager != nil {
			scopeSnap, snapErr := p.createAndStoreScopeSnapshot(timeoutCtx, scopeManager, inv, tool)
			if snapErr != nil {
				return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
					Code:     capability.ErrorCodeScopeDenied,
					Category: capability.ToolErrorCategoryPermission,
					Message:  fmt.Sprintf("scope snapshot failed: %s", snapErr.Error()),
				}))), nil
			}
			inv.ScopeSnapshotID = scopeSnap.SnapshotID
		}
	}

	if inv.ParentID != "" && inv.RootID != inv.InvocationID {
		if err := p.checkChildScopeEscalation(timeoutCtx, inv); err != nil {
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:     capability.ErrorCodeScopeDenied,
				Category: capability.ToolErrorCategoryPermission,
				Message:  err.Error(),
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
				Code:     capability.ErrorCodePermissionDenied,
				Category: capability.ToolErrorCategoryPermission,
				Message:  "permission denied",
			}))), nil
		case PermissionRequireApproval:
			approvalResult := p.handleApproval(timeoutCtx, tool, inv, decision, budget)
			if approvalResult != nil {
				return *approvalResult, nil
			}
		}
	}

	if inv.PermissionSnapshotID == "" && p.PermissionGate != nil {
		grantedPerms := collectGrantedPermissionIDs(tool)
		if len(grantedPerms) > 0 || len(tool.Permissions) > 0 {
			p.createPermissionSnapshot(timeoutCtx, inv, grantedPerms, nil, inv.ScopeSnapshotID)
		}
	}

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	if p.ResourceQuotaCtrl != nil {
		quotaDec := p.ResourceQuotaCtrl.Evaluate(timeoutCtx, tool, inv)
		if quotaDec.Blocked {
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:      quotaDec.ErrorCode,
				Category:  capability.ToolErrorCategoryResource,
				Message:   quotaDec.Error.Error(),
				Retryable: false,
			}))), nil
		}
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

	var idempotencyReservation IdempotencyReservation
	if p.IdempotencyGuard != nil && inv.IdempotencyKey != "" {
		if budget.Expired(p.TimeoutCtrl.Now()) {
			result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
			return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
		}
		identity := BuildIdempotencyIdentity(toolID, inv, inv.IdempotencyKey)
		fingerprint := BuildRequestFingerprintSHA(request.Input, tool.ToolVersion, inv.Generation)
		res, hit, err := p.IdempotencyGuard.Begin(timeoutCtx, identity, fingerprint)
		idempotencyReservation = res
		if err != nil {
			code := capability.ErrorCodeIdempotencyConflict
			if errors.Is(err, ErrIdempotencyIndeterminate) {
				code = capability.ErrorCodeIdempotencyIndeterminate
			}
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:     code,
				Category: capability.ToolErrorCategoryConflict,
				Message:  err.Error(),
			}))), nil
		}
		if hit {
			cached := capability.UnifiedToolResult{}
			if len(idempotencyReservation.PriorWorkResultJSON) > 0 {
				_ = json.Unmarshal(idempotencyReservation.PriorWorkResultJSON, &cached)
			}
			if stream != nil {
				_ = stream.Finish(timeoutCtx, cached)
			}
			return p.finalizeCancellation(timeoutCtx, inv, cached), nil
		}
	}

	if result, cancelled := p.checkTimeout(timeoutCtx, inv, toolID, budget, TimeoutPhasePreDispatch); cancelled {
		return result, nil
	}

	var rateAdmission RateLimitAdmission
	if p.RateLimiter != nil {
		admission, admitErr := p.RateLimiter.Admit(timeoutCtx, tool, inv)
		if admitErr != nil {
			if errors.Is(admitErr, context.DeadlineExceeded) || errors.Is(admitErr, context.Canceled) {
				if errors.Is(admitErr, context.DeadlineExceeded) {
					result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
					return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
				}
				result := capability.NewToolCancelledResult(inv.InvocationID, toolID)
				return p.finalizeCancellation(timeoutCtx, inv, result), nil
			}
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:     capability.ErrorCodeRateLimitPolicyInvalid,
				Category: capability.ToolErrorCategoryResource,
				Message:  admitErr.Error(),
			}))), nil
		}
		rateAdmission = admission
		if admission.Decision == RateLimitRejected {
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:     capability.ErrorCodeRateLimited,
				Category: capability.ToolErrorCategoryRateLimit,
				Message:  "rate limited",
				Details: map[string]any{
					"retryAfterMs": admission.RetryAfter.Milliseconds(),
					"reason":       admission.Reason,
				},
			}))), nil
		}
		if admission.Decision == RateLimitBackpressureRejected {
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:     capability.ErrorCodeBackpressureRejected,
				Category: capability.ToolErrorCategoryRateLimit,
				Message:  "backpressure rejected: " + admission.Reason,
				Details: map[string]any{
					"retryAfterMs": admission.RetryAfter.Milliseconds(),
					"reason":       admission.Reason,
				},
			}))), nil
		}
		_ = rateAdmission
	}

	var concurrencyLease *ConcurrencyLease
	if p.ConcurrencyCtrl != nil {
		lease, leaseErr := p.ConcurrencyCtrl.Acquire(timeoutCtx, tool, inv)
		if leaseErr != nil {
			if errors.Is(leaseErr, context.DeadlineExceeded) {
				result := capability.NewToolTimedOutResult(inv.InvocationID, toolID)
				return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
			}
			if errors.Is(leaseErr, context.Canceled) {
				result := capability.NewToolCancelledResult(inv.InvocationID, toolID)
				return p.finalizeCancellation(timeoutCtx, inv, result), nil
			}
			result := capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:     capability.ErrorCodeConcurrencyPolicyInvalid,
				Category: capability.ToolErrorCategoryResource,
				Message:  "invalid concurrency policy",
			})
			return p.finalizeCancellation(timeoutCtx, inv, result), nil
		}
		concurrencyLease = lease
		defer concurrencyLease.Release()
	}

	if inv.ScopeSnapshotID != "" || inv.PermissionSnapshotID != "" {
		if err := p.revalidateSnapshots(timeoutCtx, inv); err != nil {
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:     capability.ErrorCodePermissionDenied,
				Category: capability.ToolErrorCategoryPermission,
				Message:  err.Error(),
			}))), nil
		}
	}

	var circuitPermit CircuitPermit
	circuitCompleted := false
	dispatched := false
	if p.CircuitBreaker != nil {
		circuitPermit = p.CircuitBreaker.Acquire(timeoutCtx, tool)
		if !circuitPermit.Allowed {
			return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
				Code:     capability.ErrorCodeCircuitOpen,
				Category: capability.ToolErrorCategoryAvailability,
				Message:  "runtime temporarily unavailable",
			}))), nil
		}
	}
	defer func() {
		if p.CircuitBreaker != nil && circuitPermit.Allowed && !circuitCompleted {
			p.CircuitBreaker.Complete(circuitPermit, CircuitOutcomeNeutral)
		}
	}()

	p.registerDeadlineAbortHook(timeoutCtx, inv, budget)
	p.attachRuntimeCanceller(timeoutCtx, tool, inv)

	p.recordAuditStart(timeoutCtx, inv.InvocationID, toolID)
	if secretErr := p.issueSecretLeases(timeoutCtx, tool, inv); secretErr != nil {
		return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, secretErr))), nil
	}
	startTime := p.TimeoutCtrl.Now()

	attemptNumber := 1
	retryCount := 0

	result := p.dispatchAttempt(timeoutCtx, tool, inv, request, stream, attemptNumber, budget, &dispatched)
	result = p.finalizeAttemptResult(timeoutCtx, inv, toolID, result, stream, budget, startTime, attemptNumber, retryCount)

	for p.RetryCtrl != nil {
		if result.Status == capability.ToolResultStatusSuccess ||
			result.Status == capability.ToolResultStatusCancelled ||
			result.Status == capability.ToolResultStatusTimedOut {
			break
		}

		streamVisible := stream != nil && stream.HasVisibleOutput()
		streamFailed := stream != nil && stream.Err() != nil

		decision := p.RetryCtrl.Decide(timeoutCtx, RetryDecisionInput{
			Tool:            tool,
			Invocation:      inv,
			Result:          result,
			RetryIndex:      retryCount,
			AttemptNumber:   attemptNumber,
			RemainingBudget: budget.Remaining(p.TimeoutCtrl.Now()),
			StreamVisible:   streamVisible,
			StreamFailed:    streamFailed,
			CircuitProbe:    circuitPermit.Probe,
		})

		if !decision.Retry {
			break
		}

		if p.AuditSink != nil {
			_ = p.AuditSink.OnRetryScheduled(timeoutCtx, inv.InvocationID, attemptNumber, attemptNumber+1, retryCount+1, decision.Delay.Milliseconds(), string(decision.Reason))
		}

		if interrupted := RespectBackoff(timeoutCtx, decision.Delay); interrupted {
			if errors.Is(timeoutCtx.Err(), context.DeadlineExceeded) {
				result = capability.NewToolTimedOutResult(inv.InvocationID, toolID)
				return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
			}
			result = capability.NewToolCancelledResult(inv.InvocationID, toolID)
			return p.finalizeCancellation(timeoutCtx, inv, result), nil
		}

		if budget.Expired(p.TimeoutCtrl.Now()) {
			result = capability.NewToolTimedOutResult(inv.InvocationID, toolID)
			return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
		}
		if err := timeoutCtx.Err(); err != nil {
			result = p.classifyContextError(inv, err)
			return p.finalizeTimeout(timeoutCtx, inv, result, budget), nil
		}

		if inv.ScopeSnapshotID != "" || inv.PermissionSnapshotID != "" {
			if err := p.revalidateSnapshots(timeoutCtx, inv); err != nil {
				return p.finalizeCancellation(timeoutCtx, inv, p.failWithAudit(timeoutCtx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
					Code:     capability.ErrorCodePermissionDenied,
					Category: capability.ToolErrorCategoryPermission,
					Message:  err.Error(),
				}))), nil
			}
		}

		attemptNumber = decision.NextAttemptNumber
		retryCount++

		result = p.dispatchAttempt(timeoutCtx, tool, inv, request, stream, attemptNumber, budget, &dispatched)
		result = p.finalizeAttemptResult(timeoutCtx, inv, toolID, result, stream, budget, startTime, attemptNumber, retryCount)
	}

	if p.ResultValidator != nil {
		result = p.ResultValidator.Validate(timeoutCtx, tool, inv, result)
	}

	if p.CircuitBreaker != nil && circuitPermit.Allowed {
		outcome := CircuitOutcomeNeutral
		if p.circuitClassifier != nil {
			outcome = p.circuitClassifier.Classify(result, dispatched)
		} else {
			outcome = NewCircuitResultClassifier().Classify(result, dispatched)
		}
		p.CircuitBreaker.Complete(circuitPermit, outcome)
		circuitCompleted = true
	}

	result.DurationMS = p.TimeoutCtrl.Now().Sub(startTime).Milliseconds()

	if p.Sanitizer != nil {
		result = p.Sanitizer.Sanitize(timeoutCtx, result)
	}

	if p.IdempotencyGuard != nil && inv.IdempotencyKey != "" && idempotencyReservation.IdempotencyKey != "" {
		switch result.Status {
		case capability.ToolResultStatusCancelled, capability.ToolResultStatusTimedOut:
			_, _ = p.IdempotencyGuard.MarkIndeterminate(timeoutCtx, idempotencyReservation.IdempotencyKey)
		case capability.ToolResultStatusSuccess, capability.ToolResultStatusFailed:
			_ = p.IdempotencyGuard.Complete(timeoutCtx, idempotencyReservation, &result)
		}
	}

	if p.SideEffectRec != nil {
		p.SideEffectRec.Record(timeoutCtx, inv.InvocationID, toolID, result.SideEffects)
	}
	if p.AuditSink != nil && len(result.SideEffects) > 0 {
		_ = p.AuditSink.OnSideEffectRecorded(timeoutCtx, inv.InvocationID, result.SideEffects)
	}

	p.recordAuditFinish(timeoutCtx, inv.InvocationID, toolID, string(result.Status), p.TimeoutCtrl.Now().Sub(startTime))

	if p.MetricsRec != nil {
		p.MetricsRec.Record(timeoutCtx, tool, result, p.TimeoutCtrl.Now().Sub(startTime))
	}

	return p.finalizeCancellation(timeoutCtx, inv, result), nil
}

func (p *ExecutionPipeline) dispatchAttempt(
	ctx context.Context,
	tool capability.ToolDefinition,
	inv capability.ToolInvocationContext,
	request ToolExecutionRequest,
	stream *toolStreamSession,
	attemptNumber int,
	budget TimeoutBudget,
	dispatched *bool,
) capability.UnifiedToolResult {
	if result, cancelled := p.checkTimeout(ctx, inv, string(request.ToolID), budget, TimeoutPhaseRuntime); cancelled {
		return result
	}

	var attemptID string
	if p.AuditSink != nil {
		attemptID, _ = p.AuditSink.BeginAttempt(ctx, inv, tool, attemptNumber, p.TimeoutCtrl.Now())
	}

	var result capability.UnifiedToolResult
	if p.Dispatcher != nil {
		func() {
			defer func() {
				if r := recover(); r != nil {
					result = capability.NewToolFailureResult(inv.InvocationID, string(request.ToolID), &capability.ToolError{
						Code:    capability.ErrorCodeExecutionFailed,
						Message: fmt.Sprintf("runtime dispatch panicked: %v", r),
					})
				}
			}()
			if stream != nil && tool.ResultPolicy.Streaming.Enabled {
				result, _ = p.Dispatcher.DispatchStream(ctx, tool, inv, request.Input, stream)
			} else {
				result = p.Dispatcher.Dispatch(ctx, tool, inv, request.Input)
			}
			if dispatched != nil {
				*dispatched = true
			}
		}()
	} else {
		result = capability.NewToolFailureResult(inv.InvocationID, string(request.ToolID), &capability.ToolError{
			Code:    capability.ErrorCodeInternalError,
			Message: "no dispatcher configured",
		})
	}

	if p.AuditSink != nil && attemptID != "" {
		_ = p.AuditSink.FinishAttempt(ctx, attemptID, result, p.TimeoutCtrl.Now(), 0)
	}

	result = p.checkLateResult(ctx, inv, string(request.ToolID), result, budget)

	if result, cancelled := p.checkTimeoutAfterDispatch(ctx, inv, result, budget, TimeoutPhaseRuntime); cancelled {
		return result
	}

	return result
}

func (p *ExecutionPipeline) finalizeAttemptResult(
	ctx context.Context,
	inv capability.ToolInvocationContext,
	toolID string,
	result capability.UnifiedToolResult,
	stream *toolStreamSession,
	budget TimeoutBudget,
	startTime time.Time,
	attemptNumber int,
	retryCount int,
) capability.UnifiedToolResult {
	result = capability.NormalizeToolErrorResult(result)

	if stream != nil && stream.Err() != nil && result.Status == capability.ToolResultStatusSuccess {
		result = capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:     capability.ErrorCodeStreamDeliveryFailed,
			Category: capability.ToolErrorCategoryStream,
			Message:  fmt.Sprintf("stream delivery failed: %s", stream.Err().Error()),
		})
	}

	if p.AuditSink != nil && result.Status == capability.ToolResultStatusCancelled {
		_ = p.AuditSink.OnCancelled(ctx, inv.InvocationID, string(getCancelReasonCode(ctx, inv)))
	}

	return result
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
	if p.SecretBroker != nil {
		p.SecretBroker.RevokeByInvocation(inv.InvocationID)
	}
	if p.CancellationCtrl == nil {
		return result
	}
	return p.CancellationCtrl.Finalize(ctx, inv, result)
}

func (p *ExecutionPipeline) issueSecretLeases(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext) *capability.ToolError {
	if p.SecretBroker == nil {
		return nil
	}
	if len(tool.SecretReferences) == 0 {
		return nil
	}
	instanceID := inv.InvocationID
	for _, raw := range tool.SecretReferences {
		ref, err := secret.ParseRef(raw)
		if err != nil {
			return &capability.ToolError{
				Code:     capability.ErrorCodeSecretUnavailable,
				Category: capability.ToolErrorCategoryResource,
				Message:  fmt.Sprintf("invalid secret ref %q: %v", raw, err),
			}
		}
		_, err = p.SecretBroker.Issue(ctx, secret.LeaseRequest{
			Ref:               ref,
			Purpose:           fmt.Sprintf("tool:%s", tool.ID),
			InvocationID:      inv.InvocationID,
			RuntimeInstanceID: instanceID,
			ExtensionID:       inv.ExtensionID,
			ModuleID:          inv.ModuleID,
			UserID:            inv.UserID,
			CharacterID:       inv.CharacterID,
			ConversationID:    inv.ConversationID,
		})
		if err != nil {
			return &capability.ToolError{
				Code:     capability.ErrorCodeSecretLeaseIssueFailed,
				Category: capability.ToolErrorCategoryResource,
				Message:  fmt.Sprintf("issue lease for %q: %v", raw, err),
			}
		}
	}
	return nil
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

func (p *ExecutionPipeline) resolveTool(ctx context.Context, toolID string) (capability.ToolDefinition, error) {
	if p.ToolResolver != nil {
		return p.ToolResolver(ctx, toolID)
	}
	return capability.ToolDefinition{}, fmt.Errorf("tool resolver not configured")
}

func (p *ExecutionPipeline) failWithAudit(ctx context.Context, inv capability.ToolInvocationContext, toolID string, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	if p.AuditSink != nil {
		_ = p.AuditSink.FinishInvocation(ctx, inv, result, time.Now())
	}
	return result
}

func (p *ExecutionPipeline) finishEarlyResult(ctx context.Context, inv capability.ToolInvocationContext, toolID string, result capability.UnifiedToolResult) capability.UnifiedToolResult {
	p.recordAuditFinish(ctx, inv.InvocationID, toolID, string(result.Status), 0)
	return result
}

func (p *ExecutionPipeline) recordAuditStart(ctx context.Context, invID, toolID string) {
	if p.AuditSink != nil {
		_ = p.AuditSink.MarkInvocationRunning(ctx, invID, time.Now())
	}
}

func (p *ExecutionPipeline) recordAuditFinish(ctx context.Context, invID, toolID, status string, duration time.Duration) {
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

func (p *ExecutionPipeline) createAndStoreScopeSnapshot(ctx context.Context, manager scope.ScopeManager, inv capability.ToolInvocationContext, tool capability.ToolDefinition) (scope.ScopeSnapshot, error) {
	scopeReq := scope.ScopeResolveRequest{
		Expression:     inferScopeExpression(tool),
		CharacterID:    inv.CharacterID,
		ConversationID: inv.ConversationID,
		ExtensionID:    inv.ExtensionID,
		ModuleID:       inv.ModuleID,
		InvocationID:   inv.InvocationID,
		Generation:     inv.Generation,
	}

	snapshot, err := manager.Snapshot(ctx, scopeReq)
	if err != nil {
		return scope.ScopeSnapshot{}, err
	}

	if snapshot.SnapshotID == "" {
		return scope.ScopeSnapshot{}, fmt.Errorf("scopeSnapshot missing ID")
	}

	if p.ScopeStore != nil {
		if err := p.ScopeStore.SaveSnapshot(ctx, snapshot); err != nil {
			return scope.ScopeSnapshot{}, fmt.Errorf("save scope snapshot: %w", err)
		}
	}

	return snapshot, nil
}

func (p *ExecutionPipeline) checkChildScopeEscalation(ctx context.Context, inv capability.ToolInvocationContext) error {
	if p.ScopeStore == nil {
		return nil
	}

	parentSnapshotID := findParentScopeSnapshot(inv)
	if parentSnapshotID == "" {
		return nil
	}

	parentSnap, err := p.ScopeStore.GetSnapshot(ctx, parentSnapshotID)
	if err != nil {
		return fmt.Errorf("parent scope snapshot not found: %w", err)
	}

	if parentSnap.CharacterID != "" && inv.CharacterID != "" && parentSnap.CharacterID != inv.CharacterID {
		return fmt.Errorf("child character %s exceeds parent character %s", inv.CharacterID, parentSnap.CharacterID)
	}

	if parentSnap.ConversationID != "" && inv.ConversationID != "" && parentSnap.ConversationID != inv.ConversationID {
		return fmt.Errorf("child conversation %s exceeds parent conversation %s", inv.ConversationID, parentSnap.ConversationID)
	}

	if parentSnap.ExtensionID != "" && inv.ExtensionID != "" && parentSnap.ExtensionID != inv.ExtensionID {
		return fmt.Errorf("child extension %s exceeds parent extension %s", inv.ExtensionID, parentSnap.ExtensionID)
	}

	if parentSnap.Generation > 0 && inv.Generation > 0 && parentSnap.Generation != inv.Generation {
		return fmt.Errorf("child generation %d stale (parent %d)", inv.Generation, parentSnap.Generation)
	}

	return nil
}

func (p *ExecutionPipeline) handleApproval(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, decision PermissionDecision, budget TimeoutBudget) *capability.UnifiedToolResult {
	toolID := string(tool.ID)
	result, cancelled := p.checkTimeout(ctx, inv, toolID, budget, TimeoutPhasePreDispatch)
	if cancelled {
		return &result
	}

	if p.ApprovalGate == nil {
		r := p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:     capability.ErrorCodePermissionDenied,
			Category: capability.ToolErrorCategoryPermission,
			Message:  "approval required but approval gate not configured",
		})))
		return &r
	}

	approvalDecision := p.runApprovalWithReEvaluate(ctx, tool, inv, decision, budget)
	switch approvalDecision {
	case approvalDecisionApproved:
		grantedPerms := collectGrantedPermissionIDs(tool)
		if inv.PermissionSnapshotID == "" {
			p.createPermissionSnapshot(ctx, inv, grantedPerms, nil, inv.ScopeSnapshotID)
		}
		return nil
	case approvalDecisionDenied:
		r := p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:     capability.ErrorCodePermissionDenied,
			Category: capability.ToolErrorCategoryPermission,
			Message:  "approval denied",
		})))
		return &r
	case approvalDecisionTimedOut:
		r := p.finalizeTimeout(ctx, inv, capability.NewToolTimedOutResult(inv.InvocationID, toolID), budget)
		return &r
	case approvalDecisionCancelled:
		r := p.finalizeCancellation(ctx, inv, capability.NewToolCancelledResult(inv.InvocationID, toolID))
		return &r
	default:
		r := p.finalizeCancellation(ctx, inv, p.failWithAudit(ctx, inv, toolID, capability.NewToolFailureResult(inv.InvocationID, toolID, &capability.ToolError{
			Code:     capability.ErrorCodePermissionDenied,
			Category: capability.ToolErrorCategoryPermission,
			Message:  "approval unavailable",
		})))
		return &r
	}
}

type approvalFlowDecision string

const (
	approvalDecisionApproved  approvalFlowDecision = "approved"
	approvalDecisionDenied    approvalFlowDecision = "denied"
	approvalDecisionTimedOut  approvalFlowDecision = "timed_out"
	approvalDecisionCancelled approvalFlowDecision = "cancelled"
	approvalDecisionError     approvalFlowDecision = "error"
)

func (p *ExecutionPipeline) runApprovalWithReEvaluate(ctx context.Context, tool capability.ToolDefinition, inv capability.ToolInvocationContext, decision PermissionDecision, budget TimeoutBudget) approvalFlowDecision {
	if p.PermissionGate == nil || p.PermissionGate.Broker == nil {
		return approvalDecisionError
	}

	broker := p.PermissionGate.Broker

	approved, appErr := p.ApprovalGate.Evaluate(ctx, tool, inv, decision)

	if appErr != nil {
		if approvalTimeout(appErr) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return approvalDecisionTimedOut
		}
		if errors.Is(ctx.Err(), context.Canceled) {
			return approvalDecisionCancelled
		}
		return approvalDecisionError
	}

	if !approved {
		if errors.Is(ctx.Err(), context.Canceled) {
			return approvalDecisionCancelled
		}
		return approvalDecisionDenied
	}

	permissionIDs := collectGrantedPermissionIDs(tool)

	execCtx := permission.ExecutionContextFromInvocation(inv)
	recordReq := permission.PermissionApprovalRecordRequest{
		InvocationID:        inv.InvocationID,
		PermissionIDs:       permissionIDs,
		ScopeSnapshotID:     inv.ScopeSnapshotID,
		Decision:            permission.ApprovalDecisionApproved,
		ExecutionContext:    execCtx,
		ExecutionBindingKey: execCtx.BindingKey(),
		RiskLevel:           string(tool.RiskLevel),
	}

	record, err := broker.RecordApproval(ctx, recordReq)
	if err != nil {
		return approvalDecisionError
	}

	reEvalReq := permission.PermissionEvaluationRequest{
		Subject:          permission.SubjectForTool(tool.ExtensionID, tool.ID),
		Requirements:     buildPermissionRequirements(tool, inv),
		InvocationID:     inv.InvocationID,
		RiskLevel:        string(tool.RiskLevel),
		ScopeSnapshotID:  inv.ScopeSnapshotID,
		ApprovalMode:     string(inv.ApprovalMode),
		Generation:       inv.Generation,
		ExecutionContext: execCtx,
		ApprovalRecordID: record.RecordID,
	}

	reEvalResult := broker.Evaluate(ctx, reEvalReq)

	switch reEvalResult.Decision {
	case permission.DecisionAllow:
		return approvalDecisionApproved
	default:
		return approvalDecisionDenied
	}
}

func (p *ExecutionPipeline) createPermissionSnapshot(ctx context.Context, inv capability.ToolInvocationContext, grantedPerms, grantedScopes []string, scopeSnapshotID string) {
	if p.PermissionSnapshotStore == nil {
		return
	}

	execCtx := permission.ExecutionContextFromInvocation(inv)
	snap := permission.NewPermissionSnapshot(permission.PermissionSnapshotRequest{
		ExtensionID:      inv.ExtensionID,
		ModuleID:         inv.ModuleID,
		Generation:       inv.Generation,
		CharacterID:      inv.CharacterID,
		ConversationID:   inv.ConversationID,
		GrantedPerms:     grantedPerms,
		GrantedScopes:    grantedScopes,
		ExecutionContext: execCtx,
	})

	snap.SessionID = scopeSnapshotID

	if err := p.PermissionSnapshotStore.SaveSnapshot(ctx, snap); err == nil {
		inv.PermissionSnapshotID = snap.SnapshotID
	}
}

func (p *ExecutionPipeline) revalidateSnapshots(ctx context.Context, inv capability.ToolInvocationContext, tool capability.ToolDefinition) error {
	if inv.PermissionSnapshotID != "" && p.PermissionGate != nil && p.PermissionGate.Broker != nil {
		execCtx := permission.ExecutionContextFromInvocation(inv)
		if err := p.PermissionGate.Broker.ValidateSnapshot(ctx, inv.PermissionSnapshotID, permission.PermissionEvaluationRequest{
			Subject:          permission.SubjectForTool(tool.ExtensionID, tool.ID),
			Requirements:     buildPermissionRequirements(tool, inv),
			InvocationID:     inv.InvocationID,
			Generation:       inv.Generation,
			ExecutionContext: execCtx,
		}); err != nil {
			return fmt.Errorf("permission snapshot invalid: %w", err)
		}
	}

	if inv.ScopeSnapshotID != "" && p.ScopeStore != nil {
		_, err := p.ScopeStore.GetSnapshot(ctx, inv.ScopeSnapshotID)
		if err != nil {
			return fmt.Errorf("scope snapshot invalid: %w", err)
		}
	}

	return nil
}

func inferScopeExpression(tool capability.ToolDefinition) scope.ScopeExpression {
	scopes := []scope.ScopeRef{
		scope.NewGlobalScope(),
	}

	if tool.ExtensionID != "" {
		scopes = append(scopes, scope.NewExtensionScope(tool.ExtensionID))
	}

	return scope.ScopeExpression{
		Operator: scope.OpAND,
		Scopes:   scopes,
	}
}

func buildPermissionRequirements(tool capability.ToolDefinition, inv capability.ToolInvocationContext) []permission.PermissionRequirement {
	requirements := make([]permission.PermissionRequirement, 0)
	for _, p := range tool.Permissions {
		requirements = append(requirements, permission.PermissionRequirement{
			PermissionID: p.Capability,
		})
	}
	return requirements
}

func collectGrantedPermissionIDs(tool capability.ToolDefinition) []string {
	ids := make([]string, 0, len(tool.Permissions))
	for _, p := range tool.Permissions {
		ids = append(ids, p.Capability)
	}
	return ids
}

func findParentScopeSnapshot(inv capability.ToolInvocationContext) string {
	return ""
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
