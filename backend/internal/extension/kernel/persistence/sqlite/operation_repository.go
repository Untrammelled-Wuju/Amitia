package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
)

type Operation struct {
	OperationID   string             `json:"operationId"`
	OperationType string             `json:"operationType"`
	ExtensionID   domain.ExtensionID `json:"extensionId"`
	Status        string             `json:"status"`
	ErrorCode     string             `json:"errorCode,omitempty"`
	ErrorMessage  string             `json:"errorMessage,omitempty"`
	StartedAt     time.Time          `json:"startedAt"`
	FinishedAt    *time.Time         `json:"finishedAt,omitempty"`
}

type Invocation struct {
	InvocationID       string     `json:"invocationId"`
	ParentInvocationID string     `json:"parentInvocationId,omitempty"`
	OperationID        string     `json:"operationId"`
	ContributionID     string     `json:"contributionId"`
	RuntimeInstanceID  string     `json:"runtimeInstanceId,omitempty"`
	Status             string     `json:"status"`
	InputHash          string     `json:"inputHash,omitempty"`
	OutputHash         string     `json:"outputHash,omitempty"`
	ErrorCode          string     `json:"errorCode,omitempty"`
	StartedAt          time.Time  `json:"startedAt"`
	FinishedAt         *time.Time `json:"finishedAt,omitempty"`
}

type OperationRepository interface {
	PutOperation(ctx context.Context, op Operation) error
	GetOperation(ctx context.Context, operationID string) (Operation, error)
	ListOperations(ctx context.Context, extensionID domain.ExtensionID) ([]Operation, error)
	ListOperationsByStatus(ctx context.Context, status string) ([]Operation, error)
	PutInvocation(ctx context.Context, inv Invocation) error
	GetInvocation(ctx context.Context, invocationID string) (Invocation, error)
	ListInvocations(ctx context.Context, operationID string) ([]Invocation, error)
}

type SQLiteOperationRepository struct {
	db *sql.DB
}

func NewOperationRepository(db *sql.DB) *SQLiteOperationRepository {
	return &SQLiteOperationRepository{db: db}
}

func (r *SQLiteOperationRepository) PutOperation(ctx context.Context, op Operation) error {
	var finishedAt sql.NullTime
	if op.FinishedAt != nil {
		finishedAt = sql.NullTime{Time: op.FinishedAt.UTC(), Valid: true}
	}
	startedAt := op.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_operations (operation_id, operation_type, extension_id, status, error_code, error_message, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(operation_id) DO UPDATE SET
			operation_type = excluded.operation_type,
			extension_id = excluded.extension_id,
			status = excluded.status,
			error_code = excluded.error_code,
			error_message = excluded.error_message,
			finished_at = excluded.finished_at
	`,
		op.OperationID,
		op.OperationType,
		string(op.ExtensionID),
		op.Status,
		op.ErrorCode,
		op.ErrorMessage,
		startedAt,
		finishedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert operation: %w", err)
	}
	return nil
}

func (r *SQLiteOperationRepository) GetOperation(ctx context.Context, operationID string) (Operation, error) {
	ex := getExecutor(ctx, r.db)
	var op Operation
	var extensionID string
	var finishedAt sql.NullTime

	err := ex.QueryRowContext(ctx, `
		SELECT operation_id, operation_type, extension_id, status, error_code, error_message, started_at, finished_at
		FROM extension_operations WHERE operation_id = ?
	`, operationID).Scan(
		&op.OperationID,
		&op.OperationType,
		&extensionID,
		&op.Status,
		&op.ErrorCode,
		&op.ErrorMessage,
		&op.StartedAt,
		&finishedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Operation{}, fmt.Errorf("sqlite: operation not found: %s", operationID)
		}
		return Operation{}, fmt.Errorf("sqlite: query operation: %w", err)
	}

	op.ExtensionID = domain.ExtensionID(extensionID)
	if finishedAt.Valid {
		t := finishedAt.Time.UTC()
		op.FinishedAt = &t
	}

	return op, nil
}

func (r *SQLiteOperationRepository) ListOperations(ctx context.Context, extensionID domain.ExtensionID) ([]Operation, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT operation_id, operation_type, extension_id, status, error_code, error_message, started_at, finished_at
		FROM extension_operations WHERE extension_id = ? ORDER BY started_at DESC
	`, string(extensionID))
	if err != nil {
		return nil, fmt.Errorf("sqlite: list operations: %w", err)
	}
	defer rows.Close()

	var out []Operation
	for rows.Next() {
		var op Operation
		var extID string
		var finishedAt sql.NullTime

		err := rows.Scan(
			&op.OperationID,
			&op.OperationType,
			&extID,
			&op.Status,
			&op.ErrorCode,
			&op.ErrorMessage,
			&op.StartedAt,
			&finishedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan operation: %w", err)
		}

		op.ExtensionID = domain.ExtensionID(extID)
		if finishedAt.Valid {
			t := finishedAt.Time.UTC()
			op.FinishedAt = &t
		}

		out = append(out, op)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate operations: %w", err)
	}

	return out, nil
}

func (r *SQLiteOperationRepository) ListOperationsByStatus(ctx context.Context, status string) ([]Operation, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT operation_id, operation_type, extension_id, status, error_code, error_message, started_at, finished_at
		FROM extension_operations WHERE status = ? ORDER BY started_at DESC
	`, status)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list operations by status: %w", err)
	}
	defer rows.Close()

	var out []Operation
	for rows.Next() {
		var op Operation
		var extID string
		var finishedAt sql.NullTime

		err := rows.Scan(
			&op.OperationID,
			&op.OperationType,
			&extID,
			&op.Status,
			&op.ErrorCode,
			&op.ErrorMessage,
			&op.StartedAt,
			&finishedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan operation: %w", err)
		}

		op.ExtensionID = domain.ExtensionID(extID)
		if finishedAt.Valid {
			t := finishedAt.Time.UTC()
			op.FinishedAt = &t
		}

		out = append(out, op)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate operations: %w", err)
	}

	return out, nil
}

func (r *SQLiteOperationRepository) PutInvocation(ctx context.Context, inv Invocation) error {
	var finishedAt sql.NullTime
	if inv.FinishedAt != nil {
		finishedAt = sql.NullTime{Time: inv.FinishedAt.UTC(), Valid: true}
	}
	startedAt := inv.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}

	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_invocations (invocation_id, parent_invocation_id, operation_id, contribution_id, runtime_instance_id, status, input_hash, output_hash, error_code, started_at, finished_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(invocation_id) DO UPDATE SET
			parent_invocation_id = excluded.parent_invocation_id,
			operation_id = excluded.operation_id,
			contribution_id = excluded.contribution_id,
			runtime_instance_id = excluded.runtime_instance_id,
			status = excluded.status,
			input_hash = excluded.input_hash,
			output_hash = excluded.output_hash,
			error_code = excluded.error_code,
			finished_at = excluded.finished_at
	`,
		inv.InvocationID,
		inv.ParentInvocationID,
		inv.OperationID,
		inv.ContributionID,
		inv.RuntimeInstanceID,
		inv.Status,
		inv.InputHash,
		inv.OutputHash,
		inv.ErrorCode,
		startedAt,
		finishedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert invocation: %w", err)
	}
	return nil
}

func (r *SQLiteOperationRepository) GetInvocation(ctx context.Context, invocationID string) (Invocation, error) {
	ex := getExecutor(ctx, r.db)
	var inv Invocation
	var finishedAt sql.NullTime

	err := ex.QueryRowContext(ctx, `
		SELECT invocation_id, parent_invocation_id, operation_id, contribution_id, runtime_instance_id, status, input_hash, output_hash, error_code, started_at, finished_at
		FROM extension_invocations WHERE invocation_id = ?
	`, invocationID).Scan(
		&inv.InvocationID,
		&inv.ParentInvocationID,
		&inv.OperationID,
		&inv.ContributionID,
		&inv.RuntimeInstanceID,
		&inv.Status,
		&inv.InputHash,
		&inv.OutputHash,
		&inv.ErrorCode,
		&inv.StartedAt,
		&finishedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return Invocation{}, fmt.Errorf("sqlite: invocation not found: %s", invocationID)
		}
		return Invocation{}, fmt.Errorf("sqlite: query invocation: %w", err)
	}

	if finishedAt.Valid {
		t := finishedAt.Time.UTC()
		inv.FinishedAt = &t
	}

	return inv, nil
}

func (r *SQLiteOperationRepository) ListInvocations(ctx context.Context, operationID string) ([]Invocation, error) {
	ex := getExecutor(ctx, r.db)
	rows, err := ex.QueryContext(ctx, `
		SELECT invocation_id, parent_invocation_id, operation_id, contribution_id, runtime_instance_id, status, input_hash, output_hash, error_code, started_at, finished_at
		FROM extension_invocations WHERE operation_id = ? ORDER BY started_at DESC
	`, operationID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list invocations: %w", err)
	}
	defer rows.Close()

	var out []Invocation
	for rows.Next() {
		var inv Invocation
		var finishedAt sql.NullTime

		err := rows.Scan(
			&inv.InvocationID,
			&inv.ParentInvocationID,
			&inv.OperationID,
			&inv.ContributionID,
			&inv.RuntimeInstanceID,
			&inv.Status,
			&inv.InputHash,
			&inv.OutputHash,
			&inv.ErrorCode,
			&inv.StartedAt,
			&finishedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan invocation: %w", err)
		}

		if finishedAt.Valid {
			t := finishedAt.Time.UTC()
			inv.FinishedAt = &t
		}

		out = append(out, inv)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sqlite: iterate invocations: %w", err)
	}

	return out, nil
}

var _ OperationRepository = (*SQLiteOperationRepository)(nil)
