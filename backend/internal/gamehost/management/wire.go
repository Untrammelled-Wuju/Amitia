package management

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/extension"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
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

type extensionKernelAdapter struct {
	runtime *extension.Runtime
}

func NewExtensionKernelAdapter(rt *extension.Runtime) KernelMutation {
	return &extensionKernelAdapter{runtime: rt}
}

func (a *extensionKernelAdapter) Install(ctx context.Context, archivePath string) (KernelInstalledExtension, error) {
	if a.runtime == nil || a.runtime.Kernel == nil {
		return KernelInstalledExtension{}, ErrKernelUnavailable
	}
	installed, err := a.runtime.Kernel.Install(ctx, archivePath)
	if err != nil {
		return KernelInstalledExtension{}, err
	}
	return KernelInstalledExtension{ID: installed.ID, Name: installed.Name, Version: installed.Version}, nil
}

func (a *extensionKernelAdapter) Update(ctx context.Context, archivePath string) (KernelInstalledExtension, error) {
	if a.runtime == nil || a.runtime.Kernel == nil {
		return KernelInstalledExtension{}, ErrKernelUnavailable
	}
	installed, err := a.runtime.Kernel.Update(ctx, archivePath)
	if err != nil {
		return KernelInstalledExtension{}, err
	}
	return KernelInstalledExtension{ID: installed.ID, Name: installed.Name, Version: installed.Version}, nil
}

func (a *extensionKernelAdapter) Enable(ctx context.Context, extensionID string) error {
	if a.runtime == nil || a.runtime.Kernel == nil {
		return ErrKernelUnavailable
	}
	return a.runtime.Kernel.Enable(ctx, extensionID)
}

func (a *extensionKernelAdapter) Disable(ctx context.Context, extensionID string) error {
	if a.runtime == nil || a.runtime.Kernel == nil {
		return ErrKernelUnavailable
	}
	return a.runtime.Kernel.Disable(ctx, extensionID)
}

func (a *extensionKernelAdapter) Uninstall(ctx context.Context, extensionID string) error {
	if a.runtime == nil || a.runtime.Kernel == nil {
		return ErrKernelUnavailable
	}
	return a.runtime.Kernel.Uninstall(ctx, extensionID)
}

type gameHostTargetReaderAdapter struct {
	container *gamehost.GameHostContainer
}

func NewGameHostTargetReaderAdapter(container *gamehost.GameHostContainer) KernelTargetReader {
	return &gameHostTargetReaderAdapter{container: container}
}

func (a *gameHostTargetReaderAdapter) ListGameCenterExtensions(ctx context.Context) ([]domain.ExtensionDefinition, []domain.ExtensionInstallation, error) {
	return nil, nil, fmt.Errorf("not implemented: use KernelReader")
}

func (a *gameHostTargetReaderAdapter) GetGameCenterExtension(ctx context.Context, extensionID string) (*domain.ExtensionDefinition, *domain.ExtensionInstallation, error) {
	return nil, nil, fmt.Errorf("not implemented: use KernelReader")
}

func (a *gameHostTargetReaderAdapter) ListGameCenterContributions(ctx context.Context, extensionID string) ([]domain.ContributionDefinition, error) {
	return nil, fmt.Errorf("not implemented: use KernelReader")
}

func NewProductionPackageMutationService(extRuntime *extension.Runtime, kernelReader KernelTargetReader, pluginReg PluginRegistryReader) *PackageMutationService {
	var kv KernelMutation
	if extRuntime != nil {
		kv = NewExtensionKernelAdapter(extRuntime)
	}
	return NewPackageMutationService(PackageMutationServiceOptions{
		Kernel:   kv,
		Reader:   kernelReader,
		Registry: pluginReg,
	})
}

func NewProductionRuntimeMutationService(container *gamehost.GameHostContainer) *RuntimeMutationService {
	if container == nil {
		return NewRuntimeMutationService(RuntimeMutationServiceOptions{})
	}
	return NewRuntimeMutationService(RuntimeMutationServiceOptions{
		Executor: container.RuntimeExecutor,
		Manager:  NewGameHostRuntimeManager(container),
		Registry: NewGameHostPluginRegistry(container.PluginRegistry),
	})
}
