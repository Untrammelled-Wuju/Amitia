package management

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost"
	ghupgrade "github.com/u-ai/backend/internal/gamehost/upgrade"
)

func NewProductionService(container *gamehost.GameHostContainer, kernel KernelManagementReader) *GameCenterManagementService {
	if container == nil {
		return NewGameCenterManagementService(GameCenterManagementServiceOptions{
			Kernel: kernel,
		})
	}

	opts := GameCenterManagementServiceOptions{
		Kernel: kernel,
	}

	if container.PluginRegistry != nil {
		opts.Registry = NewGameHostPluginRegistry(container.PluginRegistry)
	}

	if container.RuntimeManager != nil {
		opts.Runtimes = NewGameHostRuntimeManager(container.RuntimeManager)
	}

	if container.RuntimeTopologyStore != nil {
		opts.Topology = NewGameHostTopologyStore(container.RuntimeTopologyStore)
	}

	if container.RuntimeHealth != nil {
		opts.Health = NewGameHostHealthAdapter(container.RuntimeHealth)
	}

	if container.ConnectionRegistry != nil {
		opts.Connections = NewGameHostConnectionRegistry(container.ConnectionRegistry)
	}

	if container.HandshakeManager != nil {
		opts.Handshake = NewGameHostHandshakeManager(container.HandshakeManager)
	}

	if container.AuthorityManager != nil {
		opts.Authority = NewGameHostAuthorityManager(container.AuthorityManager)
	}

	return NewGameCenterManagementService(opts)
}

type KernelMutationOptions struct {
	InstallFn   func(ctx context.Context, archivePath string) (KernelInstalledExtension, error)
	UpdateFn    func(ctx context.Context, archivePath string) (KernelInstalledExtension, error)
	EnableFn    func(ctx context.Context, extensionID string) error
	DisableFn   func(ctx context.Context, extensionID string) error
	UninstallFn func(ctx context.Context, extensionID string) error
}

type kernelMutationFromFuncs struct {
	opts KernelMutationOptions
}

func NewKernelMutationFromFuncs(opts KernelMutationOptions) KernelMutation {
	return &kernelMutationFromFuncs{opts: opts}
}

func (f *kernelMutationFromFuncs) Install(ctx context.Context, archivePath string) (KernelInstalledExtension, error) {
	return f.opts.InstallFn(ctx, archivePath)
}

func (f *kernelMutationFromFuncs) Update(ctx context.Context, archivePath string) (KernelInstalledExtension, error) {
	return f.opts.UpdateFn(ctx, archivePath)
}

func (f *kernelMutationFromFuncs) Enable(ctx context.Context, extensionID string) error {
	return f.opts.EnableFn(ctx, extensionID)
}

func (f *kernelMutationFromFuncs) Disable(ctx context.Context, extensionID string) error {
	return f.opts.DisableFn(ctx, extensionID)
}

func (f *kernelMutationFromFuncs) Uninstall(ctx context.Context, extensionID string) error {
	return f.opts.UninstallFn(ctx, extensionID)
}

type packageUpgradeCoordinatorAdapter struct {
	coordinator gamehostUpgradeCoordinator
}

type gamehostUpgradeCoordinator interface {
	ExecuteUpgradeByArchive(ctx context.Context, extensionID, archivePath string) error
}

// GameHostUpgradeCoordinatorAdapter 公开适配器，让外部包可以把 upgrade.UpgradeCoordinator 包装为 management.PackageUpgradeCoordinator
type GameHostUpgradeCoordinatorAdapter struct {
	UC *ghupgrade.UpgradeCoordinator
}

func (a *GameHostUpgradeCoordinatorAdapter) ExecuteUpgrade(ctx context.Context, extensionID, archivePath string) error {
	if a.UC == nil {
		return fmt.Errorf("management: gamehost upgrade coordinator is nil")
	}
	return a.UC.ExecuteUpgradeByArchive(ctx, extensionID, archivePath)
}

func (a *packageUpgradeCoordinatorAdapter) ExecuteUpgrade(ctx context.Context, extensionID, archivePath string) error {
	return a.coordinator.ExecuteUpgradeByArchive(ctx, extensionID, archivePath)
}

func NewProductionPackageMutationServiceFromKernelReader(kernelReader *KernelReader, pluginReg PluginRegistryReader, kernelMutation KernelMutation, upgradeCoordinator PackageUpgradeCoordinator) *PackageMutationService {
	var reader KernelTargetReader
	if kernelReader != nil {
		reader = kernelReader
	}
	var uc PackageUpgradeCoordinator = upgradeCoordinator
	return NewPackageMutationService(PackageMutationServiceOptions{
		Kernel:             kernelMutation,
		Reader:             reader,
		Registry:           pluginReg,
		UpgradeCoordinator: uc,
	})
}

func NewProductionPackageMutationServiceWithPreflight(kernelReader *KernelReader, pluginReg PluginRegistryReader, kernelMutation KernelMutation, upgradeCoordinator PackageUpgradeCoordinator, preflight PackageTargetPreflight) *PackageMutationService {
	var reader KernelTargetReader
	if kernelReader != nil {
		reader = kernelReader
	}
	var uc PackageUpgradeCoordinator = upgradeCoordinator
	return NewPackageMutationService(PackageMutationServiceOptions{
		Kernel:             kernelMutation,
		Reader:             reader,
		Registry:           pluginReg,
		UpgradeCoordinator: uc,
		Preflight:          preflight,
	})
}

func NewProductionRuntimeMutationService(container *gamehost.GameHostContainer) *RuntimeMutationService {
	if container == nil {
		return NewRuntimeMutationService(RuntimeMutationServiceOptions{})
	}

	var lister RuntimeLister
	if container.RuntimeManager != nil {
		lister = NewGameHostRuntimeManagerAdapter(container.RuntimeManager)
	}

	var pluginReg PluginRegistryReader
	if container.PluginRegistry != nil {
		pluginReg = NewGameHostPluginRegistry(container.PluginRegistry)
	}

	return NewRuntimeMutationService(RuntimeMutationServiceOptions{
		Executor:       container.RuntimeExecutor,
		RuntimeLister:  lister,
		PluginRegistry: pluginReg,
	})
}

func NewGameHostPluginRegistryFromContainer(container *gamehost.GameHostContainer) PluginRegistryReader {
	if container == nil || container.PluginRegistry == nil {
		return nil
	}
	return NewGameHostPluginRegistry(container.PluginRegistry)
}

type ControlServiceOptions struct {
	TakeoverFn      TakeoverFunc
	ReleaseFn       ReleaseFunc
	EmergencyStopFn EmergencyStopFunc
}

func NewControlHandlerFromFuncs(opts ControlServiceOptions) *ControlHandler {
	return NewControlHandler(opts.TakeoverFn, opts.ReleaseFn, opts.EmergencyStopFn)
}
