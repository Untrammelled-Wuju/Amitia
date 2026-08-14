package capability

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type ToolFilter struct {
	Enabled         *bool
	Source          ToolSource
	IncludeInternal bool
}

// ToolRegistry 是 Tool/Capability definition 的 canonical registry。
// 后续 Capability Provider Registry 与 Provider Instance 必须属于 capability 领域。
// runtimeorchestrator.ProviderRegistry 不是此层 Provider Registry。
type ToolRegistry struct {
	mu          sync.RWMutex
	items       map[string]ToolDefinition
	names       map[string]string
	byOwner     map[string][]string
	bySource    map[ToolSource][]string
	schemaCache *JSONSchemaCache
}

func NewToolRegistry(schemaCaches ...*JSONSchemaCache) *ToolRegistry {
	cache := (*JSONSchemaCache)(nil)
	if len(schemaCaches) > 0 {
		cache = schemaCaches[0]
	}
	if cache == nil {
		cache = NewJSONSchemaCache()
	}

	return &ToolRegistry{
		items:       map[string]ToolDefinition{},
		names:       map[string]string{},
		byOwner:     map[string][]string{},
		bySource:    map[ToolSource][]string{},
		schemaCache: cache,
	}
}

func toolOwnerKey(definition ToolDefinition) string {
	if definition.ExtensionID != "" {
		return "extension:" + definition.ExtensionID
	}
	return "system:core"
}

func (r *ToolRegistry) resolveModelNameUnsafe(modelName string, toolID string) string {
	if modelName == "" {
		return ""
	}

	if existing, ok := r.names[modelName]; !ok || existing == toolID {
		return modelName
	}

	for suffix := 2; ; suffix++ {
		candidate := fmt.Sprintf("%s_%d", modelName, suffix)

		existing, exists := r.names[candidate]
		if !exists || existing == toolID {
			return candidate
		}
	}
}

func (r *ToolRegistry) prepareDefinitionUnsafe(definition ToolDefinition) (ToolDefinition, error) {
	if definition.ID == "" {
		return ToolDefinition{}, fmt.Errorf("tool id is required")
	}

	if err := r.schemaCache.ValidateToolDefinition(definition); err != nil {
		return ToolDefinition{}, fmt.Errorf("tool %s schema contract: %w", definition.ID, err)
	}

	definition.ModelName = r.resolveModelNameUnsafe(
		definition.ModelName,
		definition.ID,
	)

	return definition, nil
}

func (r *ToolRegistry) storeUnsafe(definition ToolDefinition) {
	if definition.ModelName != "" {
		r.names[definition.ModelName] = definition.ID
	}

	ownerKey := toolOwnerKey(definition)

	r.items[definition.ID] = definition

	r.byOwner[ownerKey] = append(
		r.byOwner[ownerKey],
		definition.ID,
	)

	r.bySource[definition.Source] = append(
		r.bySource[definition.Source],
		definition.ID,
	)
}

func (r *ToolRegistry) register(ctx context.Context, definition ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.items[definition.ID]; exists {
		return fmt.Errorf(
			"tool %s already registered",
			definition.ID,
		)
	}

	prepared, err := r.prepareDefinitionUnsafe(definition)
	if err != nil {
		return err
	}

	r.storeUnsafe(prepared)

	return nil
}

func (r *ToolRegistry) Register(ctx context.Context, definition ToolDefinition) error {
	return r.register(ctx, definition)
}

func (r *ToolRegistry) Replace(ctx context.Context, definition ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	existing, exists := r.items[definition.ID]
	if exists {
		existingOwner := toolOwnerKey(existing)
		incomingOwner := toolOwnerKey(definition)

		if existingOwner != incomingOwner {
			return fmt.Errorf(
				"tool %s is owned by %s, cannot be replaced by %s",
				definition.ID,
				existingOwner,
				incomingOwner,
			)
		}

		if err := r.removeUnsafe(definition.ID); err != nil {
			return err
		}
	}

	prepared, err := r.prepareDefinitionUnsafe(definition)
	if err != nil {
		return err
	}

	r.storeUnsafe(prepared)

	return nil
}

func copyModelNameIndex(src map[string]string) map[string]string {
	dst := make(map[string]string, len(src))

	for k, v := range src {
		dst[k] = v
	}

	return dst
}

func (r *ToolRegistry) BatchRegister(ctx context.Context, definitions []ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	batchIDs := make(map[string]bool, len(definitions))
	for _, d := range definitions {
		if batchIDs[d.ID] {
			return fmt.Errorf("duplicate tool id %s in batch", d.ID)
		}
		batchIDs[d.ID] = true
	}

	for _, d := range definitions {
		if _, exists := r.items[d.ID]; exists {
			return fmt.Errorf("tool %s already registered, batch aborted", d.ID)
		}
	}

	for _, d := range definitions {
		if err := r.schemaCache.ValidateToolDefinition(d); err != nil {
			return fmt.Errorf("tool %s schema contract: %w", d.ID, err)
		}
	}

	tmpNames := copyModelNameIndex(r.names)
	prepared := make([]ToolDefinition, 0, len(definitions))

	for _, d := range definitions {
		if d.ID == "" {
			return fmt.Errorf("tool id is required")
		}

		resolvedName := d.ModelName
		if resolvedName != "" {
			if existing, ok := tmpNames[resolvedName]; !ok || existing == d.ID {
				tmpNames[resolvedName] = d.ID
			} else {
				for suffix := 2; ; suffix++ {
					candidate := fmt.Sprintf("%s_%d", resolvedName, suffix)
					if existing, exists := tmpNames[candidate]; !exists || existing == d.ID {
						resolvedName = candidate
						tmpNames[candidate] = d.ID
						break
					}
				}
			}
		}

		d.ModelName = resolvedName
		prepared = append(prepared, d)
	}

	for _, d := range prepared {
		r.storeUnsafe(d)
	}

	return nil
}

func (r *ToolRegistry) BatchReplace(ctx context.Context, definitions []ToolDefinition) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	batchIDs := make(map[string]bool, len(definitions))
	for _, d := range definitions {
		if batchIDs[d.ID] {
			return fmt.Errorf("duplicate tool id %s in batch", d.ID)
		}
		batchIDs[d.ID] = true
	}

	for _, d := range definitions {
		existing, exists := r.items[d.ID]
		if exists {
			existingOwner := toolOwnerKey(existing)
			incomingOwner := toolOwnerKey(d)

			if existingOwner != incomingOwner {
				return fmt.Errorf(
					"tool %s is owned by %s, cannot be replaced by %s",
					d.ID,
					existingOwner,
					incomingOwner,
				)
			}
		}
	}

	for _, d := range definitions {
		if err := r.schemaCache.ValidateToolDefinition(d); err != nil {
			return fmt.Errorf("tool %s schema contract: %w", d.ID, err)
		}
	}

	prepared := make([]ToolDefinition, 0, len(definitions))
	for _, d := range definitions {
		p, err := r.prepareDefinitionUnsafe(d)
		if err != nil {
			return err
		}
		prepared = append(prepared, p)
	}

	for _, d := range prepared {
		if _, exists := r.items[d.ID]; exists {
			if err := r.removeUnsafe(d.ID); err != nil {
				return err
			}
		}
	}

	for _, d := range prepared {
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

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

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

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

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

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

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

func (r *ToolRegistry) ResolveModelName(name string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.names[name]
}

func (r *ToolRegistry) SchemaCache() *JSONSchemaCache {
	return r.schemaCache
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

func (r *ToolRegistry) removeUnsafe(id string) error {
	item, ok := r.items[id]
	if !ok {
		return fmt.Errorf("tool %s not found", id)
	}

	if item.ModelName != "" {
		if mappedID, exists := r.names[item.ModelName]; exists &&
			mappedID == id {
			delete(r.names, item.ModelName)
		}
	}

	ownerKey := toolOwnerKey(item)

	ownerSlice := removeFromSlice(
		r.byOwner[ownerKey],
		id,
	)

	if len(ownerSlice) == 0 {
		delete(r.byOwner, ownerKey)
	} else {
		r.byOwner[ownerKey] = ownerSlice
	}

	sourceSlice := removeFromSlice(
		r.bySource[item.Source],
		id,
	)

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
