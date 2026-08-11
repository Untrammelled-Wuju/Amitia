package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/task_runtime"
)

type TaskRepository struct {
	db *sql.DB
}

func NewTaskRepository(db *sql.DB) *TaskRepository {
	return &TaskRepository{db: db}
}

func (r *TaskRepository) PutTaskDefinition(ctx context.Context, def *task_runtime.TaskDefinition) error {
	now := time.Now().UTC()
	defJSON, err := json.Marshal(def)
	if err != nil {
		return fmt.Errorf("sqlite: marshal task definition: %w", err)
	}
	ex := getExecutor(ctx, r.db)
	_, err = ex.ExecContext(ctx, `
		INSERT INTO extension_task_definitions
			(task_definition_id, extension_id, module_id, contribution_id, runtime_type, entry, definition_json, definition_hash, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_definition_id) DO UPDATE SET
			definition_json = excluded.definition_json,
			definition_hash = excluded.definition_hash,
			updated_at = excluded.updated_at
	`,
		def.TaskID, def.ExtensionID, def.ModuleID, def.ContributionID,
		def.RuntimeType, def.Entry, string(defJSON), def.DefinitionHash,
		now, now,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert task definition: %w", err)
	}
	return nil
}

func (r *TaskRepository) GetTaskDefinition(ctx context.Context, defID string) (*task_runtime.TaskDefinition, error) {
	ex := getExecutor(ctx, r.db)
	var defJSON string
	err := ex.QueryRowContext(ctx,
		`SELECT definition_json FROM extension_task_definitions WHERE task_definition_id = ?`, defID,
	).Scan(&defJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: task definition not found: %s", defID)
		}
		return nil, fmt.Errorf("sqlite: query task definition: %w", err)
	}
	var def task_runtime.TaskDefinition
	if err := json.Unmarshal([]byte(defJSON), &def); err != nil {
		return nil, fmt.Errorf("sqlite: unmarshal task definition: %w", err)
	}
	return &def, nil
}

func (r *TaskRepository) ListTaskDefinitions(ctx context.Context, extensionID string) ([]*task_runtime.TaskDefinition, error) {
	ex := getExecutor(ctx, r.db)
	var rows *sql.Rows
	var err error
	if extensionID != "" {
		rows, err = ex.QueryContext(ctx, `SELECT definition_json FROM extension_task_definitions WHERE extension_id = ? ORDER BY created_at DESC`, extensionID)
	} else {
		rows, err = ex.QueryContext(ctx, `SELECT definition_json FROM extension_task_definitions ORDER BY created_at DESC`)
	}
	if err != nil {
		return nil, fmt.Errorf("sqlite: list task definitions: %w", err)
	}
	defer rows.Close()
	var out []*task_runtime.TaskDefinition
	for rows.Next() {
		var defJSON string
		if err := rows.Scan(&defJSON); err != nil {
			return nil, fmt.Errorf("sqlite: scan task definition: %w", err)
		}
		var def task_runtime.TaskDefinition
		if err := json.Unmarshal([]byte(defJSON), &def); err != nil {
			return nil, fmt.Errorf("sqlite: unmarshal task definition: %w", err)
		}
		out = append(out, &def)
	}
	return out, rows.Err()
}

func (r *TaskRepository) DeleteTaskDefinition(ctx context.Context, defID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_task_definitions WHERE task_id = ?`, defID)
	if err != nil {
		return fmt.Errorf("sqlite: delete task definition: %w", err)
	}
	return nil
}

func (r *TaskRepository) DeleteByExtension(ctx context.Context, extensionID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_task_definitions WHERE extension_id = ?`, extensionID)
	if err != nil {
		return fmt.Errorf("sqlite: delete task definitions by extension: %w", err)
	}
	return nil
}

func (r *TaskRepository) PutTaskRun(ctx context.Context, run *task_runtime.TaskRun) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_task_runs
			(task_run_id, operation_id, invocation_id, task_definition_id, extension_id, module_id,
			 status, priority, input_json, input_hash, input_artifact_id,
			 scope_snapshot_id, permission_snapshot_id, dependency_snapshot_id,
			 runtime_instance_id, checkpoint_id, result_artifact_id,
			 attempt, max_attempts, created_at, queued_at, started_at, finished_at,
			 deadline_at, cancel_requested_at, error_code, error_message, generation)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_run_id) DO UPDATE SET
			status = excluded.status, priority = excluded.priority,
			input_json = excluded.input_json, input_hash = excluded.input_hash,
			runtime_instance_id = excluded.runtime_instance_id, checkpoint_id = excluded.checkpoint_id,
			result_artifact_id = excluded.result_artifact_id,
			attempt = excluded.attempt, max_attempts = excluded.max_attempts,
			queued_at = excluded.queued_at, started_at = excluded.started_at,
			finished_at = excluded.finished_at, deadline_at = excluded.deadline_at,
			cancel_requested_at = excluded.cancel_requested_at,
			error_code = excluded.error_code, error_message = excluded.error_message,
			generation = excluded.generation
	`,
		run.TaskRunID, run.OperationID, run.InvocationID, run.TaskDefinitionID,
		run.ExtensionID, run.ModuleID, string(run.Status), run.Priority,
		string(run.Input), run.InputHash, nullableString(run.InputArtifactID),
		run.ScopeSnapshotID, run.PermissionSnapshotID, run.DependencySnapshotID,
		nullableString(run.RuntimeInstanceID), nullableString(run.CheckpointID),
		nullableString(run.ResultArtifactID),
		run.Attempt, run.MaxAttempts, run.CreatedAt,
		nullableTime(run.QueuedAt), nullableTime(run.StartedAt), nullableTime(run.FinishedAt),
		nullableTime(run.DeadlineAt), nullableTime(run.CancelRequestedAt),
		nullableString(run.ErrorCode), nullableString(run.ErrorMessage),
		run.Generation,
	)
	if err != nil {
		return fmt.Errorf("sqlite: upsert task run: %w", err)
	}
	return nil
}

func (r *TaskRepository) UpdateTaskRunCAS(ctx context.Context, run *task_runtime.TaskRun, expectedStatus task_runtime.TaskRunStatus, expectedGeneration int64) (bool, error) {
	ex := getExecutor(ctx, r.db)
	res, err := ex.ExecContext(ctx, `
		UPDATE extension_task_runs
		SET status = ?, priority = ?,
		    input_json = ?, input_hash = ?,
		    runtime_instance_id = ?, checkpoint_id = ?,
		    result_artifact_id = ?,
		    attempt = ?, max_attempts = ?,
		    queued_at = ?, started_at = ?, finished_at = ?,
		    deadline_at = ?, cancel_requested_at = ?,
		    pause_reason = ?, pause_requested_at = ?, paused_at = ?, resumed_at = ?,
		    error_code = ?, error_message = ?,
		    generation = ?
		WHERE task_run_id = ? AND status = ? AND generation = ?
	`,
		string(run.Status), run.Priority,
		string(run.Input), run.InputHash,
		nullableString(run.RuntimeInstanceID), nullableString(run.CheckpointID),
		nullableString(run.ResultArtifactID),
		run.Attempt, run.MaxAttempts,
		nullableTime(run.QueuedAt), nullableTime(run.StartedAt), nullableTime(run.FinishedAt),
		nullableTime(run.DeadlineAt), nullableTime(run.CancelRequestedAt),
		nullableString(run.PauseReason), nullableTime(run.PauseRequestedAt), nullableTime(run.PausedAt), nullableTime(run.ResumedAt),
		nullableString(run.ErrorCode), nullableString(run.ErrorMessage),
		run.Generation,
		run.TaskRunID, string(expectedStatus), expectedGeneration,
	)
	if err != nil {
		return false, fmt.Errorf("sqlite: cas update task run: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return false, nil
	}
	return true, nil
}

func (r *TaskRepository) GetTaskRun(ctx context.Context, runID string) (*task_runtime.TaskRun, error) {
	ex := getExecutor(ctx, r.db)
	var run task_runtime.TaskRun
	var inputJSON sql.NullString
	var inputArtifactID, runtimeInstanceID, checkpointID, resultArtifactID sql.NullString
	var queuedAt, startedAt, finishedAt, deadlineAt, cancelRequestedAt sql.NullTime
	var errorCode, errorMessage sql.NullString
	var invocationID sql.NullString

	err := ex.QueryRowContext(ctx, `
		SELECT task_run_id, operation_id, invocation_id, task_definition_id, extension_id, module_id,
		       status, priority, input_json, input_hash, input_artifact_id,
		       scope_snapshot_id, permission_snapshot_id, dependency_snapshot_id,
		       runtime_instance_id, checkpoint_id, result_artifact_id,
		       attempt, max_attempts, created_at, queued_at, started_at, finished_at,
		       deadline_at, cancel_requested_at, error_code, error_message, generation
		FROM extension_task_runs WHERE task_run_id = ?
	`, runID).Scan(
		&run.TaskRunID, &run.OperationID, &invocationID, &run.TaskDefinitionID,
		&run.ExtensionID, &run.ModuleID, &run.Status, &run.Priority,
		&inputJSON, &run.InputHash, &inputArtifactID,
		&run.ScopeSnapshotID, &run.PermissionSnapshotID, &run.DependencySnapshotID,
		&runtimeInstanceID, &checkpointID, &resultArtifactID,
		&run.Attempt, &run.MaxAttempts, &run.CreatedAt,
		&queuedAt, &startedAt, &finishedAt, &deadlineAt, &cancelRequestedAt,
		&errorCode, &errorMessage, &run.Generation,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("sqlite: task run not found: %s", runID)
		}
		return nil, fmt.Errorf("sqlite: query task run: %w", err)
	}
	run.Input = json.RawMessage(inputJSON.String)
	run.InvocationID = invocationID.String
	run.InputArtifactID = stringPtr(inputArtifactID)
	run.RuntimeInstanceID = stringPtr(runtimeInstanceID)
	run.CheckpointID = stringPtr(checkpointID)
	run.ResultArtifactID = stringPtr(resultArtifactID)
	run.QueuedAt = timePtr(queuedAt)
	run.StartedAt = timePtr(startedAt)
	run.FinishedAt = timePtr(finishedAt)
	run.DeadlineAt = timePtr(deadlineAt)
	run.CancelRequestedAt = timePtr(cancelRequestedAt)
	run.ErrorCode = stringPtr(errorCode)
	run.ErrorMessage = stringPtr(errorMessage)
	return &run, nil
}

func (r *TaskRepository) ListTaskRuns(ctx context.Context, filter task_runtime.ListTasksFilter) ([]*task_runtime.TaskRun, error) {
	ex := getExecutor(ctx, r.db)
	query := `SELECT task_run_id, operation_id, invocation_id, task_definition_id, extension_id, module_id,
		       status, priority, input_json, input_hash, input_artifact_id,
		       scope_snapshot_id, permission_snapshot_id, dependency_snapshot_id,
		       runtime_instance_id, checkpoint_id, result_artifact_id,
		       attempt, max_attempts, created_at, queued_at, started_at, finished_at,
		       deadline_at, cancel_requested_at, error_code, error_message, generation
		FROM extension_task_runs`
	var args []interface{}
	where := ""
	if filter.ExtensionID != "" {
		where = " WHERE extension_id = ?"
		args = append(args, filter.ExtensionID)
	}
	if filter.Status != "" {
		if where != "" {
			where += " AND status = ?"
		} else {
			where = " WHERE status = ?"
		}
		args = append(args, filter.Status)
	}
	query += where + " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	rows, err := ex.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("sqlite: list task runs: %w", err)
	}
	defer rows.Close()

	var out []*task_runtime.TaskRun
	for rows.Next() {
		var run task_runtime.TaskRun
		var inputJSON sql.NullString
		var inputArtifactID, runtimeInstanceID, checkpointID, resultArtifactID sql.NullString
		var queuedAt, startedAt, finishedAt, deadlineAt, cancelRequestedAt sql.NullTime
		var errorCode, errorMessage sql.NullString
		var invocationID sql.NullString

		err := rows.Scan(
			&run.TaskRunID, &run.OperationID, &invocationID, &run.TaskDefinitionID,
			&run.ExtensionID, &run.ModuleID, &run.Status, &run.Priority,
			&inputJSON, &run.InputHash, &inputArtifactID,
			&run.ScopeSnapshotID, &run.PermissionSnapshotID, &run.DependencySnapshotID,
			&runtimeInstanceID, &checkpointID, &resultArtifactID,
			&run.Attempt, &run.MaxAttempts, &run.CreatedAt,
			&queuedAt, &startedAt, &finishedAt, &deadlineAt, &cancelRequestedAt,
			&errorCode, &errorMessage, &run.Generation,
		)
		if err != nil {
			return nil, fmt.Errorf("sqlite: scan task run: %w", err)
		}
		run.Input = json.RawMessage(inputJSON.String)
		run.InvocationID = invocationID.String
		run.InputArtifactID = stringPtr(inputArtifactID)
		run.RuntimeInstanceID = stringPtr(runtimeInstanceID)
		run.CheckpointID = stringPtr(checkpointID)
		run.ResultArtifactID = stringPtr(resultArtifactID)
		run.QueuedAt = timePtr(queuedAt)
		run.StartedAt = timePtr(startedAt)
		run.FinishedAt = timePtr(finishedAt)
		run.DeadlineAt = timePtr(deadlineAt)
		run.CancelRequestedAt = timePtr(cancelRequestedAt)
		run.ErrorCode = stringPtr(errorCode)
		run.ErrorMessage = stringPtr(errorMessage)
		out = append(out, &run)
	}
	return out, rows.Err()
}

func (r *TaskRepository) ListTaskRunsByStatus(ctx context.Context, status string) ([]*task_runtime.TaskRun, error) {
	return r.ListTaskRuns(ctx, task_runtime.ListTasksFilter{Status: status})
}

func (r *TaskRepository) EnqueueTask(ctx context.Context, entry *task_runtime.TaskQueueEntry) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_task_queue (task_run_id, priority, available_at, lease_owner, lease_expires_at, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_run_id) DO UPDATE SET
			priority = excluded.priority, available_at = excluded.available_at,
			lease_owner = NULL, lease_expires_at = NULL
	`,
		entry.TaskRunID, entry.Priority, entry.AvailableAt,
		nil, nil, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: enqueue task: %w", err)
	}
	return nil
}

func (r *TaskRepository) DequeueTask(ctx context.Context, leaseOwner string, leaseDuration time.Duration) (*task_runtime.TaskQueueEntry, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("sqlite: begin dequeue tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now().UTC()
	leaseExpires := now.Add(leaseDuration)

	row := tx.QueryRowContext(ctx, `
		SELECT task_run_id, priority, available_at, created_at
		FROM extension_task_queue
		WHERE (lease_expires_at IS NULL OR lease_expires_at < ?)
		  AND available_at <= ?
		ORDER BY priority DESC, available_at ASC, created_at ASC
		LIMIT 1
	`, now, now)

	var entry task_runtime.TaskQueueEntry
	err = row.Scan(&entry.TaskRunID, &entry.Priority, &entry.AvailableAt, &entry.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("sqlite: dequeue query: %w", err)
	}

	_, err = tx.ExecContext(ctx, `
		UPDATE extension_task_queue SET lease_owner = ?, lease_expires_at = ? WHERE task_run_id = ?
	`, leaseOwner, leaseExpires, entry.TaskRunID)
	if err != nil {
		return nil, fmt.Errorf("sqlite: lease task: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("sqlite: commit dequeue: %w", err)
	}

	entry.LeaseOwner = leaseOwner
	entry.LeaseExpiresAt = &leaseExpires
	return &entry, nil
}

func (r *TaskRepository) RemoveFromQueue(ctx context.Context, taskRunID string) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `DELETE FROM extension_task_queue WHERE task_run_id = ?`, taskRunID)
	if err != nil {
		return fmt.Errorf("sqlite: remove from queue: %w", err)
	}
	return nil
}

func (r *TaskRepository) ReclaimExpiredLeases(ctx context.Context) (int, error) {
	ex := getExecutor(ctx, r.db)
	now := time.Now().UTC()
	res, err := ex.ExecContext(ctx, `
		UPDATE extension_task_queue SET lease_owner = NULL, lease_expires_at = NULL
		WHERE lease_expires_at IS NOT NULL AND lease_expires_at < ?
	`, now)
	if err != nil {
		return 0, fmt.Errorf("sqlite: reclaim expired leases: %w", err)
	}
	n, _ := res.RowsAffected()
	return int(n), nil
}

func (r *TaskRepository) GetQueueEntry(ctx context.Context, taskRunID string) (*task_runtime.TaskQueueEntry, error) {
	ex := getExecutor(ctx, r.db)
	var entry task_runtime.TaskQueueEntry
	var leaseOwner sql.NullString
	var leaseExpires sql.NullTime
	err := ex.QueryRowContext(ctx, `
		SELECT task_run_id, priority, available_at, lease_owner, lease_expires_at, created_at
		FROM extension_task_queue WHERE task_run_id = ?
	`, taskRunID).Scan(&entry.TaskRunID, &entry.Priority, &entry.AvailableAt, &leaseOwner, &leaseExpires, &entry.CreatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("sqlite: get queue entry: %w", err)
	}
	entry.LeaseOwner = leaseOwner.String
	entry.LeaseExpiresAt = timePtr(leaseExpires)
	return &entry, nil
}

func (r *TaskRepository) PutCheckpoint(ctx context.Context, cp *task_runtime.TaskCheckpoint) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_task_checkpoints
			(checkpoint_id, task_run_id, checkpoint_version, payload_json, payload_hash, definition_hash, input_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(checkpoint_id) DO UPDATE SET
			payload_json = excluded.payload_json, payload_hash = excluded.payload_hash
	`,
		cp.CheckpointID, cp.TaskRunID, cp.Version,
		string(cp.Payload), cp.PayloadHash, cp.DefinitionHash, cp.InputHash, cp.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: put checkpoint: %w", err)
	}
	return nil
}

func (r *TaskRepository) GetLatestCheckpoint(ctx context.Context, taskRunID string) (*task_runtime.TaskCheckpoint, error) {
	ex := getExecutor(ctx, r.db)
	var cp task_runtime.TaskCheckpoint
	var payloadJSON string
	var defHash, inputHash sql.NullString
	err := ex.QueryRowContext(ctx, `
		SELECT checkpoint_id, task_run_id, checkpoint_version, payload_json, payload_hash, definition_hash, input_hash, created_at
		FROM extension_task_checkpoints WHERE task_run_id = ?
		ORDER BY checkpoint_version DESC LIMIT 1
	`, taskRunID).Scan(
		&cp.CheckpointID, &cp.TaskRunID, &cp.Version, &payloadJSON,
		&cp.PayloadHash, &defHash, &inputHash, &cp.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("sqlite: get latest checkpoint: %w", err)
	}
	cp.Payload = json.RawMessage(payloadJSON)
	cp.DefinitionHash = defHash.String
	cp.InputHash = inputHash.String
	return &cp, nil
}

func (r *TaskRepository) PutProgress(ctx context.Context, taskRunID string, seq int64, progressJSON []byte) error {
	ex := getExecutor(ctx, r.db)
	now := time.Now().UTC()
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_task_progress (task_run_id, sequence, progress_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(task_run_id) DO UPDATE SET
			sequence = excluded.sequence, progress_json = excluded.progress_json, updated_at = excluded.updated_at
	`, taskRunID, seq, string(progressJSON), now)
	if err != nil {
		return fmt.Errorf("sqlite: put progress: %w", err)
	}
	return nil
}

func (r *TaskRepository) GetProgress(ctx context.Context, taskRunID string) (*task_runtime.TaskRunProgress, error) {
	ex := getExecutor(ctx, r.db)
	var prog task_runtime.TaskRunProgress
	var progressJSON string
	err := ex.QueryRowContext(ctx, `
		SELECT task_run_id, sequence, progress_json, updated_at
		FROM extension_task_progress WHERE task_run_id = ?
	`, taskRunID).Scan(&prog.TaskRunID, &prog.Sequence, &progressJSON, &prog.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("sqlite: get progress: %w", err)
	}
	prog.Details = json.RawMessage(progressJSON)
	return &prog, nil
}

func (r *TaskRepository) PutResult(ctx context.Context, result *task_runtime.TaskRunResult) error {
	ex := getExecutor(ctx, r.db)
	_, err := ex.ExecContext(ctx, `
		INSERT INTO extension_task_results (task_run_id, result_type, result_json, artifact_id, result_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(task_run_id) DO UPDATE SET
			result_type = excluded.result_type, result_json = excluded.result_json,
			artifact_id = excluded.artifact_id, result_hash = excluded.result_hash
	`,
		result.TaskRunID, string(result.ResultType),
		string(result.ResultJSON), result.ArtifactID, result.ResultHash, result.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("sqlite: put result: %w", err)
	}
	return nil
}

func (r *TaskRepository) GetResult(ctx context.Context, taskRunID string) (*task_runtime.TaskRunResult, error) {
	ex := getExecutor(ctx, r.db)
	var result task_runtime.TaskRunResult
	var resultJSON sql.NullString
	var artifactID sql.NullString
	err := ex.QueryRowContext(ctx, `
		SELECT task_run_id, result_type, result_json, artifact_id, result_hash, created_at
		FROM extension_task_results WHERE task_run_id = ?
	`, taskRunID).Scan(
		&result.TaskRunID, &result.ResultType, &resultJSON,
		&artifactID, &result.ResultHash, &result.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("sqlite: get result: %w", err)
	}
	if resultJSON.Valid {
		result.ResultJSON = json.RawMessage(resultJSON.String)
	}
	result.ArtifactID = artifactID.String
	return &result, nil
}

func (r *TaskRepository) CountActiveByExtension(ctx context.Context, extensionID string) (int, error) {
	ex := getExecutor(ctx, r.db)
	var count int
	err := ex.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM extension_task_runs
		WHERE extension_id = ? AND status IN ('starting', 'running', 'checkpointing', 'pausing', 'resuming')
	`, extensionID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: count active by extension: %w", err)
	}
	return count, nil
}

func (r *TaskRepository) CountActiveByDefinition(ctx context.Context, defID string) (int, error) {
	ex := getExecutor(ctx, r.db)
	var count int
	err := ex.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM extension_task_runs
		WHERE task_definition_id = ? AND status IN ('starting', 'running', 'checkpointing', 'pausing', 'resuming')
	`, defID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: count active by definition: %w", err)
	}
	return count, nil
}

func (r *TaskRepository) CountActive(ctx context.Context) (int, error) {
	ex := getExecutor(ctx, r.db)
	var count int
	err := ex.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM extension_task_runs
		WHERE status IN ('starting', 'running', 'checkpointing', 'pausing', 'resuming')
	`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("sqlite: count active: %w", err)
	}
	return count, nil
}

var _ task_runtime.TaskStore = (*TaskRepository)(nil)

func nullableString(s *string) interface{} {
	if s == nil {
		return nil
	}
	return *s
}

func stringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	v := ns.String
	return &v
}

func nullableTime(t *time.Time) interface{} {
	if t == nil {
		return nil
	}
	return *t
}

func timePtr(nt sql.NullTime) *time.Time {
	if !nt.Valid {
		return nil
	}
	t := nt.Time.UTC()
	return &t
}
