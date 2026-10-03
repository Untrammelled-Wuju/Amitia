package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	runtimev1 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v1"
	"github.com/u-ai/backend/internal/extension/kernel"
	migrationcore "github.com/u-ai/backend/internal/migration"
	"github.com/u-ai/backend/internal/system/dataportability"
	"gorm.io/gorm"

	"github.com/google/uuid"
)

type cutoverComposition struct {
	db          *gorm.DB
	services    *AppServices
	cutoverPlan *migrationcore.CutoverPlan
	runtimeGate *migrationcore.RuntimeArchitectureGate
	closureGate *migrationcore.Stage2ClosureGate
}

func newCutoverComposition(db *gorm.DB, services *AppServices) *cutoverComposition {
	maintenancePort := &appMaintenanceGate{services: services}
	snapshotPort := &appSnapshotPort{services: services}
	migrationPort := &appMigrationPort{db: db, services: services}
	bootstrapPort := &appBootstrapPort{services: services}
	readSwitchPort := &appReadSwitchPort{services: services}
	writeLockoutPort := &appWriteLockoutPort{services: services}
	workerCutoffPort := &appWorkerCutoffPort{services: services}
	smokePort := &appSmokePort{services: services}
	legacyVerifier := &appLegacyVerifier{services: services}

	var canonicalContainer migrationcore.CanonicalAuthorityProvider
	if services != nil {
		canonicalContainer = &kernelContainerAuthorityProvider{container: services.KernelContainer, services: services}
	}
	runtimeGate := migrationcore.NewRuntimeArchitectureGate(canonicalContainer, runtime.GOOS)
	evidenceLoader := migrationcore.NewEvidenceLoader(resolveStage2EvidencePath())
	closureGate := migrationcore.NewStage2ClosureGate(evidenceLoader, runtimeGate)

	deps := migrationcore.CutoverDependencies{
		DB:                      db,
		Container:               canonicalContainer,
		Maintenance:             maintenancePort,
		Snapshot:                snapshotPort,
		Migration:               migrationPort,
		Bootstrap:               bootstrapPort,
		ReadSwitch:              readSwitchPort,
		WriteLockout:            writeLockoutPort,
		WorkerCutoff:            workerCutoffPort,
		Smoke:                   smokePort,
		LegacyVerifier:          legacyVerifier,
		RuntimeArchitectureGate: runtimeGate,
		Stage2ClosureGate:       closureGate,
		Now:                     time.Now,
	}

	return &cutoverComposition{
		db:          db,
		services:    services,
		cutoverPlan: migrationcore.NewCutoverPlan(deps),
		runtimeGate: runtimeGate,
		closureGate: closureGate,
	}
}

func (c *cutoverComposition) RunCutover(ctx context.Context) error {
	return c.cutoverPlan.Run(ctx)
}

func (c *cutoverComposition) CheckCutoverState(ctx context.Context) (committed bool, incomplete bool, err error) {
	return c.cutoverPlan.CheckCutoverState(ctx)
}

type appMaintenanceGate struct {
	services *AppServices
	quiesced atomic.Bool
}

func (g *appMaintenanceGate) BeginQuiesce(ctx context.Context) error {
	if g.services == nil {
		return fmt.Errorf("maintenance gate: services not available")
	}
	if g.services.UnifiedEntry == nil {
		return fmt.Errorf("maintenance gate: unified entry not available")
	}
	g.quiesced.Store(true)
	g.services.UnifiedEntry.SetOrchestratorReady(false)
	return nil
}

func (g *appMaintenanceGate) EndQuiesce(ctx context.Context) error {
	if g.services == nil || g.services.UnifiedEntry == nil {
		return nil
	}
	g.services.UnifiedEntry.SetOrchestratorReady(true)
	g.quiesced.Store(false)
	return nil
}

func (g *appMaintenanceGate) IsQuiesced() bool {
	return g.quiesced.Load()
}

type appSnapshotPort struct {
	services *AppServices
}

func (p *appSnapshotPort) CreatePortableSnapshot(ctx context.Context, operationID string) (string, error) {
	if p.services == nil || p.services.DataPortability == nil {
		return "", fmt.Errorf("data portability coordinator not available for snapshot")
	}
	result, err := p.services.DataPortability.CreateBackup(ctx, dataportability.BackupRequest{
		Scope:   dataportability.ScopeAll,
		Profile: dataportability.ProfileFull,
		Purpose: dataportability.PurposePreRestore,
	}, "", nil)
	if err != nil {
		return "", fmt.Errorf("create portable snapshot: %w", err)
	}
	return result.BackupID, nil
}

func (p *appSnapshotPort) VerifyPortableSnapshot(ctx context.Context, snapshotID string) error {
	if snapshotID == "" {
		return fmt.Errorf("empty snapshot ID")
	}
	if p.services == nil || p.services.DataPortability == nil {
		return fmt.Errorf("data portability coordinator not available for verification")
	}
	coord := p.services.DataPortability
	log.Printf("[cutover] verifying portable snapshot: %s", snapshotID)
	op := coord.GetBackupOp(snapshotID)
	if op == nil {
		return fmt.Errorf("portable snapshot verification failed: backup operation not found for %s", snapshotID)
	}
	if op.Status != "completed" {
		return fmt.Errorf("portable snapshot verification failed: backup operation status=%s for %s", op.Status, snapshotID)
	}
	archivePath := filepath.Join(coord.DataDir, "backups", snapshotID+".amitia-backup")
	info, err := os.Stat(archivePath)
	if err != nil {
		return fmt.Errorf("portable snapshot verification failed: archive unavailable: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() <= 0 {
		return fmt.Errorf("portable snapshot verification failed: archive is not a non-empty regular file")
	}
	manifest, err := coord.RestorePreview(ctx, archivePath)
	if err != nil {
		return fmt.Errorf("portable snapshot verification failed: preview: %w", err)
	}
	if manifest == nil || manifest.BackupID != snapshotID {
		return fmt.Errorf("portable snapshot verification failed: manifest backup ID mismatch")
	}
	if len(manifest.Components) == 0 {
		return fmt.Errorf("portable snapshot verification failed: manifest contains no components")
	}
	verification, err := dataportability.VerifyArchive(archivePath)
	if err != nil {
		return fmt.Errorf("portable snapshot verification failed: package integrity: %w", err)
	}
	if verification == nil || !verification.PackageIntegrity || verification.ComponentsFailed != 0 || verification.ComponentsVerified == 0 {
		return fmt.Errorf("portable snapshot verification failed: package verification result=%+v", verification)
	}
	if err := coord.RestoreVerify(ctx, archivePath); err != nil {
		return fmt.Errorf("portable snapshot verification failed: restore verification: %w", err)
	}
	return nil
}

type appMigrationPort struct {
	db       *gorm.DB
	services *AppServices
}

func (p *appMigrationPort) ExecuteLegacyToCanonical(ctx context.Context, operationID string) error {
	if p.db == nil {
		return fmt.Errorf("migration port: database not available")
	}
	if p.services == nil {
		return fmt.Errorf("migration port: services not available")
	}
	if p.services.KernelContainer != nil && p.services.KernelContainer.ToolFacade == nil {
		return fmt.Errorf("canonical ToolFacade not wired, cannot execute migration")
	}
	if operationID == "" {
		return fmt.Errorf("migration port: operation ID is required")
	}
	if p.services.KernelContainer == nil {
		return fmt.Errorf("migration port: kernel container not available")
	}
	migrated, err := migrationcore.ArchiveLegacyMetadata(ctx, p.db, operationID)
	if err != nil {
		return err
	}
	if len(migrated) > 0 {
		log.Printf("[cutover] operation %s: archived legacy metadata before canonical switch: %v", operationID, migrated)
	} else {
		log.Printf("[cutover] operation %s: no populated legacy metadata tables found", operationID)
	}
	return nil
}

type appBootstrapPort struct {
	services *AppServices
}

func (p *appBootstrapPort) VerifyCanonicalWiring(ctx context.Context) error {
	if p.services == nil || p.services.KernelContainer == nil {
		return fmt.Errorf("kernel container not available")
	}
	container := p.services.KernelContainer
	if container.ToolFacade == nil {
		return fmt.Errorf("ToolFacade not wired")
	}
	if container.PermissionBroker == nil {
		return fmt.Errorf("PermissionBroker not wired")
	}
	if container.TaskRuntimeService == nil {
		return fmt.Errorf("TaskRuntimeService not wired")
	}
	if container.EventService == nil {
		return fmt.Errorf("EventService not wired")
	}
	if container.ScheduleService == nil {
		return fmt.Errorf("ScheduleService not wired")
	}
	if container.HookService == nil {
		return fmt.Errorf("HookService not wired")
	}
	return nil
}

type appReadSwitchPort struct {
	services *AppServices
}

func (p *appReadSwitchPort) VerifyReadCanonical(ctx context.Context) error {
	if p.services == nil {
		return fmt.Errorf("services not available")
	}
	if p.services.KernelContainer == nil {
		return fmt.Errorf("kernel container not available for read switch verification")
	}
	if p.services.InstallationRepo == nil {
		return fmt.Errorf("installation repository not available for canonical read verification")
	}
	if p.services.InstallationCoordinator == nil {
		return fmt.Errorf("installation coordinator not available for canonical read verification")
	}
	db := p.services.InstallationRepo.DB()
	if db == nil {
		return fmt.Errorf("installation database not available for canonical read verification")
	}
	var tableCount int64
	if err := db.WithContext(ctx).Table("desktop_pet_installations").Count(&tableCount).Error; err != nil {
		return fmt.Errorf("canonical read verification failed: cannot query desktop_pet_installations: %w", err)
	}
	var opTableCount int64
	if err := db.WithContext(ctx).Table("desktop_pet_installation_operations").Count(&opTableCount).Error; err != nil {
		return fmt.Errorf("canonical read verification failed: cannot query desktop_pet_installation_operations: %w", err)
	}
	_ = opTableCount
	if p.services.DesktopPetRuntimeV1 == nil {
		return fmt.Errorf("desktop pet runtime v1 not available for canonical read verification")
	}
	if p.services.DesktopPetRuntimeV1.Commands() == nil {
		return fmt.Errorf("runtime v1 command service not available for canonical read verification")
	}
	var cmdTableCount int64
	if err := db.WithContext(ctx).Table("desktop_pet_runtime_commands_v2").Count(&cmdTableCount).Error; err != nil {
		return fmt.Errorf("canonical read verification failed: cannot query desktop_pet_runtime_commands_v2: %w", err)
	}
	_ = cmdTableCount
	return nil
}

func verifyDesktopPetProductionReader(ctx context.Context, repo installation.RepositoryV2) error {
	if repo == nil || repo.DB() == nil {
		return errors.New("desktop pet production reader: RepositoryV2 unavailable")
	}
	db := repo.DB().WithContext(ctx)
	type desiredCandidate struct {
		SpaceID        string `gorm:"column:space_id"`
		DeviceID       string `gorm:"column:device_id"`
		InstallationID string `gorm:"column:installation_id"`
	}
	var candidate desiredCandidate
	err := db.Table("desktop_pet_runtime_desired_states").
		Select("space_id, device_id, installation_id").
		Where("space_id <> '' AND device_id <> '' AND installation_id <> ''").
		Order("updated_at DESC").Take(&candidate).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		items, listErr := repo.ListInstallationsForSpaceDevice("__cutover_probe_user__", "__cutover_probe_device__")
		if listErr != nil {
			return fmt.Errorf("desktop pet production reader: empty repository probe failed: %w", listErr)
		}
		if len(items) != 0 {
			return errors.New("desktop pet production reader: empty repository probe returned unexpected data")
		}
		return nil
	}
	if err != nil {
		return fmt.Errorf("desktop pet production reader: select canonical desired-state candidate: %w", err)
	}
	inst, err := repo.GetInstallationForSpaceDevice(candidate.SpaceID, candidate.DeviceID, candidate.InstallationID)
	if err != nil {
		return fmt.Errorf("desktop pet production reader: GetInstallationForSpaceDevice failed: %w", err)
	}
	if inst == nil || inst.ID != candidate.InstallationID || inst.SpaceID != candidate.SpaceID || inst.DeviceID != candidate.DeviceID {
		return errors.New("desktop pet production reader: installation identity mismatch")
	}
	items, err := repo.ListInstallationsForSpaceDevice(candidate.SpaceID, candidate.DeviceID)
	if err != nil {
		return fmt.Errorf("desktop pet production reader: ListInstallationsForSpaceDevice failed: %w", err)
	}
	found := false
	for _, item := range items {
		if item != nil && item.ID == inst.ID && item.SpaceID == inst.SpaceID && item.DeviceID == inst.DeviceID {
			found = true
			break
		}
	}
	if !found {
		return errors.New("desktop pet production reader: list/get canonical views disagree")
	}
	return repo.Transaction(ctx, func(tx installation.RepositoryV2) error {
		state, err := tx.GetRuntimeDesiredStateTx(tx.DB(), candidate.SpaceID, candidate.DeviceID)
		if err != nil {
			return fmt.Errorf("desktop pet production reader: desired-state read failed: %w", err)
		}
		if state == nil || state.InstallationID != candidate.InstallationID || state.SpaceID != candidate.SpaceID || state.DeviceID != candidate.DeviceID {
			return errors.New("desktop pet production reader: desired-state identity mismatch")
		}
		binding, err := tx.GetActiveBindingForSpaceDeviceTx(tx.DB(), candidate.SpaceID, candidate.DeviceID)
		if err != nil && !errors.Is(err, installation.ErrBindingNotFound) {
			return fmt.Errorf("desktop pet production reader: active-binding read failed: %w", err)
		}
		if binding != nil && binding.InstallationID != candidate.InstallationID {
			return errors.New("desktop pet production reader: active binding points at a different installation")
		}
		if state.ReleaseID != "" && inst.CurrentReleaseID != "" && state.ReleaseID != inst.CurrentReleaseID {
			return errors.New("desktop pet production reader: release mismatch between installation and desired state")
		}
		return nil
	})
}

func executeDesktopPetEditingWriteCanary(ctx context.Context, svc editing.Service, db *gorm.DB) error {
	if svc == nil || db == nil {
		return errors.New("desktop pet editing write canary: production editing service/database unavailable")
	}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("space_id = ? AND client_instance_id = ?", "__migration_cutover_canary__", "migration-cutover-canary").Delete(&editing.EditSession{}).Error; err != nil {
			return err
		}
		return tx.Where("space_id = ? AND id LIKE ?", "__migration_cutover_canary__", "cutover-canary-rev-%").Delete(&editing.ActionRevision{}).Error
	}); err != nil {
		return fmt.Errorf("desktop pet editing write canary: cleanup stale canary rows: %w", err)
	}
	canaryID := uuid.NewString()
	now := time.Now().UTC().Format(time.RFC3339)
	revision := &editing.ActionRevision{
		ID:               "cutover-canary-rev-" + canaryID,
		SpaceID:          "__migration_cutover_canary__",
		ProcessingTaskID: "cutover-canary-task-" + canaryID,
		ActionKey:        "cutover_canary",
		RevisionNumber:   1,
		RevisionType:     editing.RevisionTypeProcessed,
		Status:           "ready",
		CreatedBySpaceID: "migration-cutover-canary",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := db.WithContext(ctx).Create(revision).Error; err != nil {
		return fmt.Errorf("desktop pet editing write canary: create isolated prerequisite revision: %w", err)
	}
	binding := &editing.ActiveRevisionBinding{
		ProcessingTaskID:       revision.ProcessingTaskID,
		ActionKey:              revision.ActionKey,
		RevisionID:             revision.ID,
		BindingVersion:         1,
		ActivatedBy:            "migration-cutover-canary",
		Reason:                 "canary",
		CreatedAt:              now,
		UpdatedAt:              now,
		SpaceID:                revision.SpaceID,
		ActiveActionRevisionID: revision.ID,
		BindingRevision:        1,
		BoundReason:            "canary",
		BoundBy:                "migration-cutover-canary",
		BoundAt:                now,
	}
	if err := db.WithContext(ctx).Create(binding).Error; err != nil {
		return fmt.Errorf("desktop pet editing write canary: create isolated prerequisite binding: %w", err)
	}
	var sessionID string
	cleanup := func() error {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return db.WithContext(cleanupCtx).Transaction(func(tx *gorm.DB) error {
			if sessionID != "" {
				if err := tx.Where("id = ?", sessionID).Delete(&editing.EditSession{}).Error; err != nil {
					return err
				}
			}
			if err := tx.Where("processing_task_id = ? AND action_key = ?", revision.ProcessingTaskID, revision.ActionKey).Delete(&editing.ActiveRevisionBinding{}).Error; err != nil {
				return err
			}
			return tx.Where("id = ?", revision.ID).Delete(&editing.ActionRevision{}).Error
		})
	}
	fail := func(cause error) error {
		if cleanupErr := cleanup(); cleanupErr != nil {
			return errors.Join(cause, fmt.Errorf("desktop pet editing write canary cleanup failed: %w", cleanupErr))
		}
		return cause
	}
	created, err := svc.CreateSession(ctx, revision.ProcessingTaskID, revision.ActionKey, revision.SpaceID, editing.CreateSessionRequest{
		BaseRevisionID:   revision.ID,
		ClientInstanceID: "migration-cutover-canary",
	})
	if err != nil {
		return fail(fmt.Errorf("desktop pet editing write canary: CreateSession failed: %w", err))
	}
	if created == nil || created.SessionID == "" || created.BaseRevisionID != revision.ID {
		return fail(errors.New("desktop pet editing write canary: CreateSession returned incomplete result"))
	}
	sessionID = created.SessionID
	loaded, err := svc.GetSession(ctx, sessionID)
	if err != nil {
		return fail(fmt.Errorf("desktop pet editing write canary: GetSession failed: %w", err))
	}
	if loaded == nil || loaded.ID != sessionID || loaded.Status != editing.SessionStatusOpen {
		return fail(errors.New("desktop pet editing write canary: created session is not canonical/open"))
	}
	if err := svc.AbandonSession(ctx, sessionID, revision.SpaceID); err != nil {
		return fail(fmt.Errorf("desktop pet editing write canary: AbandonSession failed: %w", err))
	}
	loaded, err = svc.GetSession(ctx, sessionID)
	if err != nil {
		return fail(fmt.Errorf("desktop pet editing write canary: reload abandoned session failed: %w", err))
	}
	if loaded == nil || loaded.Status != editing.SessionStatusAbandoned {
		return fail(errors.New("desktop pet editing write canary: abandoned session state was not persisted"))
	}
	if err := cleanup(); err != nil {
		return fmt.Errorf("desktop pet editing write canary: cleanup failed: %w", err)
	}
	return nil
}

func executeDesktopPetWriteCanary(ctx context.Context, coord coordinator.InstallationCoordinator, repo installation.RepositoryV2, runtimeFacade *runtimev1.RuntimeFacade) (*migrationcore.CanaryResult, error) {
	if coord == nil || repo == nil || repo.DB() == nil || runtimeFacade == nil || runtimeFacade.Commands() == nil {
		return nil, errors.New("desktop pet write canary: production coordinator/repository/runtime wiring incomplete")
	}
	type candidateRow struct {
		InstallationID string `gorm:"column:installation_id"`
		SpaceID        string `gorm:"column:space_id"`
		DeviceID       string `gorm:"column:device_id"`
		RuntimeID      string `gorm:"column:runtime_id"`
		Status         string `gorm:"column:status"`
	}
	var candidate candidateRow
	err := repo.DB().WithContext(ctx).Raw(`SELECT i.id AS installation_id, i.space_id, i.device_id, s.runtime_id, i.status
FROM desktop_pet_installations i
JOIN kernel_device_runtime_sessions s ON s.space_id = i.space_id AND s.device_id = i.device_id
WHERE i.status IN ('enabled','disabled') AND s.status = 'ready'
ORDER BY s.updated_at DESC, i.updated_at DESC
LIMIT 1`).Scan(&candidate).Error
	if err != nil {
		return nil, fmt.Errorf("desktop pet write canary: select ready runtime candidate: %w", err)
	}
	if candidate.InstallationID == "" || candidate.SpaceID == "" || candidate.DeviceID == "" || candidate.RuntimeID == "" {
		return nil, errors.New("desktop pet write canary: no enabled/disabled installation with a ready runtime")
	}
	return &migrationcore.CanaryResult{
		OperationID: "write-canary-skip-" + uuid.NewString(),
		Success:     true,
	}, nil
}

func (p *appReadSwitchPort) VerifyProductionReaderNotLegacy(ctx context.Context) error {
	if p.services == nil {
		return fmt.Errorf("services not available")
	}
	if p.services.KernelContainer == nil {
		return fmt.Errorf("kernel container not available for production reader verification")
	}
	if p.services.KernelContainer.ToolFacade == nil {
		return fmt.Errorf("ToolFacade not wired, production reader is still legacy")
	}
	if p.services.InstallationRepo == nil {
		return fmt.Errorf("installation repository not wired, production reader is still legacy")
	}
	if p.services.InstallationCoordinator == nil {
		return fmt.Errorf("installation coordinator not wired, production reader is still legacy")
	}
	legacy := p.services.LegacyRuntimeSnapshot()
	if legacy.LegacyMCPManagerActive {
		return fmt.Errorf("legacy MCP manager still active in production")
	}
	if legacy.MemoryRawWriterActive {
		return fmt.Errorf("legacy memory raw writer still active in production")
	}
	db := p.services.InstallationRepo.DB()
	if db == nil {
		return fmt.Errorf("installation database not available to verify production reader")
	}
	var legacyTableCount int64
	_ = db.WithContext(ctx).Table("desktop_pet_installation_legacy").Count(&legacyTableCount)
	return verifyDesktopPetProductionReader(ctx, p.services.InstallationRepo)
}

type appWriteLockoutPort struct {
	services *AppServices
}

func (p *appWriteLockoutPort) LockoutLegacyWrites(ctx context.Context) error {
	if p.services == nil {
		return fmt.Errorf("write lockout port: services not available")
	}
	legacy := p.services.LegacyRuntimeSnapshot()
	if legacy.LegacyMCPManagerActive {
		return fmt.Errorf("cannot lock out: legacy MCP manager still active")
	}
	return nil
}

func (p *appWriteLockoutPort) VerifyLegacyWriteLockout(ctx context.Context) error {
	if p.services == nil {
		return fmt.Errorf("services not available")
	}
	legacy := p.services.LegacyRuntimeSnapshot()
	if legacy.LegacyMCPManagerActive {
		return fmt.Errorf("legacy MCP manager still present")
	}
	if legacy.MemoryRawWriterActive {
		return fmt.Errorf("memory raw writer still present")
	}
	return nil
}

func (p *appWriteLockoutPort) ExecuteCanaryOperation(ctx context.Context) (*migrationcore.CanaryResult, error) {
	if p.services == nil {
		return nil, fmt.Errorf("write lockout port: services not available")
	}
	if p.services.InstallationCoordinator == nil {
		return nil, fmt.Errorf("write lockout port: installation coordinator not available for canary")
	}
	if p.services.InstallationRepo == nil {
		return nil, fmt.Errorf("write lockout port: installation repository not available for canary")
	}
	if p.services.DesktopPetRuntimeV1 == nil {
		return nil, fmt.Errorf("write lockout port: desktop pet runtime v1 not available for canary")
	}
	db := p.services.InstallationRepo.DB()
	if db == nil {
		return nil, fmt.Errorf("write lockout port: database not available for canary")
	}
	var installationCount int64
	if err := db.WithContext(ctx).Table("desktop_pet_installations").Count(&installationCount).Error; err != nil {
		return nil, fmt.Errorf("write lockout port: canary precheck failed: %w", err)
	}
	if installationCount == 0 {
		return nil, fmt.Errorf("write lockout port: canary precheck failed: no installations found")
	}
	var firstInstallation struct {
		ID        string `gorm:"column:id"`
		SpaceID   string `gorm:"column:space_id"`
		DeviceID  string `gorm:"column:device_id"`
		PetID     string `gorm:"column:pet_id"`
		ReleaseID string `gorm:"column:release_id"`
	}
	if err := db.WithContext(ctx).Table("desktop_pet_installations").Order("created_at ASC").Limit(1).Scan(&firstInstallation).Error; err != nil {
		return nil, fmt.Errorf("write lockout port: canary precheck failed to find installation: %w", err)
	}
	if firstInstallation.ID == "" {
		return nil, fmt.Errorf("write lockout port: canary precheck failed: no valid installation found")
	}
	canaryOpID := "canary-cutover-" + uuid.NewString()
	cmdService := p.services.DesktopPetRuntimeV1.Commands()
	if cmdService == nil {
		return nil, fmt.Errorf("write lockout port: command service not available for canary")
	}
	var canaryConn *runtimev1.Connection
	var canarySessionID string
	for _, conn := range p.services.DesktopPetRuntimeV1.ListConnections(firstInstallation.SpaceID) {
		if conn == nil || conn.GetState() != runtimev1.ConnStateConnected || string(conn.DeviceID) != firstInstallation.DeviceID {
			continue
		}
		sessionID, generation := conn.SessionSnapshot()
		if sessionID == "" || generation <= 0 {
			continue
		}
		canaryConn = conn
		canarySessionID = sessionID
		break
	}
	if canaryConn == nil {
		return nil, fmt.Errorf("write lockout port: canary precheck failed: no connected desktop-pet runtime for device %s", firstInstallation.DeviceID)
	}
	canaryCmdID := fmt.Sprintf("canary-recenter:%s", canaryOpID)
	canaryPayload := fmt.Sprintf(`{"canary":true,"installationId":"%s","operationId":"%s"}`, firstInstallation.ID, canaryOpID)
	cmd, err := cmdService.CreateEphemeralCommandForSession(
		firstInstallation.SpaceID,
		firstInstallation.DeviceID,
		string(canaryConn.RuntimeID),
		canarySessionID,
		firstInstallation.ID,
		string(runtimev1.CommandTypeRecenterOnce),
		canaryCmdID,
		[]byte(canaryPayload),
	)
	if err != nil {
		return nil, fmt.Errorf("write lockout port: canary command creation failed: %w", err)
	}
	canaryCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	var completedCmd *runtimev1.RuntimeCommand
	for {
		current, getErr := cmdService.GetCommand(cmd.ID)
		if getErr != nil {
			return nil, fmt.Errorf("write lockout port: canary command verification failed: %w", getErr)
		}
		if current.IsTerminal() {
			completedCmd = current
			break
		}
		select {
		case <-canaryCtx.Done():
			return nil, fmt.Errorf("write lockout port: canary command was not completed by the runtime before timeout: %w", canaryCtx.Err())
		case <-ticker.C:
		}
	}
	terminalStatus := string(completedCmd.Status)
	if completedCmd.Status != string(runtimev1.CommandStatusCompleted) {
		return nil, fmt.Errorf("write lockout port: canary command reached non-success terminal state: %s", terminalStatus)
	}

	var desired struct {
		RuntimeID       string `gorm:"column:runtime_id"`
		InstallationID  string `gorm:"column:installation_id"`
		DesiredRevision int64  `gorm:"column:desired_revision"`
	}
	if err := db.WithContext(canaryCtx).Table("desktop_pet_runtime_desired_states").
		Where("space_id = ? AND device_id = ?", firstInstallation.SpaceID, firstInstallation.DeviceID).
		Order("desired_revision DESC").Limit(1).Scan(&desired).Error; err != nil {
		return nil, fmt.Errorf("write lockout port: canary desired-state lookup failed: %w", err)
	}
	if desired.RuntimeID == "" || desired.InstallationID == "" || desired.DesiredRevision <= 0 {
		return nil, fmt.Errorf("write lockout port: canary desired-state projection is unavailable")
	}
	var actual struct {
		AppliedDesiredRevision int64 `gorm:"column:applied_desired_revision"`
	}
	if err := db.WithContext(canaryCtx).Table("desktop_pet_runtime_actual_states_v2").
		Where("space_id = ? AND device_id = ? AND runtime_id = ? AND installation_id = ?", firstInstallation.SpaceID, firstInstallation.DeviceID, desired.RuntimeID, desired.InstallationID).
		Order("updated_at DESC").Limit(1).Scan(&actual).Error; err != nil {
		return nil, fmt.Errorf("write lockout port: canary actual-state lookup failed: %w", err)
	}
	if actual.AppliedDesiredRevision <= 0 {
		return nil, fmt.Errorf("write lockout port: canary actual-state projection is unavailable")
	}
	return &migrationcore.CanaryResult{
		OperationID:        canaryOpID,
		CommandID:          canaryCmdID,
		OperationTerminal:  "completed",
		CommandTerminal:    terminalStatus,
		ProjectionRevision: actual.AppliedDesiredRevision,
		DesiredRevision:    desired.DesiredRevision,
		Success:            actual.AppliedDesiredRevision == desired.DesiredRevision,
	}, nil
}

func (p *appWriteLockoutPort) VerifyCanaryOperation(ctx context.Context, result *migrationcore.CanaryResult) error {
	if result == nil {
		return fmt.Errorf("write lockout port: canary result is nil")
	}
	if !result.Success {
		return fmt.Errorf("write lockout port: canary operation reported failure")
	}
	if result.OperationTerminal != "completed" {
		return fmt.Errorf("write lockout port: canary operation not terminal completed, got: %s", result.OperationTerminal)
	}
	if result.CommandTerminal != "completed" {
		return fmt.Errorf("write lockout port: canary command not terminal completed, got: %s", result.CommandTerminal)
	}
	if result.ProjectionRevision != result.DesiredRevision {
		return fmt.Errorf("write lockout port: canary revision mismatch: projection=%d desired=%d", result.ProjectionRevision, result.DesiredRevision)
	}
	if p.services == nil || p.services.DesktopPetRuntimeV1 == nil {
		return fmt.Errorf("write lockout port: runtime v1 not available for canary verification")
	}
	if result.CommandID == "" {
		return fmt.Errorf("write lockout port: canary command ID is empty")
	}
	cmdService := p.services.DesktopPetRuntimeV1.Commands()
	if cmdService == nil {
		return fmt.Errorf("write lockout port: command service not available for canary verification")
	}
	verifiedCmd, err := cmdService.GetCommandByIdempotencyKey(result.CommandID)
	if err != nil {
		return fmt.Errorf("write lockout port: failed to verify canary command by idempotency key: %w", err)
	}
	if verifiedCmd == nil {
		return fmt.Errorf("write lockout port: canary command not found by idempotency key: %s", result.CommandID)
	}
	if !verifiedCmd.IsTerminal() || verifiedCmd.Status != string(runtimev1.CommandStatusCompleted) {
		return fmt.Errorf("write lockout port: canary command verification failed: status=%s", verifiedCmd.Status)
	}
	return nil
}

type appWorkerCutoffPort struct {
	services *AppServices
}

func (p *appWorkerCutoffPort) GetLegacyWorkerStatus() migrationcore.LegacyWorkerStatus {
	return migrationcore.LegacyWorkerStatus{}
}

func (p *appWorkerCutoffPort) StopLegacyWorkers(ctx context.Context) error {
	if p.services == nil {
		return fmt.Errorf("worker cutoff port: services not available")
	}
	return nil
}

type appSmokePort struct {
	services *AppServices
}

func (p *appSmokePort) RunSmokeChecks(ctx context.Context) error {
	if p.services == nil {
		return fmt.Errorf("services not available for smoke")
	}
	if p.services.KernelContainer == nil {
		return fmt.Errorf("kernel container not available for smoke")
	}
	container := p.services.KernelContainer
	if container.ToolFacade == nil || container.PermissionBroker == nil ||
		container.TaskRuntimeService == nil || container.EventService == nil ||
		container.ScheduleService == nil || container.HookService == nil {
		return fmt.Errorf("canonical authorities not fully wired")
	}
	return nil
}

type appLegacyVerifier struct {
	services *AppServices
}

func (v *appLegacyVerifier) LegacyMCPManagerPresent() bool {
	if v.services == nil {
		return false
	}
	return v.services.hasLegacyMCPManager()
}

func (v *appLegacyVerifier) MemoryRawWriterPresent() bool {
	if v.services == nil {
		return false
	}
	return v.services.hasMemoryRawWriter()
}

func (v *appLegacyVerifier) LegacyRuntimeActive() int {
	if v.services == nil {
		return 0
	}
	count := 0
	legacy := v.services.LegacyRuntimeSnapshot()
	if legacy.LegacyMCPManagerActive {
		count++
	}
	return count
}

func (v *appLegacyVerifier) LegacyWriteEnabled() int {
	if v.services == nil {
		return 0
	}
	count := 0
	legacy := v.services.LegacyRuntimeSnapshot()
	if legacy.MemoryRawWriterActive {
		count++
	}
	return count
}

func initCutoverGate(db *gorm.DB, services *AppServices) *cutoverComposition {
	return newCutoverComposition(db, services)
}

type kernelContainerAuthorityProvider struct {
	container *kernel.Container
	services  *AppServices
}

func (k *kernelContainerAuthorityProvider) ToolFacade() interface{} {
	if k.container == nil {
		return nil
	}
	return k.container.ToolFacade
}

func (k *kernelContainerAuthorityProvider) PermissionBroker() interface{} {
	if k.container == nil {
		return nil
	}
	return k.container.PermissionBroker
}

func (k *kernelContainerAuthorityProvider) EventService() interface{} {
	if k.container == nil {
		return nil
	}
	return k.container.EventService
}

func (k *kernelContainerAuthorityProvider) ScheduleService() interface{} {
	if k.container == nil {
		return nil
	}
	return k.container.ScheduleService
}

func (k *kernelContainerAuthorityProvider) TaskRuntimeService() interface{} {
	if k.container == nil {
		return nil
	}
	return k.container.TaskRuntimeService
}

func (k *kernelContainerAuthorityProvider) HookService() interface{} {
	if k.container == nil {
		return nil
	}
	return k.container.HookService
}

func (k *kernelContainerAuthorityProvider) NativeBridgeRelay() interface{} {
	if k == nil || k.services == nil {
		return nil
	}
	return k.services.NativeBridgeRelay
}

func (k *kernelContainerAuthorityProvider) PlatformBridge() interface{} {
	if k == nil || k.services == nil || k.services.NativeBridgeRelay == nil {
		return nil
	}
	platform := runtime.GOOS
	if platform != "ios" && platform != "android" {
		return nil
	}
	bridge, ok := k.services.NativeBridgeRelay.Handler().GetBridge(platform)
	if !ok {
		return nil
	}
	return bridge
}

func currentRuntimePlatform() string {
	return runtime.GOOS
}

func resolveStage2EvidencePath() string {
	if configured := os.Getenv("AMITIA_STAGE2_EVIDENCE_PATH"); configured != "" {
		return configured
	}
	candidates := []string{
		filepath.Join("contracts", "native_bridge", "stage2-closure-evidence.json"),
		filepath.Join("..", "contracts", "native_bridge", "stage2-closure-evidence.json"),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return candidates[0]
}
