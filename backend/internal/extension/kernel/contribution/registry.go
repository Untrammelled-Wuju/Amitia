package contribution

import (
	"sync"
)

type ContributionFilter struct {
	Type        ContributionType
	ExtensionID string
	Enabled     *bool
}

type ContributionRegistry struct {
	mu    sync.RWMutex
	items map[string]Contribution
}

func NewContributionRegistry() *ContributionRegistry {
	return &ContributionRegistry{
		items: map[string]Contribution{},
	}
}

func (r *ContributionRegistry) Register(c Contribution) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items[c.ContributionID()] = c
	return nil
}

func (r *ContributionRegistry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.items, id)
	return nil
}

func (r *ContributionRegistry) Get(id string) (Contribution, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[id]
	return item, ok
}

func (r *ContributionRegistry) List(filter ContributionFilter) []Contribution {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Contribution, 0, len(r.items))
	for _, item := range r.items {
		if filter.Type != "" && item.ContributionType() != filter.Type {
			continue
		}
		if filter.ExtensionID != "" && item.ExtensionID() != filter.ExtensionID {
			continue
		}
		result = append(result, item)
	}
	return result
}

func (r *ContributionRegistry) ListByExtension(extensionID string) []Contribution {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Contribution, 0)
	for _, item := range r.items {
		if item.ExtensionID() == extensionID {
			result = append(result, item)
		}
	}
	return result
}

func (r *ContributionRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

func (r *ContributionRegistry) CountByType(contributionType ContributionType) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, item := range r.items {
		if item.ContributionType() == contributionType {
			count++
		}
	}
	return count
}

func (r *ContributionRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.items = map[string]Contribution{}
}
