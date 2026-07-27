package migration

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type MigrationRepository struct {
	db *sql.DB
}

func NewMigrationRepository(db *sql.DB) *MigrationRepository {
	return &MigrationRepository{db: db}
}

func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func nullableStringPtr(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func nullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

func stringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	s := ns.String
	return &s
}

func timePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	t := nt.Time
	return &t
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func (r *MigrationRepository) SaveMigrationDefinition(ctx context.Context, def *MigrationDefinition) error {
	dataDomainsJSON, err := json.Marshal(def.DataDomains)
	if err != nil {
		return fmt.Errorf("migration: marshal data domains: %w", err)
	}
	preconditionJSON, err := json.Marshal(def.Precondition)
	if err != nil {
		return fmt.Errorf("migration: marshal precondition: %w", err)
	}
	postconditionJSON, err := json.Marshal(def.Postcondition)
	if err != nil {
		return fmt.Errorf("migration: marshal postcondition: %w", err)
	}
	permReqsJSON, err := json.Marshal(def.PermissionRequirements)
	if err != nil {
		return fmt.Errorf("migration: marshal permission requirements: %w", err)
	}
	scopeRuleJSON, err := json.Marshal(def.ScopeRule)
	if err != nil {
		return fmt.Errorf("migration: marshal scope rule: %w", err)
	}
	resourceLimitsJSON, err := json.Marshal(def.ResourceLimits)
	if err != nil {
		return fmt.Errorf("migration: marshal resource limits: %w", err)
	}
	now := time.Now().UTC()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO extension_migration_definitions
			(migration_id, extension_id, module_id, from_version_range, to_version, entry, runtime_type, direction,
			 data_domains_json, input_schema, output_schema, checkpoint_schema,
			 idempotency, reversibility, forward_migration_id, reverse_migration_id,
			 precondition_json, postcondition_json, permission_reqs_json, scope_rule_json, resource_limits_json,
			 definition_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(migration_id) DO UPDATE SET
			extension_id = excluded.extension_id, module_id = excluded.module_id,
			from_version_range = excluded.from_version_range, to_version = excluded.to_version,
			entry = excluded.entry, runtime_type = excluded.runtime_type, direction = excluded.direction,
			data_domains_json = excluded.data_domains_json, input_schema = excluded.input_schema,
			output_schema = excluded.output_schema, checkpoint_schema = excluded.checkpoint_schema,
			idempotency = excluded.idempotency, reversibility = excluded.reversibility,
			forward_migration_id = excluded.forward_migration_id, reverse_migration_id = excluded.reverse_migration_id,
			precondition_json = excluded.precondition_json, postcondition_json = excluded.postcondition_json,
			permission_reqs_json = excluded.permission_reqs_json, scope_rule_json = excluded.scope_rule_json,
			resource_limits_json = excluded.resource_limits_json, definition_hash = excluded.definition_hash
	`,
		def.MigrationID, def.ExtensionID, def.ModuleID,
		def.FromVersionRange, def.ToVersion,
		def.Entry, def.RuntimeType, string(def.Direction),
		string(dataDomainsJSON), nullableString(string(def.InputSchema)), nullableString(string(def.OutputSchema)), nullableString(string(def.CheckpointSchema)),
		string(def.Idempotency), string(def.Reversibility),
		nullableStringPtr(def.ForwardMigrationID), nullableStringPtr(def.ReverseMigrationID),
		string(preconditionJSON), string(postconditionJSON), string(permReqsJSON), string(scopeRuleJSON), string(resourceLimitsJSON),
		def.DefinitionHash, now,
	)
	if err != nil {
		return fmt.Errorf("migration: upsert migration definition: %w", err)
	}
	return nil
}

func scanMigrationDefinition(scanner rowScanner) (*MigrationDefinition, error) {
	var def MigrationDefinition
	var dataDomainsJSON, inputSchema, outputSchema, checkpointSchema sql.NullString
	var forwardMigrationID, reverseMigrationID sql.NullString
	var preconditionJSON, postconditionJSON, permReqsJSON, scopeRuleJSON, resourceLimitsJSON sql.NullString
	var createdAt time.Time

	err := scanner.Scan(
		&def.MigrationID, &def.ExtensionID, &def.ModuleID,
		&def.FromVersionRange, &def.ToVersion,
		&def.Entry, &def.RuntimeType, &def.Direction,
		&dataDomainsJSON, &inputSchema, &outputSchema, &checkpointSchema,
		&def.Idempotency, &def.Reversibility,
		&forwardMigrationID, &reverseMigrationID,
		&preconditionJSON, &postconditionJSON, &permReqsJSON, &scopeRuleJSON, &resourceLimitsJSON,
		&def.DefinitionHash, &createdAt,
	)
	if err != nil {
		return nil, err
	}
	if dataDomainsJSON.Valid && dataDomainsJSON.String != "" {
		if err := json.Unmarshal([]byte(dataDomainsJSON.String), &def.DataDomains); err != nil {
			return nil, fmt.Errorf("migration: unmarshal data domains: %w", err)
		}
	}
	if inputSchema.Valid && inputSchema.String != "" {
		def.InputSchema = json.RawMessage(inputSchema.String)
	}
	if outputSchema.Valid && outputSchema.String != "" {
		def.OutputSchema = json.RawMessage(outputSchema.String)
	}
	if checkpointSchema.Valid && checkpointSchema.String != "" {
		def.CheckpointSchema = json.RawMessage(checkpointSchema.String)
	}
	def.ForwardMigrationID = stringPtr(forwardMigrationID)
	def.ReverseMigrationID = stringPtr(reverseMigrationID)
	if preconditionJSON.Valid && preconditionJSON.String != "" {
		if err := json.Unmarshal([]byte(preconditionJSON.String), &def.Precondition); err != nil {
			return nil, fmt.Errorf("migration: unmarshal precondition: %w", err)
		}
	}
	if postconditionJSON.Valid && postconditionJSON.String != "" {
		if err := json.Unmarshal([]byte(postconditionJSON.String), &def.Postcondition); err != nil {
			return nil, fmt.Errorf("migration: unmarshal postcondition: %w", err)
		}
	}
	if permReqsJSON.Valid && permReqsJSON.String != "" {
		if err := json.Unmarshal([]byte(permReqsJSON.String), &def.PermissionRequirements); err != nil {
			return nil, fmt.Errorf("migration: unmarshal permission requirements: %w", err)
		}
	}
	if scopeRuleJSON.Valid && scopeRuleJSON.String != "" {
		if err := json.Unmarshal([]byte(scopeRuleJSON.String), &def.ScopeRule); err != nil {
			return nil, fmt.Errorf("migration: unmarshal scope rule: %w", err)
		}
	}
	if resourceLimitsJSON.Valid && resourceLimitsJSON.String != "" {
		if err := json.Unmarshal([]byte(resourceLimitsJSON.String), &def.ResourceLimits); err != nil {
			return nil, fmt.Errorf("migration: unmarshal resource limits: %w", err)
		}
	}
	return &def, nil
}

func (r *MigrationRepository) GetMigrationDefinition(ctx context.Context, migrationID string) (*MigrationDefinition, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT migration_id, extension_id, module_id, from_version_range, to_version, entry, runtime_type, direction,
		       data_domains_json, input_schema, output_schema, checkpoint_schema,
		       idempotency, reversibility, forward_migration_id, reverse_migration_id,
		       precondition_json, postcondition_json, permission_reqs_json, scope_rule_json, resource_limits_json,
		       definition_hash, created_at
		FROM extension_migration_definitions WHERE migration_id = ?
	`, migrationID)
	def, err := scanMigrationDefinition(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("migration: definition not found: %s", migrationID)
		}
		return nil, fmt.Errorf("migration: query migration definition: %w", err)
	}
	return def, nil
}

func (r *MigrationRepository) ListMigrationDefinitions(ctx context.Context, extensionID string) ([]MigrationDefinition, error) {
	var rows *sql.Rows
	var err error
	if extensionID != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT migration_id, extension_id, module_id, from_version_range, to_version, entry, runtime_type, direction,
			       data_domains_json, input_schema, output_schema, checkpoint_schema,
			       idempotency, reversibility, forward_migration_id, reverse_migration_id,
			       precondition_json, postcondition_json, permission_reqs_json, scope_rule_json, resource_limits_json,
			       definition_hash, created_at
			FROM extension_migration_definitions WHERE extension_id = ? ORDER BY created_at DESC
		`, extensionID)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT migration_id, extension_id, module_id, from_version_range, to_version, entry, runtime_type, direction,
			       data_domains_json, input_schema, output_schema, checkpoint_schema,
			       idempotency, reversibility, forward_migration_id, reverse_migration_id,
			       precondition_json, postcondition_json, permission_reqs_json, scope_rule_json, resource_limits_json,
			       definition_hash, created_at
			FROM extension_migration_definitions ORDER BY created_at DESC
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("migration: list migration definitions: %w", err)
	}
	defer rows.Close()
	var out []MigrationDefinition
	for rows.Next() {
		def, err := scanMigrationDefinition(rows)
		if err != nil {
			return nil, fmt.Errorf("migration: scan migration definition: %w", err)
		}
		out = append(out, *def)
	}
	return out, rows.Err()
}

func (r *MigrationRepository) SaveMigrationOperation(ctx context.Context, op *MigrationOperation) error {
	pathJSON, err := json.Marshal(op.MigrationPath)
	if err != nil {
		return fmt.Errorf("migration: marshal migration path: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO extension_migration_operations
			(operation_id, extension_id, from_version, to_version, from_definition_hash, to_definition_hash,
			 migration_path_json, snapshot_id, status, current_step, checkpoint_id, task_run_id,
			 reversibility, requires_user_confirm, user_confirmed, started_at, finished_at, error_code, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(operation_id) DO UPDATE SET
			extension_id = excluded.extension_id, from_version = excluded.from_version, to_version = excluded.to_version,
			from_definition_hash = excluded.from_definition_hash, to_definition_hash = excluded.to_definition_hash,
			migration_path_json = excluded.migration_path_json, snapshot_id = excluded.snapshot_id,
			status = excluded.status, current_step = excluded.current_step, checkpoint_id = excluded.checkpoint_id,
			task_run_id = excluded.task_run_id, reversibility = excluded.reversibility,
			requires_user_confirm = excluded.requires_user_confirm, user_confirmed = excluded.user_confirmed,
			started_at = excluded.started_at, finished_at = excluded.finished_at,
			error_code = excluded.error_code, error_message = excluded.error_message
	`,
		op.OperationID, op.ExtensionID, op.FromVersion, op.ToVersion,
		nullableString(op.FromDefinitionHash), nullableString(op.ToDefinitionHash),
		string(pathJSON), nullableString(op.SnapshotID), string(op.Status), op.CurrentStep,
		nullableString(op.CheckpointID), nullableString(op.TaskRunID),
		string(op.Reversibility), boolToInt(op.RequiresUserConfirm), boolToInt(op.UserConfirmed),
		op.StartedAt, nullableTime(op.FinishedAt),
		nullableString(op.ErrorCode), nullableString(op.ErrorMessage),
	)
	if err != nil {
		return fmt.Errorf("migration: upsert migration operation: %w", err)
	}
	return nil
}

func scanMigrationOperation(scanner rowScanner) (*MigrationOperation, error) {
	var op MigrationOperation
	var fromDefHash, toDefHash, snapshotID, checkpointID, taskRunID sql.NullString
	var pathJSON sql.NullString
	var finishedAt sql.NullTime
	var errorCode, errorMessage sql.NullString
	var requiresUserConfirm, userConfirmed int

	err := scanner.Scan(
		&op.OperationID, &op.ExtensionID, &op.FromVersion, &op.ToVersion,
		&fromDefHash, &toDefHash, &pathJSON, &snapshotID,
		&op.Status, &op.CurrentStep, &checkpointID, &taskRunID,
		&op.Reversibility, &requiresUserConfirm, &userConfirmed,
		&op.StartedAt, &finishedAt, &errorCode, &errorMessage,
	)
	if err != nil {
		return nil, err
	}
	op.FromDefinitionHash = fromDefHash.String
	op.ToDefinitionHash = toDefHash.String
	if pathJSON.Valid && pathJSON.String != "" {
		if err := json.Unmarshal([]byte(pathJSON.String), &op.MigrationPath); err != nil {
			return nil, fmt.Errorf("migration: unmarshal migration path: %w", err)
		}
	}
	op.SnapshotID = snapshotID.String
	op.CheckpointID = checkpointID.String
	op.TaskRunID = taskRunID.String
	op.RequiresUserConfirm = requiresUserConfirm != 0
	op.UserConfirmed = userConfirmed != 0
	op.FinishedAt = timePtr(finishedAt)
	op.ErrorCode = errorCode.String
	op.ErrorMessage = errorMessage.String
	return &op, nil
}

func (r *MigrationRepository) GetMigrationOperation(ctx context.Context, operationID string) (*MigrationOperation, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT operation_id, extension_id, from_version, to_version, from_definition_hash, to_definition_hash,
		       migration_path_json, snapshot_id, status, current_step, checkpoint_id, task_run_id,
		       reversibility, requires_user_confirm, user_confirmed, started_at, finished_at, error_code, error_message
		FROM extension_migration_operations WHERE operation_id = ?
	`, operationID)
	op, err := scanMigrationOperation(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("migration: operation not found: %s", operationID)
		}
		return nil, fmt.Errorf("migration: query migration operation: %w", err)
	}
	return op, nil
}

func (r *MigrationRepository) ListMigrationOperations(ctx context.Context, extensionID string) ([]MigrationOperation, error) {
	var rows *sql.Rows
	var err error
	if extensionID != "" {
		rows, err = r.db.QueryContext(ctx, `
			SELECT operation_id, extension_id, from_version, to_version, from_definition_hash, to_definition_hash,
			       migration_path_json, snapshot_id, status, current_step, checkpoint_id, task_run_id,
			       reversibility, requires_user_confirm, user_confirmed, started_at, finished_at, error_code, error_message
			FROM extension_migration_operations WHERE extension_id = ? ORDER BY started_at DESC
		`, extensionID)
	} else {
		rows, err = r.db.QueryContext(ctx, `
			SELECT operation_id, extension_id, from_version, to_version, from_definition_hash, to_definition_hash,
			       migration_path_json, snapshot_id, status, current_step, checkpoint_id, task_run_id,
			       reversibility, requires_user_confirm, user_confirmed, started_at, finished_at, error_code, error_message
			FROM extension_migration_operations ORDER BY started_at DESC
		`)
	}
	if err != nil {
		return nil, fmt.Errorf("migration: list migration operations: %w", err)
	}
	defer rows.Close()
	var out []MigrationOperation
	for rows.Next() {
		op, err := scanMigrationOperation(rows)
		if err != nil {
			return nil, fmt.Errorf("migration: scan migration operation: %w", err)
		}
		out = append(out, *op)
	}
	return out, rows.Err()
}

func (r *MigrationRepository) UpdateOperationStatus(ctx context.Context, operationID string, status MigrationOperationStatus, errorCode, errorMessage string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE extension_migration_operations
		SET status = ?, error_code = ?, error_message = ?
		WHERE operation_id = ?
	`, string(status), nullableString(errorCode), nullableString(errorMessage), operationID)
	if err != nil {
		return fmt.Errorf("migration: update operation status: %w", err)
	}
	return nil
}

func (r *MigrationRepository) SaveMigrationStep(ctx context.Context, step *MigrationStepRecord) error {
	id := fmt.Sprintf("%s-%d", step.OperationID, step.StepID)
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO extension_migration_steps
			(id, operation_id, step_id, migration_id, status, input_hash, output_hash, checkpoint_id,
			 started_at, finished_at, error_code, error_message)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(operation_id, step_id) DO UPDATE SET
			migration_id = excluded.migration_id, status = excluded.status,
			input_hash = excluded.input_hash, output_hash = excluded.output_hash,
			checkpoint_id = excluded.checkpoint_id, started_at = excluded.started_at,
			finished_at = excluded.finished_at, error_code = excluded.error_code,
			error_message = excluded.error_message
	`,
		id, step.OperationID, step.StepID, step.MigrationID, step.Status,
		nullableString(step.InputHash), nullableString(step.OutputHash), nullableString(step.CheckpointID),
		step.StartedAt, nullableTime(step.FinishedAt),
		nullableString(step.ErrorCode), nullableString(step.ErrorMessage),
	)
	if err != nil {
		return fmt.Errorf("migration: upsert migration step: %w", err)
	}
	return nil
}

func scanMigrationStep(scanner rowScanner) (*MigrationStepRecord, error) {
	var step MigrationStepRecord
	var id string
	var inputHash, outputHash, checkpointID sql.NullString
	var finishedAt sql.NullTime
	var errorCode, errorMessage sql.NullString

	err := scanner.Scan(
		&id, &step.OperationID, &step.StepID, &step.MigrationID, &step.Status,
		&inputHash, &outputHash, &checkpointID,
		&step.StartedAt, &finishedAt, &errorCode, &errorMessage,
	)
	if err != nil {
		return nil, err
	}
	step.InputHash = inputHash.String
	step.OutputHash = outputHash.String
	step.CheckpointID = checkpointID.String
	step.FinishedAt = timePtr(finishedAt)
	step.ErrorCode = errorCode.String
	step.ErrorMessage = errorMessage.String
	return &step, nil
}

func (r *MigrationRepository) ListMigrationSteps(ctx context.Context, operationID string) ([]MigrationStepRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, operation_id, step_id, migration_id, status, input_hash, output_hash, checkpoint_id,
		       started_at, finished_at, error_code, error_message
		FROM extension_migration_steps WHERE operation_id = ? ORDER BY step_id ASC
	`, operationID)
	if err != nil {
		return nil, fmt.Errorf("migration: list migration steps: %w", err)
	}
	defer rows.Close()
	var out []MigrationStepRecord
	for rows.Next() {
		step, err := scanMigrationStep(rows)
		if err != nil {
			return nil, fmt.Errorf("migration: scan migration step: %w", err)
		}
		out = append(out, *step)
	}
	return out, rows.Err()
}

func (r *MigrationRepository) SaveCheckpoint(ctx context.Context, checkpoint *MigrationCheckpoint) error {
	_, err := r.db.ExecContext(ctx, `
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
		return fmt.Errorf("migration: upsert checkpoint: %w", err)
	}
	return nil
}

func scanCheckpoint(scanner rowScanner) (*MigrationCheckpoint, error) {
	var cp MigrationCheckpoint
	var cursorJSON, inputHash, definitionHash, snapshotID sql.NullString

	err := scanner.Scan(
		&cp.CheckpointID, &cp.OperationID, &cp.StepID,
		&cp.MigrationID, &cp.Stage, &cursorJSON,
		&cp.BatchNumber, &cp.ProcessedCount,
		&inputHash, &definitionHash, &snapshotID, &cp.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if cursorJSON.Valid && cursorJSON.String != "" {
		cp.Cursor = json.RawMessage(cursorJSON.String)
	}
	cp.InputHash = inputHash.String
	cp.DefinitionHash = definitionHash.String
	cp.SnapshotID = snapshotID.String
	return &cp, nil
}

func (r *MigrationRepository) GetCheckpoint(ctx context.Context, checkpointID string) (*MigrationCheckpoint, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT checkpoint_id, operation_id, step_id, migration_id, stage, cursor_json,
		       batch_number, processed_count, input_hash, definition_hash, snapshot_id, created_at
		FROM extension_migration_checkpoints WHERE checkpoint_id = ?
	`, checkpointID)
	cp, err := scanCheckpoint(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("migration: checkpoint not found: %s", checkpointID)
		}
		return nil, fmt.Errorf("migration: query checkpoint: %w", err)
	}
	return cp, nil
}

func (r *MigrationRepository) ListCheckpointsByOperation(ctx context.Context, operationID string) ([]MigrationCheckpoint, error) {
	rows, err := r.db.QueryContext(ctx, `
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

func (r *MigrationRepository) SaveSnapshotManifest(ctx context.Context, manifest *SnapshotManifest) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migration: begin snapshot tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO extension_data_snapshots
			(snapshot_id, extension_id, operation_id, generation, total_bytes, manifest_hash, retention_policy, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(snapshot_id) DO UPDATE SET
			extension_id = excluded.extension_id, operation_id = excluded.operation_id,
			generation = excluded.generation, total_bytes = excluded.total_bytes,
			manifest_hash = excluded.manifest_hash, retention_policy = excluded.retention_policy,
			created_at = excluded.created_at
	`,
		manifest.SnapshotID, manifest.ExtensionID, nullableString(manifest.OperationID),
		manifest.Generation, manifest.TotalBytes, manifest.ManifestHash,
		manifest.RetentionPolicy, manifest.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("migration: upsert snapshot manifest: %w", err)
	}

	_, err = tx.ExecContext(ctx, `DELETE FROM extension_snapshot_entries WHERE snapshot_id = ?`, manifest.SnapshotID)
	if err != nil {
		return fmt.Errorf("migration: delete old snapshot entries: %w", err)
	}

	for _, entry := range manifest.Entries {
		_, err = tx.ExecContext(ctx, `
			INSERT INTO extension_snapshot_entries
				(entry_id, snapshot_id, entry_type, source_path, snap_path, size_bytes, hash, page_count, wal_handled)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
			ON CONFLICT(entry_id) DO UPDATE SET
				snapshot_id = excluded.snapshot_id, entry_type = excluded.entry_type,
				source_path = excluded.source_path, snap_path = excluded.snap_path,
				size_bytes = excluded.size_bytes, hash = excluded.hash,
				page_count = excluded.page_count, wal_handled = excluded.wal_handled
		`,
			entry.EntryID, entry.SnapshotID, string(entry.Type),
			entry.SourcePath, entry.SnapPath, entry.SizeBytes, entry.Hash,
			entry.PageCount, boolToInt(entry.WALHandled),
		)
		if err != nil {
			return fmt.Errorf("migration: upsert snapshot entry: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration: commit snapshot manifest: %w", err)
	}
	return nil
}

func (r *MigrationRepository) loadSnapshotEntries(ctx context.Context, snapshotID string) ([]SnapshotEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT entry_id, snapshot_id, entry_type, source_path, snap_path, size_bytes, hash, page_count, wal_handled
		FROM extension_snapshot_entries WHERE snapshot_id = ?
	`, snapshotID)
	if err != nil {
		return nil, fmt.Errorf("migration: query snapshot entries: %w", err)
	}
	defer rows.Close()
	var out []SnapshotEntry
	for rows.Next() {
		var entry SnapshotEntry
		var entryType string
		var pageCount int
		var walHandled int
		err := rows.Scan(
			&entry.EntryID, &entry.SnapshotID, &entryType,
			&entry.SourcePath, &entry.SnapPath, &entry.SizeBytes, &entry.Hash,
			&pageCount, &walHandled,
		)
		if err != nil {
			return nil, fmt.Errorf("migration: scan snapshot entry: %w", err)
		}
		entry.Type = SnapshotEntryType(entryType)
		entry.PageCount = pageCount
		entry.WALHandled = walHandled != 0
		out = append(out, entry)
	}
	return out, rows.Err()
}

func scanSnapshotManifest(scanner rowScanner) (*SnapshotManifest, error) {
	var manifest SnapshotManifest
	var operationID sql.NullString

	err := scanner.Scan(
		&manifest.SnapshotID, &manifest.ExtensionID, &operationID,
		&manifest.Generation, &manifest.TotalBytes, &manifest.ManifestHash,
		&manifest.RetentionPolicy, &manifest.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	manifest.OperationID = operationID.String
	return &manifest, nil
}

func (r *MigrationRepository) GetSnapshotManifest(ctx context.Context, snapshotID string) (*SnapshotManifest, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT snapshot_id, extension_id, operation_id, generation, total_bytes, manifest_hash, retention_policy, created_at
		FROM extension_data_snapshots WHERE snapshot_id = ?
	`, snapshotID)
	manifest, err := scanSnapshotManifest(row)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("migration: snapshot manifest not found: %s", snapshotID)
		}
		return nil, fmt.Errorf("migration: query snapshot manifest: %w", err)
	}
	entries, err := r.loadSnapshotEntries(ctx, snapshotID)
	if err != nil {
		return nil, err
	}
	manifest.Entries = entries
	return manifest, nil
}

func (r *MigrationRepository) ListSnapshotsByExtension(ctx context.Context, extensionID string) ([]SnapshotManifest, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT snapshot_id, extension_id, operation_id, generation, total_bytes, manifest_hash, retention_policy, created_at
		FROM extension_data_snapshots WHERE extension_id = ? ORDER BY created_at DESC
	`, extensionID)
	if err != nil {
		return nil, fmt.Errorf("migration: list snapshots by extension: %w", err)
	}
	defer rows.Close()
	var out []SnapshotManifest
	for rows.Next() {
		manifest, err := scanSnapshotManifest(rows)
		if err != nil {
			return nil, fmt.Errorf("migration: scan snapshot manifest: %w", err)
		}
		entries, err := r.loadSnapshotEntries(ctx, manifest.SnapshotID)
		if err != nil {
			return nil, err
		}
		manifest.Entries = entries
		out = append(out, *manifest)
	}
	return out, rows.Err()
}

func (r *MigrationRepository) DeleteSnapshotManifest(ctx context.Context, snapshotID string) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("migration: begin delete snapshot tx: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, `DELETE FROM extension_snapshot_entries WHERE snapshot_id = ?`, snapshotID)
	if err != nil {
		return fmt.Errorf("migration: delete snapshot entries: %w", err)
	}
	_, err = tx.ExecContext(ctx, `DELETE FROM extension_data_snapshots WHERE snapshot_id = ?`, snapshotID)
	if err != nil {
		return fmt.Errorf("migration: delete snapshot manifest: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migration: commit delete snapshot: %w", err)
	}
	return nil
}
