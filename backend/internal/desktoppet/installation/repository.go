// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"errors"
	"time"

	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
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
	GetInstallationOperationByIdempotencyKey(userID, idempotencyKey, opType string) (*InstallationOperation, error)
	ListPendingInstallationOperations() ([]*InstallationOperation, error)
	UpdateInstallationOperation(op *InstallationOperation) error

	GetActiveBinding(userID string) (*ActiveBinding, error)
	UpsertActiveBinding(binding *ActiveBinding) error
	DeleteActiveBinding(userID string) error
	UpdateActiveBindingSyncState(userID, syncState string, bindingRevision int) error

	CreateReleaseHistory(history *InstallationReleaseHistory) error
	ListReleaseHistory(installationID string) ([]*InstallationReleaseHistory, error)
	UpdateReleaseHistoryCurrent(installationID, releaseID string) error

	CreateValidationReport(report *PackageValidationReport) error
	GetValidationReport(releaseID string) (*PackageValidationReport, error)

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
	if err == nil {
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
