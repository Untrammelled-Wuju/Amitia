package kernel

import (
	"context"
	"fmt"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
	"github.com/u-ai/backend/internal/gamehost/upgrade"
)

type ProductionArchiveUpdater struct {
	mu        sync.RWMutex
	container *Container
}

func NewProductionArchiveUpdater() *ProductionArchiveUpdater {
	return &ProductionArchiveUpdater{}
}

func (u *ProductionArchiveUpdater) SetContainer(c *Container) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.container = c
}

func (u *ProductionArchiveUpdater) GetPreviousArchivePath(ctx context.Context, extensionID string) (string, error) {
	u.mu.RLock()
	container := u.container
	u.mu.RUnlock()

	if container == nil {
		return "", fmt.Errorf("kernel container not ready")
	}
	if container.RollbackPointStore == nil {
		return "", fmt.Errorf("rollback point store not available")
	}

	points := container.RollbackPointStore.List(ctx, extensionID)
	for _, point := range points {
		if point.ArtifactPath != "" {
			return point.ArtifactPath, nil
		}
	}
	return "", fmt.Errorf("no rollback archive available for %s", extensionID)
}

func (u *ProductionArchiveUpdater) UpdateArchive(ctx context.Context, extensionID string, archivePath string) (*upgrade.KernelUpdateResult, error) {
	u.mu.RLock()
	container := u.container
	u.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("kernel container not ready")
	}
	if container.LifecycleManager == nil {
		return nil, fmt.Errorf("kernel lifecycle manager not available")
	}

	result, err := container.LifecycleManager.Execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:        lifecycle_manager.CmdUpdate,
		ExtensionID: domain.ExtensionID(extensionID),
		PackageID:   archivePath,
	})
	if err != nil {
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, err
	}
	return &upgrade.KernelUpdateResult{
		Success:    result.Status == "completed",
		NewVersion: "",
		Reason:     result.Error,
	}, nil
}

var _ GameHostArchiveUpdater = (*ProductionArchiveUpdater)(nil)
