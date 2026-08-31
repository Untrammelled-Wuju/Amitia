package workflow

import (
	"context"
	"fmt"
	"sync"
)

type DefinitionStore interface {
	Save(ctx context.Context, def WorkflowDefinition) error
	Delete(ctx context.Context, workflowID string) error
	List(ctx context.Context) ([]WorkflowDefinition, error)
	SetEnabled(ctx context.Context, workflowID string, enabled bool) error
}

type WorkflowFilter struct {
	Enabled         *bool
	CallableByAgent *bool
}

type WorkflowRegistry struct {
	mu    sync.RWMutex
	items map[string]WorkflowDefinition
	store DefinitionStore
}

func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{
		items: map[string]WorkflowDefinition{},
	}
}

func (r *WorkflowRegistry) SetDefinitionStore(store DefinitionStore) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.store = store
}

func (r *WorkflowRegistry) LoadFromStore(ctx context.Context) error {
	if r.store == nil {
		return nil
	}
	defs, err := r.store.List(ctx)
	if err != nil {
		return fmt.Errorf("load workflow definitions from store: %w", err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, def := range defs {
		r.items[def.ID] = def
	}
	return nil
}

func (r *WorkflowRegistry) Register(definition WorkflowDefinition) error {
	computedHash := ComputeDefinitionHash(definition)
	if definition.DefinitionHash != "" && definition.DefinitionHash != computedHash {
		return fmt.Errorf("workflow %s definition hash mismatch", definition.ID)
	}
	definition.DefinitionHash = computedHash
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.items[definition.ID]; exists {
		return fmt.Errorf("workflow %s already registered", definition.ID)
	}
	r.items[definition.ID] = definition
	if r.store != nil {
		ctx := context.Background()
		if err := r.store.Save(ctx, definition); err != nil {
			delete(r.items, definition.ID)
			return fmt.Errorf("persist workflow definition: %w", err)
		}
	}
	return nil
}

func (r *WorkflowRegistry) Upsert(definition WorkflowDefinition) error {
	normalized, err := NormalizeDefinition(definition)
	if err != nil {
		return err
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	previous, existed := r.items[normalized.ID]
	r.items[normalized.ID] = normalized
	if r.store != nil {
		ctx := context.Background()
		if err := r.store.Save(ctx, normalized); err != nil {
			if existed {
				r.items[normalized.ID] = previous
			} else {
				delete(r.items, normalized.ID)
			}
			return fmt.Errorf("persist workflow definition: %w", err)
		}
	}
	return nil
}

func (r *WorkflowRegistry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("workflow %s not found", id)
	}
	delete(r.items, id)
	if r.store != nil {
		ctx := context.Background()
		if err := r.store.Delete(ctx, id); err != nil {
			r.items[id] = item
			return fmt.Errorf("persist workflow unregister: %w", err)
		}
	}
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
	if r.store != nil {
		ctx := context.Background()
		_ = r.store.SetEnabled(ctx, id, enabled)
	}
	return nil
}

func (r *WorkflowRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}
