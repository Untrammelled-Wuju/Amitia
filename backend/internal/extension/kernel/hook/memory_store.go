package hook

import (
	"context"
	"sort"
	"sync"
)

type MemoryContributionStore struct {
	mu      sync.RWMutex
	items   map[string]HookContributionDefinition
	byPoint map[string][]string
	byExt   map[string][]string
}

func NewMemoryContributionStore() *MemoryContributionStore {
	return &MemoryContributionStore{
		items:   make(map[string]HookContributionDefinition),
		byPoint: make(map[string][]string),
		byExt:   make(map[string][]string),
	}
}

func (s *MemoryContributionStore) Register(_ context.Context, contrib HookContributionDefinition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.items[contrib.ContributionID]; exists {
		return ErrContributionExists
	}
	s.items[contrib.ContributionID] = contrib
	s.byPoint[contrib.HookPointID] = append(s.byPoint[contrib.HookPointID], contrib.ContributionID)
	s.byExt[contrib.ExtensionID] = append(s.byExt[contrib.ExtensionID], contrib.ContributionID)
	return nil
}

func (s *MemoryContributionStore) Unregister(_ context.Context, contributionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.items[contributionID]
	if !ok {
		return ErrContributionMissing
	}
	delete(s.items, contributionID)
	s.removeFromIndex(s.byPoint, c.HookPointID, contributionID)
	s.removeFromIndex(s.byExt, c.ExtensionID, contributionID)
	return nil
}

func (s *MemoryContributionStore) Get(_ context.Context, contributionID string) (HookContributionDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	c, ok := s.items[contributionID]
	if !ok {
		return HookContributionDefinition{}, ErrContributionMissing
	}
	return c, nil
}

func (s *MemoryContributionStore) ListByHookPoint(_ context.Context, hookPointID string) ([]HookContributionDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byPoint[hookPointID]
	out := make([]HookContributionDefinition, 0, len(ids))
	for _, id := range ids {
		if c, ok := s.items[id]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *MemoryContributionStore) ListByExtension(_ context.Context, extensionID string) ([]HookContributionDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.byExt[extensionID]
	out := make([]HookContributionDefinition, 0, len(ids))
	for _, id := range ids {
		if c, ok := s.items[id]; ok {
			out = append(out, c)
		}
	}
	return out, nil
}

func (s *MemoryContributionStore) List(_ context.Context) ([]HookContributionDefinition, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]HookContributionDefinition, 0, len(s.items))
	for _, c := range s.items {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].ContributionID < out[j].ContributionID
	})
	return out, nil
}

func (s *MemoryContributionStore) SetEnabled(_ context.Context, contributionID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	c, ok := s.items[contributionID]
	if !ok {
		return ErrContributionMissing
	}
	c.Enabled = enabled
	s.items[contributionID] = c
	return nil
}

func (s *MemoryContributionStore) UnregisterByExtension(_ context.Context, extensionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := s.byExt[extensionID]
	for _, id := range ids {
		c, ok := s.items[id]
		if !ok {
			continue
		}
		delete(s.items, id)
		s.removeFromIndex(s.byPoint, c.HookPointID, id)
	}
	delete(s.byExt, extensionID)
	return nil
}

func (s *MemoryContributionStore) removeFromIndex(idx map[string][]string, key, id string) {
	ids := idx[key]
	for i, existing := range ids {
		if existing == id {
			idx[key] = append(ids[:i], ids[i+1:]...)
			break
		}
	}
	if len(idx[key]) == 0 {
		delete(idx, key)
	}
}
