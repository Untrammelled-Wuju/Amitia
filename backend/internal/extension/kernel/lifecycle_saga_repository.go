package kernel

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type LifecycleOperationStatus string

const (
	LifecycleOperationPending          LifecycleOperationStatus = "pending"
	LifecycleOperationRunning          LifecycleOperationStatus = "running"
	LifecycleOperationCompleted        LifecycleOperationStatus = "completed"
	LifecycleOperationCompensating     LifecycleOperationStatus = "compensating"
	LifecycleOperationRequiresRecovery LifecycleOperationStatus = "requires_recovery"
	LifecycleOperationFailed           LifecycleOperationStatus = "failed"
)

type LifecycleStepStatus string

const (
	LifecycleStepPending   LifecycleStepStatus = "pending"
	LifecycleStepRunning   LifecycleStepStatus = "running"
	LifecycleStepSucceeded LifecycleStepStatus = "succeeded"
	LifecycleStepFailed    LifecycleStepStatus = "failed"
	LifecycleStepSkipped   LifecycleStepStatus = "skipped"
)

type LifecycleCompensationStatus string

const (
	LifecycleCompensationPending   LifecycleCompensationStatus = "pending"
	LifecycleCompensationRunning   LifecycleCompensationStatus = "running"
	LifecycleCompensationSucceeded LifecycleCompensationStatus = "succeeded"
	LifecycleCompensationFailed    LifecycleCompensationStatus = "failed"
)

type LifecycleOperation struct {
	OperationID         string
	ExtensionID         string
	OperationType       string
	FromState           string
	TargetState         string
	StableGeneration    int64
	CandidateGeneration int64
	Status              LifecycleOperationStatus
	CurrentStep         string
	ErrorCode           string
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type LifecycleStep struct {
	StepID       string
	OperationID  string
	StepName     string
	Status       LifecycleStepStatus
	AttemptCount int
	ResultJSON   string
	ErrorCode    string
	StartedAt    *time.Time
	FinishedAt   *time.Time
}

type LifecycleCompensation struct {
	CompensationID   string
	OperationID      string
	StepName         string
	CompensationName string
	Status           LifecycleCompensationStatus
	ErrorCode        string
	StartedAt        *time.Time
	FinishedAt       *time.Time
}

type LifecycleSagaRepository struct {
	db *sql.DB
}

func NewLifecycleSagaRepository(db *sql.DB) *LifecycleSagaRepository {
	return &LifecycleSagaRepository{db: db}
}

func (r *LifecycleSagaRepository) CreateOperation(ctx context.Context, op *LifecycleOperation) error {
	if r == nil || r.db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	createdAt := op.CreatedAt.Format(time.RFC3339Nano)
	if createdAt == "" {
		createdAt = now
	}
	updatedAt := op.UpdatedAt.Format(time.RFC3339Nano)
	if updatedAt == "" {
		updatedAt = now
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO kernel_lifecycle_operations
		(operation_id, extension_id, operation_type, from_state, target_state,
		 stable_generation, candidate_generation, status, current_step, error_code,
		 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		op.OperationID,
		op.ExtensionID,
		op.OperationType,
		op.FromState,
		op.TargetState,
		op.StableGeneration,
		op.CandidateGeneration,
		string(op.Status),
		op.CurrentStep,
		op.ErrorCode,
		createdAt,
		updatedAt,
	)
	if err != nil {
		return fmt.Errorf("lifecycle-saga-repo: create operation %s: %w", op.OperationID, err)
	}
	return nil
}

func (r *LifecycleSagaRepository) UpdateOperationStatus(ctx context.Context, operationID string, status LifecycleOperationStatus, currentStep string, errorCode string) error {
	if r == nil || r.db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := r.db.ExecContext(ctx, `
		UPDATE kernel_lifecycle_operations
		SET status = ?, current_step = ?, error_code = ?, updated_at = ?
		WHERE operation_id = ?`,
		string(status), currentStep, errorCode, now, operationID,
	)
	if err != nil {
		return fmt.Errorf("lifecycle-saga-repo: update operation status %s: %w", operationID, err)
	}
	return nil
}

func (r *LifecycleSagaRepository) GetOperation(ctx context.Context, operationID string) (*LifecycleOperation, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT operation_id, extension_id, operation_type, from_state, target_state,
		       stable_generation, candidate_generation, status, current_step, error_code,
		       created_at, updated_at
		FROM kernel_lifecycle_operations
		WHERE operation_id = ?`, operationID)
	var op LifecycleOperation
	var statusStr string
	var createdAtStr, updatedAtStr string
	if err := row.Scan(
		&op.OperationID,
		&op.ExtensionID,
		&op.OperationType,
		&op.FromState,
		&op.TargetState,
		&op.StableGeneration,
		&op.CandidateGeneration,
		&statusStr,
		&op.CurrentStep,
		&op.ErrorCode,
		&createdAtStr,
		&updatedAtStr,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("lifecycle-saga-repo: get operation %s: %w", operationID, err)
	}
	op.Status = LifecycleOperationStatus(statusStr)
	if createdAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
			op.CreatedAt = t
		}
	}
	if updatedAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
			op.UpdatedAt = t
		}
	}
	return &op, nil
}

func (r *LifecycleSagaRepository) ListOperationsByStatus(ctx context.Context, status LifecycleOperationStatus) ([]*LifecycleOperation, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT operation_id, extension_id, operation_type, from_state, target_state,
		       stable_generation, candidate_generation, status, current_step, error_code,
		       created_at, updated_at
		FROM kernel_lifecycle_operations
		WHERE status = ?
		ORDER BY created_at`, string(status))
	if err != nil {
		return nil, fmt.Errorf("lifecycle-saga-repo: list by status %s: %w", status, err)
	}
	defer rows.Close()
	return scanLifecycleOperations(rows)
}

func (r *LifecycleSagaRepository) ListPendingOperations(ctx context.Context) ([]*LifecycleOperation, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT operation_id, extension_id, operation_type, from_state, target_state,
		       stable_generation, candidate_generation, status, current_step, error_code,
		       created_at, updated_at
		FROM kernel_lifecycle_operations
		WHERE status IN ('pending', 'running', 'compensating', 'requires_recovery')
		ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("lifecycle-saga-repo: list pending: %w", err)
	}
	defer rows.Close()
	return scanLifecycleOperations(rows)
}

func (r *LifecycleSagaRepository) SaveStep(ctx context.Context, step *LifecycleStep) error {
	if r == nil || r.db == nil {
		return nil
	}
	var startedAt, finishedAt interface{}
	if step.StartedAt != nil {
		startedAt = step.StartedAt.Format(time.RFC3339Nano)
	} else {
		startedAt = nil
	}
	if step.FinishedAt != nil {
		finishedAt = step.FinishedAt.Format(time.RFC3339Nano)
	} else {
		finishedAt = nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO kernel_lifecycle_steps
		(step_id, operation_id, step_name, status, attempt_count, result_json, error_code,
		 started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		step.StepID,
		step.OperationID,
		step.StepName,
		string(step.Status),
		step.AttemptCount,
		step.ResultJSON,
		step.ErrorCode,
		startedAt,
		finishedAt,
	)
	if err != nil {
		return fmt.Errorf("lifecycle-saga-repo: save step %s: %w", step.StepID, err)
	}
	return nil
}

func (r *LifecycleSagaRepository) UpdateStepStatus(ctx context.Context, stepID string, status LifecycleStepStatus, resultJSON string, errorCode string) error {
	if r == nil || r.db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var finishedAt interface{}
	if status == LifecycleStepSucceeded || status == LifecycleStepFailed || status == LifecycleStepSkipped {
		finishedAt = now
	} else {
		finishedAt = nil
	}
	var startedAt interface{}
	if status == LifecycleStepRunning {
		startedAt = now
	} else {
		startedAt = nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE kernel_lifecycle_steps
		SET status = ?, result_json = ?, error_code = ?,
		    started_at = COALESCE(?, started_at),
		    finished_at = ?
		WHERE step_id = ?`,
		string(status), resultJSON, errorCode, startedAt, finishedAt, stepID,
	)
	if err != nil {
		return fmt.Errorf("lifecycle-saga-repo: update step status %s: %w", stepID, err)
	}
	return nil
}

func (r *LifecycleSagaRepository) ListStepsByOperation(ctx context.Context, operationID string) ([]*LifecycleStep, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT step_id, operation_id, step_name, status, attempt_count, result_json,
		       error_code, started_at, finished_at
		FROM kernel_lifecycle_steps
		WHERE operation_id = ?
		ORDER BY rowid`, operationID)
	if err != nil {
		return nil, fmt.Errorf("lifecycle-saga-repo: list steps by operation %s: %w", operationID, err)
	}
	defer rows.Close()
	var out []*LifecycleStep
	for rows.Next() {
		var (
			step       LifecycleStep
			statusStr  string
			startedAt  sql.NullString
			finishedAt sql.NullString
		)
		if err := rows.Scan(
			&step.StepID,
			&step.OperationID,
			&step.StepName,
			&statusStr,
			&step.AttemptCount,
			&step.ResultJSON,
			&step.ErrorCode,
			&startedAt,
			&finishedAt,
		); err != nil {
			return nil, fmt.Errorf("lifecycle-saga-repo: scan step row: %w", err)
		}
		step.Status = LifecycleStepStatus(statusStr)
		if startedAt.Valid && startedAt.String != "" {
			if t, err := time.Parse(time.RFC3339Nano, startedAt.String); err == nil {
				step.StartedAt = &t
			}
		}
		if finishedAt.Valid && finishedAt.String != "" {
			if t, err := time.Parse(time.RFC3339Nano, finishedAt.String); err == nil {
				step.FinishedAt = &t
			}
		}
		out = append(out, &step)
	}
	return out, rows.Err()
}

func (r *LifecycleSagaRepository) SaveCompensation(ctx context.Context, comp *LifecycleCompensation) error {
	if r == nil || r.db == nil {
		return nil
	}
	var startedAt, finishedAt interface{}
	if comp.StartedAt != nil {
		startedAt = comp.StartedAt.Format(time.RFC3339Nano)
	} else {
		startedAt = nil
	}
	if comp.FinishedAt != nil {
		finishedAt = comp.FinishedAt.Format(time.RFC3339Nano)
	} else {
		finishedAt = nil
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO kernel_lifecycle_compensations
		(compensation_id, operation_id, step_name, compensation_name, status, error_code,
		 started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		comp.CompensationID,
		comp.OperationID,
		comp.StepName,
		comp.CompensationName,
		string(comp.Status),
		comp.ErrorCode,
		startedAt,
		finishedAt,
	)
	if err != nil {
		return fmt.Errorf("lifecycle-saga-repo: save compensation %s: %w", comp.CompensationID, err)
	}
	return nil
}

func (r *LifecycleSagaRepository) UpdateCompensationStatus(ctx context.Context, compensationID string, status LifecycleCompensationStatus, errorCode string) error {
	if r == nil || r.db == nil {
		return nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var finishedAt interface{}
	if status == LifecycleCompensationSucceeded || status == LifecycleCompensationFailed {
		finishedAt = now
	} else {
		finishedAt = nil
	}
	var startedAt interface{}
	if status == LifecycleCompensationRunning {
		startedAt = now
	} else {
		startedAt = nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE kernel_lifecycle_compensations
		SET status = ?, error_code = ?,
		    started_at = COALESCE(?, started_at),
		    finished_at = ?
		WHERE compensation_id = ?`,
		string(status), errorCode, startedAt, finishedAt, compensationID,
	)
	if err != nil {
		return fmt.Errorf("lifecycle-saga-repo: update compensation status %s: %w", compensationID, err)
	}
	return nil
}

func scanLifecycleOperations(rows *sql.Rows) ([]*LifecycleOperation, error) {
	var out []*LifecycleOperation
	for rows.Next() {
		var (
			op           LifecycleOperation
			statusStr    string
			createdAtStr string
			updatedAtStr string
		)
		if err := rows.Scan(
			&op.OperationID,
			&op.ExtensionID,
			&op.OperationType,
			&op.FromState,
			&op.TargetState,
			&op.StableGeneration,
			&op.CandidateGeneration,
			&statusStr,
			&op.CurrentStep,
			&op.ErrorCode,
			&createdAtStr,
			&updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("lifecycle-saga-repo: scan operation row: %w", err)
		}
		op.Status = LifecycleOperationStatus(statusStr)
		if createdAtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
				op.CreatedAt = t
			}
		}
		if updatedAtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
				op.UpdatedAt = t
			}
		}
		out = append(out, &op)
	}
	return out, rows.Err()
}
