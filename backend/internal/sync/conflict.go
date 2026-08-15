// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package sync

import (
	"fmt"
)

type ConflictStrategy string

const (
	StrategyServerWins   ConflictStrategy = "server_wins"
	StrategyClientWins   ConflictStrategy = "client_wins"
	StrategyLWW          ConflictStrategy = "last_write_wins"
	StrategyMergeRequire ConflictStrategy = "merge_required"
	StrategyManual       ConflictStrategy = "manual"
)

type ConflictResolver struct {
	defaultStrategy ConflictStrategy
	entityStrategies map[EntityType]ConflictStrategy
}

func NewConflictResolver(defaultStrategy ConflictStrategy) *ConflictResolver {
	return &ConflictResolver{
		defaultStrategy:  defaultStrategy,
		entityStrategies: make(map[EntityType]ConflictStrategy),
	}
}

func (r *ConflictResolver) SetEntityStrategy(entityType EntityType, strategy ConflictStrategy) {
	r.entityStrategies[entityType] = strategy
}

func (r *ConflictResolver) Resolve(entityType EntityType, baseRevision, serverRevision int64) (ConflictStrategy, error) {
	if strategy, ok := r.entityStrategies[entityType]; ok {
		return strategy, nil
	}
	return r.defaultStrategy, nil
}

type ApplyService struct {
	conflicts *ConflictResolver
}

func NewApplyService(conflicts *ConflictResolver) *ApplyService {
	return &ApplyService{conflicts: conflicts}
}

func (s *ApplyService) CheckConflict(mutation ClientMutation, serverRevision int64) (*Conflict, error) {
	if mutation.BaseRevision == 0 {
		return nil, nil
	}

	if mutation.BaseRevision >= serverRevision {
		return nil, nil
	}

	strategy, err := s.conflicts.Resolve(mutation.EntityType, mutation.BaseRevision, serverRevision)
	if err != nil {
		return nil, fmt.Errorf("apply: resolve strategy: %w", err)
	}

	return &Conflict{
		EntityID:       mutation.EntityID,
		EntityType:     mutation.EntityType,
		BaseRevision:   mutation.BaseRevision,
		ServerRevision: serverRevision,
		Resolution:     string(strategy),
	}, nil
}

func (s *ApplyService) CanApply(strategy ConflictStrategy) bool {
	switch strategy {
	case StrategyClientWins, StrategyLWW:
		return true
	case StrategyServerWins, StrategyMergeRequire, StrategyManual:
		return false
	default:
		return false
	}
}
