package kernel

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/amitiax"
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

	pkg, err := amitiax.OpenArchive(archivePath)
	if err != nil {
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, fmt.Errorf("open rollback archive: %w", err)
	}
	manifestExtensionID := strings.TrimSpace(pkg.Manifest.Extension.ID)
	if manifestExtensionID == "" || manifestExtensionID != strings.TrimSpace(extensionID) {
		err := fmt.Errorf("rollback archive extension mismatch: expected %q, got %q", extensionID, manifestExtensionID)
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, err
	}
	targetVersion, err := domain.ParseVersion(strings.TrimSpace(pkg.Manifest.Extension.Version))
	if err != nil {
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, fmt.Errorf("parse rollback archive version %q: %w", pkg.Manifest.Extension.Version, err)
	}

	result, err := container.LifecycleManager.Execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:          lifecycle_manager.CmdUpdate,
		ExtensionID:   domain.ExtensionID(extensionID),
		TargetVersion: targetVersion,
		PackageID:     archivePath,
	})
	if err != nil {
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, err
	}
	if result.Status != "completed" {
		err := fmt.Errorf("kernel archive update did not complete: status=%s reason=%s", result.Status, result.Error)
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, err
	}
	if result.FinalState.Installation == nil {
		err := fmt.Errorf("kernel archive update completed without installation state")
		return &upgrade.KernelUpdateResult{Success: false, Reason: err.Error()}, err
	}
	newVersion := strings.TrimSpace(result.FinalState.Installation.InstalledVersion.String())
	if newVersion == "" || newVersion != targetVersion.String() {
		err := fmt.Errorf("kernel archive update version mismatch: expected %s, got %q", targetVersion.String(), newVersion)
		return &upgrade.KernelUpdateResult{Success: false, NewVersion: newVersion, Reason: err.Error()}, err
	}
	return &upgrade.KernelUpdateResult{Success: true, NewVersion: newVersion}, nil
}

var _ GameHostArchiveUpdater = (*ProductionArchiveUpdater)(nil)
