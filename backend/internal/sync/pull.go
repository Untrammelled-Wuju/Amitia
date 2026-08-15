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

	cursor, err := s.cursors.GetOrCreate(req.DeviceID, req.UserID, ScopeDevice)
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

func (s *PullService) MarkApplied(deviceID string, seq Sequence) error {
	return s.cursors.MarkApplied(deviceID, seq)
}

func (s *PullService) GetStatus(deviceID string) (*CursorStatus, error) {
	serverSeq, err := s.changelog.GetServerSequence()
	if err != nil {
		return nil, fmt.Errorf("pull: server seq: %w", err)
	}
	return s.cursors.GetStatus(deviceID, serverSeq)
}
