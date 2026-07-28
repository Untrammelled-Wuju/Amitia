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
	toolID := string(request.ToolID)
	inv := request.Invocation

	if err := ctx.Err(); err != nil {
		return p.failWithAudit(ctx, inv.InvocationID, toolID, capability.ErrorCodeInternalError, fmt.Sprintf("context %s", err.Error()))
	}

	if p.InvocationValidator != nil {
		if err := p.InvocationValidator.Validate(ctx, request); err != nil {
			return p.failWithAudit(ctx, inv.InvocationID, toolID, capability.ErrorCodeInvalidInput, err.Error())
		}
	}

	tool, err := p.resolveTool(ctx, toolID)
	if err != nil {
		return p.failWithAudit(ctx, inv.InvocationID, toolID, capability.ErrorCodeNotAvailable, err.Error())
	}

	if p.InputValidator != nil {
		if err := p.InputValidator.Validate(ctx, tool, request.Input); err != nil {
			return p.failWithAudit(ctx, inv.InvocationID, toolID, capability.ErrorCodeInvalidInput, err.Error())
		}
	}

	if p.AvailabilityGate != nil {
		avail := p.AvailabilityGate.Evaluate(ctx, tool, inv)
		if !avail.Executable {
			return p.failWithAudit(ctx, inv.InvocationID, toolID, capability.ErrorCodeNotAvailable, "tool not executable")
		}
	}

	if p.ScopeGate != nil {
		if err := p.ScopeGate.Evaluate(ctx, tool, inv); err != nil {
			return p.failWithAudit(ctx, inv.InvocationID, toolID, capability.ErrorCodeScopeDenied, err.Error())
		}
	}

	if p.PermissionGate != nil {
		decision := p.PermissionGate.Evaluate(ctx, tool, inv)
		switch decision {
		case PermissionDeny:
			return p.failWithAudit(ctx, inv.InvocationID, toolID, capability.ErrorCodePermissionDenied, "permission denied")
		case PermissionRequireApproval:
			if p.ApprovalGate != nil {
				approved, appErr := p.ApprovalGate.Evaluate(ctx, tool, inv, decision)
				if appErr != nil || !approved {
					return p.failWithAudit(ctx, inv.InvocationID, toolID, capability.ErrorCodePermissionDenied, "approval denied")
				}
			}
		}
	}

	if p.DepthGuard != nil {
		if err := p.DepthGuard.Check(ctx, inv); err != nil {
			return p.failWithAudit(ctx, inv.InvocationID, toolID, "max_depth_exceeded", err.Error())
		}
	}

	if p.RateLimiter != nil {
		if err := p.RateLimiter.Allow(ctx, tool); err != nil {
			return p.failWithAudit(ctx, inv.InvocationID, toolID, capability.ErrorCodeRateLimited, err.Error())
		}
	}

	if p.ConcurrencyCtrl != nil {
		slot, slotErr := p.ConcurrencyCtrl.Acquire(ctx, tool, inv)
		if slotErr != nil {
			return p.failWithAudit(ctx, inv.InvocationID, toolID, capability.ErrorCodeRateLimited, slotErr.Error())
		}
		defer p.ConcurrencyCtrl.Release(slot)
	}

	if p.IdempotencyGuard != nil && inv.IdempotencyKey != "" {
		if cached, found := p.IdempotencyGuard.Check(ctx, inv.IdempotencyKey, toolID); found {
			return cached
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
	if p.Dispatcher != nil {
		result = p.Dispatcher.Dispatch(ctx, tool, inv, request.Input)
	} else {
		result = capability.UnifiedToolResult{
			InvocationID: inv.InvocationID,
			Status:       capability.ToolResultStatusFailed,
			Error: &capability.ToolError{
				Code:    capability.ErrorCodeInternalError,
				Message: "no dispatcher configured",
			},
		}
	}

	if p.RetryCtrl != nil && result.Status != capability.ToolResultStatusSuccess {
		if result.Error != nil {
			switch result.Error.Code {
			case capability.ErrorCodePermissionDenied, capability.ErrorCodeScopeDenied, capability.ErrorCodeInvalidInput:
				return result
			}
		}
		if shouldRetry, _ := p.RetryCtrl.ShouldRetry(ctx, tool, result); shouldRetry {
			for attempt := 1; attempt <= tool.ExecutionPolicy.RetryPolicy.MaxRetries; attempt++ {
				time.Sleep(p.RetryCtrl.Backoff(attempt))
				if p.AuditRec != nil {
					p.AuditRec.RecordRetry(inv.InvocationID, attempt)
				}
				result = p.Dispatcher.Dispatch(ctx, tool, inv, request.Input)
				if result.Status == capability.ToolResultStatusSuccess {
					break
				}
			}
		}
	}

	if p.ResultValidator != nil {
		result = p.ResultValidator.Validate(ctx, tool, result)
	}

	if p.Sanitizer != nil {
		result = p.Sanitizer.Sanitize(ctx, result)
	}

	if p.IdempotencyGuard != nil && inv.IdempotencyKey != "" {
		p.IdempotencyGuard.Record(ctx, inv.IdempotencyKey, toolID, &result)
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

	return result
}

func (p *ExecutionPipeline) resolveTool(ctx context.Context, toolID string) (capability.ToolDefinition, error) {
	if p.ToolResolver != nil {
		return p.ToolResolver(ctx, toolID)
	}
	return capability.ToolDefinition{}, fmt.Errorf("tool resolver not configured")
}

func (p *ExecutionPipeline) failWithAudit(ctx context.Context, invID, toolID, code, msg string) capability.UnifiedToolResult {
	if p.AuditRec != nil {
		p.AuditRec.RecordDenied(ctx, invID, toolID, code, msg)
	}
	return capability.UnifiedToolResult{
		InvocationID: invID,
		Status:       capability.ToolResultStatusFailed,
		Error: &capability.ToolError{
			Code:        code,
			Message:     msg,
			UserVisible: true,
		},
	}
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
