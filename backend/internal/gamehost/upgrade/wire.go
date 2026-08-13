package upgrade

import (
	"context"
	"fmt"

	ghintegration "github.com/u-ai/backend/internal/gamehost/integration"
	ghintegrationdefs "github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/config"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
	ghregistry "github.com/u-ai/backend/internal/gamehost/registry"
	ghdomain "github.com/u-ai/backend/internal/gamehost/domain"
)

type KernelLifecycleAdapter struct {
	executeUpdateFn func(ctx context.Context, extensionID string, targetVersion string, operationID UpgradeOperationID) (*KernelUpdateResult, error)
}

func NewKernelLifecycleAdapter(fn func(ctx context.Context, extensionID string, targetVersion string, operationID UpgradeOperationID) (*KernelUpdateResult, error)) *KernelLifecycleAdapter {
	return &KernelLifecycleAdapter{executeUpdateFn: fn}
}

func (a *KernelLifecycleAdapter) ExecuteUpdate(ctx context.Context, extensionID string, targetVersion string, operationID UpgradeOperationID) (*KernelUpdateResult, error) {
	if a.executeUpdateFn != nil {
		return a.executeUpdateFn(ctx, extensionID, targetVersion, operationID)
	}
	return nil, fmt.Errorf("kernel lifecycle not wired")
}

type KernelArchiveUpdaterAdapter struct {
	updateArchiveFn func(ctx context.Context, extensionID string, archivePath string) (*KernelUpdateResult, error)
}

func NewKernelArchiveUpdaterAdapter(fn func(ctx context.Context, extensionID string, archivePath string) (*KernelUpdateResult, error)) KernelArchiveUpdater {
	return &KernelArchiveUpdaterAdapter{updateArchiveFn: fn}
}

func (a *KernelArchiveUpdaterAdapter) UpdateArchive(ctx context.Context, extensionID string, archivePath string) (*KernelUpdateResult, error) {
	if a.updateArchiveFn != nil {
		return a.updateArchiveFn(ctx, extensionID, archivePath)
	}
	return nil, fmt.Errorf("kernel archive updater not wired")
}

type UpgradeCoordinatorDeps struct {
	PluginRegistry   *ghregistry.Registry
	RuntimeManager   ghruntime.RuntimeManager
	RuntimeExecutor  ghruntime.RuntimeExecutor
	DefinitionSync   *ghintegrationdefs.DefinitionSyncService
	ContributionSync *ghintegration.GamePluginSyncService
	ConfigResolver   *config.Resolver
	KernelLifecycle  KernelExtensionLifecycle
	ArchiveUpdater   KernelArchiveUpdater
}

func BuildUpgradeCoordinator(deps UpgradeCoordinatorDeps) *UpgradeCoordinator {
	c := NewUpgradeCoordinator(
		registryReaderAdapter{deps.PluginRegistry},
		deps.RuntimeManager,
		deps.RuntimeExecutor,
		deps.DefinitionSync,
		contributionReconcilerAdapter{deps.ContributionSync},
		deps.ConfigResolver,
		deps.KernelLifecycle,
		deps.ArchiveUpdater,
	)
	return c
}

type registryReaderAdapter struct {
	registry *ghregistry.Registry
}

func (a registryReaderAdapter) ListByExtension(ctx context.Context, extensionID string) ([]ghdomain.PluginDescriptor, error) {
	return a.registry.ListByExtension(ctx, extensionID)
}

func (a registryReaderAdapter) Get(ctx context.Context, pluginID ghdomain.PluginID) (ghdomain.PluginDescriptor, error) {
	return a.registry.Get(ctx, pluginID)
}

func (a registryReaderAdapter) Snapshot() []ghdomain.PluginDescriptor {
	return a.registry.Snapshot()
}

func (a registryReaderAdapter) Count() int {
	return a.registry.Count()
}

type contributionReconcilerAdapter struct {
	sync *ghintegration.GamePluginSyncService
}

func (a contributionReconcilerAdapter) SyncExtension(ctx context.Context, extensionID string) ghintegration.SyncResult {
	return a.sync.SyncExtension(ctx, extensionID)
}
