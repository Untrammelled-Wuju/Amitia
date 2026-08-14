package upgrade

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/config"
	ghintegration "github.com/u-ai/backend/internal/gamehost/integration"
	ghintegrationdefs "github.com/u-ai/backend/internal/gamehost/integration/service_definition"
	"github.com/u-ai/backend/internal/gamehost/registry"
	ghruntime "github.com/u-ai/backend/internal/gamehost/runtime"
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

type RuntimeGraphReconcilerAdapter struct {
	provisioner *ghintegration.RuntimeGraphProvisioner
}

func NewRuntimeGraphReconcilerAdapter(provisioner *ghintegration.RuntimeGraphProvisioner) RuntimeGraphReconciler {
	return &RuntimeGraphReconcilerAdapter{provisioner: provisioner}
}

func (a *RuntimeGraphReconcilerAdapter) ReconcileExtension(ctx context.Context, extensionID string) error {
	if a.provisioner == nil {
		return fmt.Errorf("runtime graph provisioner not wired")
	}
	return a.provisioner.Reconcile(ctx)
}

type DefinitionReconcilerAdapter struct {
	sync *ghintegrationdefs.DefinitionSyncService
}

func NewDefinitionReconcilerAdapter(sync *ghintegrationdefs.DefinitionSyncService) DefinitionReconciler {
	return &DefinitionReconcilerAdapter{sync: sync}
}

func (a *DefinitionReconcilerAdapter) ReconcileExtension(extensionID string) *ghintegrationdefs.ReconcileReport {
	if a.sync == nil {
		return &ghintegrationdefs.ReconcileReport{ExtensionID: extensionID, Errors: []error{fmt.Errorf("definition sync not wired")}}
	}
	return a.sync.ReconcileExtension(extensionID)
}

type UpgradeCoordinatorDeps struct {
	PluginRegistry      *registry.Registry
	RuntimeManager      ghruntime.RuntimeManager
	RuntimeExecutor     ghruntime.RuntimeExecutor
	DefinitionReconcile DefinitionReconciler
	RuntimeGraphReconcile RuntimeGraphReconciler
	ContributionSync    *ghintegration.GamePluginSyncService
	ConfigResolver      *config.Resolver
	KernelLifecycle     KernelExtensionLifecycle
	ArchiveUpdater      KernelArchiveUpdater
}

func BuildUpgradeCoordinator(deps UpgradeCoordinatorDeps) (*UpgradeCoordinator, error) {
	if deps.PluginRegistry == nil {
		return nil, fmt.Errorf("upgrade coordinator: PluginRegistry is required")
	}
	if deps.RuntimeManager == nil {
		return nil, fmt.Errorf("upgrade coordinator: RuntimeManager is required")
	}
	if deps.RuntimeExecutor == nil {
		return nil, fmt.Errorf("upgrade coordinator: RuntimeExecutor is required")
	}
	if deps.DefinitionReconcile == nil {
		return nil, fmt.Errorf("upgrade coordinator: DefinitionReconciler is required")
	}
	if deps.ContributionSync == nil {
		return nil, fmt.Errorf("upgrade coordinator: ContributionSync is required")
	}
	if deps.ConfigResolver == nil {
		return nil, fmt.Errorf("upgrade coordinator: ConfigResolver is required")
	}
	if deps.ArchiveUpdater == nil {
		return nil, fmt.Errorf("upgrade coordinator: ArchiveUpdater is required")
	}

	c, err := NewUpgradeCoordinator(
		registryReaderAdapter{deps.PluginRegistry},
		deps.RuntimeManager,
		deps.RuntimeExecutor,
		deps.DefinitionReconcile,
		deps.RuntimeGraphReconcile,
		contributionReconcilerAdapter{deps.ContributionSync},
		deps.ConfigResolver,
		deps.KernelLifecycle,
		deps.ArchiveUpdater,
	)
	if err != nil {
		return nil, fmt.Errorf("upgrade coordinator: %w", err)
	}
	return c, nil
}

type registryReaderAdapter struct {
	registry *registry.Registry
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
