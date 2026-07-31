package installation

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

func (r *repository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *repository) CreatePetIdentity(identity *PetIdentity) error {
	return r.db.Create(identity).Error
}

func (r *repository) GetPetIdentity(id string) (*PetIdentity, error) {
	var identity PetIdentity
	err := r.db.Where("id = ?", id).First(&identity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &identity, nil
}

func (r *repository) GetPetIdentityByCharacter(userID, characterID string) (*PetIdentity, error) {
	var identity PetIdentity
	err := r.db.Where("owner_user_id = ? AND source_character_id = ?", userID, characterID).First(&identity).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &identity, nil
}

func (r *repository) ListPetIdentitiesByUser(userID string) ([]*PetIdentity, error) {
	var identities []*PetIdentity
	err := r.db.Where("owner_user_id = ?", userID).Order("created_at DESC").Find(&identities).Error
	if identities == nil {
		identities = []*PetIdentity{}
	}
	return identities, err
}

func (r *repository) UpdatePetIdentity(identity *PetIdentity) error {
	now := time.Now().Format(installationTimeFormat)
	identity.UpdatedAt = now
	return r.db.Save(identity).Error
}

func (r *repository) CreateRelease(release *PackageRelease) error {
	return r.db.Create(release).Error
}

func (r *repository) GetRelease(id string) (*PackageRelease, error) {
	var release PackageRelease
	err := r.db.Where("id = ?", id).First(&release).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &release, nil
}

func (r *repository) GetReleaseByPetVersion(petID, version string) (*PackageRelease, error) {
	var release PackageRelease
	err := r.db.Where("pet_id = ? AND version = ?", petID, version).First(&release).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &release, nil
}

func (r *repository) GetReleaseByContentHash(contentRootHash string) (*PackageRelease, error) {
	var release PackageRelease
	err := r.db.Where("content_root_hash = ? AND status = ?", contentRootHash, ReleaseStatusPublished).First(&release).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &release, nil
}

func (r *repository) ListReleasesByPet(petID string) ([]*PackageRelease, error) {
	var releases []*PackageRelease
	err := r.db.Where("pet_id = ?", petID).Order("release_sequence DESC").Find(&releases).Error
	if releases == nil {
		releases = []*PackageRelease{}
	}
	return releases, err
}

func (r *repository) ListPublishedReleases(userID string) ([]*PackageRelease, error) {
	var releases []*PackageRelease
	err := r.db.Where("owner_user_id = ? AND status = ?", userID, ReleaseStatusPublished).
		Order("created_at DESC").Find(&releases).Error
	if releases == nil {
		releases = []*PackageRelease{}
	}
	return releases, err
}

func (r *repository) UpdateRelease(release *PackageRelease) error {
	now := time.Now().Format(installationTimeFormat)
	release.UpdatedAt = now
	return r.db.Save(release).Error
}

func (r *repository) GetLatestReleaseSequence(petID string) (int, error) {
	var maxSeq int
	err := r.db.Model(&PackageRelease{}).Where("pet_id = ?", petID).
		Select("COALESCE(MAX(release_sequence), 0)").Scan(&maxSeq).Error
	return maxSeq, err
}

func (r *repository) CreateReleaseFiles(files []ReleaseFile) error {
	if len(files) == 0 {
		return nil
	}
	return r.db.CreateInBatches(files, 100).Error
}

func (r *repository) GetReleaseFiles(releaseID string) ([]ReleaseFile, error) {
	var files []ReleaseFile
	err := r.db.Where("release_id = ?", releaseID).Order("path ASC").Find(&files).Error
	if files == nil {
		files = []ReleaseFile{}
	}
	return files, err
}

func (r *repository) DeleteReleaseFiles(releaseID string) error {
	return r.db.Where("release_id = ?", releaseID).Delete(&ReleaseFile{}).Error
}

func (r *repository) CreatePackageOperation(op *PackageOperation) error {
	return r.db.Create(op).Error
}

func (r *repository) GetPackageOperation(id string) (*PackageOperation, error) {
	var op PackageOperation
	err := r.db.Where("id = ?", id).First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &op, nil
}

func (r *repository) GetPackageOperationByIdempotencyKey(userID, idempotencyKey, opType string) (*PackageOperation, error) {
	var op PackageOperation
	err := r.db.Where("user_id = ? AND idempotency_key = ? AND operation_type = ?", userID, idempotencyKey, opType).First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &op, nil
}

func (r *repository) ListPendingPackageOperations() ([]*PackageOperation, error) {
	var ops []*PackageOperation
	err := r.db.Where("status IN ?", []string{OpStatusPending, OpStatusRunning, OpStatusRecovery}).
		Order("started_at ASC").Find(&ops).Error
	if ops == nil {
		ops = []*PackageOperation{}
	}
	return ops, err
}

func (r *repository) UpdatePackageOperation(op *PackageOperation) error {
	now := time.Now().Format(installationTimeFormat)
	op.UpdatedAt = now
	return r.db.Save(op).Error
}

func (r *repository) CreateInstallationOperation(op *InstallationOperation) error {
	return r.db.Create(op).Error
}

func (r *repository) GetInstallationOperation(id string) (*InstallationOperation, error) {
	var op InstallationOperation
	err := r.db.Where("id = ?", id).First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &op, nil
}

func (r *repository) GetInstallationOperationByIdempotencyKey(userID, deviceID, idempotencyKey, opType string) (*InstallationOperation, error) {
	var op InstallationOperation
	query := r.db.Where("user_id = ? AND idempotency_key = ? AND operation_type = ?", userID, idempotencyKey, opType)
	if deviceID != "" {
		query = query.Where("device_id = ? OR device_id = ''", deviceID)
	}
	err := query.First(&op).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &op, nil
}

func (r *repository) ListPendingInstallationOperations() ([]*InstallationOperation, error) {
	var ops []*InstallationOperation
	err := r.db.Where("status IN ?", []string{OpStatusPending, OpStatusRunning, OpStatusRecovery}).
		Order("started_at ASC").Find(&ops).Error
	if ops == nil {
		ops = []*InstallationOperation{}
	}
	return ops, err
}

func (r *repository) UpdateInstallationOperation(op *InstallationOperation) error {
	now := time.Now().Format(installationTimeFormat)
	op.UpdatedAt = now
	return r.db.Save(op).Error
}

func (r *repository) GetActiveBinding(userID string) (*ActiveBinding, error) {
	var binding ActiveBinding
	err := r.db.Where("user_id = ?", userID).First(&binding).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &binding, nil
}

func (r *repository) UpsertActiveBinding(binding *ActiveBinding) error {
	now := time.Now().Format(installationTimeFormat)
	binding.UpdatedAt = now
	return r.db.Save(binding).Error
}

func (r *repository) DeleteActiveBinding(userID string) error {
	return r.db.Where("user_id = ?", userID).Delete(&ActiveBinding{}).Error
}

func (r *repository) UpdateActiveBindingSyncState(userID, syncState string, bindingRevision int) error {
	now := time.Now().Format(installationTimeFormat)
	return r.db.Model(&ActiveBinding{}).Where("user_id = ?", userID).
		Updates(map[string]interface{}{
			"runtime_sync_state": syncState,
			"binding_revision":   bindingRevision,
			"updated_at":         now,
		}).Error
}

func (r *repository) CreateReleaseHistory(history *InstallationReleaseHistory) error {
	return r.db.Create(history).Error
}

func (r *repository) ListReleaseHistory(installationID string) ([]*InstallationReleaseHistory, error) {
	var histories []*InstallationReleaseHistory
	err := r.db.Where("installation_id = ?", installationID).Order("created_at DESC").Find(&histories).Error
	if histories == nil {
		histories = []*InstallationReleaseHistory{}
	}
	return histories, err
}

func (r *repository) UpdateReleaseHistoryCurrent(installationID, releaseID string) error {
	now := time.Now().Format(installationTimeFormat)
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&InstallationReleaseHistory{}).
			Where("installation_id = ?", installationID).
			Updates(map[string]interface{}{
				"is_current":      0,
				"deactivated_at":  now,
				"updated_at":      now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&InstallationReleaseHistory{}).
			Where("installation_id = ? AND release_id = ?", installationID, releaseID).
			Updates(map[string]interface{}{
				"is_current": 1,
				"updated_at": now,
			}).Error
	})
}

func (r *repository) CreateValidationReport(report *PackageValidationReport) error {
	return r.db.Create(report).Error
}

func (r *repository) GetValidationReport(releaseID string) (*PackageValidationReport, error) {
	var report PackageValidationReport
	err := r.db.Where("release_id = ?", releaseID).First(&report).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInstallationNotFound
		}
		return nil, err
	}
	return &report, nil
}

func (r *repository) AllocateDeviceDesiredRevision(userID, deviceID string) (int64, error) {
	var nextRevision int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var maxRevision int64
		query := tx.Model(&RuntimeDesiredState{}).Where("user_id = ?", userID)
		if deviceID != "" {
			query = query.Where("device_id = ? OR device_id = ''", deviceID)
		} else {
			query = query.Where("device_id = ''")
		}
		if err := query.Select("COALESCE(MAX(revision), 0)").Scan(&maxRevision).Error; err != nil {
			return err
		}
		nextRevision = maxRevision + 1
		return nil
	})
	if err != nil {
		return 0, err
	}
	return nextRevision, nil
}
