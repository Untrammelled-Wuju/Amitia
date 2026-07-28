package extension

import (
	"context"
	"log"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
)

type KernelLifecycleProxy struct {
	kernel *kernelruntime.Runtime
}

func NewKernelLifecycleProxy(kernel *kernelruntime.Runtime) *KernelLifecycleProxy {
	return &KernelLifecycleProxy{kernel: kernel}
}

func (p *KernelLifecycleProxy) container() *kernelruntime.Container {
	if p == nil || p.kernel == nil {
		return nil
	}
	return p.kernel.Container()
}

func (p *KernelLifecycleProxy) extensionExists(ctx context.Context, extensionID string) bool {
	c := p.container()
	if c == nil {
		return false
	}
	_, err := c.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	return err == nil
}

func (p *KernelLifecycleProxy) execute(ctx context.Context, cmd lifecycle_manager.LifecycleCommand) error {
	c := p.container()
	if c == nil || c.LifecycleManager == nil {
		return nil
	}
	if cmd.RequestID == "" {
		cmd.RequestID = "legacy-" + string(cmd.Kind) + "-" + string(cmd.ExtensionID)
	}
	_, err := c.LifecycleManager.Execute(ctx, cmd)
	if err != nil {
		log.Printf("[kernel-lifecycle-proxy] %s %s: %v", cmd.Kind, cmd.ExtensionID, err)
	}
	return err
}

func (p *KernelLifecycleProxy) NotifyInstall(ctx context.Context, extensionID, version string) error {
	if !p.extensionExists(ctx, extensionID) {
		return nil
	}
	parsedVersion, err := domain.ParseVersion(version)
	if err != nil {
		return nil
	}
	return p.execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:          lifecycle_manager.CmdInstall,
		ExtensionID:   domain.ExtensionID(extensionID),
		TargetVersion: parsedVersion,
	})
}

func (p *KernelLifecycleProxy) NotifyEnable(ctx context.Context, extensionID string) error {
	if !p.extensionExists(ctx, extensionID) {
		return nil
	}
	return p.execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:        lifecycle_manager.CmdEnable,
		ExtensionID: domain.ExtensionID(extensionID),
	})
}

func (p *KernelLifecycleProxy) NotifyDisable(ctx context.Context, extensionID string) error {
	if !p.extensionExists(ctx, extensionID) {
		return nil
	}
	return p.execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:        lifecycle_manager.CmdDisable,
		ExtensionID: domain.ExtensionID(extensionID),
	})
}

func (p *KernelLifecycleProxy) NotifyUninstall(ctx context.Context, extensionID string) error {
	if !p.extensionExists(ctx, extensionID) {
		return nil
	}
	return p.execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:        lifecycle_manager.CmdUninstall,
		ExtensionID: domain.ExtensionID(extensionID),
	})
}

func (p *KernelLifecycleProxy) NotifyRepair(ctx context.Context, extensionID string) error {
	if !p.extensionExists(ctx, extensionID) {
		return nil
	}
	return p.execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:        lifecycle_manager.CmdRepair,
		ExtensionID: domain.ExtensionID(extensionID),
	})
}
