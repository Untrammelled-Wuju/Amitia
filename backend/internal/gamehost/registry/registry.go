// Package registry provides the GameHost runtime projection of enabled game_plugin contributions.
//
// IMPORTANT: This registry is a DOMAIN PROJECTION, not an authoritative registry.
//
// It is NOT:
//   - The authoritative Extension installation/enable state (owned by Extension Kernel)
//   - The authoritative Capability Provider Registry (owned by capability.ProviderRegistry)
//   - The authoritative Device Registry (owned by host_registry.Registry)
//
// This registry reflects the current set of enabled game_plugin contributions as known
// to the GameHost runtime. Management mutations (install/uninstall/enable/disable) must
// go through the Extension Kernel; this registry only syncs from those upstream changes.
//
// The flow is strictly ONE-WAY:
//
//	Extension Kernel (authoritative) -> GamePluginSyncService -> GameHost Registry (projection)
package registry

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

// Registry is the GameHost runtime projection of enabled game_plugin contributions.
// It provides a local cache of plugin descriptors that GameHost has synced from the Extension Kernel.
//
// This is NOT the Extension Registry itself. Extension installation/enabled state
// remains owned by the Extension Kernel.
type Registry struct {
	mu sync.RWMutex

	plugins     map[domain.PluginID]domain.PluginDescriptor
	byExtension map[string]map[domain.PluginID]struct{}
}

// NewRegistry creates a new GameHost projection registry.
// This is NOT the authoritative Extension Registry. Extensions must be installed
// through the Extension Kernel; this registry only maintains the GameHost-side projection.
func NewRegistry() *Registry {
	return &Registry{
		plugins:     make(map[domain.PluginID]domain.PluginDescriptor),
		byExtension: make(map[string]map[domain.PluginID]struct{}),
	}
}

// Register adds a plugin descriptor to the GameHost projection.
// This is a projection-local operation and does NOT install the extension.
// Extension installation must go through the Extension Kernel.
func (r *Registry) Register(ctx context.Context, descriptor domain.PluginDescriptor) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if err := descriptor.Validate(); err != nil {
		return domain.NewHostErrorWithCause(domain.ErrInvalidArgument, "descriptor validation failed", err)
	}

	cloned := descriptor.Clone()

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.plugins[descriptor.ID]; exists {
		return domain.NewHostErrorWithCause(domain.ErrAlreadyExists, "plugin already registered",
			domain.NewHostError(domain.ErrAlreadyExists, string(descriptor.ID)))
	}

	r.plugins[cloned.ID] = cloned

	if cloned.ExtensionID != "" {
		if r.byExtension[cloned.ExtensionID] == nil {
			r.byExtension[cloned.ExtensionID] = make(map[domain.PluginID]struct{})
		}
		r.byExtension[cloned.ExtensionID][cloned.ID] = struct{}{}
	}

	return nil
}

func (r *Registry) Unregister(ctx context.Context, pluginID domain.PluginID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	plugin, exists := r.plugins[pluginID]
	if !exists {
		return domain.NewHostErrorWithCause(domain.ErrNotFound, "plugin not found",
			domain.NewHostError(domain.ErrNotFound, string(pluginID)))
	}

	if plugin.ExtensionID != "" {
		if extPlugins, ok := r.byExtension[plugin.ExtensionID]; ok {
			delete(extPlugins, pluginID)
			if len(extPlugins) == 0 {
				delete(r.byExtension, plugin.ExtensionID)
			}
		}
	}

	delete(r.plugins, pluginID)
	return nil
}

func (r *Registry) Get(ctx context.Context, pluginID domain.PluginID) (domain.PluginDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return domain.PluginDescriptor{}, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	plugin, exists := r.plugins[pluginID]
	if !exists {
		return domain.PluginDescriptor{}, domain.NewHostErrorWithCause(domain.ErrNotFound, "plugin not found",
			domain.NewHostError(domain.ErrNotFound, string(pluginID)))
	}

	return plugin.Clone(), nil
}

func (r *Registry) List(ctx context.Context) ([]domain.PluginDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.PluginDescriptor, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		result = append(result, plugin.Clone())
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func (r *Registry) Exists(ctx context.Context, pluginID domain.PluginID) bool {
	if err := ctx.Err(); err != nil {
		return false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	_, exists := r.plugins[pluginID]
	return exists
}

func (r *Registry) ListByExtension(ctx context.Context, extensionID string) ([]domain.PluginDescriptor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if extensionID == "" {
		return nil, domain.NewHostError(domain.ErrInvalidArgument, "extension id must not be empty")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	pluginIDs, ok := r.byExtension[extensionID]
	if !ok || len(pluginIDs) == 0 {
		return []domain.PluginDescriptor{}, nil
	}

	result := make([]domain.PluginDescriptor, 0, len(pluginIDs))
	for id := range pluginIDs {
		if plugin, exists := r.plugins[id]; exists {
			result = append(result, plugin.Clone())
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result, nil
}

func (r *Registry) Snapshot() []domain.PluginDescriptor {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]domain.PluginDescriptor, 0, len(r.plugins))
	for _, plugin := range r.plugins {
		result = append(result, plugin.Clone())
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	return result
}

func (r *Registry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.plugins)
}

func (r *Registry) DescriptorCapabilities(pluginID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[domain.PluginID(pluginID)]
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}
	caps := make([]string, 0, len(p.Capabilities))
	for _, c := range p.Capabilities {
		caps = append(caps, string(c))
	}
	return caps, nil
}

func (r *Registry) DescriptorChannels(pluginID string) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[domain.PluginID(pluginID)]
	if !ok {
		return nil, fmt.Errorf("plugin not found: %s", pluginID)
	}
	channels := make([]string, 0, len(p.Channels))
	for _, ch := range p.Channels {
		channels = append(channels, string(ch.ID))
	}
	return channels, nil
}

func (r *Registry) HasCapability(pluginID, capability string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.plugins[domain.PluginID(pluginID)]
	if !ok {
		return false
	}
	for _, c := range p.Capabilities {
		if string(c) == capability {
			return true
		}
	}
	return false
}
