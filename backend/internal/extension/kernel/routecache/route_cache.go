//go:build rsi_routecache

package routecache

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/u-ai/backend/internal/extension/kernel/plugincontract"
)

var (
	ErrRouteCacheLoaderMissing   = errors.New("routecache: graph loader missing")
	ErrRouteCacheSnapshotMissing = errors.New("routecache: binding snapshot missing")
	ErrRouteCacheRevisionStale   = errors.New("routecache: binding snapshot revision is no longer served")
)

type GraphLoader func() (plugincontract.ServiceGraphResolution, error)

type Stats struct {
	Revision     uint64
	GraphLoads   int
	Hits         int
	Misses       int
	Invalidated  int
	Evicted      int
	Computations int
	Entries      int
}

type generation struct {
	revision uint64
	graph    plugincontract.ServiceGraphResolution
	loaded   bool
}

type cacheEntry struct {
	revision uint64
	value    plugincontract.ProviderCandidates
}

type Cache struct {
	mu       sync.Mutex
	loader   GraphLoader
	current  generation
	previous generation
	history  bool
	entries  map[string]cacheEntry
	stats    Stats
}

func New(loader GraphLoader) *Cache {
	return &Cache{loader: loader, entries: make(map[string]cacheEntry)}
}

func (c *Cache) Stats() Stats {
	if c == nil {
		return Stats{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.snapshotStatsLocked()
}

func (c *Cache) Lookup(
	snapshot *plugincontract.BindingSnapshot,
	request plugincontract.BindingRequest,
	serviceID string,
) (plugincontract.ProviderCandidates, error) {
	if c == nil {
		return plugincontract.ProviderCandidates{}, ErrRouteCacheLoaderMissing
	}
	if snapshot == nil {
		return plugincontract.ProviderCandidates{}, ErrRouteCacheSnapshotMissing
	}
	service := strings.TrimSpace(serviceID)
	if service == "" {
		return plugincontract.ProviderCandidates{}, &plugincontract.BindingInvalidError{
			Field:  "serviceId",
			Value:  serviceID,
			Reason: "must not be empty",
		}
	}
	revision := snapshot.Revision()
	key := cacheKey(revision, request, service)

	c.mu.Lock()
	defer c.mu.Unlock()
	graph, err := c.graphForRevisionLocked(revision)
	if err != nil {
		return plugincontract.ProviderCandidates{}, err
	}
	if cached, ok := c.entries[key]; ok {
		c.stats.Hits++
		return cloneCandidates(cached.value), nil
	}
	c.stats.Misses++
	c.stats.Computations++
	candidates, err := plugincontract.SelectProviderCandidates(graph, snapshot, request, service)
	if err != nil {
		return plugincontract.ProviderCandidates{}, err
	}
	stored := cloneCandidates(candidates)
	c.entries[key] = cacheEntry{revision: revision, value: stored}
	return cloneCandidates(stored), nil
}

func (c *Cache) Apply(
	previous []plugincontract.ProviderBinding,
	next []plugincontract.ProviderBinding,
	snapshot *plugincontract.BindingSnapshot,
	resolution plugincontract.ServiceGraphResolution,
) Stats {
	if c == nil {
		return Stats{}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if snapshot == nil {
		evicted := len(c.entries)
		c.entries = make(map[string]cacheEntry)
		c.stats.Evicted += evicted
		return c.snapshotStatsLocked()
	}
	revision := snapshot.Revision()
	if c.current.loaded && revision < c.current.revision {
		return c.snapshotStatsLocked()
	}
	if c.current.loaded {
		c.previous = c.current
		c.history = true
	}
	c.current = generation{revision: revision, graph: resolution, loaded: true}
	c.stats.GraphLoads++
	invalidated := 0
	if resolution.Valid() {
		diff := plugincontract.DiffProviderBindings(previous, next)
		affected := affectedServices(diff)
		carried := make(map[string]cacheEntry, len(c.entries))
		for key, cached := range c.entries {
			if cached.revision != c.previous.revision {
				carried[key] = cached
				continue
			}
			service := keyService(key)
			if _, hit := affected[service]; hit {
				invalidated++
				continue
			}
			carried[cacheKey(revision, requestFromKey(key), service)] = cacheEntry{revision: revision, value: cached.value}
		}
		c.entries = carried
	} else {
		invalidated = len(c.entries)
		c.entries = make(map[string]cacheEntry)
	}
	c.stats.Invalidated += invalidated
	c.pruneLocked()
	return c.snapshotStatsLocked()
}

func (c *Cache) Reset() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.stats.Evicted += len(c.entries)
	c.entries = make(map[string]cacheEntry)
	c.current = generation{}
	c.previous = generation{}
	c.history = false
}

func (c *Cache) graphForRevisionLocked(revision uint64) (plugincontract.ServiceGraphResolution, error) {
	if c.current.loaded && c.current.revision == revision {
		return c.current.graph, nil
	}
	if c.history && c.previous.loaded && c.previous.revision == revision {
		return c.previous.graph, nil
	}
	if c.history && revision < c.current.revision {
		return plugincontract.ServiceGraphResolution{}, fmt.Errorf(
			"%w: request revision %d current revision %d previous revision %d",
			ErrRouteCacheRevisionStale, revision, c.current.revision, c.previous.revision,
		)
	}
	if c.loader == nil {
		return plugincontract.ServiceGraphResolution{}, ErrRouteCacheLoaderMissing
	}
	resolution, err := c.loader()
	if err != nil {
		return plugincontract.ServiceGraphResolution{}, fmt.Errorf("routecache: load service graph: %w", err)
	}
	if c.current.loaded {
		c.previous = c.current
		c.history = true
	}
	c.current = generation{revision: revision, graph: resolution, loaded: true}
	c.stats.GraphLoads++
	c.pruneLocked()
	return resolution, nil
}

func (c *Cache) pruneLocked() {
	floor := c.current.revision
	if c.history {
		floor = c.previous.revision
	}
	for key, cached := range c.entries {
		if cached.revision < floor {
			delete(c.entries, key)
			c.stats.Evicted++
		}
	}
}

func (c *Cache) snapshotStatsLocked() Stats {
	stats := c.stats
	stats.Revision = c.current.revision
	stats.Entries = len(c.entries)
	return stats
}

func affectedServices(diff plugincontract.ProviderBindingDiff) map[string]struct{} {
	affected := make(map[string]struct{}, diff.Touched())
	for _, binding := range diff.Added {
		affected[binding.ServiceID] = struct{}{}
	}
	for _, binding := range diff.Removed {
		affected[binding.ServiceID] = struct{}{}
	}
	for _, change := range diff.Changed {
		affected[change.Binding.ServiceID] = struct{}{}
	}
	return affected
}

func cacheKey(revision uint64, request plugincontract.BindingRequest, serviceID string) string {
	return fmt.Sprintf("%d\x00%s\x00%s\x00%s\x00%s",
		revision,
		strings.TrimSpace(request.CapabilityID),
		strings.TrimSpace(request.SpaceID),
		strings.TrimSpace(request.DeviceID),
		serviceID,
	)
}

func keyService(key string) string {
	index := strings.LastIndex(key, "\x00")
	if index < 0 {
		return ""
	}
	return key[index+1:]
}

func requestFromKey(key string) plugincontract.BindingRequest {
	parts := strings.Split(key, "\x00")
	if len(parts) != 5 {
		return plugincontract.BindingRequest{}
	}
	return plugincontract.BindingRequest{CapabilityID: parts[1], SpaceID: parts[2], DeviceID: parts[3]}
}

func cloneCandidates(candidates plugincontract.ProviderCandidates) plugincontract.ProviderCandidates {
	cloned := candidates
	cloned.Candidates = cloneDescriptors(candidates.Candidates)
	cloned.Selected = cloneDescriptor(candidates.Selected)
	cloned.Trail = append([]string(nil), candidates.Trail...)
	return cloned
}

func cloneDescriptors(descriptors []plugincontract.ServiceDescriptor) []plugincontract.ServiceDescriptor {
	if descriptors == nil {
		return nil
	}
	cloned := make([]plugincontract.ServiceDescriptor, len(descriptors))
	for index, descriptor := range descriptors {
		cloned[index] = cloneDescriptor(descriptor)
	}
	return cloned
}

func cloneDescriptor(descriptor plugincontract.ServiceDescriptor) plugincontract.ServiceDescriptor {
	cloned := descriptor
	cloned.Scopes = append([]plugincontract.ServiceScope(nil), descriptor.Scopes...)
	return cloned
}
