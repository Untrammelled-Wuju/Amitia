// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/journal"
	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"gorm.io/gorm"
)

type installRepoAdapter struct {
	db *gorm.DB
}

func NewInstallRepoAdapter(db *gorm.DB) RecoveryRepo {
	return &installRepoAdapter{db: db}
}

func (a *installRepoAdapter) ListExpiredLeaseOperations(leaseTimeout string, limit int) ([]*operation.InstallationOperation, error) {
	var ops []*operation.InstallationOperation
	err := a.db.Where("status IN (?, ?, ?, ?, ?) AND (lease_expires_at IS NULL OR lease_expires_at = '' OR lease_expires_at < ?)",
		operation.OpStatusCreated, operation.OpStatusQueued, operation.OpStatusRunning, operation.OpStatusWaitingRuntimeACK, operation.OpStatusCancelRequested, leaseTimeout).
		Order("created_at asc").Limit(limit).Find(&ops).Error
	if err != nil {
		return nil, err
	}
	return ops, nil
}

func (a *installRepoAdapter) RenewOperationLease(operationID, executionID string) error {
	result := a.db.Model(&operation.InstallationOperation{}).
		Where("id = ? AND lease_owner = ?", operationID, executionID).
		Updates(map[string]interface{}{
			"lease_expires_at": time.Now().Add(5 * time.Minute).Format("2006-01-02 15:04:05"),
			"heartbeat_at":     time.Now().Format("2006-01-02 15:04:05"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrRecoveryLeaseLost
	}
	return nil
}

func (a *installRepoAdapter) ReleaseOperationLease(operationID, executionID string) error {
	result := a.db.Model(&operation.InstallationOperation{}).
		Where("id = ? AND lease_owner = ?", operationID, executionID).
		Updates(map[string]interface{}{
			"lease_owner":      "",
			"lease_expires_at": "",
			"heartbeat_at":     "",
			"updated_at":       time.Now().Format("2006-01-02 15:04:05"),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrRecoveryLeaseLost
	}
	return nil
}

func (a *installRepoAdapter) ClaimOperationLease(operationID, owner string, ttl time.Duration, expectedStatuses []string) (*operation.InstallationOperation, error) {
	var op operation.InstallationOperation
	now := time.Now()
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", operationID).Take(&op).Error; err != nil {
			return err
		}
		if !containsString(expectedStatuses, op.Status) {
			return errors.New("operation status mismatch")
		}
		result := tx.Model(&operation.InstallationOperation{}).
			Where("id = ? AND status = ? AND (lease_expires_at IS NULL OR lease_expires_at = '' OR lease_expires_at < ? OR lease_owner = ?)", operationID, op.Status, now.Format("2006-01-02 15:04:05"), owner).
			Updates(map[string]interface{}{
				"lease_owner":      owner,
				"lease_expires_at": now.Add(ttl).Format("2006-01-02 15:04:05"),
				"heartbeat_at":     now.Format("2006-01-02 15:04:05"),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRecoveryLeaseLost
		}
		op.LeaseOwner = owner
		op.LeaseExpiresAt = now.Add(ttl).Format("2006-01-02 15:04:05")
		op.HeartbeatAt = now.Format("2006-01-02 15:04:05")
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func (a *installRepoAdapter) SetOperationDesiredRevisionIfMissing(operationID string, desiredRevision int64, executionID string) (*operation.InstallationOperation, error) {
	if desiredRevision <= 0 {
		return nil, errors.New("recovery: desired revision must be positive")
	}
	var op operation.InstallationOperation
	err := a.db.Transaction(func(tx *gorm.DB) error {
		lookup := tx.Where("id = ?", operationID)
		if executionID != "" {
			lookup = lookup.Where("lease_owner = ?", executionID)
		}
		if err := lookup.Take(&op).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrRecoveryLeaseLost
			}
			return err
		}
		if op.DesiredRevision > 0 {
			if op.DesiredRevision != desiredRevision {
				return fmt.Errorf("recovery: operation desired revision conflict: stored=%d authoritative=%d", op.DesiredRevision, desiredRevision)
			}
			return nil
		}

		update := tx.Model(&operation.InstallationOperation{}).
			Where("id = ? AND desired_revision = 0", operationID)
		if executionID != "" {
			update = update.Where("lease_owner = ?", executionID)
		}
		result := update.Updates(map[string]interface{}{
			"desired_revision": desiredRevision,
			"updated_at":       time.Now().UTC().Format(time.RFC3339),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCASConflict
		}
		op.DesiredRevision = desiredRevision
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func (a *installRepoAdapter) UpdateOperationStatus(operationID, oldStatus, newStatus, executionID string) (*operation.InstallationOperation, error) {
	var op operation.InstallationOperation
	err := a.db.Transaction(func(tx *gorm.DB) error {
		lookup := tx.Where("id = ? AND status = ?", operationID, oldStatus)
		if executionID != "" {
			lookup = lookup.Where("lease_owner = ?", executionID)
		}
		if err := lookup.Take(&op).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCASConflict
			}
			return err
		}

		update := tx.Model(&operation.InstallationOperation{}).
			Where("id = ? AND status = ?", operationID, oldStatus)
		if executionID != "" {
			update = update.Where("lease_owner = ?", executionID)
		}
		result := update.Updates(map[string]interface{}{
			"status":     newStatus,
			"updated_at": time.Now().UTC().Format(time.RFC3339),
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCASConflict
		}
		op.Status = newStatus
		op.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func (a *installRepoAdapter) CASUpdateOperationStage(operationID, expectedStage, newStage, executionID string) (*operation.InstallationOperation, error) {
	var op operation.InstallationOperation
	err := a.db.Transaction(func(tx *gorm.DB) error {
		lookup := tx.Where("id = ? AND stage = ?", operationID, expectedStage)
		if executionID != "" {
			lookup = lookup.Where("lease_owner = ?", executionID)
		}
		if err := lookup.Take(&op).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCASConflict
			}
			return err
		}

		update := tx.Model(&operation.InstallationOperation{}).
			Where("id = ? AND stage = ?", operationID, expectedStage)
		if executionID != "" {
			update = update.Where("lease_owner = ?", executionID)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		result := update.Updates(map[string]interface{}{
			"stage":      newStage,
			"updated_at": now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCASConflict
		}
		op.Stage = newStage
		op.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func (a *installRepoAdapter) CompleteOperation(operationID, expectedStage, expectedStatus, executionID string) (*operation.InstallationOperation, error) {
	var op operation.InstallationOperation
	err := a.db.Transaction(func(tx *gorm.DB) error {
		lookup := tx.Where("id = ? AND stage = ? AND status = ?", operationID, expectedStage, expectedStatus)
		if executionID != "" {
			lookup = lookup.Where("lease_owner = ?", executionID)
		}
		if err := lookup.Take(&op).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrCASConflict
			}
			return err
		}

		update := tx.Model(&operation.InstallationOperation{}).
			Where("id = ? AND stage = ? AND status = ?", operationID, expectedStage, expectedStatus)
		if executionID != "" {
			update = update.Where("lease_owner = ?", executionID)
		}
		now := time.Now().UTC().Format(time.RFC3339)
		result := update.Updates(map[string]interface{}{
			"stage":        operation.OpStageCompleted,
			"status":       operation.OpStatusCompleted,
			"completed_at": now,
			"updated_at":   now,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCASConflict
		}
		op.Stage = operation.OpStageCompleted
		op.Status = operation.OpStatusCompleted
		op.CompletedAt = now
		op.UpdatedAt = now
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &op, nil
}

func (a *installRepoAdapter) GetCommitJournal(operationID string) (*RecoveryCommitJournal, error) {
	var j journal.InstallationCommitJournal
	if err := a.db.Where("operation_id = ?", operationID).Take(&j).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJournalNotFound
		}
		return nil, err
	}
	return &RecoveryCommitJournal{
		ID:               j.ID,
		OperationID:      j.OperationID,
		Stage:            j.Stage,
		Status:           j.Status,
		StagingPathKey:   j.StagingPathKey,
		TargetReleaseID:  j.TargetReleaseID,
		PetID:            j.PetID,
		PublishedPathKey: j.PublishedPathKey,
		ErrorMessage:     j.ErrorMessage,
	}, nil
}

func (a *installRepoAdapter) GetSwitchJournal(operationID string) (*RecoverySwitchJournal, error) {
	var j journal.InstallationSwitchJournal
	if err := a.db.Where("operation_id = ?", operationID).Take(&j).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJournalNotFound
		}
		return nil, err
	}
	return &RecoverySwitchJournal{
		ID:                 j.ID,
		OperationID:        j.OperationID,
		Stage:              j.Stage,
		Status:             j.Status,
		NewInstallationID:  j.NewInstallationID,
		NewBindingRevision: j.NewBindingRevision,
		NewDesiredRevision: j.NewDesiredRevision,
	}, nil
}

func (a *installRepoAdapter) SetSwitchJournalDesiredRevisionIfMissing(operationID string, desiredRevision int64, executionID string) (*RecoverySwitchJournal, error) {
	if desiredRevision <= 0 {
		return nil, errors.New("recovery: switch desired revision must be positive")
	}
	var j journal.InstallationSwitchJournal
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if executionID != "" {
			var count int64
			if err := tx.Model(&operation.InstallationOperation{}).
				Where("id = ? AND lease_owner = ?", operationID, executionID).
				Count(&count).Error; err != nil {
				return err
			}
			if count != 1 {
				return ErrRecoveryLeaseLost
			}
		}
		if err := tx.Where("operation_id = ?", operationID).Take(&j).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrJournalNotFound
			}
			return err
		}
		if j.NewDesiredRevision > 0 {
			if j.NewDesiredRevision != desiredRevision {
				return fmt.Errorf("recovery: switch desired revision conflict: stored=%d authoritative=%d", j.NewDesiredRevision, desiredRevision)
			}
			return nil
		}
		result := tx.Model(&journal.InstallationSwitchJournal{}).
			Where("operation_id = ? AND new_desired_revision = 0", operationID).
			Updates(map[string]interface{}{
				"new_desired_revision": desiredRevision,
				"updated_at":           time.Now().UTC().Format(time.RFC3339),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCASConflict
		}
		j.NewDesiredRevision = desiredRevision
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &RecoverySwitchJournal{
		ID:                 j.ID,
		OperationID:        j.OperationID,
		Stage:              j.Stage,
		Status:             j.Status,
		NewInstallationID:  j.NewInstallationID,
		NewBindingRevision: j.NewBindingRevision,
		NewDesiredRevision: j.NewDesiredRevision,
	}, nil
}

func (a *installRepoAdapter) CASUpdateCommitJournalStage(operationID, expectedStage, newStage, executionID string) (*RecoveryCommitJournal, error) {
	var j journal.InstallationCommitJournal
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("operation_id = ?", operationID).Take(&j).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrJournalNotFound
			}
			return err
		}
		if j.Stage != expectedStage {
			return ErrCASConflict
		}
		result := tx.Model(&journal.InstallationCommitJournal{}).
			Where("operation_id = ? AND stage = ?", operationID, expectedStage).
			Update("stage", newStage)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCASConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	j.Stage = newStage
	return &RecoveryCommitJournal{
		ID:               j.ID,
		OperationID:      j.OperationID,
		Stage:            j.Stage,
		Status:           j.Status,
		StagingPathKey:   j.StagingPathKey,
		TargetReleaseID:  j.TargetReleaseID,
		PetID:            j.PetID,
		PublishedPathKey: j.PublishedPathKey,
		ErrorMessage:     j.ErrorMessage,
	}, nil
}

func (a *installRepoAdapter) CASUpdateSwitchJournalStage(operationID, expectedStage, newStage, executionID string) (*RecoverySwitchJournal, error) {
	var j journal.InstallationSwitchJournal
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("operation_id = ?", operationID).Take(&j).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrJournalNotFound
			}
			return err
		}
		if j.Stage != expectedStage {
			return ErrCASConflict
		}
		result := tx.Model(&journal.InstallationSwitchJournal{}).
			Where("operation_id = ? AND stage = ?", operationID, expectedStage).
			Update("stage", newStage)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrCASConflict
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	j.Stage = newStage
	return &RecoverySwitchJournal{
		ID:                 j.ID,
		OperationID:        j.OperationID,
		Stage:              j.Stage,
		Status:             j.Status,
		NewInstallationID:  j.NewInstallationID,
		NewBindingRevision: j.NewBindingRevision,
		NewDesiredRevision: j.NewDesiredRevision,
	}, nil
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
