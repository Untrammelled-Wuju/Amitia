package observability

import (
	"context"
	"database/sql"
	"encoding/json"
	"sync"
)

type SQLiteStorage struct {
	db *sql.DB
	mu sync.RWMutex
}

func NewSQLiteStorage(db *sql.DB) *SQLiteStorage {
	return &SQLiteStorage{db: db}
}

func (s *SQLiteStorage) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func (s *SQLiteStorage) Close() error {
	return nil
}

func (s *SQLiteStorage) SaveTrace(ctx context.Context, t Trace) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_observability_traces (trace_id, root_operation_id, created_at, metadata_json)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(trace_id) DO UPDATE SET
			root_operation_id = excluded.root_operation_id,
			metadata_json = excluded.metadata_json
	`, t.TraceID, t.RootOpID, t.CreatedAt, toJSON(t.Metadata))
	return err
}

func (s *SQLiteStorage) GetTrace(ctx context.Context, traceID string) (*Trace, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRowContext(ctx, `
		SELECT trace_id, root_operation_id, created_at, metadata_json
		FROM kernel_observability_traces WHERE trace_id = ?
	`, traceID)
	var t Trace
	var metadataJSON string
	if err := row.Scan(&t.TraceID, &t.RootOpID, &t.CreatedAt, &metadataJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrTraceNotFound
		}
		return nil, err
	}
	t.Metadata = fromJSONMap(metadataJSON)
	return &t, nil
}

func (s *SQLiteStorage) DeleteTrace(ctx context.Context, traceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_observability_traces WHERE trace_id = ?`, traceID)
	return err
}

func (s *SQLiteStorage) SaveOperation(ctx context.Context, op OperationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_observability_operations (
			operation_id, trace_id, parent_operation_id, type, actor_type, actor_id,
			subject_type, subject_id, status, risk_level, started_at, finished_at,
			summary, error_code, outcome, metadata_json, created_at_extension
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(operation_id) DO UPDATE SET
			status = excluded.status,
			risk_level = excluded.risk_level,
			finished_at = excluded.finished_at,
			summary = excluded.summary,
			error_code = excluded.error_code,
			outcome = excluded.outcome,
			metadata_json = excluded.metadata_json
	`, op.OperationID, op.TraceID, op.ParentOpID, string(op.Type), string(op.ActorType),
		op.ActorID, string(op.SubjectType), op.SubjectID, string(op.Status),
		string(op.RiskLevel), op.StartedAt, op.FinishedAt, op.Summary,
		op.ErrorCode, string(op.Outcome), toJSON(op.Metadata), op.CreatedAt)
	return err
}

func (s *SQLiteStorage) GetOperation(ctx context.Context, operationID string) (*OperationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRowContext(ctx, `
		SELECT operation_id, trace_id, parent_operation_id, type, actor_type, actor_id,
			subject_type, subject_id, status, risk_level, started_at, finished_at,
			summary, error_code, outcome, metadata_json, created_at_extension
		FROM kernel_observability_operations WHERE operation_id = ?
	`, operationID)
	var op OperationRecord
	var metadataJSON string
	if err := row.Scan(&op.OperationID, &op.TraceID, &op.ParentOpID, &op.Type,
		&op.ActorType, &op.ActorID, &op.SubjectType, &op.SubjectID, &op.Status,
		&op.RiskLevel, &op.StartedAt, &op.FinishedAt, &op.Summary,
		&op.ErrorCode, &op.Outcome, &metadataJSON, &op.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrOperationNotFound
		}
		return nil, err
	}
	op.Metadata = fromJSONMap(metadataJSON)
	return &op, nil
}

func (s *SQLiteStorage) ListOperations(ctx context.Context, filter OperationFilter) ([]OperationRecord, string, error) {
	return nil, "", nil
}

func (s *SQLiteStorage) UpdateOperationStatus(ctx context.Context, operationID string, status ExecutionStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		UPDATE kernel_observability_operations SET status = ? WHERE operation_id = ?
	`, string(status), operationID)
	return err
}

func (s *SQLiteStorage) SaveInvocation(ctx context.Context, inv InvocationRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_observability_invocations (
			invocation_id, trace_id, operation_id, parent_id, root_id, capability_id,
			capability_type, source, owner_type, owner_id, extension_id, module_id,
			runtime_type, runtime_id, user_id, character_id, conversation_id,
			scope_snapshot_id, permission_snapshot_id, status, risk_level, approval_mode,
			input_hash, output_hash, input_summary, output_summary, error_code,
			error_summary, retry_count, side_effect_count, created_at, queued_at,
			started_at, finished_at, duration_ms, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(invocation_id) DO UPDATE SET
			status = excluded.status,
			output_hash = excluded.output_hash,
			output_summary = excluded.output_summary,
			error_code = excluded.error_code,
			error_summary = excluded.error_summary,
			finished_at = excluded.finished_at,
			duration_ms = excluded.duration_ms,
			retry_count = excluded.retry_count,
			side_effect_count = excluded.side_effect_count,
			metadata_json = excluded.metadata_json
	`, inv.InvocationID, inv.TraceID, inv.OperationID, inv.ParentID, inv.RootID,
		inv.CapabilityID, inv.CapabilityType, inv.Source, inv.OwnerType, inv.OwnerID,
		inv.ExtensionID, inv.ModuleID, inv.RuntimeType, inv.RuntimeID,
		inv.UserID, inv.CharacterID, inv.ConversationID, inv.ScopeSnapshotID,
		inv.PermissionSnapshotID, string(inv.Status), string(inv.RiskLevel),
		string(inv.ApprovalMode), inv.InputHash, inv.OutputHash, inv.InputSummary,
		inv.OutputSummary, inv.ErrorCode, inv.ErrorSummary, inv.RetryCount,
		inv.SideEffectCount, inv.CreatedAt, inv.QueuedAt, inv.StartedAt,
		inv.FinishedAt, inv.DurationMs, toJSON(inv.Metadata))
	return err
}

func (s *SQLiteStorage) GetInvocation(ctx context.Context, invocationID string) (*InvocationRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRowContext(ctx, `
		SELECT invocation_id, trace_id, operation_id, parent_id, root_id, capability_id,
			capability_type, source, owner_type, owner_id, extension_id, module_id,
			runtime_type, runtime_id, user_id, character_id, conversation_id,
			scope_snapshot_id, permission_snapshot_id, status, risk_level, approval_mode,
			input_hash, output_hash, input_summary, output_summary, error_code,
			error_summary, retry_count, side_effect_count, created_at, queued_at,
			started_at, finished_at, duration_ms, metadata_json
		FROM kernel_observability_invocations WHERE invocation_id = ?
	`, invocationID)
	var inv InvocationRecord
	var metadataJSON string
	if err := row.Scan(&inv.InvocationID, &inv.TraceID, &inv.OperationID, &inv.ParentID,
		&inv.RootID, &inv.CapabilityID, &inv.CapabilityType, &inv.Source,
		&inv.OwnerType, &inv.OwnerID, &inv.ExtensionID, &inv.ModuleID,
		&inv.RuntimeType, &inv.RuntimeID, &inv.UserID, &inv.CharacterID,
		&inv.ConversationID, &inv.ScopeSnapshotID, &inv.PermissionSnapshotID,
		&inv.Status, &inv.RiskLevel, &inv.ApprovalMode, &inv.InputHash,
		&inv.OutputHash, &inv.InputSummary, &inv.OutputSummary, &inv.ErrorCode,
		&inv.ErrorSummary, &inv.RetryCount, &inv.SideEffectCount, &inv.CreatedAt,
		&inv.QueuedAt, &inv.StartedAt, &inv.FinishedAt, &inv.DurationMs, &metadataJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrInvocationNotFound
		}
		return nil, err
	}
	inv.Metadata = fromJSONMap(metadataJSON)
	return &inv, nil
}

func (s *SQLiteStorage) ListInvocations(ctx context.Context, filter InvocationFilter) ([]InvocationRecord, string, error) {
	return nil, "", nil
}

func (s *SQLiteStorage) UpdateInvocationStatus(ctx context.Context, invocationID string, status ExecutionStatus) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		UPDATE kernel_observability_invocations SET status = ? WHERE invocation_id = ?
	`, string(status), invocationID)
	return err
}

func (s *SQLiteStorage) GetInvocationChildren(ctx context.Context, parentID string) ([]InvocationRecord, error) {
	return nil, nil
}

func (s *SQLiteStorage) IncrementSideEffectCount(ctx context.Context, invocationID string, delta int) error {
	return nil
}

func (s *SQLiteStorage) SaveAttempt(ctx context.Context, att ExecutionAttempt) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_observability_attempts (
			attempt_id, invocation_id, attempt_number, runtime_type, runtime_id,
			status, started_at, finished_at, duration_ms, error_code, retryable,
			backoff_ms, resource_usage_json, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(attempt_id) DO UPDATE SET
			status = excluded.status,
			finished_at = excluded.finished_at,
			duration_ms = excluded.duration_ms,
			error_code = excluded.error_code,
			retryable = excluded.retryable,
			backoff_ms = excluded.backoff_ms,
			resource_usage_json = excluded.resource_usage_json,
			metadata_json = excluded.metadata_json
	`, att.AttemptID, att.InvocationID, att.AttemptNumber, att.RuntimeType,
		att.RuntimeID, string(att.Status), att.StartedAt, att.FinishedAt,
		att.DurationMs, att.ErrorCode, att.Retryable, att.BackoffMs,
		toJSON(att.ResourceUsage), toJSON(att.Metadata))
	return err
}

func (s *SQLiteStorage) GetAttempt(ctx context.Context, attemptID string) (*ExecutionAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRowContext(ctx, `
		SELECT attempt_id, invocation_id, attempt_number, runtime_type, runtime_id,
			status, started_at, finished_at, duration_ms, error_code, retryable,
			backoff_ms, resource_usage_json, metadata_json
		FROM kernel_observability_attempts WHERE attempt_id = ?
	`, attemptID)
	var att ExecutionAttempt
	var resourceJSON, metadataJSON string
	if err := row.Scan(&att.AttemptID, &att.InvocationID, &att.AttemptNumber,
		&att.RuntimeType, &att.RuntimeID, &att.Status, &att.StartedAt,
		&att.FinishedAt, &att.DurationMs, &att.ErrorCode, att.Retryable,
		&att.BackoffMs, &resourceJSON, &metadataJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAttemptNotFound
		}
		return nil, err
	}
	att.ResourceUsage = fromJSONMap(resourceJSON)
	att.Metadata = fromJSONMap(metadataJSON)
	return &att, nil
}

func (s *SQLiteStorage) ListAttemptsByInvocation(ctx context.Context, invocationID string) ([]ExecutionAttempt, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.QueryContext(ctx, `
		SELECT attempt_id, invocation_id, attempt_number, runtime_type, runtime_id,
			status, started_at, finished_at, duration_ms, error_code, retryable,
			backoff_ms, resource_usage_json, metadata_json
		FROM kernel_observability_attempts WHERE invocation_id = ?
		ORDER BY attempt_number ASC
	`, invocationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	results := make([]ExecutionAttempt, 0)
	for rows.Next() {
		var att ExecutionAttempt
		var resourceJSON, metadataJSON string
		if err := rows.Scan(&att.AttemptID, &att.InvocationID, &att.AttemptNumber,
			&att.RuntimeType, &att.RuntimeID, &att.Status, &att.StartedAt,
			&att.FinishedAt, &att.DurationMs, &att.ErrorCode, att.Retryable,
			&att.BackoffMs, &resourceJSON, &metadataJSON); err != nil {
			return nil, err
		}
		att.ResourceUsage = fromJSONMap(resourceJSON)
		att.Metadata = fromJSONMap(metadataJSON)
		results = append(results, att)
	}
	return results, rows.Err()
}

func (s *SQLiteStorage) SaveRuntimeEvent(ctx context.Context, evt RuntimeEventRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_observability_runtime_events (
			event_id, trace_id, operation_id, invocation_id, attempt_id,
			event_type, severity, timestamp, data_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, evt.EventID, evt.TraceID, evt.OperationID, evt.InvocationID, evt.AttemptID,
		evt.EventType, evt.Severity, evt.Timestamp, toJSON(evt.Data))
	return err
}

func (s *SQLiteStorage) ListRuntimeEvents(ctx context.Context, filter EventFilter) ([]RuntimeEventRecord, string, error) {
	return nil, "", nil
}

func (s *SQLiteStorage) SaveAuditEvent(ctx context.Context, evt AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_observability_audit_events (
			audit_id, trace_id, operation_id, invocation_id, actor_type, actor_id,
			subject_type, subject_id, action, decision, risk_level, scope_summary,
			permission_ids_json, target_type, target_id, result, error_code, grant_id,
			approval_id, snapshot_id, created_at, metadata_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, evt.AuditID, evt.TraceID, evt.OperationID, evt.InvocationID,
		string(evt.ActorType), evt.ActorID, string(evt.SubjectType), evt.SubjectID,
		evt.Action, evt.Decision, evt.RiskLevel, evt.ScopeSummary,
		toJSON(evt.PermissionIDs), evt.TargetType, evt.TargetID,
		evt.Result, evt.ErrorCode, evt.GrantID, evt.ApprovalID, evt.SnapshotID,
		evt.CreatedAt, toJSON(evt.Metadata))
	return err
}

func (s *SQLiteStorage) ListAuditEvents(ctx context.Context, filter AuditFilter) ([]AuditEvent, string, error) {
	return nil, "", nil
}

func (s *SQLiteStorage) GetAuditEvent(ctx context.Context, auditID string) (*AuditEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRowContext(ctx, `
		SELECT audit_id, trace_id, operation_id, invocation_id, actor_type, actor_id,
			subject_type, subject_id, action, decision, risk_level, scope_summary,
			permission_ids_json, target_type, target_id, result, error_code, grant_id,
			approval_id, snapshot_id, created_at, metadata_json
		FROM kernel_observability_audit_events WHERE audit_id = ?
	`, auditID)
	var evt AuditEvent
	var metadataJSON string
	if err := row.Scan(&evt.AuditID, &evt.TraceID, &evt.OperationID, &evt.InvocationID,
		&evt.ActorType, &evt.ActorID, &evt.SubjectType, &evt.SubjectID,
		&evt.Action, &evt.Decision, &evt.RiskLevel, &evt.ScopeSummary,
		&evt.PermissionIDs, &evt.TargetType, &evt.TargetID,
		&evt.Result, &evt.ErrorCode, &evt.GrantID, &evt.ApprovalID, &evt.SnapshotID,
		&evt.CreatedAt, &metadataJSON); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrAuditWriteRequired
		}
		return nil, err
	}
	evt.Metadata = fromJSONMap(metadataJSON)
	return &evt, nil
}

func (s *SQLiteStorage) SaveErrorRecord(ctx context.Context, rec ErrorRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO kernel_observability_errors (
			error_id, invocation_id, attempt_id, code, category, retryable,
			user_visible, sanitized_message, internal_reference, created_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, rec.ErrorID, rec.InvocationID, rec.AttemptID, rec.Code, rec.Category,
		rec.Retryable, rec.UserVisible, rec.SanitizedMessage, rec.InternalReference,
		rec.CreatedAt)
	return err
}

func (s *SQLiteStorage) GetErrorRecord(ctx context.Context, errorID string) (*ErrorRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRowContext(ctx, `
		SELECT error_id, invocation_id, attempt_id, code, category, retryable,
			user_visible, sanitized_message, internal_reference, created_at
		FROM kernel_observability_errors WHERE error_id = ?
	`, errorID)
	var rec ErrorRecord
	if err := row.Scan(&rec.ErrorID, &rec.InvocationID, &rec.AttemptID,
		&rec.Code, &rec.Category, &rec.Retryable, &rec.UserVisible,
		&rec.SanitizedMessage, &rec.InternalReference, &rec.CreatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, ErrErrorNotFound
		}
		return nil, err
	}
	return &rec, nil
}

func (s *SQLiteStorage) ListErrorRecords(ctx context.Context, invocationID string) ([]ErrorRecord, error) {
	return nil, nil
}

func toJSON(v any) string {
	if v == nil {
		return "{}"
	}
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func fromJSONMap(s string) map[string]any {
	if s == "" || s == "{}" {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	return m
}
