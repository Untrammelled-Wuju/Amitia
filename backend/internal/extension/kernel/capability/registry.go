package capability

import (
	"context"
	"fmt"
	"sync"
)

type ToolFilter struct {
	Enabled         *bool
	Source          ToolSource
	IncludeInternal bool
}

type ToolRegistry struct {
	mu       sync.RWMutex
	items    map[string]ToolDefinition
	names    map[string]string
	byOwner  map[string][]string
	bySource map[ToolSource][]string
}

func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		items:    map[string]ToolDefinition{},
		names:    map[string]string{},
		byOwner:  map[string][]string{},
		bySource: map[ToolSource][]string{},
	}
}

func (r *ToolRegistry) Register(ctx context.Context, definition ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[definition.ID]; exists {
		return fmt.Errorf("tool %s already registered", definition.ID)
	}

	r.storeUnsafe(definition)
	return nil
}

func (r *ToolRegistry) Replace(ctx context.Context, definition ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.items[definition.ID]
	if exists {
		if existing.ExtensionID != "" && definition.ExtensionID != "" && existing.ExtensionID != definition.ExtensionID {
			return fmt.Errorf("tool %s is owned by %s, cannot be replaced by %s", definition.ID, existing.ExtensionID, definition.ExtensionID)
		}
		r.removeUnsafe(definition.ID)
	}

	r.storeUnsafe(definition)
	return nil
}

func (r *ToolRegistry) BatchRegister(ctx context.Context, definitions []ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, d := range definitions {
		if _, exists := r.items[d.ID]; exists {
			for _, registered := range definitions {
				if registered.ID != d.ID {
					r.removeUnsafe(registered.ID)
				}
			}
			return fmt.Errorf("tool %s already registered, batch aborted", d.ID)
		}
	}

	for _, d := range definitions {
		r.storeUnsafe(d)
	}

	return nil
}

func (r *ToolRegistry) BatchReplace(ctx context.Context, definitions []ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	conflictCheck := make(map[string]bool)
	for _, d := range definitions {
		existing, exists := r.items[d.ID]
		if exists && existing.ExtensionID != "" && d.ExtensionID != "" && existing.ExtensionID != d.ExtensionID {
			return fmt.Errorf("tool %s is owned by %s, cannot be replaced by %s", d.ID, existing.ExtensionID, d.ExtensionID)
		}
		if conflictCheck[d.ID] {
			return fmt.Errorf("duplicate tool id %s in batch", d.ID)
		}
		conflictCheck[d.ID] = true
	}

	for _, d := range definitions {
		r.removeUnsafe(d.ID)
	}

	for _, d := range definitions {
		r.storeUnsafe(d)
	}

	return nil
}

func (r *ToolRegistry) Unregister(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if err := r.removeUnsafe(id); err != nil {
		return err
	}
	return nil
}

func (r *ToolRegistry) UnregisterByOwner(ctx context.Context, ownerKey string) ([]string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	ids := make([]string, len(r.byOwner[ownerKey]))
	copy(ids, r.byOwner[ownerKey])

	for _, id := range ids {
		r.removeUnsafe(id)
	}

	delete(r.byOwner, ownerKey)
	return ids, nil
}

func (r *ToolRegistry) Get(ctx context.Context, id string) (ToolDefinition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.items[id]
	return item, ok
}

func (r *ToolRegistry) GetByModelName(ctx context.Context, name string) (ToolDefinition, bool) {
	r.mu.RLock()
	id, ok := r.names[name]
	r.mu.RUnlock()
	if !ok {
		return ToolDefinition{}, false
	}
	return r.Get(ctx, id)
}

func (r *ToolRegistry) List(ctx context.Context, filter ToolFilter) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]ToolDefinition, 0, len(r.items))
	for _, item := range r.items {
		if item.Internal && !filter.IncludeInternal {
			continue
		}
		if filter.Enabled != nil && item.Enabled != *filter.Enabled {
			continue
		}
		if filter.Source != "" && item.Source != filter.Source {
			continue
		}
		result = append(result, item)
	}
	return result
}

func (r *ToolRegistry) ListByOwner(ctx context.Context, ownerKey string) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.byOwner[ownerKey]
	result := make([]ToolDefinition, 0, len(ids))
	for _, id := range ids {
		if item, ok := r.items[id]; ok {
			result = append(result, item)
		}
	}
	return result
}

func (r *ToolRegistry) ListBySource(ctx context.Context, source ToolSource) []ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.bySource[source]
	result := make([]ToolDefinition, 0, len(ids))
	for _, id := range ids {
		if item, ok := r.items[id]; ok {
			result = append(result, item)
		}
	}
	return result
}

func (r *ToolRegistry) SetEnabled(ctx context.Context, id string, enabled bool) error {
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

func (r *ToolRegistry) RegisterModelName(modelName string, toolID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.registerModelNameUnsafe(modelName, toolID)
}

func (r *ToolRegistry) ResolveModelName(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.names[name]
}

func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.items)
}

func (r *ToolRegistry) CountBySource(source ToolSource) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.bySource[source])
}

func (r *ToolRegistry) CountByOwner(ownerKey string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byOwner[ownerKey])
}

func (r *ToolRegistry) registerModelNameUnsafe(modelName string, toolID string) {
	if modelName == "" {
		return
	}

	if existing, ok := r.names[modelName]; ok && existing != toolID {
		suffix := 2
		for {
			resolved := fmt.Sprintf("%s_%d", modelName, suffix)
			if _, exists := r.names[resolved]; !exists {
				r.names[resolved] = toolID
				return
			}
			suffix++
		}
	}

	r.names[modelName] = toolID
}

func (r *ToolRegistry) storeUnsafe(definition ToolDefinition) {
	if definition.ModelName != "" {
		r.registerModelNameUnsafe(definition.ModelName, definition.ID)
	}
	ownerKey := ""
	if definition.ExtensionID != "" {
		ownerKey = "extension:" + definition.ExtensionID
	} else {
		ownerKey = "system:core"
	}
	r.items[definition.ID] = definition
	r.byOwner[ownerKey] = append(r.byOwner[ownerKey], definition.ID)
	r.bySource[definition.Source] = append(r.bySource[definition.Source], definition.ID)
}

func (r *ToolRegistry) removeUnsafe(id string) error {
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("tool %s not found", id)
	}
	if item.ModelName != "" {
		delete(r.names, item.ModelName)
	}
	ownerKey := ""
	if item.ExtensionID != "" {
		ownerKey = "extension:" + item.ExtensionID
	} else {
		ownerKey = "system:core"
	}
	ownerSlice := removeFromSlice(r.byOwner[ownerKey], id)
	if len(ownerSlice) == 0 {
		delete(r.byOwner, ownerKey)
	} else {
		r.byOwner[ownerKey] = ownerSlice
	}
	sourceSlice := removeFromSlice(r.bySource[item.Source], id)
	if len(sourceSlice) == 0 {
		delete(r.bySource, item.Source)
	} else {
		r.bySource[item.Source] = sourceSlice
	}
	delete(r.items, id)
	return nil
}

func removeFromSlice(slice []string, id string) []string {
	for i, v := range slice {
		if v == id {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}
