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

func NewExecutionHook(writer RecordWriter, sanitizer *RecordSanitizer) *ExecutionHook {
	return &ExecutionHook{
		writer:    writer,
		sanitizer: sanitizer,
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

// ==================== ExecutionRecorder interface implementation ====================

func (h *ExecutionHook) BeginInvocation(ctx context.Context, inv capability.ToolInvocationContext, toolID string, rawInput []byte, startedAt time.Time) error {
	traceID := inv.TraceID
	if traceID == "" {
		traceID = NewTraceID()
	}

	opID := inv.OperationID
	if opID == "" {
		opID = NewOperationID()
	}

	_ = h.writer.WriteTrace(ctx, Trace{
		TraceID:   traceID,
		RootOpID:  opID,
		CreatedAt: startedAt,
		Metadata:  cloneMap(inv.Metadata),
	})

	_ = h.writer.WriteOperation(ctx, OperationRecord{
		OperationID: opID,
		TraceID:     traceID,
		Type:        OpToolExecute,
		ActorType:   mapActorFromSource(inv.Source),
		ActorID:     inv.UserID,
		SubjectType: SubjectTool,
		SubjectID:   toolID,
		Status:      StatusRunning,
		StartedAt:   startedAt,
		CreatedAt:   startedAt,
	})

	rec := InvocationRecord{
		InvocationID:         inv.InvocationID,
		TraceID:              traceID,
		OperationID:          opID,
		ParentID:             inv.ParentID,
		RootID:               inv.RootID,
		CapabilityID:         toolID,
		Source:               string(inv.Source),
		UserID:               inv.UserID,
		CharacterID:          inv.CharacterID,
		ConversationID:       inv.ConversationID,
		ExtensionID:          inv.ExtensionID,
		ModuleID:             inv.ModuleID,
		Status:               StatusQueued,
		ApprovalMode:         ApprovalMode(inv.ApprovalMode),
		ScopeSnapshotID:      inv.ScopeSnapshotID,
		PermissionSnapshotID: inv.PermissionSnapshotID,
		CreatedAt:            startedAt,
		Metadata:             cloneMap(inv.Metadata),
	}

	if h.sanitizer != nil {
		h.sanitizer.SanitizeInvocationInput(&rec, string(rawInput))
	}

	return h.writer.WriteInvocation(ctx, rec)
}

func (h *ExecutionHook) MarkInvocationRunning(ctx context.Context, invocationID string, startedAt time.Time) error {
	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "invocation.running",
		Severity:     "info",
		Timestamp:    startedAt,
	})

	return h.writer.WriteInvocation(ctx, InvocationRecord{
		InvocationID: invocationID,
		Status:       StatusRunning,
		StartedAt:    &startedAt,
	})
}

func (h *ExecutionHook) BeginAttempt(ctx context.Context, inv capability.ToolInvocationContext, tool capability.ToolDefinition, attemptNumber int, startedAt time.Time) (string, error) {
	attemptID := NewAttemptID()

	meta := map[string]any{
		"tool_id":    tool.ID,
		"tool_name":  tool.Name,
		"tool_source": string(tool.Source),
	}

	att := ExecutionAttempt{
		AttemptID:     attemptID,
		InvocationID:  inv.InvocationID,
		AttemptNumber: attemptNumber,
		RuntimeType:   string(tool.Runtime.RuntimeType),
		RuntimeID:     tool.Runtime.RuntimeID,
		Status:        StatusRunning,
		StartedAt:     startedAt,
		Metadata:      meta,
	}

	if err := h.writer.WriteAttempt(ctx, att); err != nil {
		return "", err
	}

	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: inv.InvocationID,
		AttemptID:    attemptID,
		EventType:    "attempt.started",
		Severity:     "info",
		Timestamp:    startedAt,
		Data: map[string]any{
			"attempt_number": attemptNumber,
			"runtime_type":   string(tool.Runtime.RuntimeType),
			"runtime_id":     tool.Runtime.RuntimeID,
		},
	})

	return attemptID, nil
}

func (h *ExecutionHook) FinishAttempt(ctx context.Context, attemptID string, result capability.UnifiedToolResult, finishedAt time.Time, backoff time.Duration) error {
	status := StatusForUnifiedResult(result)
	var errCode string
	var errMsg string
	if result.Error != nil {
		errCode = result.Error.Code
		errMsg = result.Error.Message
	}

	_ = h.writer.WriteAttempt(ctx, ExecutionAttempt{
		AttemptID:  attemptID,
		Status:     status,
		FinishedAt: &finishedAt,
		DurationMs: result.DurationMS,
		ErrorCode:  errCode,
		Retryable:  result.Error != nil && result.Error.Retryable,
		BackoffMs:  backoff.Milliseconds(),
		Metadata: map[string]any{
			"error_message": errMsg,
		},
	})

	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: result.InvocationID,
		AttemptID:    attemptID,
		EventType:    "attempt.finished",
		Severity:     severityForStatus(status),
		Timestamp:    finishedAt,
		Data: map[string]any{
			"status":       string(status),
			"error_code":   errCode,
			"duration_ms":  result.DurationMS,
			"backoff_ms":   backoff.Milliseconds(),
		},
	})

	if status == StatusFailed && result.Error != nil {
		errRec := ErrorRecord{
			ErrorID:          NewErrorID(),
			InvocationID:     result.InvocationID,
			AttemptID:        attemptID,
			Code:             errCode,
			Category:         ErrorCodeToCategory(errCode),
			Retryable:        result.Error.Retryable,
			UserVisible:      result.Error.UserVisible,
			SanitizedMessage: result.Error.Message,
			CreatedAt:        finishedAt,
		}
		if h.sanitizer != nil {
			h.sanitizer.RedactErrorRecord(&errRec, result.Error.Message)
		}
		_ = h.writer.WriteErrorRecord(ctx, errRec)
	}

	return nil
}

func (h *ExecutionHook) FinishInvocation(ctx context.Context, inv capability.ToolInvocationContext, result capability.UnifiedToolResult, finishedAt time.Time) error {
	status := StatusForUnifiedResult(result)
	var errCode string
	var errMsg string
	if result.Error != nil {
		errCode = result.Error.Code
		errMsg = result.Error.Message
	}

	rec := InvocationRecord{
		InvocationID:    inv.InvocationID,
		Status:          status,
		FinishedAt:      &finishedAt,
		DurationMs:      result.DurationMS,
		ErrorCode:       errCode,
		ErrorSummary:    errMsg,
		SideEffectCount: len(result.SideEffects),
	}

	outputStr := ""
	if len(result.Structured) > 0 {
		outputStr = string(result.Structured)
	}

	if h.sanitizer != nil {
		h.sanitizer.SanitizeInvocationOutput(&rec, outputStr)
	}

	if err := h.writer.WriteInvocation(ctx, rec); err != nil {
		return err
	}

	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: inv.InvocationID,
		EventType:    "invocation.finished",
		Severity:     severityForStatus(status),
		Timestamp:    finishedAt,
		Data: map[string]any{
			"status":      string(status),
			"error_code":  errCode,
			"duration_ms": result.DurationMS,
		},
	})

	return nil
}

func (h *ExecutionHook) OnRetryScheduled(ctx context.Context, invocationID string, previousAttempt int, nextAttempt int, retryCount int, delayMs int64, reason string) error {
	_ = h.writer.WriteInvocation(ctx, InvocationRecord{
		InvocationID: invocationID,
		Status:       StatusRetrying,
		RetryCount:   retryCount,
	})

	return h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "retry.scheduled",
		Severity:     "warn",
		Timestamp:    time.Now(),
		Data: map[string]any{
			"previous_attempt": previousAttempt,
			"next_attempt":     nextAttempt,
			"retry_count":      retryCount,
			"delay_ms":         delayMs,
			"reason":           reason,
		},
	})
}

func (h *ExecutionHook) OnTimeoutTriggered(ctx context.Context, invocationID string) error {
	now := time.Now()

	_ = h.writer.WriteInvocation(ctx, InvocationRecord{
		InvocationID: invocationID,
		Status:       StatusTimedOut,
		FinishedAt:   &now,
	})

	return h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "timeout.triggered",
		Severity:     "error",
		Timestamp:    now,
	})
}

func (h *ExecutionHook) OnCancelled(ctx context.Context, invocationID string, reason string) error {
	now := time.Now()

	_ = h.writer.WriteInvocation(ctx, InvocationRecord{
		InvocationID: invocationID,
		Status:       StatusCancelled,
		FinishedAt:   &now,
	})

	return h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: invocationID,
		EventType:    "cancel.requested",
		Severity:     "warn",
		Timestamp:    now,
		Data: map[string]any{
			"reason": reason,
		},
	})
}

func (h *ExecutionHook) OnPermissionDenied(ctx context.Context, inv capability.ToolInvocationContext, toolID string, reason string) error {
	now := time.Now()

	_ = h.writer.WriteInvocation(ctx, InvocationRecord{
		InvocationID: inv.InvocationID,
		Status:       StatusDenied,
		FinishedAt:   &now,
		ErrorCode:    capability.ErrorCodePermissionDenied,
	})

	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: inv.InvocationID,
		EventType:    "permission.denied",
		Severity:     "warn",
		Timestamp:    now,
		Data: map[string]any{
			"tool_id": toolID,
		"reason":  reason,
		},
	})

	return h.writer.WriteAuditEvent(ctx, AuditEvent{
		AuditID:      NewAuditID(),
		TraceID:      inv.TraceID,
		InvocationID: inv.InvocationID,
		ActorType:    mapActorFromSource(inv.Source),
		ActorID:      inv.UserID,
		SubjectType:  SubjectTool,
		SubjectID:    toolID,
		Action:       "permission.denied",
		Decision:     "denied",
		RiskLevel:    "high",
		Result:       reason,
		ErrorCode:    capability.ErrorCodePermissionDenied,
		CreatedAt:    now,
		Metadata: map[string]any{
			"reason": reason,
		},
	})
}

func (h *ExecutionHook) OnScopeDenied(ctx context.Context, inv capability.ToolInvocationContext, toolID string, reason string) error {
	now := time.Now()

	_ = h.writer.WriteInvocation(ctx, InvocationRecord{
		InvocationID: inv.InvocationID,
		Status:       StatusDenied,
		FinishedAt:   &now,
		ErrorCode:    capability.ErrorCodeScopeDenied,
	})

	_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		InvocationID: inv.InvocationID,
		EventType:    "scope.denied",
		Severity:     "warn",
		Timestamp:    now,
		Data: map[string]any{
			"tool_id": toolID,
		"reason":  reason,
		},
	})

	return h.writer.WriteAuditEvent(ctx, AuditEvent{
		AuditID:      NewAuditID(),
		TraceID:      inv.TraceID,
		InvocationID: inv.InvocationID,
		ActorType:    mapActorFromSource(inv.Source),
		ActorID:      inv.UserID,
		SubjectType:  SubjectTool,
		SubjectID:    toolID,
		Action:       "scope.denied",
		Decision:     "denied",
		RiskLevel:    "high",
		Result:       reason,
		ErrorCode:    capability.ErrorCodeScopeDenied,
		CreatedAt:    now,
		Metadata: map[string]any{
			"reason": reason,
		},
	})
}

func (h *ExecutionHook) OnSideEffectRecorded(ctx context.Context, invocationID string, effects []capability.RecordedSideEffect) error {
	now := time.Now()
	for _, effect := range effects {
		_ = h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
			EventID:      NewEventID(),
			InvocationID: invocationID,
			EventType:    "side_effect.recorded",
			Severity:     "info",
			Timestamp:    now,
			Data: map[string]any{
				"type":        effect.Type,
				"target":      effect.Target,
				"description": effect.Description,
				"reversible":  effect.Reversible,
			},
		})
	}

	return h.writer.WriteInvocation(ctx, InvocationRecord{
		InvocationID:   invocationID,
		SideEffectCount: len(effects),
	})
}

func (h *ExecutionHook) OnCircuitStateChange(ctx context.Context, circuitKey string, fromState string, toState string, reason string, failureCount int, resultingState string) error {
	if fromState == toState {
		return nil
	}
	severity := "info"
	if toState == "open" {
		severity = "error"
	} else if toState == "half_open" {
		severity = "warn"
	}
	eventType := "circuit.state_change"
	switch toState {
	case "open":
		eventType = "circuit.opened"
	case "half_open":
		eventType = "circuit.half_opened"
	case "closed":
		eventType = "circuit.closed"
	}
	return h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:      NewEventID(),
		EventType:    eventType,
		Severity:     severity,
		Timestamp:    time.Now(),
		Data: map[string]any{
			"circuit_key":     circuitKey,
			"from_state":      fromState,
			"to_state":        toState,
			"reason":          reason,
			"failure_count":   failureCount,
			"resulting_state": resultingState,
		},
	})
}

func (h *ExecutionHook) OnConcurrencyAcquired(ctx context.Context, dimensions []string) error {
	if len(dimensions) == 0 {
		return nil
	}
	return h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:   NewEventID(),
		EventType: "concurrency.acquired",
		Severity:  "info",
		Timestamp: time.Now(),
		Data: map[string]any{
			"dimensions": dimensions,
		},
	})
}

func (h *ExecutionHook) OnConcurrencyReleased(ctx context.Context, dimensions []string, waitedMs int64) error {
	if len(dimensions) == 0 {
		return nil
	}
	return h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:   NewEventID(),
		EventType: "concurrency.released",
		Severity:  "info",
		Timestamp: time.Now(),
		Data: map[string]any{
			"dimensions":       dimensions,
			"wait_duration_ms": waitedMs,
		},
	})
}

func (h *ExecutionHook) OnConcurrencyWait(ctx context.Context, dimensions []string, waitedMs int64) error {
	if len(dimensions) == 0 {
		return nil
	}
	return h.writer.WriteRuntimeEvent(ctx, RuntimeEventRecord{
		EventID:   NewEventID(),
		EventType: "concurrency.wait_completed",
		Severity:  "info",
		Timestamp: time.Now(),
		Data: map[string]any{
			"dimensions":       dimensions,
			"wait_duration_ms": waitedMs,
		},
	})
}

func (h *ExecutionHook) OnRateLimitAdmitted(ctx context.Context, dimensions []string) error {
	return h.rateLimitEvent(ctx, "rate_limit.admitted", dimensions, "", 0, 0)
}

func (h *ExecutionHook) OnRateLimitRejected(ctx context.Context, dimensions []string, reason string, retryAfterMs int64) error {
	return h.rateLimitEvent(ctx, "rate_limit.rejected", dimensions, reason, retryAfterMs, 0)
}

func (h *ExecutionHook) OnBackpressureRejected(ctx context.Context, dimensions []string, reason string, retryAfterMs int64) error {
	return h.rateLimitEvent(ctx, "backpressure.rejected", dimensions, reason, retryAfterMs, 0)
}

func (h *ExecutionHook) OnRateLimitWait(ctx context.Context, dimensions []string, waitMs int64) error {
	return h.rateLimitEvent(ctx, "rate_limit.wait_completed", dimensions, "", 0, waitMs)
}

func (h *ExecutionHook) OnIdempotencyBegin(ctx context.Context, key string) error {
	return h.idempotencyEvent(ctx, "idempotency.begin", key, "", "", nil)
}

func (h *ExecutionHook) OnIdempotencyCacheHit(ctx context.Context, key string) error {
	return h.idempotencyEvent(ctx, "idempotency.cache_hit", key, "", "", nil)
}

func (h *ExecutionHook) OnIdempotencyConflict(ctx context.Context, key string, prevID string, prevState string) error {
	return h.idempotencyEvent(ctx, "idempotency.conflict", key, prevID, prevState, nil)
}

func (h *ExecutionHook) OnIdempotencySingleFlightJoin(ctx context.Context, key string) error {
	return h.idempotencyEvent(ctx, "idempotency.single_flight_join", key, "", "", nil)
}

func (h *ExecutionHook) OnIdempotencyComplete(ctx context.Context, key string, opErr error) error {
	return h.idempotencyEvent(ctx, "idempotency.complete", key, "", "", opErr)
}

func (h *ExecutionHook) OnIdempotencyIndeterminate(ctx context.Context, key string) error {
	return h.idempotencyEvent(ctx, "idempotency.indeterminate", key, "", "", nil)
}

func (h *ExecutionHook) OnIdempotencyReleased(ctx context.Context, key string) error {
	return h.idempotencyEvent(ctx, "idempotency.released", key, "", "", nil)
}

func (h *ExecutionHook) idempotencyEvent(_ context.Context, eventType string, key string, prevID string, prevState string, opErr error) error {
	data := map[string]any{
		"key": key,
	}
	if prevID != "" {
		data["prev_owner"] = prevID
	}
	if prevState != "" {
		data["prev_state"] = prevState
	}
	if opErr != nil {
		data["error"] = opErr.Error()
	}
	return h.writer.WriteRuntimeEvent(context.Background(), RuntimeEventRecord{
		EventType: eventType,
		Data:      data,
		Timestamp: time.Now().UTC(),
	})
}

func (h *ExecutionHook) rateLimitEvent(_ context.Context, eventType string, dimensions []string, reason string, retryAfterMs int64, waitMs int64) error {
	data := map[string]any{
		"dimensions": dimensions,
	}
	if reason != "" {
		data["reason"] = reason
	}
	if retryAfterMs > 0 {
		data["retry_after_ms"] = retryAfterMs
	}
	if waitMs > 0 {
		data["wait_duration_ms"] = waitMs
	}
	return h.writer.WriteRuntimeEvent(context.Background(), RuntimeEventRecord{
		EventID:   NewEventID(),
		EventType: eventType,
		Severity:  "info",
		Timestamp: time.Now(),
		Data:      data,
	})
}

// ==================== Helper functions ====================

func mapActorFromSource(source capability.InvocationSource) ActorType {
	switch source {
	case capability.InvocationSourceUser:
		return ActorUser
	case capability.InvocationSourceModel:
		return ActorModel
	case capability.InvocationSourceSystem, capability.InvocationSourceScheduledTask:
		return ActorSystem
	case capability.InvocationSourceWorkflow:
		return ActorWorkflow
	case capability.InvocationSourcePlugin:
		return ActorPlugin
	case capability.InvocationSourceComputerUse:
		return ActorSystem
	default:
		return ActorSystem
	}
}

func cloneMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	dst := make(map[string]any, len(m))
	for k, v := range m {
		dst[k] = v
	}
	return dst
}

// ==================== Status helpers ====================

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
