// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"gorm.io/gorm"
)

type Service struct {
	DB         *gorm.DB
	ChangeLog  *ChangeLogService
	Cursor     *CursorService
	Pull       *PullService
	Push       *PushService
	Gap        *GapDetector
	Replay     *ReplayService
	Conflicts  *ConflictResolver
	Apply      *ApplyService
}

func NewService(db *gorm.DB, applier EntityMutationApplier) *Service {
	store := NewChangeLogStore(db)
	seq := NewSequenceGenerator(db)
	cursorStore := NewCursorStore(db)

	changelog := NewChangeLogService(store, seq)
	cursors := NewCursorService(cursorStore)
	pull := NewPullService(changelog, cursors)
	push := NewPushService(db, changelog, cursors, applier)
	gap := NewGapDetector(changelog, store)
	replay := NewReplayService(changelog)
	conflicts := NewConflictResolver(StrategyServerWins)
	apply := NewApplyService(conflicts)

	return &Service{
		DB:         db,
		ChangeLog:  changelog,
		Cursor:     cursors,
		Pull:       pull,
		Push:       push,
		Gap:        gap,
		Replay:     replay,
		Conflicts:  conflicts,
		Apply:      apply,
	}
}

func NewServiceFromFunc(db *gorm.DB, applyFn ApplyFunc) *Service {
	return NewService(db, &funcApplier{fn: applyFn})
}
