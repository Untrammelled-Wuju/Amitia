package management

import (
	"context"
	"fmt"

	kerneldomain "github.com/u-ai/backend/internal/extension/kernel/domain"
	"github.com/u-ai/backend/internal/gamehost"
	"github.com/u-ai/backend/internal/gamehost/control"
	dom "github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/registry"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type KernelAdapter struct {
	container *gamehost.GameHostContainer
}

func NewKernelAdapter(container *gamehost.GameHostContainer) *KernelAdapter {
	return &KernelAdapter{container: container}
}

func (a *KernelAdapter) ListGameCenterExtensions(ctx context.Context) ([]kerneldomain.ExtensionDefinition, []kerneldomain.ExtensionInstallation, error) {
	return nil, nil, nil
}

func (a *KernelAdapter) GetGameCenterExtension(ctx context.Context, extensionID string) (*kerneldomain.ExtensionDefinition, *kerneldomain.ExtensionInstallation, error) {
	return nil, nil, nil
}

type GameHostPluginRegistry struct {
	registry *registry.Registry
}

func NewGameHostPluginRegistry(reg *registry.Registry) *GameHostPluginRegistry {
	return &GameHostPluginRegistry{registry: reg}
}

func (r *GameHostPluginRegistry) Get(ctx context.Context, pluginID string) (dom.PluginDescriptor, error) {
	return r.registry.Get(ctx, dom.PluginID(pluginID))
}

func (r *GameHostPluginRegistry) ListByExtension(ctx context.Context, extensionID string) ([]dom.PluginDescriptor, error) {
	return r.registry.ListByExtension(ctx, extensionID)
}

func (r *GameHostPluginRegistry) List(ctx context.Context) ([]dom.PluginDescriptor, error) {
	return r.registry.List(ctx)
}

type GameHostRuntimeManager struct {
	manager runtime.RuntimeManager
}

func NewGameHostRuntimeManager(manager runtime.RuntimeManager) *GameHostRuntimeManager {
	return &GameHostRuntimeManager{manager: manager}
}

func (m *GameHostRuntimeManager) ListRuntimes() []*runtime.RuntimeInstanceRef {
	return m.manager.ListRuntimes()
}

func (m *GameHostRuntimeManager) GetRuntime(runtimeID string) (*runtime.RuntimeInstanceRef, error) {
	return m.manager.GetRuntime(dom.RuntimeInstanceID(runtimeID))
}

type GameHostRuntimeManagerAdapter struct {
	manager runtime.RuntimeManager
}

func NewGameHostRuntimeManagerAdapter(manager runtime.RuntimeManager) *GameHostRuntimeManagerAdapter {
	return &GameHostRuntimeManagerAdapter{manager: manager}
}

func (a *GameHostRuntimeManagerAdapter) ListRuntimes() []*runtime.RuntimeInstanceRef {
	if a.manager == nil {
		return nil
	}
	return a.manager.ListRuntimes()
}

func (a *GameHostRuntimeManagerAdapter) GetRuntime(runtimeID string) (*runtime.RuntimeInstanceRef, error) {
	if a.manager == nil {
		return nil, fmt.Errorf("runtime manager not available")
	}
	return a.manager.GetRuntime(dom.RuntimeInstanceID(runtimeID))
}

type GameHostTopologyStore struct {
	store runtime.RuntimeTopologyStore
}

func NewGameHostTopologyStore(store runtime.RuntimeTopologyStore) *GameHostTopologyStore {
	return &GameHostTopologyStore{store: store}
}

func (t *GameHostTopologyStore) GetTopologySnapshot(runtimeID string) (runtime.RuntimeTopologySnapshot, error) {
	return t.store.GetTopologySnapshot(dom.RuntimeInstanceID(runtimeID))
}

type GameHostHandshakeManager struct {
	manager *handshake.HandshakeManager
}

func NewGameHostHandshakeManager(manager *handshake.HandshakeManager) *GameHostHandshakeManager {
	return &GameHostHandshakeManager{manager: manager}
}

func (h *GameHostHandshakeManager) GetState(connectionID string) (handshake.HandshakeState, bool) {
	return h.manager.GetState(connectionID)
}

func (h *GameHostHandshakeManager) GetSnapshot(connectionID string) *handshake.HandshakeSnapshot {
	snap := h.manager.GetSnapshot(connectionID)
	return snap
}

func (h *GameHostHandshakeManager) IsReady(connectionID string) bool {
	return h.manager.IsReady(connectionID)
}

type GameHostAuthorityManager struct {
	manager *control.ControlAuthorityManager
}

func NewGameHostAuthorityManager(manager *control.ControlAuthorityManager) *GameHostAuthorityManager {
	return &GameHostAuthorityManager{manager: manager}
}

func (m *GameHostAuthorityManager) GetSnapshot(ctx context.Context, runtimeID string) (control.ControlAuthoritySnapshot, bool) {
	snap, err := m.manager.GetSnapshot(ctx, dom.RuntimeInstanceID(runtimeID))
	if err != nil {
		return control.ControlAuthoritySnapshot{}, false
	}
	return snap, true
}

func (m *GameHostAuthorityManager) List(ctx context.Context) ([]control.ControlAuthoritySnapshot, error) {
	return m.manager.List(ctx)
}

type GameHostHealthAdapter struct {
	adapter runtime.HealthAdapter
}

func NewGameHostHealthAdapter(adapter runtime.HealthAdapter) *GameHostHealthAdapter {
	return &GameHostHealthAdapter{adapter: adapter}
}

func (h *GameHostHealthAdapter) GetServiceHealth(runtimeID string, serviceID string) (runtime.ServiceHealthSnapshot, bool) {
	return h.adapter.GetServiceHealth(dom.RuntimeInstanceID(runtimeID), dom.ServiceID(serviceID))
}

func (h *GameHostHealthAdapter) ListServiceHealth(runtimeID string) []runtime.ServiceHealthSnapshot {
	return h.adapter.ListServiceHealth(dom.RuntimeInstanceID(runtimeID))
}
