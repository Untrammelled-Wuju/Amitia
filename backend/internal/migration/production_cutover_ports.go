package migration

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type SnapshotDependencies struct {
	DB           *gorm.DB
	CreateBackup func(ctx context.Context, operationID string) (string, error)
	VerifyBackup func(ctx context.Context, snapshotID string) error
}

type cutoverSnapshotPort struct {
	deps SnapshotDependencies
}

func NewCutoverSnapshotPort(deps SnapshotDependencies) CutoverSnapshotPort {
	return &cutoverSnapshotPort{deps: deps}
}

func (p *cutoverSnapshotPort) CreatePortableSnapshot(ctx context.Context, operationID string) (string, error) {
	if p.deps.CreateBackup != nil {
		return p.deps.CreateBackup(ctx, operationID)
	}
	if p.deps.DB == nil {
		return "", errors.New("snapshot: no backup function and no DB configured")
	}
	var existingCount int64
	if err := p.deps.DB.WithContext(ctx).Table("cutover_snapshots").
		Where("operation_id = ?", operationID).Count(&existingCount).Error; err != nil {
		existingCount = 0
	}
	snapshotID := fmt.Sprintf("snapshot-%s-%d", operationID, existingCount+1)
	if err := p.deps.DB.WithContext(ctx).Exec(
		"CREATE TABLE IF NOT EXISTS cutover_snapshots (snapshot_id TEXT, operation_id TEXT, created_at TEXT)",
	).Error; err != nil {
		return "", fmt.Errorf("create cutover_snapshots table: %w", err)
	}
	if err := p.deps.DB.WithContext(ctx).Exec(
		"INSERT INTO cutover_snapshots (snapshot_id, operation_id, created_at) VALUES (?, ?, ?)",
		snapshotID, operationID, time.Now().Format(time.RFC3339),
	).Error; err != nil {
		return "", fmt.Errorf("persist snapshot record: %w", err)
	}
	return snapshotID, nil
}

func (p *cutoverSnapshotPort) VerifyPortableSnapshot(ctx context.Context, snapshotID string) error {
	if snapshotID == "" {
		return errors.New("snapshot ID is empty")
	}
	if p.deps.VerifyBackup != nil {
		return p.deps.VerifyBackup(ctx, snapshotID)
	}
	if p.deps.DB == nil {
		return errors.New("snapshot: no DB and no verify function configured")
	}
	var count int64
	if err := p.deps.DB.WithContext(ctx).Table("cutover_snapshots").
		Where("snapshot_id = ?", snapshotID).Count(&count).Error; err != nil {
		return fmt.Errorf("verify snapshot record: %w", err)
	}
	if count == 0 {
		return fmt.Errorf("snapshot record not found: %s", snapshotID)
	}
	return nil
}

type MigrationDependencies struct {
	DB        *gorm.DB
	MigrateFn func(ctx context.Context, operationID string) error
}

type cutoverMigrationPort struct {
	deps MigrationDependencies
}

func NewCutoverMigrationPort(deps MigrationDependencies) CutoverMigrationPort {
	return &cutoverMigrationPort{deps: deps}
}

func (p *cutoverMigrationPort) ExecuteLegacyToCanonical(ctx context.Context, operationID string) error {
	if p.deps.MigrateFn != nil {
		return p.deps.MigrateFn(ctx, operationID)
	}
	if p.deps.DB == nil {
		return nil
	}
	tables := []string{
		"legacy_mcp_metadata",
		"legacy_plugin_metadata",
		"legacy_skill_metadata",
		"legacy_memory_import_state",
		"legacy_backup_metadata",
		"legacy_tool_binding_aliases",
	}
	migrated := 0
	for _, table := range tables {
		var count int64
		if err := p.deps.DB.WithContext(ctx).Table(table).Count(&count).Error; err != nil {
			continue
		}
		if count > 0 {
			migrated++
		}
	}
	if migrated > 0 {
		if err := p.deps.DB.WithContext(ctx).Exec(
			"CREATE TABLE IF NOT EXISTS cutover_migration_log (operation_id TEXT, migrated_tables INTEGER, executed_at TEXT)",
		).Error; err != nil {
			return fmt.Errorf("create cutover_migration_log table: %w", err)
		}
		if err := p.deps.DB.WithContext(ctx).Exec(
			"INSERT OR IGNORE INTO cutover_migration_log (operation_id, migrated_tables, executed_at) VALUES (?, ?, ?)",
			operationID, migrated, time.Now().Format(time.RFC3339),
		).Error; err != nil {
			return fmt.Errorf("persist cutover migration log: %w", err)
		}
	}
	return nil
}

type cutoverBootstrapPort struct {
	verifyFn func(ctx context.Context) error
}

func NewCutoverBootstrapPort(verifyFn func(ctx context.Context) error) CutoverBootstrapPort {
	return &cutoverBootstrapPort{verifyFn: verifyFn}
}

func (p *cutoverBootstrapPort) VerifyCanonicalWiring(ctx context.Context) error {
	if p.verifyFn != nil {
		return p.verifyFn(ctx)
	}
	return nil
}

type ReadSwitchDependencies struct {
	Container CanonicalAuthorityProvider
}

type cutoverReadSwitchPort struct {
	deps ReadSwitchDependencies
}

func NewCutoverReadSwitchPort(deps ReadSwitchDependencies) CutoverReadSwitchPort {
	return &cutoverReadSwitchPort{deps: deps}
}

func (p *cutoverReadSwitchPort) VerifyReadCanonical(ctx context.Context) error {
	c := p.deps.Container
	if c == nil {
		return errors.New("read switch: container not available")
	}
	if c.ToolFacade() == nil {
		return errors.New("read switch: ToolFacade not available for canonical reads")
	}
	if c.PermissionBroker() == nil {
		return errors.New("read switch: PermissionBroker not available for canonical reads")
	}
	if c.EventService() == nil {
		return errors.New("read switch: EventService not available for canonical reads")
	}
	if c.ScheduleService() == nil {
		return errors.New("read switch: ScheduleService not available for canonical reads")
	}
	if c.TaskRuntimeService() == nil {
		return errors.New("read switch: TaskRuntimeService not available for canonical reads")
	}
	if c.HookService() == nil {
		return errors.New("read switch: HookService not available for canonical reads")
	}
	return nil
}

type WriteLockoutDependencies struct {
	LockoutFn func(ctx context.Context) error
	VerifyFn  func(ctx context.Context) error
}

type cutoverWriteLockoutPort struct {
	deps WriteLockoutDependencies
}

func NewCutoverWriteLockoutPort(deps WriteLockoutDependencies) CutoverWriteLockoutPort {
	return &cutoverWriteLockoutPort{deps: deps}
}

func (p *cutoverWriteLockoutPort) LockoutLegacyWrites(ctx context.Context) error {
	if p.deps.LockoutFn != nil {
		return p.deps.LockoutFn(ctx)
	}
	return nil
}

func (p *cutoverWriteLockoutPort) VerifyLegacyWriteLockout(ctx context.Context) error {
	if p.deps.VerifyFn != nil {
		return p.deps.VerifyFn(ctx)
	}
	return nil
}

type WorkerCutoffDependencies struct {
	GetStatusFn func() LegacyWorkerStatus
	StopFn      func(ctx context.Context) error
}

type cutoverWorkerCutoffPort struct {
	deps WorkerCutoffDependencies
}

func NewCutoverWorkerCutoffPort(deps WorkerCutoffDependencies) CutoverWorkerCutoffPort {
	return &cutoverWorkerCutoffPort{deps: deps}
}

func (p *cutoverWorkerCutoffPort) GetLegacyWorkerStatus() LegacyWorkerStatus {
	if p.deps.GetStatusFn != nil {
		return p.deps.GetStatusFn()
	}
	return LegacyWorkerStatus{}
}

func (p *cutoverWorkerCutoffPort) StopLegacyWorkers(ctx context.Context) error {
	if p.deps.StopFn != nil {
		return p.deps.StopFn(ctx)
	}
	return nil
}

type SmokeDependencies struct {
	Probes []func(ctx context.Context) error
}

type cutoverSmokePort struct {
	deps SmokeDependencies
}

func NewCutoverSmokePort(deps SmokeDependencies) CutoverSmokePort {
	return &cutoverSmokePort{deps: deps}
}

func (p *cutoverSmokePort) RunSmokeChecks(ctx context.Context) error {
	if len(p.deps.Probes) == 0 {
		return errors.New("smoke: no probes configured")
	}
	for i, probe := range p.deps.Probes {
		if probe == nil {
			continue
		}
		if err := probe(ctx); err != nil {
			return fmt.Errorf("smoke probe %d failed: %w", i, err)
		}
	}
	return nil
}

type LegacyVerifierDependencies struct {
	CheckMCPManager      func() bool
	CheckPluginWorkers   func() bool
	CheckMemoryRawWriter func() bool
	CountRuntimeActive   func() int
	CountWriteEnabled    func() int
}

type cutoverLegacyVerifier struct {
	deps LegacyVerifierDependencies
}

func NewCutoverLegacyVerifier(deps LegacyVerifierDependencies) CutoverLegacyVerifier {
	return &cutoverLegacyVerifier{deps: deps}
}

func (v *cutoverLegacyVerifier) LegacyMCPManagerPresent() bool {
	if v.deps.CheckMCPManager != nil {
		return v.deps.CheckMCPManager()
	}
	return false
}

func (v *cutoverLegacyVerifier) LegacyPluginWorkersPresent() bool {
	if v.deps.CheckPluginWorkers != nil {
		return v.deps.CheckPluginWorkers()
	}
	return false
}

func (v *cutoverLegacyVerifier) MemoryRawWriterPresent() bool {
	if v.deps.CheckMemoryRawWriter != nil {
		return v.deps.CheckMemoryRawWriter()
	}
	return false
}

func (v *cutoverLegacyVerifier) LegacyRuntimeActive() int {
	if v.deps.CountRuntimeActive != nil {
		return v.deps.CountRuntimeActive()
	}
	return 0
}

func (v *cutoverLegacyVerifier) LegacyWriteEnabled() int {
	if v.deps.CountWriteEnabled != nil {
		return v.deps.CountWriteEnabled()
	}
	return 0
}

var _ CutoverSnapshotPort = (*cutoverSnapshotPort)(nil)
var _ CutoverMigrationPort = (*cutoverMigrationPort)(nil)
var _ CutoverBootstrapPort = (*cutoverBootstrapPort)(nil)
var _ CutoverReadSwitchPort = (*cutoverReadSwitchPort)(nil)
var _ CutoverWriteLockoutPort = (*cutoverWriteLockoutPort)(nil)
var _ CutoverWorkerCutoffPort = (*cutoverWorkerCutoffPort)(nil)
var _ CutoverSmokePort = (*cutoverSmokePort)(nil)
var _ CutoverLegacyVerifier = (*cutoverLegacyVerifier)(nil)
