package channel

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type Registry interface {
	Register(ctx context.Context, channel RuntimeChannel) error
	Unregister(ctx context.Context, id RuntimeChannelID) error
	Get(ctx context.Context, id RuntimeChannelID) (RuntimeChannel, error)
	Resolve(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) (RuntimeChannel, error)
	List() ([]RuntimeChannel, error)
	ListByRuntime(runtimeID domain.RuntimeInstanceID) ([]RuntimeChannel, error)
	ListByService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) ([]RuntimeChannel, error)
	ListByKind(runtimeID domain.RuntimeInstanceID, kind domain.ChannelKind) ([]RuntimeChannel, error)
	RemoveByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error)
	RemoveByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (int, error)
	Count() int
	CountByRuntime(runtimeID domain.RuntimeInstanceID) int
	CountByService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) int
}

type memoryRegistry struct {
	mu       sync.RWMutex
	index    map[RuntimeChannelID]RuntimeChannel

	runtimeIndex  map[domain.RuntimeInstanceID]map[RuntimeChannelID]struct{}
	serviceIndex  map[domain.RuntimeInstanceID]map[domain.ServiceID]map[RuntimeChannelID]struct{}
	pluginIndex   map[domain.PluginID]map[RuntimeChannelID]struct{}

	validator *Validator
}

func NewRegistry(opts Options) Registry {
	return NewMemoryRegistry(opts)
}

func NewMemoryRegistry(opts Options) *memoryRegistry {
	return &memoryRegistry{
		index:        make(map[RuntimeChannelID]RuntimeChannel),
		runtimeIndex: make(map[domain.RuntimeInstanceID]map[RuntimeChannelID]struct{}),
		serviceIndex: make(map[domain.RuntimeInstanceID]map[domain.ServiceID]map[RuntimeChannelID]struct{}),
		pluginIndex:  make(map[domain.PluginID]map[RuntimeChannelID]struct{}),
		validator:    NewValidator(opts),
	}
}

func (r *memoryRegistry) Register(ctx context.Context, channel RuntimeChannel) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.index[channel.ID]; exists {
		return errors.New("channel: already exists: " + string(channel.ID))
	}

	currentByRuntime := len(r.runtimeIndex[channel.RuntimeID])
	currentByService := 0
	if si, ok := r.serviceIndex[channel.RuntimeID]; ok {
		currentByService = len(si[channel.ServiceID])
	}

	if err := r.validator.ValidateRegistration(ctx, channel, currentByRuntime, currentByService); err != nil {
		if domain.IsHostError(err, domain.ErrAlreadyExists) || errors.Is(err, ErrChannelLimitRuntime) || errors.Is(err, ErrChannelLimitService) {
			return err
		}
		return err
	}

	stored := channel.Clone()
	r.index[channel.ID] = stored

	if r.runtimeIndex[channel.RuntimeID] == nil {
		r.runtimeIndex[channel.RuntimeID] = make(map[RuntimeChannelID]struct{})
	}
	r.runtimeIndex[channel.RuntimeID][channel.ID] = struct{}{}

	if r.serviceIndex[channel.RuntimeID] == nil {
		r.serviceIndex[channel.RuntimeID] = make(map[domain.ServiceID]map[RuntimeChannelID]struct{})
	}
	if r.serviceIndex[channel.RuntimeID][channel.ServiceID] == nil {
		r.serviceIndex[channel.RuntimeID][channel.ServiceID] = make(map[RuntimeChannelID]struct{})
	}
	r.serviceIndex[channel.RuntimeID][channel.ServiceID][channel.ID] = struct{}{}

	if r.pluginIndex[channel.PluginID] == nil {
		r.pluginIndex[channel.PluginID] = make(map[RuntimeChannelID]struct{})
	}
	r.pluginIndex[channel.PluginID][channel.ID] = struct{}{}

	return nil
}

func (r *memoryRegistry) Unregister(ctx context.Context, id RuntimeChannelID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	return r.unregisterLocked(id)
}

func (r *memoryRegistry) unregisterLocked(id RuntimeChannelID) error {
	ch, exists := r.index[id]
	if !exists {
		return nil
	}

	delete(r.index, id)

	if ri, ok := r.runtimeIndex[ch.RuntimeID]; ok {
		delete(ri, id)
		if len(ri) == 0 {
			delete(r.runtimeIndex, ch.RuntimeID)
		}
	}

	if si, ok := r.serviceIndex[ch.RuntimeID]; ok {
		if ci, ok := si[ch.ServiceID]; ok {
			delete(ci, id)
			if len(ci) == 0 {
				delete(si, ch.ServiceID)
			}
		}
		if len(si) == 0 {
			delete(r.serviceIndex, ch.RuntimeID)
		}
	}

	if pi, ok := r.pluginIndex[ch.PluginID]; ok {
		delete(pi, id)
		if len(pi) == 0 {
			delete(r.pluginIndex, ch.PluginID)
		}
	}

	return nil
}

func (r *memoryRegistry) Get(ctx context.Context, id RuntimeChannelID) (RuntimeChannel, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeChannel{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	ch, ok := r.index[id]
	if !ok {
		return RuntimeChannel{}, ErrChannelNotFound
	}
	return ch.Clone(), nil
}

func (r *memoryRegistry) Resolve(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID, channelID domain.ChannelID) (RuntimeChannel, error) {
	if err := ctx.Err(); err != nil {
		return RuntimeChannel{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	id := NewRuntimeChannelID(runtimeID, serviceID, channelID)
	ch, ok := r.index[id]
	if !ok {
		return RuntimeChannel{}, ErrChannelNotFound
	}
	return ch.Clone(), nil
}

func (r *memoryRegistry) List() ([]RuntimeChannel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]RuntimeChannel, 0, len(r.index))
	for _, ch := range r.index {
		result = append(result, ch.Clone())
	}
	sortRuntimeChannels(result)
	return result, nil
}

func (r *memoryRegistry) ListByRuntime(runtimeID domain.RuntimeInstanceID) ([]RuntimeChannel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.listByIDsLocked(r.runtimeIndex[runtimeID]), nil
}

func (r *memoryRegistry) ListByService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) ([]RuntimeChannel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.serviceIndex[runtimeID][serviceID]
	return r.listByIDsLocked(ids), nil
}

func (r *memoryRegistry) ListByKind(runtimeID domain.RuntimeInstanceID, kind domain.ChannelKind) ([]RuntimeChannel, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ids := r.runtimeIndex[runtimeID]
	result := make([]RuntimeChannel, 0)
	for id := range ids {
		if ch, ok := r.index[id]; ok && ch.Kind == kind {
			result = append(result, ch.Clone())
		}
	}
	sortRuntimeChannels(result)
	return result, nil
}

func (r *memoryRegistry) listByIDsLocked(ids map[RuntimeChannelID]struct{}) []RuntimeChannel {
	result := make([]RuntimeChannel, 0, len(ids))
	for id := range ids {
		if ch, ok := r.index[id]; ok {
			result = append(result, ch.Clone())
		}
	}
	sortRuntimeChannels(result)
	return result
}

func (r *memoryRegistry) RemoveByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	ids := r.runtimeIndex[runtimeID]
	count := len(ids)
	for id := range ids {
		r.unregisterLocked(id)
	}
	return count, nil
}

func (r *memoryRegistry) RemoveByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	ids := r.serviceIndex[runtimeID][serviceID]
	count := len(ids)
	for id := range ids {
		r.unregisterLocked(id)
	}
	return count, nil
}

func (r *memoryRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.index)
}

func (r *memoryRegistry) CountByRuntime(runtimeID domain.RuntimeInstanceID) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.runtimeIndex[runtimeID])
}

func (r *memoryRegistry) CountByService(runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if si, ok := r.serviceIndex[runtimeID]; ok {
		return len(si[serviceID])
	}
	return 0
}

func sortRuntimeChannels(channels []RuntimeChannel) {
	sort.SliceStable(channels, func(i, j int) bool {
		ci, cj := channels[i], channels[j]
		if ci.RuntimeID != cj.RuntimeID {
			return ci.RuntimeID < cj.RuntimeID
		}
		if ci.ServiceID != cj.ServiceID {
			return ci.ServiceID < cj.ServiceID
		}
		return ci.ChannelID < cj.ChannelID
	})
}

func deepCopyRawMessage(m map[string]json.RawMessage) map[string]json.RawMessage {
	if m == nil {
		return nil
	}
	cp := make(map[string]json.RawMessage, len(m))
	for k, v := range m {
		rv := make(json.RawMessage, len(v))
		copy(rv, v)
		cp[k] = rv
	}
	return cp
}
