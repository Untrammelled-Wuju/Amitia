package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type CheckpointManager struct {
	db *sql.DB
}

func NewCheckpointManager(db *sql.DB) *CheckpointManager {
	return &CheckpointManager{db: db}
}

func (m *CheckpointManager) CreateCheckpoint(ctx context.Context, operationID string, stepID int, migrationID, stage string, cursor json.RawMessage, batchNum, processedCount int, inputHash, defHash, snapshotID string) (*MigrationCheckpoint, error) {
	checkpoint := &MigrationCheckpoint{
		CheckpointID:   fmt.Sprintf("ckpt-%s-%d-%d", operationID, stepID, batchNum),
		OperationID:    operationID,
		StepID:         stepID,
		MigrationID:    migrationID,
		Stage:          stage,
		Cursor:         cursor,
		BatchNumber:    batchNum,
		ProcessedCount: processedCount,
		InputHash:      inputHash,
		DefinitionHash: defHash,
		SnapshotID:     snapshotID,
		CreatedAt:      time.Now().UTC(),
	}

	_, err := m.db.ExecContext(ctx, `
		INSERT INTO extension_migration_checkpoints
			(checkpoint_id, operation_id, step_id, migration_id, stage, cursor_json,
			 batch_number, processed_count, input_hash, definition_hash, snapshot_id, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(checkpoint_id) DO UPDATE SET
			operation_id = excluded.operation_id, step_id = excluded.step_id,
			migration_id = excluded.migration_id, stage = excluded.stage,
			cursor_json = excluded.cursor_json, batch_number = excluded.batch_number,
			processed_count = excluded.processed_count, input_hash = excluded.input_hash,
			definition_hash = excluded.definition_hash, snapshot_id = excluded.snapshot_id,
			created_at = excluded.created_at
	`,
		checkpoint.CheckpointID, checkpoint.OperationID, checkpoint.StepID,
		checkpoint.MigrationID, checkpoint.Stage, nullableString(string(checkpoint.Cursor)),
		checkpoint.BatchNumber, checkpoint.ProcessedCount,
		nullableString(checkpoint.InputHash), nullableString(checkpoint.DefinitionHash),
		nullableString(checkpoint.SnapshotID), checkpoint.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("migration: create checkpoint: %w", err)
	}
	return checkpoint, nil
}

func (m *CheckpointManager) GetCheckpoint(ctx context.Context, checkpointID string) (*MigrationCheckpoint, error) {
	row := m.db.QueryRowContext(ctx, `
		SELECT checkpoint_id, operation_id, step_id, migration_id, stage, cursor_json,
		       batch_number, processed_count, input_hash, definition_hash, snapshot_id, created_at
		FROM extension_migration_checkpoints WHERE checkpoint_id = ?
	`, checkpointID)
	cp, err := scanCheckpoint(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("migration: checkpoint not found: %s", checkpointID)
		}
		return nil, fmt.Errorf("migration: get checkpoint: %w", err)
	}
	return cp, nil
}

func (m *CheckpointManager) GetLatestCheckpoint(ctx context.Context, operationID string, stepID int) (*MigrationCheckpoint, error) {
	row := m.db.QueryRowContext(ctx, `
		SELECT checkpoint_id, operation_id, step_id, migration_id, stage, cursor_json,
		       batch_number, processed_count, input_hash, definition_hash, snapshot_id, created_at
		FROM extension_migration_checkpoints
		WHERE operation_id = ? AND step_id = ?
		ORDER BY batch_number DESC
		LIMIT 1
	`, operationID, stepID)
	cp, err := scanCheckpoint(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("migration: get latest checkpoint: %w", err)
	}
	return cp, nil
}

func (m *CheckpointManager) ListCheckpoints(ctx context.Context, operationID string) ([]MigrationCheckpoint, error) {
	rows, err := m.db.QueryContext(ctx, `
		SELECT checkpoint_id, operation_id, step_id, migration_id, stage, cursor_json,
		       batch_number, processed_count, input_hash, definition_hash, snapshot_id, created_at
		FROM extension_migration_checkpoints WHERE operation_id = ? ORDER BY batch_number ASC
	`, operationID)
	if err != nil {
		return nil, fmt.Errorf("migration: list checkpoints: %w", err)
	}
	defer rows.Close()
	var out []MigrationCheckpoint
	for rows.Next() {
		cp, err := scanCheckpoint(rows)
		if err != nil {
			return nil, fmt.Errorf("migration: scan checkpoint: %w", err)
		}
		out = append(out, *cp)
	}
	return out, rows.Err()
}

func (m *CheckpointManager) ValidateCheckpoint(ctx context.Context, checkpoint *MigrationCheckpoint) (*ValidationResult, error) {
	result := &ValidationResult{Passed: true}
	if checkpoint.CheckpointID == "" {
		result.Passed = false
		result.Errors = append(result.Errors, "checkpoint_id is empty")
	}
	if checkpoint.OperationID == "" {
		result.Passed = false
		result.Errors = append(result.Errors, "operation_id is empty")
	}
	if checkpoint.DefinitionHash == "" {
		result.Passed = false
		result.Errors = append(result.Errors, "definition_hash is empty")
	}
	if len(checkpoint.Cursor) == 0 || !json.Valid(checkpoint.Cursor) {
		result.Passed = false
		result.Errors = append(result.Errors, "cursor is not valid JSON")
	}
	return result, nil
}

func (m *CheckpointManager) DeleteCheckpoint(ctx context.Context, checkpointID string) error {
	_, err := m.db.ExecContext(ctx, `DELETE FROM extension_migration_checkpoints WHERE checkpoint_id = ?`, checkpointID)
	if err != nil {
		return fmt.Errorf("migration: delete checkpoint: %w", err)
	}
	return nil
}

func (m *CheckpointManager) DeleteCheckpointsByOperation(ctx context.Context, operationID string) error {
	_, err := m.db.ExecContext(ctx, `DELETE FROM extension_migration_checkpoints WHERE operation_id = ?`, operationID)
	if err != nil {
		return fmt.Errorf("migration: delete checkpoints by operation: %w", err)
	}
	return nil
}
