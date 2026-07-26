package observability

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/permission"
	"github.com/u-ai/backend/internal/extension/kernel/scope"
)

type ExecutionHook struct {
	writer    RecordWriter
	sanitizer *RecordSanitizer
}

func NewExecutionHook(writer RecordWriter) *ExecutionHook {
	return &ExecutionHook{
		writer:    writer,
		sanitizer: NewRecordSanitizer(),
	}
}

func (h *ExecutionHook) OnInvocationCreated(ctx context.Context, inv capability.ToolInvocationContext, toolID string) string {
	now := time.Now()

	rec := InvocationRecord{
		InvocationID:   inv.InvocationID,
		TraceID:        inv.TraceID,
		ParentID:       inv.ParentID,
		UserID:         inv.UserID,
		CharacterID:    inv.CharacterID,
		ConversationID: inv.ConversationID,
		ExtensionID:    inv.ExtensionID,
		ModuleID:       inv.ModuleID,
		CapabilityID:   toolID,
		Source:         string(inv.Source),
		ApprovalMode:   ApprovalMode(inv.ApprovalMode),
		Status:         StatusCreated,
		CreatedAt:      now,
	}

	if inv.TraceID == "" {
		rec.TraceID = NewTraceID()
	}

	_ = h.writer.WriteInvocation(ctx, rec)
	return rec.TraceID
}

func (h *ExecutionHook) OnInvocationQueued(ctx context.Context, invocationID string) {
	now := time.Now()
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "invocation.queued",
		Severity:     "info",
		Timestamp:    now,
	})
}

func (h *ExecutionHook) OnInvocationStarted(ctx context.Context, invocationID string) {
	now := time.Now()
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "invocation.started",
		Severity:     "info",
		Timestamp:    now,
	})
}

func (h *ExecutionHook) OnInvocationFinished(ctx context.Context, invocationID string, status ExecutionStatus, errCode string) {
	now := time.Now()
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "invocation.finished",
		Severity:     severityForStatus(status),
		Timestamp:    now,
		Data: map[string]any{
			"status":     string(status),
			"error_code": errCode,
		},
	})
}

func (h *ExecutionHook) OnAttemptStarted(ctx context.Context, invocationID string, attemptNumber int, runtimeType, runtimeID string) string {
	now := time.Now()
	attemptID := NewAttemptID()

	att := ExecutionAttempt{
		AttemptID:     attemptID,
		InvocationID:  invocationID,
		AttemptNumber: attemptNumber,
		RuntimeType:   runtimeType,
		RuntimeID:     runtimeID,
		Status:        StatusRunning,
		StartedAt:     now,
	}

	_ = h.writer.WriteAttempt(ctx, att)
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		AttemptID:    attemptID,
		EventType:    "attempt.started",
		Severity:     "info",
		Timestamp:    now,
	})

	return attemptID
}

func (h *ExecutionHook) OnAttemptFinished(ctx context.Context, invocationID, attemptID string, status ExecutionStatus, errCode string, durationMs int64) {
	now := time.Now()
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		AttemptID:    attemptID,
		EventType:    "attempt.finished",
		Severity:     severityForStatus(status),
		Timestamp:    now,
		Data: map[string]any{
			"status":      string(status),
			"error_code":  errCode,
			"duration_ms": durationMs,
		},
	})
}

func (h *ExecutionHook) OnRetryScheduled(ctx context.Context, invocationID string, attemptNumber int, backoffMs int64) {
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "retry.scheduled",
		Severity:     "warn",
		Timestamp:    time.Now(),
		Data: map[string]any{
			"attempt_number": attemptNumber,
			"backoff_ms":     backoffMs,
		},
	})
}

func (h *ExecutionHook) OnPermissionDecision(ctx context.Context, inv capability.ToolInvocationContext, toolID string, result permission.PermissionEvaluationResult) {
	decision := string(result.Decision)
	event := AuditEvent{
		AuditID:      NewAuditID(),
		TraceID:      inv.TraceID,
		InvocationID: inv.InvocationID,
		ActorType:    ActorUser,
		ActorID:      inv.UserID,
		SubjectType:  SubjectTool,
		SubjectID:    toolID,
		Action:       "permission.evaluate",
		Decision:     decision,
		Result:       fmt.Sprintf("decision=%s matched=%d missing=%d", result.Decision, len(result.MatchedGrants), len(result.Missing)),
		CreatedAt:    time.Now(),
		Metadata: map[string]any{
			"invocation_id": inv.InvocationID,
		},
	}

	if result.Decision == permission.DecisionDeny {
		event.RiskLevel = "high"
	}

	_ = h.writer.WriteAuditEvent(ctx, event)
}

func (h *ExecutionHook) OnScopeDecision(ctx context.Context, inv capability.ToolInvocationContext, toolID string, req scope.ScopeEvaluationRequest, decision scope.ScopeDecision) {
	event := AuditEvent{
		AuditID:      NewAuditID(),
		TraceID:      inv.TraceID,
		InvocationID: inv.InvocationID,
		ActorType:    ActorUser,
		ActorID:      inv.UserID,
		SubjectType:  SubjectTool,
		SubjectID:    toolID,
		Action:       "scope.evaluate",
		CreatedAt:    time.Now(),
		Metadata: map[string]any{
			"character_id":    req.CharacterID,
			"conversation_id": req.ConversationID,
		},
	}

	if decision.Allowed {
		event.Decision = "allowed"
		event.Result = "scope_allowed"
	} else {
		event.Decision = "denied"
		event.Result = "scope_denied"
		event.RiskLevel = "high"
	}

	_ = h.writer.WriteAuditEvent(ctx, event)
}

func (h *ExecutionHook) OnSideEffectRecorded(ctx context.Context, invocationID string, effects []capability.RecordedSideEffect) {
	for _, effect := range effects {
		_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
			EventID:      NewEventID(),
			InvocationID: invocationID,
			EventType:    "side_effect.recorded",
			Severity:     "info",
			Timestamp:    time.Now(),
			Data: map[string]any{
				"type":        effect.Type,
				"target":      effect.Target,
				"description": effect.Description,
			},
		})
	}
}

func (h *ExecutionHook) OnTimeoutTriggered(ctx context.Context, invocationID string) {
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "timeout.triggered",
		Severity:     "warn",
		Timestamp:    time.Now(),
	})
}

func (h *ExecutionHook) OnCancelled(ctx context.Context, invocationID string) {
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "cancel.requested",
		Severity:     "warn",
		Timestamp:    time.Now(),
	})
}

func (h *ExecutionHook) OnCircuitOpen(ctx context.Context, invocationID, runtimeID string) {
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "circuit.opened",
		Severity:     "error",
		Timestamp:    time.Now(),
		Data: map[string]any{
			"runtime_id": runtimeID,
		},
	})
}

func (h *ExecutionHook) OnCircuitClosed(ctx context.Context, invocationID, runtimeID string) {
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "circuit.closed",
		Severity:     "info",
		Timestamp:    time.Now(),
		Data: map[string]any{
			"runtime_id": runtimeID,
		},
	})
}

func (h *ExecutionHook) RecordLifecycleEvent(ctx context.Context, opType OperationType, actorType ActorType, actorID string, subjectType SubjectType, subjectID string, status ExecutionStatus) {
	op := OperationRecord{
		OperationID: NewOperationID(),
		TraceID:     NewTraceID(),
		Type:        opType,
		ActorType:   actorType,
		ActorID:     actorID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Status:      status,
		StartedAt:   time.Now(),
		CreatedAt:   time.Now(),
	}

	if status.IsTerminal() {
		now := time.Now()
		op.FinishedAt = &now
	}

	_ = h.writer.WriteOperation(ctx, op)

	_ = h.writer.WriteAuditEvent(ctx, AuditEvent{
		AuditID:     NewAuditID(),
		TraceID:     op.TraceID,
		OperationID: op.OperationID,
		ActorType:   actorType,
		ActorID:     actorID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Action:      string(opType),
		Decision:    "completed",
		Result:      string(status),
		CreatedAt:   time.Now(),
	})
}

func (h *ExecutionHook) WriteAuditForGrant(ctx context.Context, grant permission.PermissionGrant, actorType ActorType, actorID string) {
	_ = h.writer.WriteAuditEvent(ctx, AuditEvent{
		AuditID:       NewAuditID(),
		TraceID:       NewTraceID(),
		ActorType:     actorType,
		ActorID:       actorID,
		SubjectType:   SubjectPermissionGrant,
		SubjectID:     grant.GrantID,
		Action:        "permission.grant",
		Decision:      string(grant.Decision),
		Result:        "granted",
		PermissionIDs: []string{grant.PermissionID},
		GrantID:       grant.GrantID,
		CreatedAt:     time.Now(),
	})
}

func (h *ExecutionHook) WriteAuditForScopeBind(ctx context.Context, binding scope.ScopeBinding, actorType ActorType, actorID string) {
	_ = h.writer.WriteAuditEvent(ctx, AuditEvent{
		AuditID:     NewAuditID(),
		TraceID:     NewTraceID(),
		ActorType:   actorType,
		ActorID:     actorID,
		SubjectType: SubjectScopeBinding,
		SubjectID:   binding.BindingID,
		Action:      "scope.bind",
		Decision:    "completed",
		Result:      "bound",
		TargetType:  string(binding.Scope.Type),
		TargetID:    binding.Scope.CharacterID + binding.Scope.ConversationID + binding.Scope.ExtensionID + binding.Scope.ModuleID,
		CreatedAt:   time.Now(),
	})
}

func StatusForUnifiedResult(res capability.UnifiedToolResult) ExecutionStatus {
	switch res.Status {
	case capability.ToolResultStatusSuccess:
		return StatusSucceeded
	case capability.ToolResultStatusFailed:
		if res.Error != nil {
			switch res.Error.Code {
			case capability.ErrorCodePermissionDenied:
				return StatusDenied
			case capability.ErrorCodeCancelled:
				return StatusCancelled
			case capability.ErrorCodeTimeout:
				return StatusTimedOut
			case capability.ErrorCodeRateLimited:
				return StatusRateLimited
			}
		}
		return StatusFailed
	case capability.ToolResultStatusCancelled:
		return StatusCancelled
	case capability.ToolResultStatusTimedOut:
		return StatusTimedOut
	default:
		return StatusFailed
	}
}

func StatusForExecutionError(err *capability.ToolError) ExecutionStatus {
	if err == nil {
		return StatusFailed
	}
	switch err.Code {
	case capability.ErrorCodePermissionDenied:
		return StatusDenied
	case capability.ErrorCodeCancelled:
		return StatusCancelled
	case capability.ErrorCodeTimeout:
		return StatusTimedOut
	case capability.ErrorCodeRateLimited:
		return StatusRateLimited
	default:
		return StatusFailed
	}
}

func severityForStatus(status ExecutionStatus) string {
	switch status {
	case StatusSucceeded, StatusCancelled:
		return "info"
	case StatusFailed, StatusTimedOut, StatusInterrupted:
		return "error"
	case StatusDenied, StatusRateLimited, StatusCircuitOpen:
		return "warn"
	default:
		return "info"
	}
}

func ErrorCodeToCategory(code string) string {
	switch code {
	case "invalid_input", "invalid_result":
		return "validation"
	case "permission_denied", "scope_denied":
		return "security"
	case "timeout", "cancelled":
		return "timeout"
	case "rate_limited", "circuit_open":
		return "capacity"
	case "runtime_unavailable", "connection_lost":
		return "infrastructure"
	case "execution_failed", "internal_error":
		return "execution"
	default:
		return "unknown"
	}
}

func ErrorCodeToRetryable(code string) bool {
	switch code {
	case "timeout", "connection_lost", "rate_limited", "circuit_open":
		return true
	case "runtime_unavailable":
		return true
	default:
		return false
	}
}
