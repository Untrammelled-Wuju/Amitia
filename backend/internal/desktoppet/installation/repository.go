// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/installation/binding"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/device"
	"github.com/u-ai/backend/internal/desktoppet/installation/journal"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/installation/projection"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const installationTimeFormat = "2006-01-02 15:04:05"

type Repository interface {
	DB() *gorm.DB

	CreateInstallation(installation *Installation) error
	GetInstallation(id string) (*Installation, error)
	GetInstallationByPackageVersion(packageID, packageVersion string) (*Installation, error)
	ListInstallationsByUser(userID string) ([]*Installation, error)
	ListInstallationsByCharacter(characterID string) ([]*Installation, error)
	UpdateInstallationStatus(id, status string) error
	SetActiveInstallation(userID, installationID string) error
	GetActiveInstallation(userID string) (*Installation, error)
	DeleteInstallation(id string) error

	CreateRuntimeSettings(settings *RuntimeSettings) error
	GetRuntimeSettings(installationID string) (*RuntimeSettings, error)
	UpdateRuntimeSettings(installationID string, updates map[string]interface{}) error
	UpdateRuntimeSettingsWithCAS(installationID string, expectedRevision int, updates map[string]interface{}) (*RuntimeSettings, error)

	CreatePetIdentity(identity *PetIdentity) error
	GetPetIdentity(id string) (*PetIdentity, error)
	GetPetIdentityByCharacter(userID, characterID string) (*PetIdentity, error)
	ListPetIdentitiesByUser(userID string) ([]*PetIdentity, error)
	UpdatePetIdentity(identity *PetIdentity) error

	CreateRelease(release *PackageRelease) error
	GetRelease(id string) (*PackageRelease, error)
	GetReleaseByPetVersion(petID, version string) (*PackageRelease, error)
	GetReleaseByContentHash(contentRootHash string) (*PackageRelease, error)
	ListReleasesByPet(petID string) ([]*PackageRelease, error)
	ListPublishedReleases(userID string) ([]*PackageRelease, error)
	UpdateRelease(release *PackageRelease) error
	GetLatestReleaseSequence(petID string) (int, error)

	CreateReleaseFiles(files []ReleaseFile) error
	GetReleaseFiles(releaseID string) ([]ReleaseFile, error)
	DeleteReleaseFiles(releaseID string) error

	CreatePackageOperation(op *PackageOperation) error
	GetPackageOperation(id string) (*PackageOperation, error)
	GetPackageOperationByIdempotencyKey(userID, idempotencyKey, opType string) (*PackageOperation, error)
	ListPendingPackageOperations() ([]*PackageOperation, error)
	UpdatePackageOperation(op *PackageOperation) error

	CreateInstallationOperation(op *InstallationOperation) error
	GetInstallationOperation(id string) (*InstallationOperation, error)
	GetInstallationOperationByIdempotencyKey(userID, deviceID, idempotencyKey, opType string) (*InstallationOperation, error)
	ListPendingInstallationOperations() ([]*InstallationOperation, error)
	UpdateInstallationOperation(op *InstallationOperation) error

	GetActiveBinding(userID string) (*ActiveBinding, error)
	GetActiveBindingForUserDevice(userID, deviceID string) (*ActiveBinding, error)
	UpsertActiveBinding(binding *ActiveBinding) error
	DeleteActiveBinding(userID string) error
	UpsertActiveBindingForUserDevice(binding *ActiveBinding) error
	UpdateActiveBindingSyncState(userID, syncState string, bindingRevision int) error

	CreateReleaseHistory(history *InstallationReleaseHistory) error
	ListReleaseHistory(installationID string) ([]*InstallationReleaseHistory, error)
	UpdateReleaseHistoryCurrent(installationID, releaseID string) error

	CreateValidationReport(report *PackageValidationReport) error
	GetValidationReport(releaseID string) (*PackageValidationReport, error)

	GetInstallationByUserDevicePet(userID, deviceID, petID string) (*Installation, error)
	ListInstallationsByUserDevice(userID, deviceID string) ([]*Installation, error)

	GetRuntimeDesiredState(installationID string) (*RuntimeDesiredState, error)
	UpsertRuntimeDesiredState(state *RuntimeDesiredState) error
	ListDesiredStatesByUser(userID string) ([]*RuntimeDesiredState, error)

	CreateCommitJournal(journal *InstallationCommitJournal) error
	UpdateCommitJournal(journal *InstallationCommitJournal) error
	GetCommitJournalByOperation(operationID string) (*InstallationCommitJournal, error)
	ListPendingCommitJournals() ([]*InstallationCommitJournal, error)

	CreateSwitchJournal(journal *InstallationSwitchJournal) error
	UpdateSwitchJournal(journal *InstallationSwitchJournal) error

	CreateLegacyInstallationMapping(mapping *LegacyInstallationMapping) error
	GetLegacyInstallationMappingByLegacyID(legacyInstallationID string) (*LegacyInstallationMapping, error)

	GetActiveBindingByUserDevice(userID, deviceID string) (*ActiveBinding, error)
	UpsertActiveBindingByUserDevice(binding *ActiveBinding) error
	GetRuntimeDesiredStateByUserDevice(userID, deviceID string) (*RuntimeDesiredState, error)
	UpsertRuntimeDesiredStateWithCAS(state *RuntimeDesiredState) error
	AllocateDeviceDesiredRevision(userID, deviceID string) (int64, error)
	CASUpdateCommitJournal(operationID string, expectedState string, executionID string) (*InstallationCommitJournal, error)
	CASUpdateSwitchJournal(operationID string, expectedState string) (*InstallationSwitchJournal, error)
	ListInstallationsForUserDevice(userID, deviceID string) ([]*Installation, error)
	CreateInstallationWithDevice(installation *Installation) error
	GetInstallationForUserDevice(userID, deviceID, petID string) (*Installation, error)

	Transaction(fn func(tx *gorm.DB) error) error
}

type repository struct {
	db  *gorm.DB
	ctx *app.AppContext
}

func NewRepository(db *gorm.DB, ctx *app.AppContext) Repository {
	return &repository{db: db, ctx: ctx}
}

func (r *repository) DB() *gorm.DB { return r.db }

func (r *repository) CreateInstallation(installation *Installation) error {
	var existing Installation
	err := r.db.Where("package_id = ? AND package_version = ?", installation.PackageID, installation.PackageVersion).
		First(&existing).Error
	if err == nil && existing.ID != installation.ID {
		return ErrInstallationDuplicate
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(installation).Error
}

func (r *repository) GetInstallation(id string) (*Installation, error) {
	var inst Installation
	err := r.db.Where("id = ?", id).First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &inst, nil
}

func (r *repository) GetInstallationByPackageVersion(packageID, packageVersion string) (*Installation, error) {
	var inst Installation
	err := r.db.Where("package_id = ? AND package_version = ?", packageID, packageVersion).First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &inst, nil
}

func (r *repository) ListInstallationsByUser(userID string) ([]*Installation, error) {
	var installations []*Installation
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").
		Find(&installations).Error
	if installations == nil {
		installations = []*Installation{}
	}
	return installations, err
}

func (r *repository) ListInstallationsByCharacter(characterID string) ([]*Installation, error) {
	var installations []*Installation
	err := r.db.Where("character_id = ?", characterID).
		Order("created_at DESC").
		Find(&installations).Error
	if installations == nil {
		installations = []*Installation{}
	}
	return installations, err
}

func (r *repository) UpdateInstallationStatus(id, status string) error {
	now := time.Now().Format(installationTimeFormat)
	updates := map[string]interface{}{
		"status":     status,
		"updated_at": now,
	}
	switch status {
	case StatusEnabled:
		updates["last_enabled_at"] = now
	case StatusDisabled:
		updates["last_disabled_at"] = now
	case StatusUninstalling:
		updates["lifecycle_state"] = LifecycleUninstalling
	case StatusUninstalled:
		updates["lifecycle_state"] = LifecycleUninstalled
	case StatusInvalid:
		updates["lifecycle_state"] = LifecycleInvalid
	}
	return r.db.Model(&Installation{}).Where("id = ?", id).Updates(updates).Error
}

func (r *repository) SetActiveInstallation(userID, installationID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Format(installationTimeFormat)
		if err := tx.Model(&Installation{}).
			Where("user_id = ? AND is_active = ?", userID, 1).
			Updates(map[string]interface{}{
				"is_active":  0,
				"updated_at": now,
			}).Error; err != nil {
			return err
		}
		if installationID == "" {
			return nil
		}
		var inst Installation
		err := tx.Where("id = ?", installationID).First(&inst).Error
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInstallationNotFound
			}
			return err
		}
		if inst.UserID != userID {
			return ErrInstallationInvalid
		}
		return tx.Model(&Installation{}).
			Where("id = ?", installationID).
			Updates(map[string]interface{}{
				"is_active":  1,
				"updated_at": now,
			}).Error
	})
}

func (r *repository) GetActiveInstallation(userID string) (*Installation, error) {
	var inst Installation
	err := r.db.Where("user_id = ? AND is_active = ?", userID, 1).First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &inst, nil
}

func (r *repository) DeleteInstallation(id string) error {
	now := time.Now().Format(installationTimeFormat)
	return r.db.Model(&Installation{}).Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":     StatusUninstalled,
			"is_active":  0,
			"updated_at": now,
		}).Error
}

func (r *repository) CreateRuntimeSettings(settings *RuntimeSettings) error {
	return r.db.Create(settings).Error
}

func (r *repository) GetRuntimeSettings(installationID string) (*RuntimeSettings, error) {
	var settings RuntimeSettings
	err := r.db.Where("installation_id = ?", installationID).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRuntimeSettingsNotFound
		}
		return nil, err
	}
	return &settings, nil
}

func (r *repository) UpdateRuntimeSettings(installationID string, updates map[string]interface{}) error {
	return r.db.Model(&RuntimeSettings{}).Where("installation_id = ?", installationID).Updates(updates).Error
}

func (r *repository) UpdateRuntimeSettingsWithCAS(installationID string, expectedRevision int, updates map[string]interface{}) (*RuntimeSettings, error) {
	var result *RuntimeSettings
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var current RuntimeSettings
		if err := tx.Where("installation_id = ?", installationID).First(&current).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRuntimeSettingsNotFound
			}
			return err
		}
		if current.SettingsRevision != expectedRevision {
			return &RevisionConflictError{Expected: expectedRevision, Actual: current.SettingsRevision}
		}
		updates["settings_revision"] = current.SettingsRevision + 1
		if err := tx.Model(&RuntimeSettings{}).Where("installation_id = ? AND settings_revision = ?", installationID, expectedRevision).
			Updates(updates).Error; err != nil {
			return err
		}
		var updated RuntimeSettings
		if err := tx.Where("installation_id = ?", installationID).First(&updated).Error; err != nil {
			return err
		}
		result = &updated
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *repository) GetInstallationByUserDevicePet(userID, deviceID, petID string) (*Installation, error) {
	var inst Installation
	query := r.db.Where("user_id = ? AND pet_id = ?", userID, petID)
	if deviceID != "" {
		query = query.Where("device_id = ? OR device_id = ''", deviceID)
	}
	query = query.Where("status NOT IN ?", []string{StatusUninstalled, StatusUninstalling, StatusInvalid})
	err := query.First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &inst, nil
}

func (r *repository) ListInstallationsByUserDevice(userID, deviceID string) ([]*Installation, error) {
	var installations []*Installation
	query := r.db.Where("user_id = ?", userID)
	if deviceID != "" {
		query = query.Where("device_id = ? OR device_id = ''", deviceID)
	}
	err := query.Order("created_at DESC").Find(&installations).Error
	if installations == nil {
		installations = []*Installation{}
	}
	return installations, err
}

func (r *repository) GetRuntimeDesiredState(installationID string) (*RuntimeDesiredState, error) {
	var state RuntimeDesiredState
	err := r.db.Where("installation_id = ?", installationID).First(&state).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &state, nil
}

func (r *repository) UpsertRuntimeDesiredState(state *RuntimeDesiredState) error {
	now := time.Now().Format(installationTimeFormat)
	var existing RuntimeDesiredState
	err := r.db.Where("installation_id = ?", state.InstallationID).First(&existing).Error
	if err == nil {
		state.ID = existing.ID
		state.CreatedAt = existing.CreatedAt
		state.Revision = existing.Revision + 1
		state.UpdatedAt = now
		return r.db.Save(state).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		state.Revision = 1
		state.UpdatedAt = now
		if state.CreatedAt == "" {
			state.CreatedAt = now
		}
		return r.db.Create(state).Error
	}
	return err
}

func (r *repository) ListDesiredStatesByUser(userID string) ([]*RuntimeDesiredState, error) {
	var states []*RuntimeDesiredState
	err := r.db.Where("user_id = ?", userID).Order("updated_at DESC").Find(&states).Error
	if states == nil {
		states = []*RuntimeDesiredState{}
	}
	return states, err
}

func (r *repository) CreateCommitJournal(journal *InstallationCommitJournal) error {
	now := time.Now().Format(installationTimeFormat)
	if journal.CreatedAt == "" {
		journal.CreatedAt = now
	}
	journal.UpdatedAt = now
	return r.db.Create(journal).Error
}

func (r *repository) UpdateCommitJournal(journal *InstallationCommitJournal) error {
	journal.UpdatedAt = time.Now().Format(installationTimeFormat)
	return r.db.Save(journal).Error
}

func (r *repository) GetCommitJournalByOperation(operationID string) (*InstallationCommitJournal, error) {
	var journal InstallationCommitJournal
	err := r.db.Where("operation_id = ?", operationID).First(&journal).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &journal, nil
}

func (r *repository) ListPendingCommitJournals() ([]*InstallationCommitJournal, error) {
	var journals []*InstallationCommitJournal
	err := r.db.Where("state NOT IN ?", []string{JournalStateCompleted, JournalStateFailed}).
		Order("created_at ASC").Find(&journals).Error
	if journals == nil {
		journals = []*InstallationCommitJournal{}
	}
	return journals, err
}

func (r *repository) CreateSwitchJournal(journal *InstallationSwitchJournal) error {
	now := time.Now().Format(installationTimeFormat)
	if journal.CreatedAt == "" {
		journal.CreatedAt = now
	}
	journal.UpdatedAt = now
	return r.db.Create(journal).Error
}

func (r *repository) UpdateSwitchJournal(journal *InstallationSwitchJournal) error {
	journal.UpdatedAt = time.Now().Format(installationTimeFormat)
	return r.db.Save(journal).Error
}

func (r *repository) CreateLegacyInstallationMapping(mapping *LegacyInstallationMapping) error {
	now := time.Now().Format(installationTimeFormat)
	if mapping.CreatedAt == "" {
		mapping.CreatedAt = now
	}
	mapping.UpdatedAt = now
	return r.db.Create(mapping).Error
}

func (r *repository) GetLegacyInstallationMappingByLegacyID(legacyInstallationID string) (*LegacyInstallationMapping, error) {
	var mapping LegacyInstallationMapping
	err := r.db.Where("legacy_installation_id = ?", legacyInstallationID).First(&mapping).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &mapping, nil
}

func (r *repository) GetActiveBindingByUserDevice(userID, deviceID string) (*ActiveBinding, error) {
	var binding ActiveBinding
	query := r.db.Where("user_id = ?", userID)
	if deviceID != "" {
		query = query.Where("device_id = ? OR device_id = ''", deviceID)
	}
	err := query.First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &binding, nil
}

func (r *repository) UpsertActiveBindingByUserDevice(binding *ActiveBinding) error {
	now := time.Now().Format(installationTimeFormat)
	var existing ActiveBinding
	query := r.db.Where("user_id = ?", binding.UserID)
	if binding.DeviceID != "" {
		query = query.Where("device_id = ? OR device_id = ''", binding.DeviceID)
	}
	err := query.First(&existing).Error
	if err == nil {
		binding.UserID = existing.UserID
		binding.CreatedAt = existing.CreatedAt
		binding.BindingRevision = existing.BindingRevision + 1
		binding.UpdatedAt = now
		return r.db.Save(binding).Error
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		binding.BindingRevision = 1
		binding.UpdatedAt = now
		if binding.CreatedAt == "" {
			binding.CreatedAt = now
		}
		return r.db.Create(binding).Error
	}
	return err
}

func (r *repository) GetActiveBindingForUserDevice(userID, deviceID string) (*ActiveBinding, error) {
	return r.GetActiveBindingByUserDevice(userID, deviceID)
}

func (r *repository) UpsertActiveBindingForUserDevice(binding *ActiveBinding) error {
	return r.UpsertActiveBindingByUserDevice(binding)
}

func (r *repository) GetRuntimeDesiredStateByUserDevice(userID, deviceID string) (*RuntimeDesiredState, error) {
	var state RuntimeDesiredState
	query := r.db.Where("user_id = ?", userID)
	if deviceID != "" {
		query = query.Where("device_id = ? OR device_id = ''", deviceID)
	}
	err := query.First(&state).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &state, nil
}

func (r *repository) UpsertRuntimeDesiredStateWithCAS(state *RuntimeDesiredState) error {
	now := time.Now().Format(installationTimeFormat)
	return r.db.Transaction(func(tx *gorm.DB) error {
		var existing RuntimeDesiredState
		query := tx.Where("user_id = ?", state.UserID)
		if state.DeviceID != "" {
			query = query.Where("device_id = ? OR device_id = ''", state.DeviceID)
		}
		if err := query.First(&existing).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				state.Revision = 1
				state.UpdatedAt = now
				if state.CreatedAt == "" {
					state.CreatedAt = now
				}
				return tx.Create(state).Error
			}
			return err
		}
		if state.Revision != existing.Revision {
			return ErrRevisionConflict
		}
		state.ID = existing.ID
		state.CreatedAt = existing.CreatedAt
		state.Revision = existing.Revision + 1
		state.UpdatedAt = now
		return tx.Save(state).Error
	})
}

func (r *repository) CASUpdateCommitJournal(operationID string, expectedState string, executionID string) (*InstallationCommitJournal, error) {
	var journal InstallationCommitJournal
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("operation_id = ?", operationID).First(&journal).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInstallationNotFound
			}
			return err
		}
		if journal.State != expectedState {
			return ErrRevisionConflict
		}
		journal.UpdatedAt = time.Now().Format(installationTimeFormat)
		return tx.Save(&journal).Error
	})
	if err != nil {
		return nil, err
	}
	return &journal, nil
}

func (r *repository) CASUpdateSwitchJournal(operationID string, expectedState string) (*InstallationSwitchJournal, error) {
	var journal InstallationSwitchJournal
	err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("operation_id = ?", operationID).First(&journal).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrInstallationNotFound
			}
			return err
		}
		if journal.State != expectedState {
			return ErrRevisionConflict
		}
		journal.UpdatedAt = time.Now().Format(installationTimeFormat)
		return tx.Save(&journal).Error
	})
	if err != nil {
		return nil, err
	}
	return &journal, nil
}

func (r *repository) ListInstallationsForUserDevice(userID, deviceID string) ([]*Installation, error) {
	return r.ListInstallationsByUserDevice(userID, deviceID)
}

func (r *repository) CreateInstallationWithDevice(installation *Installation) error {
	var existing Installation
	query := r.db.Where("user_id = ? AND pet_id = ?", installation.UserID, installation.PetID)
	if installation.DeviceID != "" {
		query = query.Where("device_id = ? OR device_id = ''", installation.DeviceID)
	}
	err := query.First(&existing).Error
	if err == nil && existing.ID != installation.ID {
		return ErrInstallationDuplicate
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(installation).Error
}

func (r *repository) GetInstallationForUserDevice(userID, deviceID, petID string) (*Installation, error) {
	return r.GetInstallationByUserDevicePet(userID, deviceID, petID)
}

func (r *repository) GetInstallationForUserDeviceV2(userID, deviceID, installationID string) (*Installation, error) {
	var inst Installation
	err := r.db.Where("id = ? AND user_id = ? AND (device_id = ? OR (device_id = '' AND ? = ''))",
		installationID, userID, deviceID, deviceID).First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &inst, nil
}

func (r *repository) ListInstallationsForUserDeviceV2(userID, deviceID string) ([]*Installation, error) {
	var list []*Installation
	tx := r.db.Where("user_id = ?", userID)
	if deviceID != "" {
		tx = tx.Where("device_id = ? OR device_id = ''", deviceID)
	}
	if err := tx.Order("installed_at desc").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

func (r *repository) CreateInstallationTx(tx *gorm.DB, inst *Installation) error {
	return tx.Create(inst).Error
}

func (r *repository) UpdateInstallationTx(tx *gorm.DB, inst *Installation) error {
	return tx.Save(inst).Error
}

func (r *repository) GetInstallationByUserDevicePetTx(tx *gorm.DB, userID, deviceID, petID string) (*Installation, error) {
	var inst Installation
	err := tx.Where("user_id = ? AND pet_id = ?", userID, petID).
		Where("device_id = ? OR device_id = ''", deviceID).
		First(&inst).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &inst, nil
}

func (r *repository) DeleteInstallationTx(tx *gorm.DB, id string) error {
	return tx.Delete(&Installation{}, "id = ?", id).Error
}

func (r *repository) GetRuntimeSettingsForUserDevice(userID, deviceID, installationID string) (*RuntimeSettings, error) {
	var settings RuntimeSettings
	err := r.db.Where("installation_id = ?", installationID).First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &settings, nil
}

func (r *repository) CreateRuntimeSettingsTx(tx *gorm.DB, settings *RuntimeSettings) error {
	return tx.Create(settings).Error
}

func (r *repository) UpdateRuntimeSettingsCAS(tx *gorm.DB, installationID, userID, deviceID string, expectedRevision int, updates map[string]interface{}) (*RuntimeSettings, error) {
	var settings RuntimeSettings
	err := tx.Where("installation_id = ? AND settings_revision = ?", installationID, expectedRevision).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&settings).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSettingsRevisionConflict
		}
		return nil, err
	}
	updates["settings_revision"] = expectedRevision + 1
	updates["updated_at"] = time.Now().Format(installationTimeFormat)
	if err := tx.Model(&settings).Updates(updates).Error; err != nil {
		return nil, err
	}
	return &settings, nil
}

func (r *repository) GetActiveBindingForUserDeviceTx(tx *gorm.DB, userID, deviceID string) (*binding.DeviceActiveInstallationBinding, error) {
	var b binding.DeviceActiveInstallationBinding
	err := tx.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&b).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBindingNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (r *repository) UpsertActiveBindingTx(tx *gorm.DB, b *binding.DeviceActiveInstallationBinding) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "device_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"installation_id", "pet_id", "release_id", "binding_revision", "bound_reason", "bound_at", "bound_by", "updated_at"}),
	}).Create(b).Error
}

func (r *repository) DeleteActiveBindingTx(tx *gorm.DB, userID, deviceID string) error {
	return tx.Where("user_id = ? AND device_id = ?", userID, deviceID).
		Delete(&binding.DeviceActiveInstallationBinding{}).Error
}

func (r *repository) InsertBindingHistoryTx(tx *gorm.DB, entry *binding.BindingHistoryEntry) error {
	return tx.Create(entry).Error
}

func (r *repository) GetRuntimeDesiredStateTx(tx *gorm.DB, userID, deviceID string) (*desired.RuntimeDesiredState, error) {
	var state desired.RuntimeDesiredState
	err := tx.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&state).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		return nil, nil
	}
	return &state, nil
}

func (r *repository) UpsertRuntimeDesiredStateCAS(tx *gorm.DB, userID, deviceID string, state *desired.RuntimeDesiredState, expectedRevision int64) (*desired.RuntimeDesiredState, error) {
	existing, err := r.GetRuntimeDesiredStateTx(tx, userID, deviceID)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		if expectedRevision != -1 {
			return nil, ErrDesiredStateRevisionConflict
		}
		state.ID = uuidPrefix("desst_")
		if err := tx.Create(state).Error; err != nil {
			return nil, err
		}
		return state, nil
	}
	if expectedRevision != -1 && existing.DesiredRevision != expectedRevision {
		return nil, ErrDesiredStateRevisionConflict
	}
	state.ID = existing.ID
	state.DesiredRevision = existing.DesiredRevision
	if err := tx.Save(state).Error; err != nil {
		return nil, err
	}
	return state, nil
}

func (r *repository) AllocateDeviceDesiredRevisionCAS(tx *gorm.DB, userID, deviceID string) (int64, error) {
	var counter desired.DeviceDesiredRevisionCounter
	err := tx.Where("user_id = ? AND device_id = ?", userID, deviceID).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&counter).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, err
	}
	newRevision := counter.CurrentRevision + 1
	if errors.Is(err, gorm.ErrRecordNotFound) {
		counter = desired.DeviceDesiredRevisionCounter{
			UserID:          userID,
			DeviceID:        deviceID,
			CurrentRevision: newRevision,
			UpdatedAt:       time.Now().Format(installationTimeFormat),
		}
		if err := tx.Create(&counter).Error; err != nil {
			return 0, err
		}
	} else {
		counter.CurrentRevision = newRevision
		counter.UpdatedAt = time.Now().Format(installationTimeFormat)
		if err := tx.Save(&counter).Error; err != nil {
			return 0, err
		}
	}
	return newRevision, nil
}

func (r *repository) GetDeviceDesiredRevisionCounterTx(tx *gorm.DB, userID, deviceID string) (*desired.DeviceDesiredRevisionCounter, error) {
	var counter desired.DeviceDesiredRevisionCounter
	err := tx.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&counter).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDesiredStateRevisionNotFound
		}
		return nil, err
	}
	return &counter, nil
}

func (r *repository) CreateOutboxEventTx(tx *gorm.DB, event *desired.DesiredStateOutboxEvent) error {
	return tx.Create(event).Error
}

func (r *repository) ListPendingOutboxEvents(limit int) ([]*desired.DesiredStateOutboxEvent, error) {
	var events []*desired.DesiredStateOutboxEvent
	err := r.db.Where("status = ? AND available_at <= ?", "pending", time.Now().Format(installationTimeFormat)).
		Order("desired_revision asc").Limit(limit).Find(&events).Error
	if err != nil {
		return nil, err
	}
	return events, nil
}

func (r *repository) MarkOutboxEventPublished(tx *gorm.DB, eventID string) error {
	now := time.Now().Format(installationTimeFormat)
	return tx.Model(&desired.DesiredStateOutboxEvent{}).Where("event_id = ?", eventID).
		Updates(map[string]interface{}{"status": "published", "published_at": now}).Error
}

func (r *repository) MarkOutboxEventFailed(tx *gorm.DB, eventID, errorMsg string) error {
	return tx.Model(&desired.DesiredStateOutboxEvent{}).Where("event_id = ?", eventID).
		Updates(map[string]interface{}{"status": "failed", "last_error": errorMsg}).Error
}

func (r *repository) RequeueOutboxEventsBefore(tx *gorm.DB, availableBefore string) error {
	now := time.Now().Format(installationTimeFormat)
	return tx.Model(&desired.DesiredStateOutboxEvent{}).Where("status = ? AND available_at < ?", "pending", availableBefore).
		Updates(map[string]interface{}{"attempt_count": gorm.Expr("attempt_count + 1"), "available_at": now}).Error
}

func (r *repository) CreateOperationTxV2(tx *gorm.DB, op *operation.InstallationOperation) error {
	return tx.Create(op).Error
}

func (r *repository) GetOperationTxV2(tx *gorm.DB, operationID string) (*operation.InstallationOperation, error) {
	var op operation.InstallationOperation
	err := tx.Where("id = ?", operationID).First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOperationNotFound
		}
		return nil, err
	}
	return &op, nil
}

func (r *repository) GetOperationByIdempotencyKeyTx(tx *gorm.DB, userID, deviceID, idempotencyKey, opType string) (*operation.InstallationOperation, error) {
	var op operation.InstallationOperation
	err := tx.Where("idempotency_key = ? AND operation_type = ?", idempotencyKey, opType).
		Where("user_id = ? AND device_id = ?", userID, deviceID).
		First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOperationNotFound
		}
		return nil, err
	}
	return &op, nil
}

func (r *repository) UpdateOperationStatusCAS(tx *gorm.DB, operationID, expectedStatus, newStatus, executionID string) (*operation.InstallationOperation, error) {
	var op operation.InstallationOperation
	err := tx.Where("id = ? AND status = ?", operationID, expectedStatus).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOperationLeaseLost
		}
		return nil, err
	}
	now := time.Now().Format(installationTimeFormat)
	updates := map[string]interface{}{
		"status":     newStatus,
		"updated_at": now,
	}
	if executionID != "" {
		updates["execution_id"] = executionID
	}
	if err := tx.Model(&op).Updates(updates).Error; err != nil {
		return nil, err
	}
	op.Status = newStatus
	op.UpdatedAt = now
	return &op, nil
}

func (r *repository) ClaimOperationLeaseCAS(tx *gorm.DB, lease *operation.Lease, expectedStatuses []string) (*operation.InstallationOperation, error) {
	placeholders := make([]string, len(expectedStatuses))
	args := make([]interface{}, 0, len(expectedStatuses)+3)
	for i := range expectedStatuses {
		placeholders[i] = "?"
		args = append(args, expectedStatuses[i])
	}
	query := fmt.Sprintf("id = ? AND (lease_expires_at IS NULL OR lease_expires_at < ? OR status IN (%s))",
		strings.Join(placeholders, ","))
	args = append([]interface{}{lease.OperationID, time.Now().Format(installationTimeFormat)}, args...)
	var op operation.InstallationOperation
	err := tx.Where(query, args...).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrOperationLeaseNotAvailable
		}
		return nil, err
	}
	now := time.Now().Format(installationTimeFormat)
	leaseDuration := 5 * time.Minute
	newExpiry := time.Now().Add(leaseDuration).Format(installationTimeFormat)
	attemptNumber := op.AttemptNumber + 1
	if err := tx.Model(&op).Updates(map[string]interface{}{
		"lease_owner":      lease.Owner,
		"lease_expires_at": newExpiry,
		"heartbeat_at":     now,
		"attempt_number":   attemptNumber,
		"status":           operation.OpStatusRunning,
		"updated_at":       now,
	}).Error; err != nil {
		return nil, err
	}
	op.LeaseOwner = lease.Owner
	op.AttemptNumber = attemptNumber
	return &op, nil
}

func (r *repository) RenewOperationLeaseTx(tx *gorm.DB, operationID, executionID string) error {
	newExpiry := time.Now().Add(5 * time.Minute).Format(installationTimeFormat)
	now := time.Now().Format(installationTimeFormat)
	return tx.Model(&operation.InstallationOperation{}).Where("id = ? AND lease_owner = ?", operationID, executionID).
		Updates(map[string]interface{}{"lease_expires_at": newExpiry, "heartbeat_at": now, "updated_at": now}).Error
}

func (r *repository) ListPendingOperations(limit int) ([]*operation.InstallationOperation, error) {
	var ops []*operation.InstallationOperation
	err := r.db.Where("status IN (?, ?, ?)",
		operation.OpStatusCreated, operation.OpStatusQueued, operation.OpStatusWaitingRuntimeACK).
		Order("created_at asc").Limit(limit).Find(&ops).Error
	if err != nil {
		return nil, err
	}
	return ops, nil
}

func (r *repository) ListExpiredLeaseOperations(leaseTimeout string, limit int) ([]*operation.InstallationOperation, error) {
	var ops []*operation.InstallationOperation
	err := r.db.Where("lease_expires_at < ? AND status IN (?, ?)", leaseTimeout,
		operation.OpStatusRunning, operation.OpStatusWaitingRuntimeACK).
		Order("lease_expires_at asc").Limit(limit).Find(&ops).Error
	if err != nil {
		return nil, err
	}
	return ops, nil
}

func (r *repository) CreateCommitJournalTxV2(tx *gorm.DB, journal *journal.InstallationCommitJournal) error {
	return tx.Create(journal).Error
}

func (r *repository) GetCommitJournalTxV2(tx *gorm.DB, operationID string) (*journal.InstallationCommitJournal, error) {
	var j journal.InstallationCommitJournal
	err := tx.Where("operation_id = ?", operationID).First(&j).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJournalNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *repository) CASUpdateCommitJournalStageTx(tx *gorm.DB, operationID, expectedStage, newStage, executionID string) (*journal.InstallationCommitJournal, error) {
	var j journal.InstallationCommitJournal
	err := tx.Where("operation_id = ? AND stage = ?", operationID, expectedStage).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&j).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJournalStageConflict
		}
		return nil, err
	}
	now := time.Now().Format(installationTimeFormat)
	if err := tx.Model(&j).Updates(map[string]interface{}{
		"stage":        newStage,
		"execution_id": executionID,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, err
	}
	j.Stage = newStage
	j.UpdatedAt = now
	return &j, nil
}

func (r *repository) CreateSwitchJournalTxV2(tx *gorm.DB, j *journal.InstallationSwitchJournal) error {
	return tx.Create(j).Error
}

func (r *repository) GetSwitchJournalTxV2(tx *gorm.DB, operationID string) (*journal.InstallationSwitchJournal, error) {
	var j journal.InstallationSwitchJournal
	err := tx.Where("operation_id = ?", operationID).First(&j).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJournalNotFound
		}
		return nil, err
	}
	return &j, nil
}

func (r *repository) CASUpdateSwitchJournalStageTx(tx *gorm.DB, operationID, expectedStage, newStage, executionID string) (*journal.InstallationSwitchJournal, error) {
	var j journal.InstallationSwitchJournal
	err := tx.Where("operation_id = ? AND stage = ?", operationID, expectedStage).
		Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&j).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJournalStageConflict
		}
		return nil, err
	}
	now := time.Now().Format(installationTimeFormat)
	if err := tx.Model(&j).Updates(map[string]interface{}{
		"stage":        newStage,
		"execution_id": executionID,
		"updated_at":   now,
	}).Error; err != nil {
		return nil, err
	}
	j.Stage = newStage
	j.UpdatedAt = now
	return &j, nil
}

func (r *repository) ListPendingSwitchJournals(limit int) ([]*journal.InstallationSwitchJournal, error) {
	var journals []*journal.InstallationSwitchJournal
	err := r.db.Where("stage NOT IN (?, ?, ?)",
		journal.SwitchJournalCompleted, journal.SwitchJournalFailedRetryable, journal.SwitchJournalFailedTerminal).
		Order("created_at asc").Limit(limit).Find(&journals).Error
	if err != nil {
		return nil, err
	}
	return journals, nil
}

func (r *repository) CreateTrashEntryTx(tx *gorm.DB, entry *TrashEntry) error {
	return tx.Create(entry).Error
}

func (r *repository) ListExpiredTrashEntries(retainBefore string, limit int) ([]*TrashEntry, error) {
	var entries []*TrashEntry
	err := r.db.Where("status = ? AND retain_until < ?", "active", retainBefore).
		Order("retain_until asc").Limit(limit).Find(&entries).Error
	if err != nil {
		return nil, err
	}
	return entries, nil
}

func (r *repository) MarkTrashEntryPurged(tx *gorm.DB, id string) error {
	now := time.Now().Format(installationTimeFormat)
	return tx.Model(&TrashEntry{}).Where("id = ?", id).
		Updates(map[string]interface{}{"status": "purged", "purged_at": now}).Error
}

func (r *repository) GetRuntimeProjectionTx(tx *gorm.DB, userID, deviceID string) (*projection.InstallationRuntimeProjection, error) {
	var p projection.InstallationRuntimeProjection
	err := tx.Where("user_id = ? AND device_id = ?", userID, deviceID).First(&p).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrProjectionNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *repository) UpsertRuntimeProjectionTx(tx *gorm.DB, p *projection.InstallationRuntimeProjection) error {
	return tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "device_id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"runtime_id", "installation_id", "pet_id", "applied_desired_revision",
			"applied_settings_revision", "actual_release_id", "actual_visible",
			"actual_action_key", "actual_health", "runtime_sync_state",
			"last_applied_at", "last_heartbeat_at", "updated_at",
		}),
	}).Create(p).Error
}

func (r *repository) GetOrCreateDeviceContext(ctx context.Context, userID string, reqCtx device.RequestContext) (*device.DeviceContext, error) {
	if reqCtx.RuntimeID != "" {
		var client runtimeClient
		if err := r.db.Where("runtime_id = ?", reqCtx.RuntimeID).First(&client).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, device.ErrDeviceNotFound
			}
			return nil, err
		}
		if client.UserID != userID && userID != "" {
			return nil, device.ErrDeviceNotOwned
		}
		return &device.DeviceContext{
			UserID:    userID,
			DeviceID:  client.DeviceID,
			RuntimeID: reqCtx.RuntimeID,
			Source:    "runtime_registry",
		}, nil
	}
	if reqCtx.DeviceIDHeader != "" {
		return &device.DeviceContext{
			UserID:   userID,
			DeviceID: reqCtx.DeviceIDHeader,
			Source:   "request_header",
		}, nil
	}
	return nil, device.ErrDeviceNotFound
}

func uuidPrefix(prefix string) string {
	return prefix + uuid.New().String()
}

type runtimeClient struct {
	RuntimeID string `gorm:"primaryKey"`
	DeviceID  string
	UserID    string
}

func (runtimeClient) TableName() string {
	return "desktop_pet_runtime_clients"
}
