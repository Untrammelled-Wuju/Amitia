package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type HostAPIAuditLog struct {
	CallID               string     `json:"callId"`
	TraceID              string     `json:"traceId"`
	OperationID          string     `json:"operationId"`
	InvocationID         string     `json:"invocationId"`
	AttemptID            string     `json:"attemptId"`
	ExtensionID          string     `json:"extensionId"`
	ModuleID             string     `json:"moduleId"`
	Method               string     `json:"method"`
	Generation           int64      `json:"generation"`
	PermissionSnapshotID string     `json:"permissionSnapshotId"`
	ScopeSnapshotID      string     `json:"scopeSnapshotId"`
	StartedAt            time.Time  `json:"startedAt"`
	FinishedAt           *time.Time `json:"finishedAt,omitempty"`
	Result               string     `json:"result"`
	ErrorCode            string     `json:"errorCode,omitempty"`
	ErrorMessage         string     `json:"errorMessage,omitempty"`
	SideEffect           string     `json:"sideEffect,omitempty"`
	InputMasked          string     `json:"inputMasked,omitempty"`
	Phase                string     `json:"phase"`
}

type HostAPIAuditFilter struct {
	ExtensionID string
	Method      string
	Result      string
	TraceID     string
	Limit       int
	Offset      int
}

type HostAPIAuditRepository interface {
	PutAuditLog(ctx context.Context, entry HostAPIAuditLog) error
	ListAuditLogs(ctx context.Context, filter HostAPIAuditFilter) ([]HostAPIAuditLog, error)
	GetAuditLog(ctx context.Context, callID string) (HostAPIAuditLog, error)
	CountAuditLogs(ctx context.Context, filter HostAPIAuditFilter) (int64, error)
}

type SQLiteHostAPIAuditRepository struct {
	db *sql.DB
}

func NewHostAPIAuditRepository(db *sql.DB) *SQLiteHostAPIAuditRepository {
	return &SQLiteHostAPIAuditRepository{db: db}
}

func (r *SQLiteHostAPIAuditRepository) PutAuditLog(ctx context.Context, entry HostAPIAuditLog) error {
	callID := entry.CallID
	if callID == "" {
		return fmt.Errorf("sqlite: host_api_audit call_id is required")
	}
	startedAt := entry.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	var finishedAt sql.NullTime
	if entry.FinishedAt != nil {
		finishedAt = sql.NullTime{Time: entry.FinishedAt.UTC(), Valid: true}
	}
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO host_api_audit_logs (
			call_id, trace_id, operation_id, invocation_id, attempt_id,
			extension_id, module_id, method, generation,
			permission_snapshot_id, scope_snapshot_id,
			started_at, finished_at, result, error_code, error_message,
			side_effect, input_masked, phase
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(call_id) DO UPDATE SET
			finished_at = excluded.finished_at,
			result = excluded.result,
			error_code = excluded.error_code,
			error_message = excluded.error_message,
			side_effect = excluded.side_effect,
			input_masked = excluded.input_masked,
			phase = excluded.phase
	`,
		callID,
		entry.TraceID,
		entry.OperationID,
		entry.InvocationID,
		entry.AttemptID,
		entry.ExtensionID,
		entry.ModuleID,
		entry.Method,
		entry.Generation,
		entry.PermissionSnapshotID,
		entry.ScopeSnapshotID,
		startedAt,
		finishedAt,
		entry.Result,
		entry.ErrorCode,
		entry.ErrorMessage,
		entry.SideEffect,
		entry.InputMasked,
		entry.Phase,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert host_api_audit_log: %w", err)
	}
	return nil
}

func (r *SQLiteHostAPIAuditRepository) ListAuditLogs(ctx context.Context, filter HostAPIAuditFilter) ([]HostAPIAuditLog, error) {
	q := `SELECT call_id, trace_id, operation_id, invocation_id, attempt_id,
		extension_id, module_id, method, generation,
		permission_snapshot_id, scope_snapshot_id,
		started_at, finished_at, result, error_code, error_message,
		side_effect, input_masked, phase
		FROM host_api_audit_logs WHERE 1=1`
	args := []interface{}{}
	if filter.ExtensionID != "" {
		q += " AND extension_id = ?"
		args = append(args, filter.ExtensionID)
	}
	if filter.Method != "" {
		q += " AND method = ?"
		args = append(args, filter.Method)
	}
	if filter.Result != "" {
		q += " AND result = ?"
		args = append(args, filter.Result)
	}
	if filter.TraceID != "" {
		q += " AND trace_id = ?"
		args = append(args, filter.TraceID)
	}
	q += " ORDER BY started_at DESC"
	if filter.Limit > 0 {
		q += " LIMIT ?"
		args = append(args, filter.Limit)
	} else {
		q += " LIMIT 200"
	}
	if filter.Offset > 0 {
		q += " OFFSET ?"
		args = append(args, filter.Offset)
	}
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list host_api_audit_logs: %w", err)
	}
	defer rows.Close()
	var out []HostAPIAuditLog
	for rows.Next() {
		var entry HostAPIAuditLog
		var finishedAt sql.NullTime
		err := rows.Scan(
			&entry.CallID,
			&entry.TraceID,
			&entry.OperationID,
			&entry.InvocationID,
			&entry.AttemptID,
			&entry.ExtensionID,
			&entry.ModuleID,
			&entry.Method,
			&entry.Generation,
			&entry.PermissionSnapshotID,
			&entry.ScopeSnapshotID,
			&entry.StartedAt,
			&finishedAt,
			&entry.Result,
			&entry.ErrorCode,
			&entry.ErrorMessage,
			&entry.SideEffect,
			&entry.InputMasked,
			&entry.Phase,
		)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan host_api_audit_log: %w", err)
		}
		if finishedAt.Valid {
			t := finishedAt.Time.UTC()
			entry.FinishedAt = &t
		}
		out = append(out, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate host_api_audit_logs: %w", err)
	}
	return out, nil
}

func (r *SQLiteHostAPIAuditRepository) GetAuditLog(ctx context.Context, callID string) (HostAPIAuditLog, error) {
	ex := getExecutor(ctx, r.db)
	var entry HostAPIAuditLog
	var finishedAt sql.NullTime
	err := ex.QueryRowContext(ctx, `
		SELECT call_id, trace_id, operation_id, invocation_id, attempt_id,
		extension_id, module_id, method, generation,
		permission_snapshot_id, scope_snapshot_id,
		started_at, finished_at, result, error_code, error_message,
		side_effect, input_masked, phase
		FROM host_api_audit_logs WHERE call_id = ?
	`, callID).Scan(
		&entry.CallID,
		&entry.TraceID,
		&entry.OperationID,
		&entry.InvocationID,
		&entry.AttemptID,
		&entry.ExtensionID,
		&entry.ModuleID,
		&entry.Method,
		&entry.Generation,
		&entry.PermissionSnapshotID,
		&entry.ScopeSnapshotID,
		&entry.StartedAt,
		&finishedAt,
		&entry.Result,
		&entry.ErrorCode,
		&entry.ErrorMessage,
		&entry.SideEffect,
		&entry.InputMasked,
		&entry.Phase,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return HostAPIAuditLog{}, fmt.Errorf("sqlite: host_api_audit_log not found: %s", callID)
		}
		return HostAPIAuditLog{}, fmt.Errorf("sqlite: query host_api_audit_log: %w", err)
	}
	if finishedAt.Valid {
		t := finishedAt.Time.UTC()
		entry.FinishedAt = &t
	}
	return entry, nil
}

func (r *SQLiteHostAPIAuditRepository) CountAuditLogs(ctx context.Context, filter HostAPIAuditFilter) (int64, error) {
	q := `SELECT COUNT(*) FROM host_api_audit_logs WHERE 1=1`
	args := []interface{}{}
	if filter.ExtensionID != "" {
		q += " AND extension_id = ?"
		args = append(args, filter.ExtensionID)
	}
	if filter.Method != "" {
		q += " AND method = ?"
		args = append(args, filter.Method)
	}
	if filter.Result != "" {
		q += " AND result = ?"
		args = append(args, filter.Result)
	}
	if filter.TraceID != "" {
		q += " AND trace_id = ?"
		args = append(args, filter.TraceID)
	}
	ex := getExecutor(ctx, r.db)
	var count int64
	err := ex.QueryRowContext(ctx, q, args...).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: count host_api_audit_logs: %w", err)
	}
	return count, nil
}

var _ HostAPIAuditRepository = (*SQLiteHostAPIAuditRepository)(nil)
