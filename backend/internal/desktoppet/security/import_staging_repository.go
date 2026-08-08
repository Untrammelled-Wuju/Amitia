// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package security

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"
)

type ImportStagingRepository interface {
	Create(ctx context.Context, s *ImportStaging) error
	GetForUser(ctx context.Context, stagingID string, userID string) (*ImportStaging, error)
	ListForUser(ctx context.Context, userID string) ([]*ImportStaging, error)
	BeginConsumptionCAS(ctx context.Context, stagingID string, userID string, expectedRevision int64) (bool, error)
	FailConsumptionCAS(ctx context.Context, stagingID string, userID string, expectedRevision int64, failureReason string) (bool, error)
	CompleteConsumptionCAS(ctx context.Context, stagingID string, userID string, expectedRevision int64) (bool, error)
	CompleteConsumptionTx(tx *gorm.DB, stagingID string, userID string, expectedRevision int64, consumedAt string) error
	UpdateQuarantinePath(ctx context.Context, stagingID string, userID string, quarantinePath string) (bool, error)
	UpdateInventory(ctx context.Context, stagingID string, userID string, inventoryJSON string, inventoryHash string) (bool, error)
	SetRejected(ctx context.Context, stagingID string, userID string, reason string) (bool, error)
	DeleteExpired(ctx context.Context, before string) (int64, error)
}

type importStagingRepository struct {
	db *gorm.DB
}

func NewImportStagingRepository(db *gorm.DB) ImportStagingRepository {
	return &importStagingRepository{db: db}
}

func (r *importStagingRepository) Create(ctx context.Context, s *ImportStaging) error {
	if s == nil {
		return errors.New("nil staging")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	if s.CreatedAt == "" {
		s.CreatedAt = now
	}
	if s.UpdatedAt == "" {
		s.UpdatedAt = now
	}
	if s.StateRevision == 0 {
		s.StateRevision = 1
	}
	return r.db.WithContext(ctx).Create(s).Error
}

func (r *importStagingRepository) GetForUser(ctx context.Context, stagingID string, userID string) (*ImportStaging, error) {
	if stagingID == "" || userID == "" {
		return nil, ErrNotFound
	}
	var s ImportStaging
	err := r.db.WithContext(ctx).
		Where("id = ? AND owner_user_id = ?", stagingID, userID).
		Take(&s).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}

func (r *importStagingRepository) ListForUser(ctx context.Context, userID string) ([]*ImportStaging, error) {
	var stagings []*ImportStaging
	err := r.db.WithContext(ctx).
		Where("owner_user_id = ?", userID).
		Order("created_at DESC").
		Find(&stagings).Error
	return stagings, err
}

func (r *importStagingRepository) BeginConsumptionCAS(ctx context.Context, stagingID string, userID string, expectedRevision int64) (bool, error) {
	if stagingID == "" || userID == "" {
		return false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result := r.db.WithContext(ctx).Model(&ImportStaging{}).
		Where("id = ? AND owner_user_id = ? AND status = ? AND state_revision = ? AND expires_at > ?",
			stagingID, userID, StagingStatusReady, expectedRevision, now).
		Updates(map[string]interface{}{
			"status":                 StagingStatusConsuming,
			"consumption_started_at": now,
			"state_revision":         expectedRevision + 1,
			"updated_at":             now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *importStagingRepository) CompleteConsumptionCAS(ctx context.Context, stagingID string, userID string, expectedRevision int64) (bool, error) {
	if stagingID == "" || userID == "" {
		return false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result := r.db.WithContext(ctx).Model(&ImportStaging{}).
		Where("id = ? AND owner_user_id = ? AND status = ? AND state_revision = ?",
			stagingID, userID, StagingStatusConsuming, expectedRevision).
		Updates(map[string]interface{}{
			"status":         StagingStatusConsumed,
			"consumed_at":    now,
			"state_revision": expectedRevision + 1,
			"updated_at":     now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *importStagingRepository) CompleteConsumptionTx(tx *gorm.DB, stagingID string, userID string, expectedRevision int64, consumedAt string) error {
	if tx == nil {
		return errors.New("complete consumption tx is nil")
	}
	result := tx.Model(&ImportStaging{}).
		Where("id = ? AND owner_user_id = ? AND status = ? AND state_revision = ?",
			stagingID, userID, StagingStatusConsuming, expectedRevision).
		Updates(map[string]interface{}{
			"status":         StagingStatusConsumed,
			"consumed_at":    consumedAt,
			"state_revision": gorm.Expr("state_revision + 1"),
			"updated_at":     consumedAt,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrRevisionConflict
	}
	return nil
}

func (r *importStagingRepository) FailConsumptionCAS(ctx context.Context, stagingID string, userID string, expectedRevision int64, failureReason string) (bool, error) {
	if stagingID == "" || userID == "" {
		return false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result := r.db.WithContext(ctx).Model(&ImportStaging{}).
		Where("id = ? AND owner_user_id = ? AND status = ? AND state_revision = ?",
			stagingID, userID, StagingStatusConsuming, expectedRevision).
		Updates(map[string]interface{}{
			"status":         StagingStatusFailed,
			"failed_at":      now,
			"failure_reason": failureReason,
			"state_revision": expectedRevision + 1,
			"updated_at":     now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *importStagingRepository) UpdateQuarantinePath(ctx context.Context, stagingID string, userID string, quarantinePath string) (bool, error) {
	if stagingID == "" || userID == "" {
		return false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result := r.db.WithContext(ctx).Model(&ImportStaging{}).
		Where("id = ? AND owner_user_id = ? AND status IN (?, ?)",
			stagingID, userID, StagingStatusUploading, StagingStatusQuarantined).
		Updates(map[string]interface{}{
			"quarantine_path": quarantinePath,
			"status":          StagingStatusQuarantined,
			"updated_at":      now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *importStagingRepository) UpdateInventory(ctx context.Context, stagingID string, userID string, inventoryJSON string, inventoryHash string) (bool, error) {
	if stagingID == "" || userID == "" {
		return false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result := r.db.WithContext(ctx).Model(&ImportStaging{}).
		Where("id = ? AND owner_user_id = ? AND status IN (?, ?, ?)",
			stagingID, userID, StagingStatusQuarantined, StagingStatusInspecting, StagingStatusReady).
		Updates(map[string]interface{}{
			"inventory_json": inventoryJSON,
			"inventory_hash": inventoryHash,
			"status":         StagingStatusReady,
			"updated_at":     now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *importStagingRepository) SetRejected(ctx context.Context, stagingID string, userID string, reason string) (bool, error) {
	if stagingID == "" || userID == "" {
		return false, nil
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	result := r.db.WithContext(ctx).Model(&ImportStaging{}).
		Where("id = ? AND owner_user_id = ? AND status IN (?, ?, ?)",
			stagingID, userID, StagingStatusUploading, StagingStatusQuarantined, StagingStatusInspecting).
		Updates(map[string]interface{}{
			"status":          StagingStatusRejected,
			"rejected_reason": reason,
			"updated_at":      now,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *importStagingRepository) DeleteExpired(ctx context.Context, before string) (int64, error) {
	if before == "" {
		before = time.Now().UTC().Format(time.RFC3339Nano)
	}
	result := r.db.WithContext(ctx).
		Where("expires_at < ? AND status NOT IN (?, ?)", before, StagingStatusConsumed, StagingStatusRejected).
		Delete(&ImportStaging{})
	if result.Error != nil {
		return 0, result.Error
	}
	return result.RowsAffected, nil
}
