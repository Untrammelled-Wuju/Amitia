package dataportability

import (
	"context"
	"fmt"
	"sync"
)

type BackupOperation struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Profile   string `json:"profile"`
	Scope     string `json:"scope"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type RestoreOperation struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
}

type ImportOperation struct {
	ID        string           `json:"id"`
	Status    string           `json:"status"`
	Scope     string           `json:"scope"`
	Error     string           `json:"error,omitempty"`
	Stats     map[string]int64 `json:"stats,omitempty"`
	CreatedAt string           `json:"createdAt"`
	UpdatedAt string           `json:"updatedAt"`
}

type Coordinator struct {
	DataDir      string
	AppVersion   string
	Platform     string
	SchemaFinger string
	Contributors []BackupContributor
	RestorePorts RestorePorts
	Staging      *StagingManager
	diskChecker  DiskSpaceChecker

	mu         sync.RWMutex
	backupOps  map[string]*BackupOperation
	restoreOps map[string]*RestoreOperation
	importOps  map[string]*ImportOperation
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

func (c *Coordinator) RegisterContributors(list ...BackupContributor) error {
	seen := make(map[string]bool)
	for _, existing := range c.Contributors {
		if seen[existing.ID()] {
			return fmt.Errorf("duplicate contributor ID: %s", existing.ID())
		}
		seen[existing.ID()] = true
	}
	for _, ct := range list {
		if seen[ct.ID()] {
			return fmt.Errorf("duplicate contributor ID: %s", ct.ID())
		}
		seen[ct.ID()] = true
		c.Contributors = append(c.Contributors, ct)
	}
	return nil
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

func (c *Coordinator) ExecuteFullRestore(ctx context.Context, archivePath string, opts RestoreOptions) error {
	if !c.RestorePorts.HasAny() {
		return ErrRestoreNoPorts
	}

	reader, err := NewArchiveReader(archivePath)
	if err != nil {
		return err
	}
	defer reader.Close()

	manifest, err := reader.ReadManifest()
	if err != nil {
		return err
	}
	_ = manifest

	br := &archiveBackupReader{r: reader}

	if c.RestorePorts.Character != nil {
		if err := c.RestorePorts.Character.RestoreCharacters(ctx, br, opts); err != nil {
			return fmt.Errorf("restore characters: %w", err)
		}
	}
	if c.RestorePorts.Chat != nil {
		if err := c.RestorePorts.Chat.RestoreChats(ctx, br, opts); err != nil {
			return fmt.Errorf("restore chats: %w", err)
		}
	}
	if c.RestorePorts.Episodic != nil {
		if err := c.RestorePorts.Episodic.RestoreEpisodic(ctx, br, opts); err != nil {
			return fmt.Errorf("restore episodic: %w", err)
		}
	}
	if c.RestorePorts.Psyche != nil {
		if err := c.RestorePorts.Psyche.RestorePsyche(ctx, br, opts); err != nil {
			return fmt.Errorf("restore psyche: %w", err)
		}
	}
	if c.RestorePorts.Relationship != nil {
		if err := c.RestorePorts.Relationship.RestoreRelationships(ctx, br, opts); err != nil {
			return fmt.Errorf("restore relationships: %w", err)
		}
	}
	if c.RestorePorts.Worldbook != nil {
		if err := c.RestorePorts.Worldbook.RestoreWorldbook(ctx, br, opts); err != nil {
			return fmt.Errorf("restore worldbook: %w", err)
		}
	}
	if c.RestorePorts.ModelConfig != nil {
		if err := c.RestorePorts.ModelConfig.RestoreModelConfigs(ctx, br, opts); err != nil {
			return fmt.Errorf("restore model configs: %w", err)
		}
	}
	if c.RestorePorts.Voice != nil {
		if err := c.RestorePorts.Voice.RestoreVoices(ctx, br, opts); err != nil {
			return fmt.Errorf("restore voices: %w", err)
		}
	}
	if c.RestorePorts.Embedding != nil {
		if err := c.RestorePorts.Embedding.RestoreEmbeddings(ctx, br, opts); err != nil {
			return fmt.Errorf("restore embeddings: %w", err)
		}
	}
	if c.RestorePorts.Resource != nil {
		if err := c.RestorePorts.Resource.RestoreResources(ctx, br, opts); err != nil {
			return fmt.Errorf("restore resources: %w", err)
		}
	}
	if c.RestorePorts.Extension != nil {
		if err := c.RestorePorts.Extension.RestoreExtensions(ctx, br, opts); err != nil {
			return fmt.Errorf("restore extensions: %w", err)
		}
	}
	if c.RestorePorts.Workspace != nil {
		if err := c.RestorePorts.Workspace.RestoreWorkspaces(ctx, br, opts); err != nil {
			return fmt.Errorf("restore workspaces: %w", err)
		}
	}
	if c.RestorePorts.Memory != nil {
		if err := c.RestorePorts.Memory.RestoreMemories(ctx, br, opts); err != nil {
			return fmt.Errorf("restore memories: %w", err)
		}
	}
	return nil
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
