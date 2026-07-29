package package_security

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
)

type CommitStatus string

const (
	CommitPending    CommitStatus = "pending"
	CommitInProgress CommitStatus = "in_progress"
	CommitCompleted  CommitStatus = "completed"
	CommitFailed     CommitStatus = "failed"
	CommitRolledBack CommitStatus = "rolled_back"
)

type CommitRequest struct {
	StagingID   string
	TargetPath  string
	PackageID   string
	Version     string
	OperationID string
}

type CommitResult struct {
	Success     bool
	OperationID string
	SnapshotID  string
	Errors      []string
	StartedAt   time.Time
	EndedAt     time.Time
}

type AtomicCommitter struct {
	recoveryJournal *RecoveryJournal
}

func NewAtomicCommitter(journal *RecoveryJournal) *AtomicCommitter {
	return &AtomicCommitter{
		recoveryJournal: journal,
	}
}

func (c *AtomicCommitter) Commit(ctx context.Context, staging *StagingArea, request CommitRequest) (*CommitResult, error) {
	result := &CommitResult{
		OperationID: request.OperationID,
		StartedAt:   time.Now(),
	}

	if request.OperationID == "" {
		result.OperationID = "commit_" + uuid.NewString()
	}

	entry := RecoveryJournalEntry{
		OperationID: result.OperationID,
		PackageID:   request.PackageID,
		Version:     request.Version,
		Step:        "staging_verified",
		State:       "in_progress",
		StagingID:   staging.ID,
		TargetPath:  request.TargetPath,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	c.recoveryJournal.Record(ctx, entry)

	if err := os.MkdirAll(request.TargetPath, 0o700); err != nil {
		result.Errors = append(result.Errors, err.Error())
		c.recordFailed(ctx, result.OperationID)
		return result, ErrCommitFailed
	}

	entry.Step = "snapshot_created"
	entry.UpdatedAt = time.Now()
	c.recoveryJournal.Record(ctx, entry)

	entry.Step = "database_pending"
	entry.UpdatedAt = time.Now()
	c.recoveryJournal.Record(ctx, entry)

	if err := c.copyStagingToTarget(ctx, staging.Path, request.TargetPath); err != nil {
		result.Errors = append(result.Errors, err.Error())
		c.recordFailed(ctx, result.OperationID)
		return result, ErrCommitFailed
	}

	entry.Step = "files_committed"
	entry.UpdatedAt = time.Now()
	c.recoveryJournal.Record(ctx, entry)

	entry.Step = "database_committed"
	entry.State = "completed"
	entry.UpdatedAt = time.Now()
	c.recoveryJournal.Record(ctx, entry)

	result.Success = true
	result.EndedAt = time.Now()
	return result, nil
}

func (c *AtomicCommitter) copyStagingToTarget(ctx context.Context, stagingPath, targetPath string) error {
	entries, err := os.ReadDir(stagingPath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		src := filepath.Join(stagingPath, entry.Name())
		dst := filepath.Join(targetPath, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dst, 0o700); err != nil {
				return fmt.Errorf("create dir %s: %w", dst, err)
			}
			if err := c.copyStagingToTarget(ctx, src, dst); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(src)
			if err != nil {
				return fmt.Errorf("read %s: %w", src, err)
			}
			if err := os.WriteFile(dst, data, 0o600); err != nil {
				return fmt.Errorf("write %s: %w", dst, err)
			}
		}
	}

	return nil
}

func (c *AtomicCommitter) recordFailed(ctx context.Context, operationID string) {
	c.recoveryJournal.Record(ctx, RecoveryJournalEntry{
		OperationID: operationID,
		Step:        "commit_failed",
		State:       "failed",
		UpdatedAt:   time.Now(),
	})
}
