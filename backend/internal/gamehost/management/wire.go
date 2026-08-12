package management

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost"
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

func NewProductionPackageMutationServiceFromKernelReader(kernelReader *KernelReader, pluginReg PluginRegistryReader, kernelMutation KernelMutation) *PackageMutationService {
	var reader KernelTargetReader
	if kernelReader != nil {
		reader = kernelReader
	}
	return NewPackageMutationService(PackageMutationServiceOptions{
		Kernel:   kernelMutation,
		Reader:   reader,
		Registry: pluginReg,
	})
}

func NewProductionRuntimeMutationService(container *gamehost.GameHostContainer, runtimeLister RuntimeLister) *RuntimeMutationService {
	if container == nil {
		return NewRuntimeMutationService(RuntimeMutationServiceOptions{})
	}
	var lister RuntimeLister
	if runtimeLister != nil {
		lister = runtimeLister
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
