package extension

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

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

func (p *KernelLifecycleProxy) legacyMigrationRecorded(ctx context.Context, extensionID string) bool {
	c := p.container()
	if c == nil || c.Store == nil {
		return false
	}
	var count int
	return c.Store.DB().QueryRowContext(ctx, `SELECT COUNT(1) FROM kernel_legacy_package_migrations WHERE legacy_extension_id = ?`, extensionID).Scan(&count) == nil && count == 1
}

func (p *KernelLifecycleProxy) recordLegacyMigration(ctx context.Context, extensionID, version, status, reason string, migratedAt any) error {
	c := p.container()
	if c == nil || c.Store == nil {
		return fmt.Errorf("extension kernel migration store unavailable")
	}
	_, err := c.Store.DB().ExecContext(ctx, `
		INSERT INTO kernel_legacy_package_migrations (legacy_extension_id, legacy_version, status, reason, migrated_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(legacy_extension_id) DO UPDATE SET legacy_version = excluded.legacy_version, status = excluded.status, reason = excluded.reason, migrated_at = excluded.migrated_at, updated_at = excluded.updated_at
	`, extensionID, version, status, reason, migratedAt, time.Now().UTC())
	return err
}

func (p *KernelLifecycleProxy) execute(ctx context.Context, cmd lifecycle_manager.LifecycleCommand) error {
	c := p.container()
	if c == nil || c.LifecycleManager == nil {
		return fmt.Errorf("extension kernel lifecycle unavailable")
	}
	if cmd.RequestID == "" {
		cmd.RequestID = "legacy-" + string(cmd.Kind) + "-" + string(cmd.ExtensionID)
	}
	_, err := c.LifecycleManager.Execute(ctx, cmd)
	return err
}

func (p *KernelLifecycleProxy) InstallPackage(ctx context.Context, raw []byte, fileName, expectedExtensionID string) (kernelruntime.KernelInstallResult, bool, error) {
	if p == nil || p.kernel == nil || p.container() == nil {
		return kernelruntime.KernelInstallResult{}, false, fmt.Errorf("extension kernel install unavailable")
	}
	temp, err := os.CreateTemp("", "amitia-kernel-*.amitiax")
	if err != nil {
		return kernelruntime.KernelInstallResult{}, false, err
	}
	path := temp.Name()
	defer os.Remove(path)
	if fileName != "" {
		path = filepath.Join(filepath.Dir(path), filepath.Base(path)+"-"+filepath.Base(fileName))
		defer os.Remove(path)
	}
	if _, err := temp.Write(raw); err != nil {
		temp.Close()
		return kernelruntime.KernelInstallResult{}, false, err
	}
	if err := temp.Close(); err != nil {
		return kernelruntime.KernelInstallResult{}, false, err
	}
	if path != temp.Name() {
		if err := os.Rename(temp.Name(), path); err != nil {
			return kernelruntime.KernelInstallResult{}, false, err
		}
	}
	preview, err := p.kernel.PreviewInstall(ctx, path)
	if err != nil {
		return kernelruntime.KernelInstallResult{}, false, err
	}
	if !preview.Installable {
		return kernelruntime.KernelInstallResult{}, false, fmt.Errorf("extension kernel rejected package: %v", preview.Issues)
	}
	if expectedExtensionID != "" && preview.ExtensionID != expectedExtensionID {
		return kernelruntime.KernelInstallResult{}, false, fmt.Errorf("extension kernel package id mismatch: expected %s, got %s", expectedExtensionID, preview.ExtensionID)
	}
	wasInstalled := p.extensionExists(ctx, preview.ExtensionID)
	result, err := p.kernel.ExecuteInstall(ctx, path)
	return result, wasInstalled, err
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
	return p.execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:        lifecycle_manager.CmdEnable,
		ExtensionID: domain.ExtensionID(extensionID),
	})
}

func (p *KernelLifecycleProxy) NotifyDisable(ctx context.Context, extensionID string) error {
	return p.execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:        lifecycle_manager.CmdDisable,
		ExtensionID: domain.ExtensionID(extensionID),
	})
}

func (p *KernelLifecycleProxy) NotifyUninstall(ctx context.Context, extensionID string) error {
	return p.execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:        lifecycle_manager.CmdUninstall,
		ExtensionID: domain.ExtensionID(extensionID),
	})
}

func (p *KernelLifecycleProxy) Rollback(ctx context.Context, extensionID, version string) error {
	parsedVersion, err := domain.ParseVersion(version)
	if err != nil {
		return err
	}
	return p.execute(ctx, lifecycle_manager.LifecycleCommand{Kind: lifecycle_manager.CmdRollback, ExtensionID: domain.ExtensionID(extensionID), TargetVersion: parsedVersion})
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
