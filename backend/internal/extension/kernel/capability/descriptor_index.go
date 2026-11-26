package capability

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type DescriptorEntry struct {
	ToolID          string
	ModelName       string
	Description     string
	Source          ToolSource
	CapabilityID    string
	SchemaRef       SchemaRef
	SchemaBytes     int
	EstimatedTokens int
}

type DescriptorIndex struct {
	mu        sync.RWMutex
	entries   []DescriptorEntry
	byID      map[string]int
	revision  uint64
	mutations uint64
	rebuilds  uint64
	skipped   uint64
}

func assembleDescriptors(store *SchemaStore, tools []ToolDefinition, base map[string]DescriptorEntry) ([]DescriptorEntry, map[string]int, error) {
	if store == nil {
		return nil, nil, fmt.Errorf("%w: schema store is required", ErrDescriptorInvalid)
	}
	entries := make([]DescriptorEntry, 0, len(tools)+len(base))
	byID := map[string]int{}
	for _, tool := range tools {
		if strings.TrimSpace(tool.ID) == "" {
			return nil, nil, fmt.Errorf("%w: tool id is empty", ErrDescriptorInvalid)
		}
		if strings.TrimSpace(tool.ModelName) == "" {
			return nil, nil, fmt.Errorf("%w: tool %s has no model name", ErrDescriptorInvalid, tool.ID)
		}
		if _, exists := byID[tool.ID]; exists {
			return nil, nil, fmt.Errorf("%w: duplicate tool id %s", ErrDescriptorInvalid, tool.ID)
		}
		entry, err := descriptorEntryFor(store, tool)
		if err != nil {
			return nil, nil, err
		}
		byID[entry.ToolID] = len(entries)
		entries = append(entries, entry)
	}
	for id, entry := range base {
		if _, exists := byID[id]; exists {
			continue
		}
		byID[id] = len(entries)
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(left, right int) bool {
		return entries[left].ToolID < entries[right].ToolID
	})
	for position, entry := range entries {
		byID[entry.ToolID] = position
	}
	return entries, byID, nil
}

func descriptorEntryFor(store *SchemaStore, tool ToolDefinition) (DescriptorEntry, error) {
	ref, _, err := store.Put(tool.InputSchema)
	if err != nil {
		return DescriptorEntry{}, fmt.Errorf("tool %s: %w", tool.ID, err)
	}
	return DescriptorEntry{
		ToolID:          tool.ID,
		ModelName:       tool.ModelName,
		Description:     tool.Description,
		Source:          tool.Source,
		CapabilityID:    string(tool.CapabilityID),
		SchemaRef:       ref,
		SchemaBytes:     len(tool.InputSchema),
		EstimatedTokens: (len(tool.ModelName) + len(tool.Description)) / 4,
	}, nil
}

func BuildDescriptorIndex(store *SchemaStore, tools []ToolDefinition) (*DescriptorIndex, error) {
	entries, byID, err := assembleDescriptors(store, tools, nil)
	if err != nil {
		return nil, err
	}
	return &DescriptorIndex{entries: entries, byID: byID, revision: 1}, nil
}

func (i *DescriptorIndex) Len() int {
	if i == nil {
		return 0
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	return len(i.entries)
}

func (i *DescriptorIndex) Revision() uint64 {
	if i == nil {
		return 0
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.revision
}

func (i *DescriptorIndex) Mutations() uint64 {
	if i == nil {
		return 0
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.mutations
}

func (i *DescriptorIndex) Rebuilds() uint64 {
	if i == nil {
		return 0
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.rebuilds
}

func (i *DescriptorIndex) Skipped() uint64 {
	if i == nil {
		return 0
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.skipped
}

func (i *DescriptorIndex) Entry(toolID string) (DescriptorEntry, bool) {
	if i == nil {
		return DescriptorEntry{}, false
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	position, ok := i.byID[strings.TrimSpace(toolID)]
	if !ok {
		return DescriptorEntry{}, false
	}
	return i.entries[position], true
}

func (i *DescriptorIndex) Query(filter ToolFilter) []DescriptorEntry {
	if i == nil {
		return []DescriptorEntry{}
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	result := make([]DescriptorEntry, 0, len(i.entries))
	for _, entry := range i.entries {
		if filter.Source != "" && entry.Source != filter.Source {
			continue
		}
		result = append(result, entry)
	}
	return result
}

func (i *DescriptorIndex) Refs() []SchemaRef {
	if i == nil {
		return []SchemaRef{}
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	refs := make([]SchemaRef, 0, len(i.entries))
	for _, entry := range i.entries {
		refs = append(refs, entry.SchemaRef)
	}
	return refs
}

type HydrateOptions struct {
	RequireFresh bool
}

func (i *DescriptorIndex) Hydrate(ctx context.Context, store *SchemaStore, toolIDs []string, options HydrateOptions) ([]ModelToolView, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if store == nil {
		return nil, fmt.Errorf("%w: schema store is required", ErrDescriptorInvalid)
	}
	views := make([]ModelToolView, 0, len(toolIDs))
	for _, toolID := range toolIDs {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		entry, ok := i.Entry(toolID)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrDescriptorUnknown, toolID)
		}
		schema, err := store.Load(entry.SchemaRef, options.RequireFresh)
		if err != nil {
			return nil, fmt.Errorf("tool %s: %w", entry.ToolID, err)
		}
		views = append(views, ModelToolView{
			Name:         entry.ModelName,
			Description:  entry.Description,
			InputSchema:  schema,
			CapabilityID: entry.CapabilityID,
		})
	}
	return views, nil
}
