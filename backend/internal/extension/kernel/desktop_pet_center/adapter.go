package desktop_pet_center

import (
	"context"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/persistence/sqlite"
)

type kernelAdapter struct {
	runtime *kernelruntime.Runtime
}

func newKernelAdapter(runtime *kernelruntime.Runtime) *kernelAdapter {
	return &kernelAdapter{runtime: runtime}
}

func (a *kernelAdapter) ListInstallations(ctx context.Context) ([]domain.ExtensionInstallation, error) {
	if a.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	container := a.runtime.Container()
	if container == nil {
		return nil, ErrKernelUnavailable
	}
	return container.InstallationRepository.ListInstallations(ctx)
}

func (a *kernelAdapter) GetInstallation(ctx context.Context, id domain.ExtensionID) (domain.ExtensionInstallation, error) {
	if a.runtime == nil {
		return domain.ExtensionInstallation{}, ErrKernelUnavailable
	}
	container := a.runtime.Container()
	if container == nil {
		return domain.ExtensionInstallation{}, ErrKernelUnavailable
	}
	return container.InstallationRepository.GetInstallation(ctx, id)
}

func (a *kernelAdapter) ListModules(ctx context.Context, extID domain.ExtensionID) ([]domain.ModuleDefinition, error) {
	if a.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	container := a.runtime.Container()
	if container == nil {
		return nil, ErrKernelUnavailable
	}
	return container.ModuleRepository.ListModules(ctx, extID)
}

func (a *kernelAdapter) ListContributions(ctx context.Context, extID domain.ExtensionID) ([]domain.ContributionDefinition, error) {
	if a.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	container := a.runtime.Container()
	if container == nil {
		return nil, ErrKernelUnavailable
	}
	return container.ContributionRepository.ListContributions(ctx, extID)
}

func (a *kernelAdapter) ListGrants(ctx context.Context, extID domain.ExtensionID) ([]sqlite.PermissionGrant, error) {
	if a.runtime == nil {
		return nil, ErrKernelUnavailable
	}
	container := a.runtime.Container()
	if container == nil {
		return nil, ErrKernelUnavailable
	}
	return container.PermissionRepository.ListGrants(ctx, extID)
}

type kernelRuntimeAdapter struct {
	runtime *kernelruntime.Runtime
}

func newKernelRuntimeAdapter(runtime *kernelruntime.Runtime) *kernelRuntimeAdapter {
	return &kernelRuntimeAdapter{runtime: runtime}
}

func (a *kernelRuntimeAdapter) Install(ctx context.Context, archivePath string) (kernelInstalledExtension, error) {
	if a.runtime == nil {
		return kernelInstalledExtension{}, ErrKernelUnavailable
	}
	installed, err := a.runtime.Install(ctx, archivePath)
	if err != nil {
		return kernelInstalledExtension{}, err
	}
	return kernelInstalledExtension{ID: installed.ID, Name: installed.Name, Version: installed.Version}, nil
}

func (a *kernelRuntimeAdapter) Update(ctx context.Context, archivePath string) (kernelInstalledExtension, error) {
	if a.runtime == nil {
		return kernelInstalledExtension{}, ErrKernelUnavailable
	}
	installed, err := a.runtime.Update(ctx, archivePath)
	if err != nil {
		return kernelInstalledExtension{}, err
	}
	return kernelInstalledExtension{ID: installed.ID, Name: installed.Name, Version: installed.Version}, nil
}

func (a *kernelRuntimeAdapter) Enable(ctx context.Context, extensionID string) error {
	if a.runtime == nil {
		return ErrKernelUnavailable
	}
	return a.runtime.Enable(ctx, extensionID)
}

func (a *kernelRuntimeAdapter) Disable(ctx context.Context, extensionID string) error {
	if a.runtime == nil {
		return ErrKernelUnavailable
	}
	return a.runtime.Disable(ctx, extensionID)
}

func (a *kernelRuntimeAdapter) Uninstall(ctx context.Context, extensionID string) error {
	if a.runtime == nil {
		return ErrKernelUnavailable
	}
	return a.runtime.Uninstall(ctx, extensionID)
}

func NewServiceFromRuntime(runtime *kernelruntime.Runtime) *DesktopPetPluginManagementService {
	return NewDesktopPetPluginManagementService(
		newKernelAdapter(runtime),
		newKernelRuntimeAdapter(runtime),
		nil,
	)
}

func NewServiceFromRuntimeWithPreflight(runtime *kernelruntime.Runtime, preflight PackageTargetPreflight) *DesktopPetPluginManagementService {
	return NewDesktopPetPluginManagementService(
		newKernelAdapter(runtime),
		newKernelRuntimeAdapter(runtime),
		preflight,
	)
}
