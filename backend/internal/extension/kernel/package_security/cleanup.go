package package_security

import (
	"context"
	"os"
	"time"
)

type CleanupJobType string

const (
	CleanupJobStaging  CleanupJobType = "staging"
	CleanupJobSnapshot CleanupJobType = "snapshot"
	CleanupJobExtract  CleanupJobType = "extract"
	CleanupJobTemp     CleanupJobType = "temp"
	CleanupJobDownload CleanupJobType = "download"
	CleanupJobLock     CleanupJobType = "lock"
	CleanupJobOrphan   CleanupJobType = "orphan"
)

type CleanupManager struct {
	stagingManager  *StagingManager
	snapshotManager *SnapshotManager
	staleDirs       []string
}

func NewCleanupManager(stagingMgr *StagingManager, snapshotMgr *SnapshotManager) *CleanupManager {
	return &CleanupManager{
		stagingManager:  stagingMgr,
		snapshotManager: snapshotMgr,
	}
}

func (m *CleanupManager) RegisterStaleDir(dir string) {
	m.staleDirs = append(m.staleDirs, dir)
}

func (m *CleanupManager) CleanupAll(ctx context.Context) *CleanupResult {
	result := &CleanupResult{StartedAt: time.Now()}

	cleaned := m.stagingManager.CleanupExpired(ctx)
	result.StagingCleaned = len(cleaned)

	snapCleaned := m.snapshotManager.CleanupExpired(ctx)
	result.SnapshotsCleaned = len(snapCleaned)

	for _, dir := range m.staleDirs {
		if err := os.RemoveAll(dir); err == nil {
			result.TempCleaned++
		} else {
			result.Errors = append(result.Errors, err.Error())
		}
	}

	result.EndedAt = time.Now()
	result.Success = len(result.Errors) == 0
	return result
}

func (m *CleanupManager) CleanupStaging(ctx context.Context, stagingID string) error {
	return m.stagingManager.Cleanup(ctx, stagingID)
}

func (m *CleanupManager) CleanupSnapshot(ctx context.Context, snapshotID string) error {
	return m.snapshotManager.Delete(ctx, snapshotID)
}

type CleanupResult struct {
	Success          bool
	StagingCleaned   int
	SnapshotsCleaned int
	TempCleaned      int
	Errors           []string
	StartedAt        time.Time
	EndedAt          time.Time
}
