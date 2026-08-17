// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"fmt"
)

type PullService struct {
	changelog *ChangeLogService
	cursors   *CursorService
}

func NewPullService(changelog *ChangeLogService, cursors *CursorService) *PullService {
	return &PullService{
		changelog: changelog,
		cursors:   cursors,
	}
}

func (s *PullService) Pull(req PullRequest) (*PullResult, error) {
	if req.Limit <= 0 || req.Limit > 1000 {
		req.Limit = 500
	}

	scope := ScopeDevice
	if req.Scope != "" {
		scope = req.Scope
	}

	identity := CursorIdentity{
		UserID:   req.UserID,
		Scope:    scope,
		DeviceID: req.DeviceID,
	}

	cursor, err := s.cursors.GetOrCreate(identity)
	if err != nil {
		return nil, fmt.Errorf("pull: cursor: %w", err)
	}

	startCursor := req.LastCursor
	if startCursor == 0 {
		startCursor = cursor.LastApplied
	}

	changes, nextCursor, hasMore, err := s.changelog.Pull(startCursor, req.Limit, EntityType(req.EntityType))
	if err != nil {
		return nil, fmt.Errorf("pull: changelog: %w", err)
	}

	serverSeq, err := s.changelog.GetServerSequence()
	if err != nil {
		return nil, fmt.Errorf("pull: server seq: %w", err)
	}

	return &PullResult{
		Changes:        changes,
		NextCursor:     nextCursor,
		HasMore:        hasMore,
		ServerSequence: serverSeq,
	}, nil
}

func (s *PullService) MarkApplied(userID string, deviceID string, scope CursorScope, seq Sequence) error {
	identity := CursorIdentity{
		UserID:   userID,
		Scope:    scope,
		DeviceID: deviceID,
	}
	return s.cursors.MarkApplied(identity, seq)
}

func (s *PullService) GetStatus(userID string, deviceID string, scope CursorScope) (*CursorStatus, error) {
	serverSeq, err := s.changelog.GetServerSequence()
	if err != nil {
		return nil, fmt.Errorf("pull: server seq: %w", err)
	}
	identity := CursorIdentity{
		UserID:   userID,
		Scope:    scope,
		DeviceID: deviceID,
	}
	return s.cursors.GetStatus(identity, serverSeq)
}
