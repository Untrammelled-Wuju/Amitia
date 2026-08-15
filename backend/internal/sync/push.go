// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"fmt"
)

type ApplyFunc func(mutation ClientMutation) (int64, error)

type PushService struct {
	changelog *ChangeLogService
	cursors   *CursorService
	applyFn   ApplyFunc
}

func NewPushService(changelog *ChangeLogService, cursors *CursorService, applyFn ApplyFunc) *PushService {
	return &PushService{
		changelog: changelog,
		cursors:   cursors,
		applyFn:   applyFn,
	}
}

func (s *PushService) Push(req PushRequest) (*PushResult, error) {
	result := &PushResult{
		Accepted: []MutationResult{},
		Rejected: []MutationResult{},
	}

	var maxSeq Sequence

	for _, mutation := range req.Mutations {
		mutResult := s.applyMutation(req.DeviceID, req.UserID, mutation)
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
		if err := s.cursors.MarkPushed(req.DeviceID, maxSeq); err != nil {
			return result, fmt.Errorf("push: mark pushed: %w", err)
		}
	}

	serverSeq, err := s.changelog.GetServerSequence()
	if err != nil {
		return result, fmt.Errorf("push: server seq: %w", err)
	}
	result.ServerSeq = serverSeq
	result.ApplyCursor = maxSeq

	return result, nil
}

func (s *PushService) applyMutation(deviceID, userID string, mutation ClientMutation) MutationResult {
	if s.applyFn == nil {
		return MutationResult{
			MutationID: mutation.MutationID,
			Success:    false,
			ErrorCode:  "no_apply_handler",
			Message:    "no apply handler configured",
		}
	}

	revision, err := s.applyFn(mutation)
	if err != nil {
		return MutationResult{
			MutationID: mutation.MutationID,
			Success:    false,
			ErrorCode:  "apply_failed",
			Message:    err.Error(),
		}
	}

	record, err := s.changelog.Append(
		mutation.EntityType,
		mutation.EntityID,
		mutation.Operation,
		revision,
		mutation.MutationID,
		deviceID,
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
