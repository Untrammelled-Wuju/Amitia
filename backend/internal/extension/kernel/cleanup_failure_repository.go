package kernel

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/extension/kernel/dev_mode"
)

type SQLiteCleanupFailureStore struct {
	db *sql.DB
}

func NewSQLiteCleanupFailureStore(db *sql.DB) *SQLiteCleanupFailureStore {
	return &SQLiteCleanupFailureStore{db: db}
}

func (s *SQLiteCleanupFailureStore) Save(ctx context.Context, record *dev_mode.CleanupFailureRecord) error {
	if s == nil || s.db == nil {
		return nil
	}
	if record.FailureID == "" {
		record.FailureID = "cleanup-fail-" + uuid.NewString()
	}
	now := time.Now().UTC()
	if record.CreatedAt.IsZero() {
		record.CreatedAt = now
	}
	record.UpdatedAt = now
	if record.MaxRetries == 0 {
		record.MaxRetries = 5
	}
	if record.NextRetryAt.IsZero() {
		record.NextRetryAt = now
	}
	if record.Status == "" {
		record.Status = dev_mode.CleanupFailurePending
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO kernel_reload_cleanup_failures
		(failure_id, workspace_id, extension_id, old_instance_id, old_generation,
		 new_instance_id, new_generation, error_code, error_message,
		 retry_count, max_retries, next_retry_at, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.FailureID,
		string(record.WorkspaceID),
		record.ExtensionID,
		record.OldInstanceID,
		record.OldGeneration,
		record.NewInstanceID,
		record.NewGeneration,
		record.ErrorCode,
		record.ErrorMessage,
		record.RetryCount,
		record.MaxRetries,
		record.NextRetryAt.Format(time.RFC3339Nano),
		string(record.Status),
		record.CreatedAt.Format(time.RFC3339Nano),
		record.UpdatedAt.Format(time.RFC3339Nano),
	)
	if err != nil {
		return fmt.Errorf("cleanup-failure-store: save %s: %w", record.FailureID, err)
	}
	return nil
}

func (s *SQLiteCleanupFailureStore) ListPending(ctx context.Context) ([]*dev_mode.CleanupFailureRecord, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT failure_id, workspace_id, extension_id, old_instance_id, old_generation,
		       new_instance_id, new_generation, error_code, error_message,
		       retry_count, max_retries, next_retry_at, status, created_at, updated_at
		FROM kernel_reload_cleanup_failures
		WHERE status = ? OR status = ?`,
		string(dev_mode.CleanupFailurePending),
		string(dev_mode.CleanupFailureRetrying),
	)
	if err != nil {
		return nil, fmt.Errorf("cleanup-failure-store: list pending: %w", err)
	}
	defer rows.Close()
	return scanCleanupFailures(rows)
}

func (s *SQLiteCleanupFailureStore) ListAll(ctx context.Context) ([]*dev_mode.CleanupFailureRecord, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT failure_id, workspace_id, extension_id, old_instance_id, old_generation,
		       new_instance_id, new_generation, error_code, error_message,
		       retry_count, max_retries, next_retry_at, status, created_at, updated_at
		FROM kernel_reload_cleanup_failures`)
	if err != nil {
		return nil, fmt.Errorf("cleanup-failure-store: list all: %w", err)
	}
	defer rows.Close()
	return scanCleanupFailures(rows)
}

func (s *SQLiteCleanupFailureStore) Delete(ctx context.Context, failureID string) error {
	if s == nil || s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `DELETE FROM kernel_reload_cleanup_failures WHERE failure_id = ?`, failureID)
	if err != nil {
		return fmt.Errorf("cleanup-failure-store: delete %s: %w", failureID, err)
	}
	return nil
}

func (s *SQLiteCleanupFailureStore) UpdateRetry(ctx context.Context, failureID string, retryCount int, nextRetryAt time.Time, status dev_mode.CleanupFailureStatus) error {
	if s == nil || s.db == nil {
		return nil
	}
	_, err := s.db.ExecContext(ctx, `
		UPDATE kernel_reload_cleanup_failures
		SET retry_count = ?, next_retry_at = ?, status = ?, updated_at = ?
		WHERE failure_id = ?`,
		retryCount,
		nextRetryAt.Format(time.RFC3339Nano),
		string(status),
		time.Now().UTC().Format(time.RFC3339Nano),
		failureID,
	)
	if err != nil {
		return fmt.Errorf("cleanup-failure-store: update retry %s: %w", failureID, err)
	}
	return nil
}

func (s *SQLiteCleanupFailureStore) Count(ctx context.Context) (int64, error) {
	if s == nil || s.db == nil {
		return 0, nil
	}
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM kernel_reload_cleanup_failures
		WHERE status = ? OR status = ?`,
		string(dev_mode.CleanupFailurePending),
		string(dev_mode.CleanupFailureRetrying),
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("cleanup-failure-store: count: %w", err)
	}
	return count, nil
}

func scanCleanupFailures(rows *sql.Rows) ([]*dev_mode.CleanupFailureRecord, error) {
	var records []*dev_mode.CleanupFailureRecord
	for rows.Next() {
		var (
			record       dev_mode.CleanupFailureRecord
			wsID         string
			nextRetryStr string
			statusStr    string
			createdAtStr string
			updatedAtStr string
		)
		if err := rows.Scan(
			&record.FailureID,
			&wsID,
			&record.ExtensionID,
			&record.OldInstanceID,
			&record.OldGeneration,
			&record.NewInstanceID,
			&record.NewGeneration,
			&record.ErrorCode,
			&record.ErrorMessage,
			&record.RetryCount,
			&record.MaxRetries,
			&nextRetryStr,
			&statusStr,
			&createdAtStr,
			&updatedAtStr,
		); err != nil {
			return nil, fmt.Errorf("cleanup-failure-store: scan row: %w", err)
		}
		record.WorkspaceID = dev_mode.WorkspaceID(wsID)
		record.Status = dev_mode.CleanupFailureStatus(statusStr)
		if nextRetryStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, nextRetryStr); err == nil {
				record.NextRetryAt = t
			}
		}
		if createdAtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, createdAtStr); err == nil {
				record.CreatedAt = t
			}
		}
		if updatedAtStr != "" {
			if t, err := time.Parse(time.RFC3339Nano, updatedAtStr); err == nil {
				record.UpdatedAt = t
			}
		}
		records = append(records, &record)
	}
	return records, rows.Err()
}

var _ dev_mode.CleanupFailureStore = (*SQLiteCleanupFailureStore)(nil)
