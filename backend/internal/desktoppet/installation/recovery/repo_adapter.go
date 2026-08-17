// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"errors"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/operation"
	"github.com/u-ai/backend/internal/desktoppet/installation/journal"
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
	err := a.db.Where("lease_expires_at < ? AND status IN (?, ?, ?)", leaseTimeout,
		operation.OpStatusRunning, operation.OpStatusWaitingRuntimeACK, operation.OpStatusCancelRequested).
		Order("lease_expires_at asc").Limit(limit).Find(&ops).Error
	if err != nil {
		return nil, err
	}
	return ops, nil
}

func (a *installRepoAdapter) RenewOperationLease(operationID, executionID string) error {
	return a.db.Model(&operation.InstallationOperation{}).
		Where("id = ? AND lease_owner = ?", operationID, executionID).
		Updates(map[string]interface{}{
			"lease_expires_at": time.Now().Add(5 * time.Minute),
			"heartbeat_at":     time.Now(),
		}).Error
}

func (a *installRepoAdapter) ClaimOperationLease(operationID, owner string, ttl time.Duration, expectedStatuses []string) (*operation.InstallationOperation, error) {
	var op operation.InstallationOperation
	err := a.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("id = ?", operationID).Take(&op).Error; err != nil {
			return err
		}
		if !containsString(expectedStatuses, op.Status) {
			return errors.New("operation status mismatch")
		}
		result := tx.Model(&operation.InstallationOperation{}).
			Where("id = ? AND status = ?", operationID, op.Status).
			Updates(map[string]interface{}{
				"lease_owner":       owner,
				"lease_expires_at":  time.Now().Add(ttl),
				"heartbeat_at":      time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("lease claim failed")
		}
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
		if err := tx.Where("id = ?", operationID).Take(&op).Error; err != nil {
			return err
		}
		if op.Status != oldStatus {
			return errors.New("status mismatch")
		}
		result := tx.Model(&operation.InstallationOperation{}).
			Where("id = ? AND status = ?", operationID, oldStatus).
			Updates(map[string]interface{}{
				"status":      newStatus,
				"updated_at":  time.Now(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return errors.New("update failed")
		}
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
