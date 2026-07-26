package package_security

import (
	"context"
	"os"
	"time"
)

type RollbackResult struct {
	Success     bool
	SnapshotID  string
	Errors      []string
	RestoredAt  time.Time
}

type RollbackPrepareRequest struct {
	PackageID string
	Version   string
	TargetPath string
	OwnerID   string
}

type RollbackCoordinator struct {
	snapshotManager *SnapshotManager
}

func NewRollbackCoordinator(snapshotManager *SnapshotManager) *RollbackCoordinator {
	return &RollbackCoordinator{
		snapshotManager: snapshotManager,
	}
}

func (c *RollbackCoordinator) Prepare(ctx context.Context, request RollbackPrepareRequest) (*RollbackSnapshot, error) {
	snapshot, err := c.snapshotManager.CreateSnapshot(ctx, request.TargetPath, request.PackageID, request.Version, request.OwnerID)
	if err != nil {
		return nil, err
	}

	return snapshot, nil
}

func (c *RollbackCoordinator) Restore(ctx context.Context, snapshotID string, targetPath string) *RollbackResult {
	result := &RollbackResult{
		SnapshotID: snapshotID,
	}

	snapshot, err := c.snapshotManager.GetSnapshot(ctx, snapshotID)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
		return result
	}

	if err := os.RemoveAll(targetPath); err != nil {
		result.Errors = append(result.Errors, "cleanup target: "+err.Error())
		return result
	}

	if err := os.MkdirAll(targetPath, 0o700); err != nil {
		result.Errors = append(result.Errors, "create target: "+err.Error())
		return result
	}

	if err := copyDir(snapshot.ArtifactPath, targetPath); err != nil {
		result.Errors = append(result.Errors, "restore files: "+err.Error())
		return result
	}

	result.Success = true
	result.RestoredAt = time.Now()
	return result
}

func (c *RollbackCoordinator) Verify(ctx context.Context, snapshotID string) error {
	snapshot, err := c.snapshotManager.GetSnapshot(ctx, snapshotID)
	if err != nil {
		return err
	}

	if _, err := os.Stat(snapshot.ArtifactPath); os.IsNotExist(err) {
		return ErrSnapshotNotFound
	}

	hasher := NewContentHasher()
	currentHash := computeDirHash(snapshot.ArtifactPath, hasher)
	if currentHash != snapshot.ContentHash {
		return ErrStagingTampered
	}

	return nil
}
