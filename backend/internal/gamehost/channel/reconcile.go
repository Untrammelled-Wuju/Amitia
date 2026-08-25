package channel

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type ReconcileResult struct {
	Added   []RuntimeChannelID
	Removed []RuntimeChannelID
	Updated []RuntimeChannelID
	Noop    []RuntimeChannelID
}

type Reconciler struct {
	registry Registry
	mapper   *Mapper
}

func NewReconciler(registry Registry, mapper *Mapper) *Reconciler {
	return &Reconciler{
		registry: registry,
		mapper:   mapper,
	}
}

func (r *Reconciler) ReconcileRuntimeChannels(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	inputs []ChannelMappingInput,
) (ReconcileResult, error) {
	var allDesired []RuntimeChannel
	for _, input := range inputs {
		result, err := r.mapper.Map(ctx, input)
		if err != nil {
			return ReconcileResult{}, err
		}
		if len(result.Errors) > 0 {
			return ReconcileResult{}, result.Errors[0]
		}
		for _, ch := range result.Channels {
			if err := ch.Validate(); err != nil {
				return ReconcileResult{}, err
			}
		}
		allDesired = append(allDesired, result.Channels...)
	}

	currentChannels, err := r.registry.ListByRuntime(runtimeID)
	if err != nil {
		return ReconcileResult{}, err
	}

	currentMap := make(map[RuntimeChannelID]RuntimeChannel, len(currentChannels))
	for _, ch := range currentChannels {
		currentMap[ch.ID] = ch
	}

	desiredMap := make(map[RuntimeChannelID]RuntimeChannel, len(allDesired))
	for _, ch := range allDesired {
		desiredMap[ch.ID] = ch
	}

	result := ReconcileResult{}

	for id, desired := range desiredMap {
		current, exists := currentMap[id]
		if !exists {
			result.Added = append(result.Added, id)
		} else if !runtimeChannelsEqual(current, desired) {
			result.Updated = append(result.Updated, id)
		} else {
			result.Noop = append(result.Noop, id)
		}
	}

	for id := range currentMap {
		if _, exists := desiredMap[id]; !exists {
			result.Removed = append(result.Removed, id)
		}
	}

	for _, id := range result.Removed {
		if err := r.registry.Unregister(ctx, id); err != nil {
			return ReconcileResult{}, err
		}
	}

	for _, id := range result.Added {
		if err := r.registry.Register(ctx, desiredMap[id]); err != nil {
			return ReconcileResult{}, err
		}
	}

	for _, id := range result.Updated {
		if err := r.registry.Unregister(ctx, id); err != nil {
			return ReconcileResult{}, err
		}
		if err := r.registry.Register(ctx, desiredMap[id]); err != nil {
			return ReconcileResult{}, err
		}
	}

	return result, nil
}

func runtimeChannelsEqual(a, b RuntimeChannel) bool {
	if a.ID != b.ID ||
		a.PluginID != b.PluginID ||
		a.RuntimeID != b.RuntimeID ||
		a.ServiceID != b.ServiceID ||
		a.ChannelID != b.ChannelID ||
		a.Kind != b.Kind ||
		a.SchemaID != b.SchemaID ||
		a.Direction != b.Direction {
		return false
	}

	if a.Frequency != nil && b.Frequency != nil && *a.Frequency != *b.Frequency {
		return false
	}
	if (a.Frequency == nil) != (b.Frequency == nil) {
		return false
	}

	if len(a.Metadata) != len(b.Metadata) {
		return false
	}
	for k, v := range a.Metadata {
		bv, ok := b.Metadata[k]
		if !ok || string(v) != string(bv) {
			return false
		}
	}
	return true
}
