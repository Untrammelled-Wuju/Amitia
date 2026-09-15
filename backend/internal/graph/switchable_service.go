// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package graph

import (
	"context"
	"errors"
	"sync"
)

var errGraphServiceUnavailable = errors.New("graph service unavailable")

// SwitchableService keeps the dependency identity stable while allowing the
// backing SurrealDB client/service to be replaced after a managed process
// restart. Business services retain this wrapper instead of a stale client.
//
// Every delegated operation keeps an RLock for the whole call. Swap therefore
// waits until in-flight operations using the previous service have completed
// before it closes the old SurrealDB websocket. This avoids racing a reconnect
// against a graph request that already captured the previous service.
type SwitchableService struct {
	mu      sync.RWMutex
	current Service
}

var _ Service = (*SwitchableService)(nil)

func NewSwitchableService(initial Service) *SwitchableService {
	return &SwitchableService{current: initial}
}

func (s *SwitchableService) Swap(next Service) {
	if s == nil || next == nil {
		return
	}

	s.mu.Lock()
	previous := s.current
	s.current = next
	if closer, ok := previous.(interface{ Close() }); ok && previous != next {
		closer.Close()
	}
	s.mu.Unlock()
}

func (s *SwitchableService) Close() {
	if s == nil {
		return
	}
	s.mu.Lock()
	previous := s.current
	s.current = nil
	if closer, ok := previous.(interface{ Close() }); ok {
		closer.Close()
	}
	s.mu.Unlock()
}

func (s *SwitchableService) Current() Service {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.current
}

func (s *SwitchableService) withService(fn func(Service) error) error {
	if s == nil {
		return errGraphServiceUnavailable
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == nil {
		return errGraphServiceUnavailable
	}
	return fn(s.current)
}

func (s *SwitchableService) SyncNode(entityType, entityID, label string, properties map[string]interface{}) error {
	return s.withService(func(current Service) error {
		return current.SyncNode(entityType, entityID, label, properties)
	})
}

func (s *SwitchableService) SyncEdge(sourceID, targetID, relationType string, weight float64) error {
	return s.withService(func(current Service) error {
		return current.SyncEdge(sourceID, targetID, relationType, weight)
	})
}

func (s *SwitchableService) DeleteNode(entityID string) error {
	return s.withService(func(current Service) error {
		return current.DeleteNode(entityID)
	})
}

func (s *SwitchableService) DeleteNodeForSpace(entityID, spaceID string) error {
	return s.withService(func(current Service) error {
		return current.DeleteNodeForSpace(entityID, spaceID)
	})
}

func (s *SwitchableService) DeleteNodeIfOrphan(entityID string) error {
	return s.withService(func(current Service) error {
		return current.DeleteNodeIfOrphan(entityID)
	})
}

func (s *SwitchableService) DeleteNodesByProperty(entityType, propertyKey, propertyValue string) error {
	return s.withService(func(current Service) error {
		return current.DeleteNodesByProperty(entityType, propertyKey, propertyValue)
	})
}

func (s *SwitchableService) QueryNeighbors(entityID string, depth int, spaceID string) (result map[string]interface{}, err error) {
	err = s.withService(func(current Service) error {
		result, err = current.QueryNeighbors(entityID, depth, spaceID)
		return err
	})
	return result, err
}

func (s *SwitchableService) FindPaths(sourceID, targetID string, maxDepth int) (result []map[string]interface{}, err error) {
	err = s.withService(func(current Service) error {
		result, err = current.FindPaths(sourceID, targetID, maxDepth)
		return err
	})
	return result, err
}

func (s *SwitchableService) FindPathsForSpace(sourceID, targetID string, maxDepth int, spaceID string) (result []map[string]interface{}, err error) {
	err = s.withService(func(current Service) error {
		result, err = current.FindPathsForSpace(sourceID, targetID, maxDepth, spaceID)
		return err
	})
	return result, err
}

func (s *SwitchableService) DeleteOrphanNodes() error {
	return s.withService(func(current Service) error {
		return current.DeleteOrphanNodes()
	})
}

func (s *SwitchableService) GetStats(spaceID string) (result map[string]interface{}, err error) {
	err = s.withService(func(current Service) error {
		result, err = current.GetStats(spaceID)
		return err
	})
	return result, err
}

func (s *SwitchableService) GetAllNodes(spaceID string) (result []map[string]interface{}, err error) {
	err = s.withService(func(current Service) error {
		result, err = current.GetAllNodes(spaceID)
		return err
	})
	return result, err
}

func (s *SwitchableService) GetAllEdges(spaceID string) (result []map[string]interface{}, err error) {
	err = s.withService(func(current Service) error {
		result, err = current.GetAllEdges(spaceID)
		return err
	})
	return result, err
}

func (s *SwitchableService) Name() (name string) {
	if s == nil {
		return "图谱关系"
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.current == nil {
		return "图谱关系"
	}
	return s.current.Name()
}

func (s *SwitchableService) Process(ctx context.Context, convID string, messages []map[string]string, newReply string) error {
	return s.withService(func(current Service) error {
		return current.Process(ctx, convID, messages, newReply)
	})
}
