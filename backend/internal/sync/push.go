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

	err := s.db.Transaction(func(tx *gorm.DB) error {
		for _, mutation := range req.Mutations {
			mutResult := s.applyMutation(tx, req.DeviceID, req.UserID, mutation)
			if mutResult.Success {
				result.Accepted = append(result.Accepted, mutResult)
				if mutResult.Sequence > maxSeq {
					maxSeq = mutResult.Sequence
				}
			} else {
				result.Rejected = append(result.Rejected, mutResult)
			}
		}

		if maxSeq > 0 {
			identity := CursorIdentity{
				UserID:   req.UserID,
				Scope:    ScopeDevice,
				DeviceID: req.DeviceID,
			}
			if err := s.cursors.MarkPushed(identity, maxSeq); err != nil {
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

func (s *PushService) applyMutation(tx *gorm.DB, deviceID, userID string, mutation ClientMutation) MutationResult {
	if s.applier == nil {
		return MutationResult{
			MutationID: mutation.MutationID,
			Success:    false,
			ErrorCode:  "no_apply_handler",
			Message:    "no apply handler configured",
		}
	}

	revision, err := s.applier.Apply(tx, mutation)
	if err != nil {
		return MutationResult{
			MutationID: mutation.MutationID,
			Success:    false,
			ErrorCode:  "apply_failed",
			Message:    err.Error(),
		}
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
		mutation.Payload,
	)
	if err != nil {
		return MutationResult{
			MutationID: mutation.MutationID,
			Success:    false,
			ErrorCode:  "changelog_failed",
			Message:    err.Error(),
		}
	}

	return MutationResult{
		MutationID: mutation.MutationID,
		Success:    true,
		ChangeID:   record.ChangeID,
		Sequence:   record.Sequence,
		Revision:   record.Revision,
	}
}
