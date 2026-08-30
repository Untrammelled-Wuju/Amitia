// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/installation"
	"github.com/u-ai/backend/internal/desktoppet/installation/binding"
	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/journal"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/installation/projection"
	"github.com/u-ai/backend/internal/desktoppet/packageformat"
	runtimev2 "github.com/u-ai/backend/internal/desktoppet/runtime/protocol/v2"
	security "github.com/u-ai/backend/internal/desktoppet/security"
	"gorm.io/gorm"
)

type ProductionStagingRepo struct {
	db        *gorm.DB
	stager    coordinator.ReleaseStager
	registry  *security.PathRootRegistry
	responder *security.SafeArtifactResponder
}

func NewProductionStagingRepo(db *gorm.DB, stager coordinator.ReleaseStager, registry *security.PathRootRegistry) *ProductionStagingRepo {
	return &ProductionStagingRepo{db: db, stager: stager, registry: registry, responder: security.NewSafeArtifactResponder(registry)}
}

func (p *ProductionStagingRepo) CleanupStaging(opID string) error {
	if p == nil || p.db == nil || p.registry == nil {
		return errors.New("production staging recovery: not configured")
	}
	var op operation.InstallationOperation
	if err := p.db.Where("id = ?", opID).First(&op).Error; err != nil {
		return err
	}
	key := p.stagingKey(op)
	resolved, err := p.registry.Resolve(security.RootInstallations, key)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(resolved); errors.Is(err, os.ErrNotExist) {
		return nil
	} else if err != nil {
		return err
	}
	entityID := op.InstallationID
	if entityID == "" {
		entityID = op.ID
	}
	return p.responder.SafeDelete(security.RootInstallations, key, security.DeleteExpectation{EntityType: "installation_staging", EntityID: entityID})
}

func (p *ProductionStagingRepo) ReprepareStaging(opID, stagingPathKey, targetReleaseID string) (string, error) {
	if p == nil || p.db == nil || p.stager == nil {
		return "", errors.New("production staging recovery: not configured")
	}
	var op operation.InstallationOperation
	if err := p.db.Where("id = ?", opID).First(&op).Error; err != nil {
		return "", err
	}
	entityID := op.InstallationID
	if entityID == "" {
		entityID = op.ID
	}
	return p.stager.PrepareStagingCopy(context.Background(), targetReleaseID, entityID)
}

func (p *ProductionStagingRepo) VerifyStagingIntegrity(stagingPathKey, targetReleaseID string) (bool, error) {
	if p == nil || p.stager == nil {
		return false, errors.New("production staging recovery: not configured")
	}
	entityID := filepath.Base(filepath.FromSlash(stagingPathKey))
	if entityID == "." || entityID == string(filepath.Separator) || entityID == "" {
		return false, errors.New("production staging recovery: invalid staging path key")
	}
	if err := p.stager.VerifyStagingCopy(context.Background(), targetReleaseID, entityID, stagingPathKey); err != nil {
		return false, err
	}
	return true, nil
}

func (p *ProductionStagingRepo) PublishStaging(stagingPathKey, targetReleaseID string) (string, error) {
	valid, err := p.VerifyStagingIntegrity(stagingPathKey, targetReleaseID)
	if err != nil || !valid {
		return "", err
	}
	return stagingPathKey, nil
}

func (p *ProductionStagingRepo) stagingKey(op operation.InstallationOperation) string {
	var j journal.InstallationCommitJournal
	if p.db.Where("operation_id = ?", op.ID).First(&j).Error == nil && strings.TrimSpace(j.StagingPathKey) != "" {
		return j.StagingPathKey
	}
	entityID := op.InstallationID
	if entityID == "" {
		entityID = op.ID
	}
	return filepath.ToSlash(filepath.Join(".staging", entityID))
}

type ProductionDBRepo struct {
	db   *gorm.DB
	repo installation.RepositoryV2
}

func NewProductionDBRepo(db *gorm.DB, repo installation.RepositoryV2) *ProductionDBRepo {
	return &ProductionDBRepo{db: db, repo: repo}
}

func (p *ProductionDBRepo) DBCommitBatch(opID, installationID, targetReleaseID, previousReleaseID string) error {
	if p == nil || p.db == nil || p.repo == nil {
		return errors.New("production db recovery: not configured")
	}
	return p.repo.Transaction(context.Background(), func(tx installation.RepositoryV2) error {
		op, err := tx.GetOperationTx(tx.DB(), opID)
		if err != nil {
			return err
		}
		j, err := tx.GetCommitJournalTx(tx.DB(), opID)
		if err != nil {
			return err
		}
		if installationID == "" {
			installationID = op.InstallationID
		}
		if installationID == "" {
			return errors.New("production db recovery: installation id missing")
		}
		targetReleaseID = firstNonEmpty(targetReleaseID, j.TargetReleaseID, op.TargetReleaseID)
		petID := firstNonEmpty(op.PetID, j.PetID)
		now := time.Now().UTC().Format("2006-01-02 15:04:05")
		presentation, err := loadRecoveryReleasePresentation(tx.DB(), targetReleaseID)
		if err != nil {
			return err
		}
		installRel, _ := security.DefaultRelativePath(security.RootInstallations)
		installPath := filepath.ToSlash(filepath.Join(installRel, filepath.FromSlash(j.StagingPathKey)))
		manifestPath := filepath.ToSlash(filepath.Join(installPath, "manifest.json"))
		previewPath := ""
		if presentation.Manifest.Preview != "" {
			previewPath = filepath.ToSlash(filepath.Join(installPath, filepath.FromSlash(presentation.Manifest.Preview)))
		}
		defaultActionKey := presentation.Manifest.DefaultAction

		var inst installation.Installation
		err = tx.DB().Where("id = ? AND user_id = ? AND device_id = ?", installationID, op.UserID, op.DeviceID).First(&inst).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		characterID := presentation.Manifest.Binding.SourceCharacterID
		if errors.Is(err, gorm.ErrRecordNotFound) {
			inst = installation.Installation{
				ID:                     installationID,
				UserID:                 op.UserID,
				DeviceID:               op.DeviceID,
				PetID:                  petID,
				CharacterID:            characterID,
				PackageID:              targetReleaseID,
				PackageVersion:         presentation.Version,
				Name:                   presentation.Manifest.Name,
				CurrentReleaseID:       targetReleaseID,
				Status:                 installation.StatusEnabled,
				IsActive:               1,
				InstallPath:            installPath,
				ManifestPath:           manifestPath,
				PreviewPath:            previewPath,
				PreviewArtifactPath:    previewPath,
				DefaultActionKey:       defaultActionKey,
				DefaultActionReleaseID: targetReleaseID,
				CanvasWidth:            presentation.Manifest.Canvas.Width,
				CanvasHeight:           presentation.Manifest.Canvas.Height,
				PackageHash:            presentation.ContentRootHash,
				InstalledContentHash:   presentation.ContentRootHash,
				InstallStorageKey:      j.StagingPathKey,
				LifecycleState:         installation.LifecycleInstalled,
				IntegrityStatus:        installation.IntegrityVerified,
				DesiredState:           installation.DesiredEnabled,
				RuntimeSyncState:       installation.SyncPending,
				InstalledAt:            now,
				CreatedAt:              now,
				UpdatedAt:              now,
			}
			if err := tx.CreateInstallationTx(tx.DB(), &inst); err != nil {
				return err
			}
		} else {
			inst.PetID = petID
			inst.CharacterID = firstNonEmpty(inst.CharacterID, characterID)
			inst.PackageID = targetReleaseID
			inst.PackageVersion = presentation.Version
			inst.Name = presentation.Manifest.Name
			inst.CurrentReleaseID = targetReleaseID
			inst.Status = installation.StatusEnabled
			inst.IsActive = 1
			inst.InstallPath = installPath
			inst.ManifestPath = manifestPath
			inst.PreviewPath = previewPath
			inst.PreviewArtifactPath = previewPath
			inst.DefaultActionKey = defaultActionKey
			inst.DefaultActionReleaseID = targetReleaseID
			inst.CanvasWidth = presentation.Manifest.Canvas.Width
			inst.CanvasHeight = presentation.Manifest.Canvas.Height
			inst.PackageHash = presentation.ContentRootHash
			inst.InstalledContentHash = presentation.ContentRootHash
			inst.InstallStorageKey = j.StagingPathKey
			inst.LifecycleState = installation.LifecycleInstalled
			inst.IntegrityStatus = installation.IntegrityVerified
			inst.DesiredState = installation.DesiredEnabled
			inst.RuntimeSyncState = installation.SyncPending
			inst.UpdatedAt = now
			if inst.InstalledAt == "" {
				inst.InstalledAt = now
			}
			if err := tx.UpdateInstallationTx(tx.DB(), &inst); err != nil {
				return err
			}
		}
		if err := tx.DB().Model(&installation.Installation{}).Where("user_id = ? AND device_id = ? AND id <> ? AND is_active = 1", op.UserID, op.DeviceID, installationID).Updates(map[string]interface{}{
			"is_active": 0, "status": installation.StatusDisabled, "desired_state": installation.DesiredDisabled, "last_disabled_at": now, "updated_at": now,
		}).Error; err != nil {
			return err
		}

		settings, err := ensureRecoveryRuntimeSettings(tx, installationID, now)
		if err != nil {
			return err
		}
		settingsJSON, err := json.Marshal(settings)
		if err != nil {
			return err
		}

		existing, err := tx.GetRuntimeDesiredStateTx(tx.DB(), op.UserID, op.DeviceID)
		if err != nil {
			return err
		}
		expected := int64(-1)
		if existing != nil {
			expected = existing.DesiredRevision
		}
		revision, err := tx.AllocateDeviceDesiredRevisionCAS(tx.DB(), op.UserID, op.DeviceID)
		if err != nil {
			return err
		}
		state := &desired.RuntimeDesiredState{
			UserID: op.UserID, DeviceID: op.DeviceID, RuntimeID: op.RuntimeID,
			InstallationID: installationID, PetID: petID, ReleaseID: targetReleaseID,
			DesiredEnabled: true, DesiredVisible: true, DesiredRevision: revision,
			DesiredActionKey: defaultActionKey, SettingsRevision: int64(settings.SettingsRevision),
			SettingsSnapshotJSON: string(settingsJSON), OperationID: op.ID, CreatedAt: now, UpdatedAt: now,
		}
		if _, err := tx.UpsertRuntimeDesiredStateCAS(tx.DB(), op.UserID, op.DeviceID, state, expected); err != nil {
			return err
		}
		if err := tx.UpsertActiveBindingTx(tx.DB(), &binding.DeviceActiveInstallationBinding{
			UserID: op.UserID, DeviceID: op.DeviceID, InstallationID: installationID,
			PetID: petID, ReleaseID: targetReleaseID, BindingRevision: revision,
			BoundReason: binding.BoundReasonRestore, BoundAt: now, BoundBy: "recovery",
			CreatedAt: now, UpdatedAt: now,
		}); err != nil {
			return err
		}
		op.InstallationID = installationID
		op.DesiredRevision = revision
		op.Stage = operation.OpStageDatabaseCommitted
		return tx.UpdateOperationTx(tx.DB(), op)
	})
}

type recoveryReleasePresentation struct {
	Version         string
	ContentRootHash string
	Manifest        packageformat.Manifest
}

func loadRecoveryReleasePresentation(db *gorm.DB, releaseID string) (*recoveryReleasePresentation, error) {
	var row struct {
		Version         string `gorm:"column:version"`
		ContentRootHash string `gorm:"column:content_root_hash"`
		ManifestJSON    string `gorm:"column:manifest_json"`
	}
	if err := db.Table("desktop_pet_package_releases").Select("version, content_root_hash, manifest_json").Where("id = ?", releaseID).Take(&row).Error; err != nil {
		return nil, err
	}
	var manifest packageformat.Manifest
	if err := json.Unmarshal([]byte(row.ManifestJSON), &manifest); err != nil {
		return nil, err
	}
	return &recoveryReleasePresentation{Version: row.Version, ContentRootHash: row.ContentRootHash, Manifest: manifest}, nil
}

func ensureRecoveryRuntimeSettings(tx installation.RepositoryV2, installationID, now string) (*installation.RuntimeSettings, error) {
	var settings installation.RuntimeSettings
	err := tx.DB().Where("installation_id = ?", installationID).First(&settings).Error
	if err == nil {
		return &settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	settings = installation.RuntimeSettings{
		ID: "rts_" + uuid.NewString(), InstallationID: installationID, AlwaysOnTop: 1, Scale: 1.0,
		IdleEnabled: 1, IdleIntervalMinSeconds: 30, IdleIntervalMaxSeconds: 120,
		ClickThroughMode: "off", SettingsRevision: 0, RestoreOnAppStart: 1, PositionMode: "absolute",
		RelativeX: 0.5, RelativeY: 0.5, CreatedAt: now, UpdatedAt: now,
	}
	if err := tx.CreateRuntimeSettingsTx(tx.DB(), &settings); err != nil {
		return nil, err
	}
	return &settings, nil
}

func (p *ProductionDBRepo) DBMarkDatabaseCommitted(opID string) error {
	return p.db.Model(&operation.InstallationOperation{}).Where("id = ?", opID).Updates(map[string]interface{}{
		"stage":      operation.OpStageDatabaseCommitted,
		"updated_at": time.Now().UTC().Format("2006-01-02 15:04:05"),
	}).Error
}

func (p *ProductionDBRepo) GetInstallation(installationID string) (interface{}, error) {
	var inst installation.Installation
	if err := p.db.Where("id = ?", installationID).First(&inst).Error; err != nil {
		return nil, err
	}
	return &inst, nil
}

type ProductionRuntimeRepo struct {
	db     *gorm.DB
	facade *runtimev2.RuntimeFacade
}

func NewProductionRuntimeRepo(db *gorm.DB, facade *runtimev2.RuntimeFacade) *ProductionRuntimeRepo {
	return &ProductionRuntimeRepo{db: db, facade: facade}
}

func (p *ProductionRuntimeRepo) SendDesiredCommand(ctx context.Context, opID, userID, deviceID, runtimeID, installationID string, desiredRevision int64) error {
	if p == nil || p.db == nil || p.facade == nil {
		return errors.New("production runtime recovery: runtime v2 unavailable")
	}
	var op operation.InstallationOperation
	if err := p.db.WithContext(ctx).Where("id = ? AND user_id = ? AND device_id = ?", opID, userID, deviceID).First(&op).Error; err != nil {
		return err
	}
	if op.OperationType == operation.TypeRecenter {
		var targetConn *runtimev2.Connection
		targetSessionID := ""
		targetGeneration := int64(0)
		for _, conn := range p.facade.ListConnections(userID) {
			if conn == nil || conn.GetState() != runtimev2.ConnStateConnected || string(conn.DeviceID) != deviceID {
				continue
			}
			if runtimeID != "" && string(conn.RuntimeID) != runtimeID {
				continue
			}
			sessionID, generation := conn.SessionSnapshot()
			if sessionID == "" || generation <= 0 {
				continue
			}
			targetConn, targetSessionID, targetGeneration = conn, sessionID, generation
			break
		}
		if targetConn == nil {
			return errors.New("production runtime recovery: recenter target runtime is offline")
		}
		payload := []byte(fmt.Sprintf(`{"installationId":%q}`, firstNonEmpty(installationID, op.InstallationID)))
		key := fmt.Sprintf("recenter:%s:%s", opID, targetSessionID)
		cmd, err := p.facade.Commands().CreateEphemeralCommandForSession(
			userID, deviceID, string(targetConn.RuntimeID), targetSessionID, firstNonEmpty(installationID, op.InstallationID),
			string(runtimev2.CommandTypeRecenterOnce), key, payload,
		)
		if err != nil && !errors.Is(err, runtimev2.ErrCommandDuplication) {
			return err
		}
		if cmd == nil {
			return errors.New("production runtime recovery: recenter command creation returned nil")
		}
		currentSessionID, currentGeneration := targetConn.SessionSnapshot()
		if targetConn.GetState() != runtimev2.ConnStateConnected || currentSessionID != targetSessionID || currentGeneration != targetGeneration {
			if markErr := p.facade.Commands().MarkSuperseded(cmd.ID, "runtime session changed during recenter creation", time.Now().UTC()); markErr != nil {
				return fmt.Errorf("production runtime recovery: recenter session changed and stale command fencing failed: %w", markErr)
			}
			return errors.New("production runtime recovery: recenter runtime session changed")
		}
		if cmd.RuntimeSessionID != targetSessionID {
			return errors.New("production runtime recovery: duplicate recenter belongs to stale runtime session")
		}
		currentSessionID, currentGeneration = targetConn.SessionSnapshot()
		if targetConn.GetState() != runtimev2.ConnStateConnected || currentSessionID != targetSessionID || currentGeneration != targetGeneration {
			if markErr := p.facade.Commands().MarkSuperseded(cmd.ID, "runtime session changed after recenter route bind", time.Now().UTC()); markErr != nil {
				return fmt.Errorf("production runtime recovery: recenter route-bind session changed and stale command fencing failed: %w", markErr)
			}
			return errors.New("production runtime recovery: recenter runtime session changed after bind")
		}
		return nil
	}

	var state desired.RuntimeDesiredState
	if err := p.db.WithContext(ctx).Where("user_id = ? AND device_id = ?", userID, deviceID).First(&state).Error; err != nil {
		return err
	}
	if desiredRevision == 0 {
		desiredRevision = state.DesiredRevision
	}
	seq, err := p.facade.Commands().AllocateDeviceSequence(nil, userID, deviceID, time.Now())
	if err != nil {
		return err
	}
	ensureAbsent := op.OperationType == operation.TypeUninstall || (!state.DesiredEnabled && !state.DesiredVisible && op.OperationType == operation.TypeUninstall)
	payload := runtimev2.SyncDesiredStatePayload{
		DesiredRevision:        desiredRevision,
		DesiredHash:            state.DesiredHash,
		EnsureAbsent:           ensureAbsent,
		InstallationID:         firstNonEmpty(installationID, state.InstallationID),
		PetID:                  state.PetID,
		ReleaseID:              state.ReleaseID,
		RuntimeContractVersion: runtimev2.CurrentSchemaVersion,
		DefaultActionKey:       state.DesiredActionKey,
		SettingsRevision:       state.SettingsRevision,
	}
	commandType := runtimev2.CommandTypeSyncDesiredState
	if ensureAbsent {
		commandType = runtimev2.CommandTypeEnsureAbsent
	}
	_, err = p.facade.Commands().CreateDurableCommand(userID, deviceID, string(commandType), fmt.Sprintf("desired:%s:%d", deviceID, desiredRevision), fmt.Sprintf("desired:%s", deviceID), seq, payload)
	if errors.Is(err, runtimev2.ErrCommandDuplication) {
		return nil
	}
	return err
}

func (p *ProductionRuntimeRepo) ResolveDesiredRevision(ctx context.Context, opID, userID, deviceID string) (int64, error) {
	if p == nil || p.db == nil {
		return 0, errors.New("production runtime recovery: database unavailable")
	}
	var outbox desired.DesiredStateOutboxEvent
	err := p.db.WithContext(ctx).
		Where("operation_id = ? AND user_id = ? AND device_id = ? AND desired_revision > 0", opID, userID, deviceID).
		Order("created_at DESC").
		First(&outbox).Error
	if err == nil && outbox.DesiredRevision > 0 {
		return outbox.DesiredRevision, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}

	// Older recovery records may predate the outbox row but still preserve the
	// operation identity on the authoritative desired-state snapshot.
	var state desired.RuntimeDesiredState
	err = p.db.WithContext(ctx).
		Where("operation_id = ? AND user_id = ? AND device_id = ? AND desired_revision > 0", opID, userID, deviceID).
		First(&state).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, errors.New("production runtime recovery: authoritative desired revision not found for operation")
		}
		return 0, err
	}
	return state.DesiredRevision, nil
}

func (p *ProductionRuntimeRepo) CancelDesiredCommand(ctx context.Context, opID, userID, deviceID, runtimeID string) error {
	if p == nil || p.db == nil || p.facade == nil || p.facade.Commands() == nil {
		return errors.New("production runtime recovery: runtime v2 unavailable")
	}
	var op operation.InstallationOperation
	if err := p.db.WithContext(ctx).Where("id = ? AND user_id = ? AND device_id = ?", opID, userID, deviceID).First(&op).Error; err != nil {
		return err
	}
	idempotencyKey := fmt.Sprintf("desired:%s:%d", deviceID, op.DesiredRevision)
	var command *runtimev2.RuntimeCommand
	var err error
	if op.OperationType == operation.TypeRecenter {
		prefix := fmt.Sprintf("recenter:%s", op.ID)
		var latest runtimev2.RuntimeCommand
		err = p.db.WithContext(ctx).Where(
			"idempotency_key = ? OR idempotency_key LIKE ?", prefix, prefix+":%",
		).Order("created_at DESC").First(&latest).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err == nil {
			command = &latest
		}
	} else {
		command, err = p.facade.Commands().GetCommandByIdempotencyKey(idempotencyKey)
	}
	if err != nil {
		return err
	}
	if command == nil || command.IsTerminal() {
		return nil
	}
	if runtimeID != "" && command.RuntimeID != "" && command.RuntimeID != runtimeID {
		return errors.New("production runtime recovery: command runtime identity mismatch")
	}
	return p.facade.Commands().MarkCancelled(command.ID, time.Now())
}

func (p *ProductionRuntimeRepo) QueryRuntimeAppliedState(ctx context.Context, userID, deviceID, runtimeID string) (int64, string, error) {
	var proj projection.InstallationRuntimeProjection
	query := p.db.WithContext(ctx).Where("user_id = ? AND device_id = ?", userID, deviceID)
	if runtimeID != "" {
		query = query.Where("runtime_id = ?", runtimeID)
	}
	if err := query.First(&proj).Error; err != nil {
		return 0, "", err
	}
	if proj.RuntimeSyncState != projection.SyncStateApplied {
		return proj.AppliedDesiredRevision, proj.ActualReleaseID, errors.New("runtime projection not applied")
	}
	return proj.AppliedDesiredRevision, proj.ActualReleaseID, nil
}

func (p *ProductionRuntimeRepo) MarkRuntimeApplied(opID string, appliedRevision int64) error {
	result := p.db.Model(&operation.InstallationOperation{}).Where("id = ? AND desired_revision <= ?", opID, appliedRevision).Updates(map[string]interface{}{
		"stage":      operation.OpStageRuntimeApplied,
		"updated_at": time.Now().UTC().Format("2006-01-02 15:04:05"),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("production runtime recovery: runtime applied operation CAS failed")
	}
	return nil
}

func (p *ProductionRuntimeRepo) QueryCommandTerminalStatusByIdempotencyKey(ctx context.Context, idempotencyKey string) (status string, found bool, err error) {
	if p == nil || p.db == nil {
		return "", false, errors.New("production runtime recovery: runtime v2 unavailable")
	}
	var cmd runtimev2.RuntimeCommand
	query := p.db.WithContext(ctx)
	if strings.HasPrefix(idempotencyKey, "recenter:") {
		query = query.Where("idempotency_key = ? OR idempotency_key LIKE ?", idempotencyKey, idempotencyKey+":%")
	} else {
		query = query.Where("idempotency_key = ?", idempotencyKey)
	}
	if err := query.Order("created_at DESC").First(&cmd).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return cmd.Status, true, nil
}

type ProductionSwitchRepo struct {
	runtime *ProductionRuntimeRepo
}

func NewProductionSwitchRepo(runtime *ProductionRuntimeRepo) *ProductionSwitchRepo {
	return &ProductionSwitchRepo{runtime: runtime}
}

func (p *ProductionSwitchRepo) ResolveDesiredRevision(ctx context.Context, opID, userID, deviceID string) (int64, error) {
	if p == nil || p.runtime == nil {
		return 0, errors.New("production switch recovery: runtime repository is not configured")
	}
	return p.runtime.ResolveDesiredRevision(ctx, opID, userID, deviceID)
}

func (p *ProductionSwitchRepo) PublishSwitchDesired(ctx context.Context, opID, userID, deviceID, runtimeID, newInstallationID string, newDesiredRevision int64) error {
	if p == nil || p.runtime == nil || p.runtime.db == nil {
		return errors.New("production switch recovery: runtime repository is not configured")
	}
	var state desired.RuntimeDesiredState
	if err := p.runtime.db.WithContext(ctx).Where("user_id = ? AND device_id = ?", userID, deviceID).First(&state).Error; err != nil {
		return fmt.Errorf("production switch recovery: load desired state: %w", err)
	}
	if state.InstallationID != newInstallationID {
		return fmt.Errorf("production switch recovery: desired installation mismatch: expected=%s actual=%s", newInstallationID, state.InstallationID)
	}
	if state.DesiredRevision < newDesiredRevision {
		return fmt.Errorf("production switch recovery: desired revision not persisted: expected>=%d actual=%d", newDesiredRevision, state.DesiredRevision)
	}
	var outboxCount int64
	if err := p.runtime.db.WithContext(ctx).Table("desktop_pet_runtime_desired_state_outbox").
		Where("user_id = ? AND device_id = ? AND installation_id = ? AND desired_revision = ?", userID, deviceID, newInstallationID, newDesiredRevision).
		Count(&outboxCount).Error; err != nil {
		return fmt.Errorf("production switch recovery: verify desired outbox: %w", err)
	}
	if outboxCount == 0 {
		return errors.New("production switch recovery: desired outbox event missing")
	}
	return nil
}

func (p *ProductionSwitchRepo) SendSwitchCommand(ctx context.Context, opID, userID, deviceID, runtimeID, newInstallationID string, newDesiredRevision int64) error {
	return p.runtime.SendDesiredCommand(ctx, opID, userID, deviceID, runtimeID, newInstallationID, newDesiredRevision)
}

func (p *ProductionSwitchRepo) QuerySwitchApplied(ctx context.Context, userID, deviceID, runtimeID string, newDesiredRevision int64) (bool, error) {
	rev, _, err := p.runtime.QueryRuntimeAppliedState(ctx, userID, deviceID, runtimeID)
	if err != nil {
		return false, err
	}
	return rev >= newDesiredRevision, nil
}

type ProductionRuntimeFinalizer struct {
	db       *gorm.DB
	repo     installation.RepositoryV2
	registry *security.PathRootRegistry
}

func NewProductionRuntimeFinalizer(db *gorm.DB, repo installation.RepositoryV2, registry *security.PathRootRegistry) *ProductionRuntimeFinalizer {
	return &ProductionRuntimeFinalizer{db: db, repo: repo, registry: registry}
}

func (f *ProductionRuntimeFinalizer) FinalizeRuntimeApplied(ctx context.Context, op *operation.InstallationOperation) error {
	if f == nil || f.db == nil || f.repo == nil {
		return errors.New("production runtime finalizer: not configured")
	}
	if op.OperationType != operation.TypeUninstall {
		return f.finalizeNonUninstall(ctx, op)
	}
	return f.finalizeUninstall(ctx, op)
}

func (f *ProductionRuntimeFinalizer) finalizeNonUninstall(ctx context.Context, op *operation.InstallationOperation) error {
	if op.InstallationID == "" {
		return nil
	}
	return f.repo.Transaction(ctx, func(tx installation.RepositoryV2) error {
		inst, err := loadInstallationForRecovery(tx.DB(), op.UserID, op.DeviceID, op.InstallationID)
		if err != nil {
			return err
		}
		inst.RuntimeSyncState = installation.SyncConfirmed
		switch op.OperationType {
		case operation.TypeEnable:
			inst.Status = installation.StatusEnabled
			inst.DesiredState = installation.DesiredEnabled
		case operation.TypeDisable:
			inst.Status = installation.StatusDisabled
			inst.DesiredState = installation.DesiredDisabled
		case operation.TypeInstall, operation.TypeUpgrade, operation.TypeDowngrade, operation.TypeRepair, operation.TypeSwitch:
			if inst.Status != installation.StatusEnabled && inst.Status != installation.StatusDisabled {
				inst.Status = installation.StatusInstalled
			}
			inst.LifecycleState = installation.LifecycleInstalled
		}
		return tx.UpdateInstallationTx(tx.DB(), inst)
	})
}

func (f *ProductionRuntimeFinalizer) finalizeUninstall(ctx context.Context, op *operation.InstallationOperation) error {
	var proj projection.InstallationRuntimeProjection
	if err := f.db.WithContext(ctx).Where("user_id = ? AND device_id = ?", op.UserID, op.DeviceID).First(&proj).Error; err != nil {
		return err
	}
	if proj.AppliedDesiredRevision < op.DesiredRevision {
		return errors.New("uninstall finalizer: runtime has not applied ensure_absent revision")
	}
	if proj.InstallationID == op.InstallationID && proj.ActualReleaseID != "" {
		return errors.New("uninstall finalizer: runtime still reports installation present")
	}

	return f.repo.Transaction(ctx, func(tx installation.RepositoryV2) error {
		inst, err := loadInstallationForRecovery(tx.DB(), op.UserID, op.DeviceID, op.InstallationID)
		if err != nil {
			return err
		}
		var existing installation.TrashEntry
		trashErr := tx.DB().Where("operation_id = ? AND installation_id = ? AND status <> ?", op.ID, op.InstallationID, "purged").First(&existing).Error
		hasTrash := trashErr == nil
		if trashErr != nil && !errors.Is(trashErr, gorm.ErrRecordNotFound) {
			return trashErr
		}
		if !hasTrash {
			if strings.TrimSpace(inst.InstallStorageKey) == "" {
				return errors.New("uninstall finalizer: installation storage key missing")
			}
			resolved, err := f.registry.Resolve(security.RootInstallations, inst.InstallStorageKey)
			if err != nil {
				return err
			}
			trashEntityID := op.ID + "_" + inst.ID
			dest, err := f.registry.MoveToTrashWithDestination(resolved, trashEntityID)
			if err != nil {
				if !errors.Is(err, os.ErrNotExist) {
					return err
				}
				dest, err = findRecoveredTrashPath(f.registry, op.ID, inst.ID, filepath.Base(resolved))
				if err != nil {
					return errors.Join(errors.New("uninstall finalizer: installation storage missing and no trash entry exists"), err)
				}
			}
			trashKey, err := f.registry.StorageKeyFromPath(security.RootStorageTrash, dest)
			if err != nil {
				return err
			}
			if err := tx.CreateTrashEntryTx(tx.DB(), &installation.TrashEntry{
				ID:             uuid.NewString(),
				OperationID:    op.ID,
				InstallationID: inst.ID,
				StorageKey:     trashKey,
				Reason:         "uninstall",
				ContentHash:    inst.InstalledContentHash,
				RetainUntil:    time.Now().UTC().Add(30 * 24 * time.Hour).Format(time.RFC3339),
				Status:         "retained",
				CreatedAt:      time.Now().UTC().Format(time.RFC3339),
			}); err != nil {
				return err
			}
		}
		bindingEntry, bindErr := tx.GetActiveBindingForUserDeviceTx(tx.DB(), op.UserID, op.DeviceID)
		if bindErr != nil && !errors.Is(bindErr, installation.ErrBindingNotFound) {
			return bindErr
		}
		if bindErr == nil && bindingEntry != nil && bindingEntry.InstallationID == inst.ID {
			if err := tx.DeleteActiveBindingTx(tx.DB(), op.UserID, op.DeviceID); err != nil {
				return err
			}
		}
		inst.Status = installation.StatusUninstalled
		inst.LifecycleState = installation.LifecycleUninstalled
		inst.IsActive = 0
		inst.DesiredState = installation.DesiredDisabled
		inst.RuntimeSyncState = installation.SyncConfirmed
		inst.UpdatedAt = time.Now().UTC().Format("2006-01-02 15:04:05")
		return tx.UpdateInstallationTx(tx.DB(), inst)
	})
}

func findRecoveredTrashPath(registry *security.PathRootRegistry, operationID, installationID, baseName string) (string, error) {
	if registry == nil {
		return "", errors.New("uninstall finalizer: path registry not configured")
	}
	trashRoot, err := registry.Root(security.RootStorageTrash)
	if err != nil {
		return "", err
	}
	entries, err := os.ReadDir(trashRoot)
	if err != nil {
		return "", err
	}
	suffix := "_" + operationID + "_" + installationID + "_" + baseName
	var match string
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			return "", err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("uninstall finalizer: recovered trash path is a symlink")
		}
		if match != "" {
			return "", errors.New("uninstall finalizer: multiple recovered trash paths found")
		}
		match = filepath.Join(trashRoot, entry.Name())
	}
	if match == "" {
		return "", os.ErrNotExist
	}
	return match, nil
}

func loadInstallationForRecovery(db *gorm.DB, userID, deviceID, installationID string) (*installation.Installation, error) {
	var inst installation.Installation
	if err := db.Where("id = ? AND user_id = ? AND device_id = ?", installationID, userID, deviceID).First(&inst).Error; err != nil {
		return nil, err
	}
	return &inst, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (f *ProductionRuntimeFinalizer) FinalizeDesiredStateApplied(ctx context.Context, op *operation.InstallationOperation) error {
	return f.FinalizeRuntimeApplied(ctx, op)
}

var _ StagingRepo = (*ProductionStagingRepo)(nil)
var _ DBRepo = (*ProductionDBRepo)(nil)
var _ RuntimeRepo = (*ProductionRuntimeRepo)(nil)
var _ SwitchRepo = (*ProductionSwitchRepo)(nil)
var _ RuntimeAppliedFinalizer = (*ProductionRuntimeFinalizer)(nil)
var _ DesiredStateFinalizer = (*ProductionRuntimeFinalizer)(nil)
