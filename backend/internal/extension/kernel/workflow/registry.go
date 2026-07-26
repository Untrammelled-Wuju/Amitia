package workflow

import (
	"fmt"
	"sync"
)

type WorkflowFilter struct {
	Enabled         *bool
	CallableByAgent *bool
}

type WorkflowRegistry struct {
	mu    sync.RWMutex
	items map[string]WorkflowDefinition
}

func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		items: map[string]WorkflowDefinition{},
	}
}

func (r *WorkflowRegistry) Register(definition WorkflowDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[definition.ID]; exists {
		return fmt.Errorf("workflow %s already registered", definition.ID)
	}
	r.items[definition.ID] = definition
	return nil
}

func (r *WorkflowRegistry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return fmt.Errorf("workflow %s not found", id)
	}
	delete(r.items, id)
	return nil
}

func (r *WorkflowRegistry) Get(id string) (WorkflowDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[id]
	return item, ok
}

func (r *WorkflowRegistry) List(filter WorkflowFilter) []WorkflowDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]WorkflowDefinition, 0, len(r.items))
	for _, item := range r.items {
		if filter.Enabled != nil && item.Enabled != *filter.Enabled {
			continue
		}
		if filter.CallableByAgent != nil && item.CallableByAgent != *filter.CallableByAgent {
			continue
		}
		result = append(result, item)
	}
	return result
}

func (r *WorkflowRegistry) SetEnabled(id string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return nil
	}
	item.Enabled = enabled
	r.items[id] = item
	return nil
}

func (r *WorkflowRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}
