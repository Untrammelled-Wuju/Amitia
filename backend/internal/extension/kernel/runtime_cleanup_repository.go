package kernel

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type RuntimeCleanupState string

const (
	CleanupStatePending                RuntimeCleanupState = "pending"
	CleanupStateDraining               RuntimeCleanupState = "draining"
	CleanupStateDrained                RuntimeCleanupState = "drained"
	CleanupStateStopping               RuntimeCleanupState = "stopping"
	CleanupStateStopFailed             RuntimeCleanupState = "stop_failed"
	CleanupStateStopped                RuntimeCleanupState = "stopped"
	CleanupStateVerified               RuntimeCleanupState = "verified"
	CleanupStateCompleted              RuntimeCleanupState = "completed"
	CleanupStateRequiresManualRecovery RuntimeCleanupState = "requires_manual_recovery"
)

type RuntimeCleanupTask struct {
	CleanupID           string
	ExtensionID         string
	ModuleID            string
	OldGeneration       int64
	RuntimeDefinitionID string
	RuntimeInstanceID   string
	RuntimeType         string
	ProcessID           int
	CleanupState        RuntimeCleanupState
	AttemptCount        int
	LastErrorCode       string
	LastErrorMessage    string
	NextRetryAt         time.Time
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type RuntimeCleanupRepository struct {
	db *sql.DB
}

func NewRuntimeCleanupRepository(db *sql.DB) *RuntimeCleanupRepository {
	return &RuntimeCleanupRepository{db: db}
}

func (r *RuntimeCleanupRepository) SaveTask(ctx context.Context, task *RuntimeCleanupTask) error {
	if r == nil || r.db == nil {
		return nil
	}
	if task.CleanupID == "" {
		task.CleanupID = "rt-cleanup-" + uuid.NewString()
	}
	now := time.Now().UTC()
	if task.CreatedAt.IsZero() {
		task.CreatedAt = now
	}
	task.UpdatedAt = now
	if task.CleanupState == "" {
		task.CleanupState = CleanupStatePending
	}
	var nextRetryStr interface{}
	if !task.NextRetryAt.IsZero() {
		nextRetryStr = task.NextRetryAt.Format(time.RFC3339Nano)
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO kernel_runtime_cleanup_tasks
		(cleanup_id, extension_id, module_id, old_generation, runtime_definition_id,
		 runtime_instance_id, runtime_type, process_id, cleanup_state,
		 attempt_count, last_error_code, last_error_message, next_retry_at,
		 created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		task.CleanupID,
		task.ExtensionID,
		task.ModuleID,
		task.OldGeneration,
		task.RuntimeDefinitionID,
		task.RuntimeInstanceID,
		task.RuntimeType,
		task.ProcessID,
		string(task.CleanupState),
		task.AttemptCount,
		task.LastErrorCode,
		task.LastErrorMessage,
		nextRetryStr,
		task.CreatedAt.Format(time.RFC3339Nano),
		task.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("runtime-cleanup-repo: save %s: %w", task.CleanupID, err)
	}
	return nil
}

func (r *RuntimeCleanupRepository) GetTask(ctx context.Context, cleanupID string) (*RuntimeCleanupTask, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT cleanup_id, extension_id, module_id, old_generation, runtime_definition_id,
		       runtime_instance_id, runtime_type, process_id, cleanup_state,
		       attempt_count, last_error_code, last_error_message, next_retry_at,
		       created_at, updated_at
		FROM kernel_runtime_cleanup_tasks
		WHERE cleanup_id = ?`, cleanupID)
	return scanCleanupTask(row)
}

func (r *RuntimeCleanupRepository) ListByState(ctx context.Context, state RuntimeCleanupState) ([]*RuntimeCleanupTask, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT cleanup_id, extension_id, module_id, old_generation, runtime_definition_id,
		       runtime_instance_id, runtime_type, process_id, cleanup_state,
		       attempt_count, last_error_code, last_error_message, next_retry_at,
		       created_at, updated_at
		FROM kernel_runtime_cleanup_tasks
		WHERE cleanup_state = ?`, string(state))
	if err != nil {
		return nil, fmt.Errorf("runtime-cleanup-repo: list by state %s: %w", state, err)
	}
	defer rows.Close()
	return scanCleanupTasks(rows)
}

func (r *RuntimeCleanupRepository) ListPending(ctx context.Context) ([]*RuntimeCleanupTask, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT cleanup_id, extension_id, module_id, old_generation, runtime_definition_id,
		       runtime_instance_id, runtime_type, process_id, cleanup_state,
		       attempt_count, last_error_code, last_error_message, next_retry_at,
		       created_at, updated_at
		FROM kernel_runtime_cleanup_tasks
		WHERE cleanup_state = ? OR cleanup_state = ? OR cleanup_state = ? OR cleanup_state = ?`,
		string(CleanupStatePending),
		string(CleanupStateDraining),
		string(CleanupStateStopping),
		string(CleanupStateStopFailed),
	)
	if err != nil {
		return nil, fmt.Errorf("runtime-cleanup-repo: list pending: %w", err)
	}
	defer rows.Close()
	return scanCleanupTasks(rows)
}

func (r *RuntimeCleanupRepository) ListByExtension(ctx context.Context, extensionID string) ([]*RuntimeCleanupTask, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT cleanup_id, extension_id, module_id, old_generation, runtime_definition_id,
		       runtime_instance_id, runtime_type, process_id, cleanup_state,
		       attempt_count, last_error_code, last_error_message, next_retry_at,
		       created_at, updated_at
		FROM kernel_runtime_cleanup_tasks
		WHERE extension_id = ?`, extensionID)
	if err != nil {
		return nil, fmt.Errorf("runtime-cleanup-repo: list by extension %s: %w", extensionID, err)
	}
	defer rows.Close()
	return scanCleanupTasks(rows)
}

func (r *RuntimeCleanupRepository) ListByCleanupIDPrefix(ctx context.Context, prefix string) ([]*RuntimeCleanupTask, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT cleanup_id, extension_id, module_id, old_generation, runtime_definition_id,
		       runtime_instance_id, runtime_type, process_id, cleanup_state,
		       attempt_count, last_error_code, last_error_message, next_retry_at,
		       created_at, updated_at
		FROM kernel_runtime_cleanup_tasks
		WHERE cleanup_id LIKE ?`, prefix+"%")
	if err != nil {
		return nil, fmt.Errorf("runtime-cleanup-repo: list by prefix %s: %w", prefix, err)
	}
	defer rows.Close()
	return scanCleanupTasks(rows)
}

func (r *RuntimeCleanupRepository) ListByRuntimeInstanceID(ctx context.Context, instanceID string) (*RuntimeCleanupTask, error) {
	if r == nil || r.db == nil {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT cleanup_id, extension_id, module_id, old_generation, runtime_definition_id,
		       runtime_instance_id, runtime_type, process_id, cleanup_state,
		       attempt_count, last_error_code, last_error_message, next_retry_at,
		       created_at, updated_at
		FROM kernel_runtime_cleanup_tasks
		WHERE runtime_instance_id = ?`, instanceID)
	return scanCleanupTask(row)
}

func (r *RuntimeCleanupRepository) UpdateState(ctx context.Context, cleanupID string, state RuntimeCleanupState, errorCode string, errorMessage string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE kernel_runtime_cleanup_tasks
		SET cleanup_state = ?, last_error_code = ?, last_error_message = ?, updated_at = ?
		WHERE cleanup_id = ?`,
		string(state),
		errorCode,
		errorMessage,
		time.Now().UTC().Format(time.RFC3339Nano),
		cleanupID,
	)
	if err != nil {
		return fmt.Errorf("runtime-cleanup-repo: update state %s: %w", cleanupID, err)
	}
	return nil
}

func (r *RuntimeCleanupRepository) UpdateRetry(ctx context.Context, cleanupID string, attemptCount int, nextRetryAt time.Time, state RuntimeCleanupState) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE kernel_runtime_cleanup_tasks
		SET attempt_count = ?, next_retry_at = ?, cleanup_state = ?, updated_at = ?
		WHERE cleanup_id = ?`,
		attemptCount,
		nextRetryAt.Format(time.RFC3339Nano),
		string(state),
		time.Now().UTC().Format(time.RFC3339Nano),
		cleanupID,
	)
	if err != nil {
		return fmt.Errorf("runtime-cleanup-repo: update retry %s: %w", cleanupID, err)
	}
	return nil
}

func (r *RuntimeCleanupRepository) DeleteTask(ctx context.Context, cleanupID string) error {
	if r == nil || r.db == nil {
		return nil
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM kernel_runtime_cleanup_tasks WHERE cleanup_id = ?`, cleanupID)
	if err != nil {
		return fmt.Errorf("runtime-cleanup-repo: delete %s: %w", cleanupID, err)
	}
	return nil
}

func (r *RuntimeCleanupRepository) CountByState(ctx context.Context, state RuntimeCleanupState) (int64, error) {
	if r == nil || r.db == nil {
		return 0, nil
	}
	var count int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM kernel_runtime_cleanup_tasks
		WHERE cleanup_state = ?`, string(state)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("runtime-cleanup-repo: count by state %s: %w", state, err)
	}
	return count, nil
}

type cleanupTaskScanner interface {
	Scan(dest ...any) error
}

func scanCleanupTask(row cleanupTaskScanner) (*RuntimeCleanupTask, error) {
	var (
		task         RuntimeCleanupTask
		stateStr     string
		nextRetryStr sql.NullString
		createdAtStr string
		updatedAtStr string
	)
	if err := row.Scan(
		&task.CleanupID,
		&task.ExtensionID,
		&task.ModuleID,
		&task.OldGeneration,
		&task.RuntimeDefinitionID,
		&task.RuntimeInstanceID,
		&task.RuntimeType,
		&task.ProcessID,
		&stateStr,
		&task.AttemptCount,
		&task.LastErrorCode,
		&task.LastErrorMessage,
		&nextRetryStr,
		&createdAtStr,
		&updatedAtStr,
	); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("runtime-cleanup-repo: scan row: %w", err)
	}
	task.CleanupState = RuntimeCleanupState(stateStr)
	if nextRetryStr.Valid && nextRetryStr.String != "" {
		if t, err := time.Parse(time.RFC3339Nano, nextRetryStr.String); err == nil {
			task.NextRetryAt = t
		}
	}
	if createdAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
			task.CreatedAt = t
		}
	}
	if updatedAtStr != "" {
		if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
			task.UpdatedAt = t
		}
	}
	return &task, nil
}

func scanCleanupTasks(rows *sql.Rows) ([]*RuntimeCleanupTask, error) {
	var tasks []*RuntimeCleanupTask
	for rows.Next() {
		task, err := scanCleanupTask(rows)
		if err != nil {
			return nil, err
		}
		if task != nil {
			tasks = append(tasks, task)
		}
	}
	return tasks, rows.Err()
}
