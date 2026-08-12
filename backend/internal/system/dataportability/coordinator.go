package dataportability

import (
	"context"
	"sync"
)

type BackupOperation struct {
	ID        string       `json:"id"`
	Status    string       `json:"status"`
	Profile   string       `json:"profile"`
	Scope     string       `json:"scope"`
	Error     string       `json:"error,omitempty"`
	CreatedAt string       `json:"createdAt"`
	UpdatedAt string       `json:"updatedAt"`
}

type RestoreOperation struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type ImportOperation struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Scope     string `json:"scope"`
	Error     string `json:"error,omitempty"`
	Stats     map[string]int64 `json:"stats,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type Coordinator struct {
	DataDir       string
	AppVersion    string
	Platform      string
	SchemaFinger  string
	Contributors  []BackupContributor
	Staging       *StagingManager
	diskChecker   DiskSpaceChecker

	mu        sync.RWMutex
	backupOps map[string]*BackupOperation
	restoreOps map[string]*RestoreOperation
	importOps map[string]*ImportOperation
}

type DiskSpaceChecker interface {
	FreeBytes(path string) (uint64, error)
}

type DefaultDiskChecker struct{}

func (DefaultDiskChecker) FreeBytes(path string) (uint64, error) {
	return GetDiskFreeSpace(path)
}

func NewCoordinator(dataDir, appVersion, platform, schemaFinger string, staging *StagingManager) *Coordinator {
	return &Coordinator{
		DataDir:      dataDir,
		AppVersion:   appVersion,
		Platform:     platform,
		SchemaFinger: schemaFinger,
		Contributors: make([]BackupContributor, 0),
		Staging:      staging,
		diskChecker:  DefaultDiskChecker{},
		backupOps:    make(map[string]*BackupOperation),
		restoreOps:   make(map[string]*RestoreOperation),
		importOps:    make(map[string]*ImportOperation),
	}
}

func (c *Coordinator) RegisterContributors(list ...BackupContributor) {
	c.Contributors = append(c.Contributors, list...)
}

func (c *Coordinator) GetBackupOp(id string) *BackupOperation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.backupOps[id]
}

func (c *Coordinator) GetRestoreOp(id string) *RestoreOperation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.restoreOps[id]
}

func (c *Coordinator) GetImportOp(id string) *ImportOperation {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.importOps[id]
}

func (c *Coordinator) EstimateDiskRequired(req BackupRequest) uint64 {
	var total uint64 = 0
	for _, ct := range c.Contributors {
		plans, err := ct.Plan(context.Background(), req)
		if err != nil {
			continue
		}
		for _, p := range plans {
			total += uint64(p.EstimatedSize)
		}
	}
	total += 100 * 1024 * 1024
	return total
}

func (c *Coordinator) CheckDiskSpace(req BackupRequest, targetPath string) error {
	required := c.EstimateDiskRequired(req)
	free, err := c.diskChecker.FreeBytes(targetPath)
	if err != nil {
		return nil
	}
	if free < required {
		return ErrBackupDiskSpace
	}
	return nil
}
