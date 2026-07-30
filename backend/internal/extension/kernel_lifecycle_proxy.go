package extension

import (
	"context"
	"fmt"
	"io"
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

func (p *KernelLifecycleProxy) ReadContainer() *kernelruntime.Container {
	return p.container()
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
	return c.Store.DB().QueryRowContext(ctx, `SELECT COUNT(1) FROM extension_package_legacy_migrations WHERE extension_id = ? AND migration_status = 'migrated'`, extensionID).Scan(&count) == nil && count == 1
}

func (p *KernelLifecycleProxy) LegacyReadAllowed(ctx context.Context, extensionID string) bool {
	c := p.container()
	if c == nil || c.Store == nil {
		return false
	}
	var status string
	err := c.Store.DB().QueryRowContext(ctx, `SELECT migration_status FROM extension_package_legacy_migrations WHERE extension_id = ?`, extensionID).Scan(&status)
	return err == nil && status == "requires_manual_migration"
}

func (p *KernelLifecycleProxy) recordLegacyMigration(ctx context.Context, extensionID, version, status, reason string, migratedAt any) error {
	c := p.container()
	if c == nil || c.Store == nil {
		return fmt.Errorf("extension kernel migration store unavailable")
	}
	_, err := c.Store.DB().ExecContext(ctx, `INSERT INTO extension_package_legacy_migrations
		(extension_id, migration_status, attempt_count, last_error, legacy_path, artifact_id, updated_at)
		VALUES (?, ?, 1, ?, '', '', ?)
		ON CONFLICT(extension_id) DO UPDATE SET migration_status=excluded.migration_status,
		attempt_count=extension_package_legacy_migrations.attempt_count+1,
		last_error=excluded.last_error, updated_at=excluded.updated_at`, extensionID, status, reason, time.Now().UTC().Format(time.RFC3339Nano))
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

func (p *KernelLifecycleProxy) PreviewPackage(ctx context.Context, request kernelruntime.PackagePreviewRequest, reader io.Reader) (kernelruntime.InstallPreview, error) {
	if p == nil || p.kernel == nil || p.container() == nil {
		return kernelruntime.InstallPreview{}, fmt.Errorf("extension kernel preview unavailable")
	}
	return p.kernel.PreviewPackage(ctx, request, reader)
}

func (p *KernelLifecycleProxy) InstallPreviewedPackage(ctx context.Context, request kernelruntime.PackageInstallRequest) (kernelruntime.KernelInstallResult, error) {
	if p == nil || p.kernel == nil || p.container() == nil {
		return kernelruntime.KernelInstallResult{}, fmt.Errorf("extension kernel install unavailable")
	}
	return p.kernel.ExecutePackageInstall(ctx, request)
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
