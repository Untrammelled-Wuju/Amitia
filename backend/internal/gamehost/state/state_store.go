package state

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

const (
	DefaultMaxStateKeysPerRuntime = 4096
	DefaultMaxStatePayloadBytes   = 1 << 20
)

type Options struct {
	MaxStateKeysPerRuntime int
	MaxStatePayloadBytes   int64
	Clock                  func() time.Time
}

type LatestStateStore struct {
	mu               sync.RWMutex
	index            map[string]*stateEntry
	runtimeIndex     map[domain.RuntimeInstanceID]map[string]struct{}
	serviceIndex     map[domain.RuntimeInstanceID]map[domain.ServiceID]map[string]struct{}
	pluginIndex      map[domain.PluginID]map[string]*struct{}
	opts             Options
	globalVersion    atomic.Uint64
}

type stateEntry struct {
	snapshot StateSnapshot
}

func NewOptions() Options {
	return Options{
		MaxStateKeysPerRuntime: DefaultMaxStateKeysPerRuntime,
		MaxStatePayloadBytes:   DefaultMaxStatePayloadBytes,
		Clock:                  func() time.Time { return time.Now().UTC() },
	}
}

func NewLatestStateStore(opts Options) *LatestStateStore {
	if opts.MaxStateKeysPerRuntime <= 0 {
		opts.MaxStateKeysPerRuntime = DefaultMaxStateKeysPerRuntime
	}
	if opts.MaxStatePayloadBytes <= 0 {
		opts.MaxStatePayloadBytes = DefaultMaxStatePayloadBytes
	}
	if opts.Clock == nil {
		opts.Clock = func() time.Time { return time.Now().UTC() }
	}
	return &LatestStateStore{
		index:        make(map[string]*stateEntry),
		runtimeIndex: make(map[domain.RuntimeInstanceID]map[string]struct{}),
		serviceIndex: make(map[domain.RuntimeInstanceID]map[domain.ServiceID]map[string]struct{}),
		pluginIndex:  make(map[domain.PluginID]map[string]*struct{}),
		opts:         opts,
	}
}

func (s *LatestStateStore) Put(ctx context.Context, update StateUpdate) (StateSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return StateSnapshot{}, err
	}
	if err := s.validateUpdate(update); err != nil {
		return StateSnapshot{}, err
	}

	key := s.buildIndexKey(update.RuntimeID, update.ServiceID, update.Key)
	payloadCopy := deepCopyRaw(update.Payload)
	metadataCopy := deepCopyMetadata(update.Metadata)

	s.mu.Lock()
	defer s.mu.Unlock()

	existing, exists := s.index[key]
	runtimeKeys := s.runtimeIndex[update.RuntimeID]
	if !exists {
		if len(runtimeKeys) >= s.opts.MaxStateKeysPerRuntime {
			return StateSnapshot{}, domain.NewHostError(domain.ErrResourceExhausted, "state: max state keys per runtime reached")
		}
	}

	now := s.opts.Clock()
	var newVersion uint64
	if exists {
		newVersion = existing.snapshot.Version + 1
	} else {
		newVersion = s.globalVersion.Add(1)
		runtimeKeys = s.lazyRuntimeLocked(update.RuntimeID)
		serviceKeys := s.lerviceLockedLocked(update.RuntimeID, update.ServiceID)
		runtimeKeys[key] = struct{}{}
		serviceKeys[key] = struct{}{}
		s.pluginIndex[update.PluginID][key] = nil
	}

	snapshot := StateSnapshot{
		PluginID:        update.PluginID,
		RuntimeID:       update.RuntimeID,
		ServiceID:       update.ServiceID,
		Key:             update.Key,
		Payload:         payloadCopy,
		Metadata:        metadataCopy,
		SourceMessageID: update.ID,
		Version:         newVersion,
		UpdatedAt:       now,
	}
	s.index[key] = &stateEntry{snapshot: snapshot}
	return snapshot, nil
}

func (s *LatestStateStore) Get(ctx context.Context, key StateKey) (StateSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return StateSnapshot{}, err
	}
	if err := key.Validate(); err != nil {
		return StateSnapshot{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	idx := s.buildIndexKey(key.RuntimeID, key.ServiceID, key.Key)
	entry, ok := s.index[idx]
	if !ok {
		return StateSnapshot{}, domain.NewHostError(domain.ErrNotFound, "state: key not found")
	}
	return cloneSnapshot(entry.snapshot), nil
}

func (s *LatestStateStore) List(ctx context.Context, filter StateFilter) ([]StateSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]StateSnapshot, 0)
	for _, entry := range s.index {
		if filter.Match(entry.snapshot) {
			results = append(results, cloneSnapshot(entry.snapshot))
		}
	}
	sort.Slice(results, func(i, j int) bool {
		a, b := results[i], results[j]
		if a.RuntimeID != b.RuntimeID {
			return a.RuntimeID < b.RuntimeID
		}
		if a.ServiceID != b.ServiceID {
			return a.ServiceID < b.ServiceID
		}
		return a.Key < b.Key
	})
	return results, nil
}

func (s *LatestStateStore) Remove(ctx context.Context, key StateKey) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := key.Validate(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	idx := s.buildIndexKey(key.RuntimeID, key.ServiceID, key.Key)
	if _, ok := s.index[idx]; !ok {
		return nil
	}
	delete(s.index, idx)
	if rtKeys, ok := s.runtimeIndex[key.RuntimeID]; ok {
		delete(rtKeys, idx)
		if len(rtKeys) == 0 {
			delete(s.runtimeIndex, key.RuntimeID)
		}
	}
	if svcKeys, ok := s.serviceIndex[key.RuntimeID]; ok {
		if keys, ok := svcKeys[key.ServiceID]; ok {
			delete(keys, idx)
			if len(keys) == 0 {
				delete(svcKeys, key.ServiceID)
				if len(svcKeys) == 0 {
					delete(s.serviceIndex, key.RuntimeID)
				}
			}
		}
	}
	if pKeys, ok := s.pluginIndex[key.PluginID]; ok {
		delete(pKeys, idx)
		if len(pKeys) == 0 {
			delete(s.pluginIndex, key.PluginID)
		}
	}
	return nil
}

func (s *LatestStateStore) RemoveByService(ctx context.Context, runtimeID domain.RuntimeInstanceID, serviceID domain.ServiceID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	svcMap, ok := s.serviceIndex[runtimeID]
	if !ok {
		return nil
	}
	keys, ok := svcMap[serviceID]
	if !ok {
		return nil
	}
	for idx := range keys {
		if entry, ok := s.index[idx]; ok {
			delete(s.index, idx)
			if rtKeys, ok := s.runtimeIndex[runtimeID]; ok {
				delete(rtKeys, idx)
				if len(rtKeys) == 0 {
					delete(s.runtimeIndex, runtimeID)
				}
			}
			if pKeys, ok := s.pluginIndex[entry.snapshot.PluginID]; ok {
				delete(pKeys, idx)
				if len(pKeys) == 0 {
					delete(s.pluginIndex, entry.snapshot.PluginID)
				}
			}
		}
	}
	delete(svcMap, serviceID)
	if len(svcMap) == 0 {
		delete(s.serviceIndex, runtimeID)
	}
	return nil
}

func (s *LatestStateStore) RemoveByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	rtKeys, ok := s.runtimeIndex[runtimeID]
	if !ok {
		return nil
	}
	for idx := range rtKeys {
		if entry, ok := s.index[idx]; ok {
			delete(s.index, idx)
			if pKeys, ok := s.pluginIndex[entry.snapshot.PluginID]; ok {
				delete(pKeys, idx)
				if len(pKeys) == 0 {
					delete(s.pluginIndex, entry.snapshot.PluginID)
				}
			}
		}
	}
	delete(s.runtimeIndex, runtimeID)
	if svcMap, ok := s.serviceIndex[runtimeID]; ok {
		for svcID, keys := range svcMap {
			for idx := range keys {
				delete(s.index, idx)
			}
			delete(svcMap, svcID)
		}
		delete(s.serviceIndex, runtimeID)
	}
	return nil
}

func (s *LatestStateStore) RemoveByPlugin(ctx context.Context, pluginID domain.PluginID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	pKeys, ok := s.pluginIndex[pluginID]
	if !ok {
		return nil
	}
	for idx := range pKeys {
		if entry, ok := s.index[idx]; ok {
			delete(s.index, idx)
			if rtKeys, ok := s.runtimeIndex[entry.snapshot.RuntimeID]; ok {
				delete(rtKeys, idx)
				if len(rtKeys) == 0 {
					delete(s.runtimeIndex, entry.snapshot.RuntimeID)
				}
			}
			if svcMap, ok := s.serviceIndex[entry.snapshot.RuntimeID]; ok {
				if svcKeys, ok := svcMap[entry.snapshot.ServiceID]; ok {
					delete(svcKeys, idx)
					if len(svcKeys) == 0 {
						delete(svcMap, entry.snapshot.ServiceID)
						if len(svcMap) == 0 {
							delete(s.serviceIndex, entry.snapshot.RuntimeID)
						}
					}
				}
			}
		}
	}
	delete(s.pluginIndex, pluginID)
	return nil
}

func (s *LatestStateStore) Count(ctx context.Context) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.index)
}

func (s *LatestStateStore) CountByRuntime(ctx context.Context, runtimeID domain.RuntimeInstanceID) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.runtimeIndex[runtimeID])
}

func (s *LatestStateStore) validateUpdate(update StateUpdate) error {
	key := NewStateKey(update.PluginID, update.RuntimeID, update.ServiceID, update.Key)
	if err := key.Validate(); err != nil {
		return err
	}
	if int64(len(update.Payload)) > s.opts.MaxStatePayloadBytes {
		return domain.NewHostError(domain.ErrInvalidArgument, "state: payload exceeds maximum size")
	}
	return nil
}

func (s *LatestStateStore) buildIndexKey(runtime domain.RuntimeInstanceID, service domain.ServiceID, key string) string {
	return string(runtime) + "\x00" + string(service) + "\x00" + key
}

func (s *LatestStateStore) lazyRuntimeLocked(runtime domain.RuntimeInstanceID) map[string]struct{} {
	if m, ok := s.runtimeIndex[runtime]; ok {
		return m
	}
	m := make(map[string]struct{})
	s.runtimeIndex[runtime] = m
	return m
}

func (s *LatestStateStore) lerviceLockedLocked(runtime domain.RuntimeInstanceID, service domain.ServiceID) map[string]struct{} {
	m, ok := s.serviceIndex[runtime]
	if !ok {
		m = make(map[domain.ServiceID]map[string]struct{})
		s.serviceIndex[runtime] = m
	}
	if keys, ok := m[service]; ok {
		return keys
	}
	keys := make(map[string]struct{})
	m[service] = keys
	return keys
}

func cloneSnapshot(src StateSnapshot) StateSnapshot {
	dst := src
	dst.Payload = deepCopyRaw(src.Payload)
	dst.Metadata = deepCopyMetadata(src.Metadata)
	return dst
}

func deepCopyRaw(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	dst := make(json.RawMessage, len(src))
	copy(dst, src)
	return dst
}

func deepCopyMetadata(src map[string]json.RawMessage) map[string]json.RawMessage {
	if src == nil {
		return nil
	}
	dst := make(map[string]json.RawMessage, len(src))
	for k, v := range src {
		dst[k] = deepCopyRaw(v)
	}
	return dst
}

var _ = errors.New
