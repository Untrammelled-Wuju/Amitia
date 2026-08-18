// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"fmt"

	"gorm.io/gorm"
)

type ApplyFunc func(tx *gorm.DB, mutation ClientMutation) (int64, error)

type PushService struct {
	db        *gorm.DB
	changelog *ChangeLogService
	cursors   *CursorService
	applier   EntityMutationApplier
}

func NewPushService(db *gorm.DB, changelog *ChangeLogService, cursors *CursorService, applier EntityMutationApplier) *PushService {
	return &PushService{
		db:        db,
		changelog: changelog,
		cursors:   cursors,
		applier:   applier,
	}
}

func NewPushServiceFromFunc(db *gorm.DB, changelog *ChangeLogService, cursors *CursorService, applyFn ApplyFunc) *PushService {
	return &PushService{
		db:        db,
		changelog: changelog,
		cursors:   cursors,
		applier:   &funcApplier{fn: applyFn},
	}
}

type funcApplier struct {
	fn ApplyFunc
}

func (f *funcApplier) Apply(tx *gorm.DB, mutation ClientMutation) (int64, error) {
	if f.fn == nil {
		return 0, &ApplierError{Code: "no_apply_handler", Message: "no apply handler configured"}
	}
	return f.fn(tx, mutation)
}

func (f *funcApplier) Supports(entityType EntityType) bool {
	return f.fn != nil
}

func (s *PushService) Push(req PushRequest) (*PushResult, error) {
	result := &PushResult{
		Accepted: []MutationResult{},
		Rejected: []MutationResult{},
	}

	var maxSeq Sequence

	scope := ScopeDevice
	err := s.db.Transaction(func(tx *gorm.DB) error {
		for i, mutation := range req.Mutations {
			savepoint := fmt.Sprintf("sync_mutation_%d", i)
			if err := tx.SavePoint(savepoint).Error; err != nil {
				return fmt.Errorf("push: create mutation savepoint: %w", err)
			}
			mutResult, txErr := s.applyMutation(tx, req.DeviceID, req.UserID, scope, mutation)
			if txErr != nil {
				return txErr
			}
			if mutResult.Success {
				result.Accepted = append(result.Accepted, *mutResult)
				if mutResult.Sequence > maxSeq {
					maxSeq = mutResult.Sequence
				}
			} else {
				// A rejected mutation must leave no entity, claim, sequence, or changelog
				// side effects. Roll back only this mutation and continue the batch.
				if err := tx.RollbackTo(savepoint).Error; err != nil {
					return fmt.Errorf("push: rollback rejected mutation: %w", err)
				}
				result.Rejected = append(result.Rejected, *mutResult)
			}
		}

		if maxSeq > 0 {
			identity := CursorIdentity{
				UserID:   req.UserID,
				Scope:    scope,
				DeviceID: req.DeviceID,
			}
			if err := s.cursors.MarkPushedTx(tx, identity, maxSeq); err != nil {
				return fmt.Errorf("push: mark pushed: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return result, err
	}

	serverSeq, err := s.changelog.GetServerSequence()
	if err != nil {
		return result, fmt.Errorf("push: server seq: %w", err)
	}
	result.ServerSeq = serverSeq
	result.ApplyCursor = maxSeq

	return result, nil
}

func (s *PushService) applyMutation(tx *gorm.DB, deviceID, userID string, scope CursorScope, mutation ClientMutation) (*MutationResult, error) {
	if s.applier == nil {
		return &MutationResult{
			MutationID: mutation.MutationID,
			Success:    false,
			ErrorCode:  "no_apply_handler",
			Message:    "no apply handler configured",
		}, nil
	}

	if mutation.MutationID != "" {
		claimed, claim, err := s.changelog.ClaimMutationTx(tx, mutation.MutationID, userID, scope)
		if err != nil {
			return nil, fmt.Errorf("changelog: claim mutation: %w", err)
		}
		if !claimed {
			if claim.Status == MutationClaimStatusCommitted {
				existingRecord, err := s.changelog.GetByMutationIDUserScopeTx(tx, mutation.MutationID, userID, scope)
				if err != nil {
					return nil, fmt.Errorf("changelog: get committed mutation record: %w", err)
				}
				if existingRecord != nil {
					return &MutationResult{
						MutationID: mutation.MutationID,
						Success:    true,
						ChangeID:   existingRecord.ChangeID,
						Sequence:   existingRecord.Sequence,
						Revision:   existingRecord.Revision,
					}, nil
				}
				return &MutationResult{
					MutationID: mutation.MutationID,
					Success:    false,
					ErrorCode:  "mutation_result_missing",
					Message:    "committed mutation has no durable change result",
				}, nil
			}
			return &MutationResult{
				MutationID: mutation.MutationID,
				Success:    false,
				ErrorCode:  "mutation_pending",
				Message:    "mutation is being processed",
			}, nil
		}
	}

	revision, err := s.applier.Apply(tx, mutation)
	if err != nil {
		if mutation.MutationID != "" {
			if rollbackErr := s.changelog.RollbackClaimTx(tx, mutation.MutationID, userID, scope); rollbackErr != nil {
				return nil, fmt.Errorf("changelog: rollback mutation claim: %w", rollbackErr)
			}
		}
		result := &MutationResult{
			MutationID: mutation.MutationID,
			Success:    false,
			ErrorCode:  "apply_failed",
			Message:    err.Error(),
		}
		if appErr, ok := err.(*ApplierError); ok {
			result.ErrorCode = appErr.Code
			if appErr.Code == "conflict" {
				result.ServerRevision = appErr.ServerRevision
			}
		}
		return result, nil
	}

	record, err := s.changelog.AppendTx(
		tx,
		mutation.EntityType,
		mutation.EntityID,
		mutation.Operation,
		revision,
		mutation.MutationID,
		deviceID,
		userID,
		scope,
		mutation.Payload,
	)
	if err != nil {
		if mutation.MutationID != "" {
			if rollbackErr := s.changelog.RollbackClaimTx(tx, mutation.MutationID, userID, scope); rollbackErr != nil {
				return nil, fmt.Errorf("changelog: append failed and rollback claim failed: %v: %w", err, rollbackErr)
			}
		}
		return nil, fmt.Errorf("changelog append: %w", err)
	}

	if mutation.MutationID != "" {
		if err := s.changelog.CommitClaimTx(tx, mutation.MutationID, userID, scope); err != nil {
			return nil, fmt.Errorf("changelog commit claim: %w", err)
		}
	}

	return &MutationResult{
		MutationID: mutation.MutationID,
		Success:    true,
		ChangeID:   record.ChangeID,
		Sequence:   record.Sequence,
		Revision:   record.Revision,
	}, nil
}
