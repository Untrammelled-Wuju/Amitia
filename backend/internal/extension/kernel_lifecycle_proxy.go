//go:build legacy_migration

package extension

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	kernelruntime "github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/extension/kernel/lifecycle_manager"
)

type LegacyMigrationState string

const (
	LegacyMigrationNotStarted     LegacyMigrationState = "not_started"
	LegacyMigrationAnalyzing      LegacyMigrationState = "analyzing"
	LegacyMigrationReady          LegacyMigrationState = "ready"
	LegacyMigrationMigrating      LegacyMigrationState = "migrating"
	LegacyMigrationCompleted      LegacyMigrationState = "completed"
	LegacyMigrationBlocked        LegacyMigrationState = "blocked"
	LegacyMigrationManualRequired LegacyMigrationState = "manual_required"
	LegacyMigrationPendingManual  LegacyMigrationState = "pending_manual_migration"
)

type legacyMigrationToolContextKey struct{}

func WithLegacyMigrationToolContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, legacyMigrationToolContextKey{}, true)
}

func isLegacyMigrationToolContext(ctx context.Context) bool {
	allowed, _ := ctx.Value(legacyMigrationToolContextKey{}).(bool)
	return allowed
}

type legacyMigrationRecord struct {
	ExtensionID string
	State       LegacyMigrationState
	Failure     string
	Source      string
	ArtifactID  string
}

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

func (p *KernelLifecycleProxy) extensionExists(ctx context.Context, extensionID string) (bool, error) {
	c := p.container()
	if c == nil || c.InstallationRepository == nil {
		return false, fmt.Errorf("extension kernel installation repository unavailable")
	}
	_, err := c.InstallationRepository.GetInstallation(ctx, domain.ExtensionID(extensionID))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, domain.ErrInvalidExtensionID) {
		return false, nil
	}
	return false, fmt.Errorf("extension kernel installation query failed: %w", err)
}

func normalizeLegacyMigrationState(status string) (LegacyMigrationState, error) {
	switch LegacyMigrationState(strings.TrimSpace(status)) {
	case LegacyMigrationNotStarted, LegacyMigrationAnalyzing, LegacyMigrationReady, LegacyMigrationMigrating,
		LegacyMigrationCompleted, LegacyMigrationBlocked, LegacyMigrationManualRequired, LegacyMigrationPendingManual:
		return LegacyMigrationState(strings.TrimSpace(status)), nil
	case "migrated":
		return LegacyMigrationCompleted, nil
	case "requires_manual_migration":
		return LegacyMigrationManualRequired, nil
	default:
		return "", fmt.Errorf("legacy migration state is not recognized: %s", status)
	}
}

func (p *KernelLifecycleProxy) legacyMigrationStatus(ctx context.Context, extensionID string) (LegacyMigrationState, bool, error) {
	c := p.container()
	if c == nil || c.Store == nil {
		return "", false, fmt.Errorf("extension kernel migration store unavailable")
	}
	var status string
	err := c.Store.DB().QueryRowContext(ctx, `SELECT migration_status FROM extension_package_legacy_migrations WHERE extension_id = ?`, extensionID).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("extension kernel migration status query failed: %w", err)
	}
	normalized, err := normalizeLegacyMigrationState(status)
	if err != nil {
		return "", false, err
	}
	if string(normalized) != status {
		if _, err := c.Store.DB().ExecContext(ctx, `UPDATE extension_package_legacy_migrations SET migration_status = ?, updated_at = ? WHERE extension_id = ? AND migration_status = ?`, normalized, time.Now().UTC().Format(time.RFC3339Nano), extensionID, status); err != nil {
			return "", false, fmt.Errorf("extension kernel migration state normalization failed: %w", err)
		}
	}
	return normalized, true, nil
}

func (p *KernelLifecycleProxy) legacyMigrationRecorded(ctx context.Context, extensionID string) (bool, error) {
	state, found, err := p.legacyMigrationStatus(ctx, extensionID)
	return found && (state == LegacyMigrationCompleted || state == LegacyMigrationManualRequired), err
}

func (p *KernelLifecycleProxy) LegacyReadState(ctx context.Context, extensionID string) (string, error) {
	status, found, err := p.legacyMigrationStatus(ctx, extensionID)
	if err != nil {
		return "unknown", err
	}
	if !found {
		return "unknown", fmt.Errorf("legacy migration state is not registered")
	}
	return string(status), nil
}

func (p *KernelLifecycleProxy) LegacyReadAllowed(ctx context.Context, extensionID string) bool {
	return false
}

func (p *KernelLifecycleProxy) LegacyMigrationReadAllowed(ctx context.Context, extensionID string) bool {
	if !isLegacyMigrationToolContext(ctx) {
		return false
	}
	status, err := p.LegacyReadState(ctx, extensionID)
	return err == nil && status == string(LegacyMigrationManualRequired)
}

func (p *KernelLifecycleProxy) recordLegacyMigration(ctx context.Context, extensionID, version, status, reason string, migratedAt any) error {
	state, err := normalizeLegacyMigrationState(status)
	if err != nil {
		return err
	}
	return p.recordLegacyMigrationState(ctx, legacyMigrationRecord{ExtensionID: extensionID, State: state, Failure: reason, Source: version})
}

func (p *KernelLifecycleProxy) recordLegacyMigrationState(ctx context.Context, record legacyMigrationRecord) error {
	c := p.container()
	if c == nil || c.Store == nil {
		return fmt.Errorf("extension kernel migration store unavailable")
	}
	if _, err := normalizeLegacyMigrationState(string(record.State)); err != nil {
		return err
	}
	if strings.TrimSpace(record.ExtensionID) == "" {
		return fmt.Errorf("legacy migration extension id required")
	}
	_, err := c.Store.DB().ExecContext(ctx, `INSERT INTO extension_package_legacy_migrations
		(extension_id, migration_status, attempt_count, last_error, legacy_path, artifact_id, updated_at)
		VALUES (?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT(extension_id) DO UPDATE SET migration_status=excluded.migration_status,
		attempt_count=extension_package_legacy_migrations.attempt_count+1,
		last_error=excluded.last_error, legacy_path=excluded.legacy_path,
		artifact_id=excluded.artifact_id, updated_at=excluded.updated_at`, record.ExtensionID, record.State, record.Failure, record.Source, record.ArtifactID, time.Now().UTC().Format(time.RFC3339Nano))
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
	exists, err := p.extensionExists(ctx, extensionID)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	parsedVersion, err := domain.ParseVersion(version)
	if err != nil {
		return err
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
	exists, err := p.extensionExists(ctx, extensionID)
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	return p.execute(ctx, lifecycle_manager.LifecycleCommand{
		Kind:        lifecycle_manager.CmdRepair,
		ExtensionID: domain.ExtensionID(extensionID),
	})
}
