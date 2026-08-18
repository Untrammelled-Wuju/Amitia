// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"fmt"
)

type GapDetector struct {
	changelog *ChangeLogService
	store     ChangeLogStore
}

func NewGapDetector(changelog *ChangeLogService, store ChangeLogStore) *GapDetector {
	return &GapDetector{
		changelog: changelog,
		store:     store,
	}
}

func (d *GapDetector) Check(cursor Sequence, limit int) (*GapReport, error) {
	serverSeq, err := d.store.GetLatestSequence()
	if err != nil {
		return nil, fmt.Errorf("gap: server seq: %w", err)
	}

	if cursor < 0 {
		return &GapReport{
			Detected:      true,
			InvalidCursor: true,
			Message:       "cursor cannot be negative",
		}, nil
	}

	if cursor > serverSeq {
		return &GapReport{
			Detected:      true,
			FromSeq:       cursor,
			ToSeq:         serverSeq,
			InvalidCursor: true,
			Message:       fmt.Sprintf("cursor %d ahead of server sequence %d", cursor, serverSeq),
		}, nil
	}

	if cursor > 0 && cursor < serverSeq {
		count, err := d.store.Count()
		if err == nil && count == 0 {
			return &GapReport{
				Detected: true,
				FromSeq:  cursor,
				ToSeq:    serverSeq,
				Message:  fmt.Sprintf("cursor %d points beyond retention (server seq: %d, records: 0)", cursor, serverSeq),
			}, nil
		}
	}

	return &GapReport{Detected: false}, nil
}

type ReplayService struct {
	changelog *ChangeLogService
}

func NewReplayService(changelog *ChangeLogService) *ReplayService {
	return &ReplayService{changelog: changelog}
}

func (s *ReplayService) Replay(cursor Sequence, limit int, entityType EntityType) (*PullResult, error) {
	changes, nextCursor, hasMore, err := s.changelog.Pull("", ScopeUser, cursor, limit, entityType)
	if err != nil {
		return nil, err
	}
	serverSeq, _ := s.changelog.GetServerSequence()
	return &PullResult{
		Changes:        changes,
		NextCursor:     nextCursor,
		HasMore:        hasMore,
		ServerSequence: serverSeq,
	}, nil
}
