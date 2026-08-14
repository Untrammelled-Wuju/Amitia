package integration

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/registry"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type HostAPIRuntimeAdapter struct {
	manager *runtime.Manager
	plugins *registry.Registry
}

func NewHostAPIRuntimeAdapter(manager *runtime.Manager, plugins *registry.Registry) *HostAPIRuntimeAdapter {
	return &HostAPIRuntimeAdapter{manager: manager, plugins: plugins}
}

func (a *HostAPIRuntimeAdapter) RuntimeExists(ctx context.Context, runtimeID string) (bool, error) {
	if a.manager == nil {
		return false, fmt.Errorf("hostapi: runtime store not configured")
	}
	_, err := a.manager.Get(ctx, domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return false, nil
	}
	return true, nil
}

func (a *HostAPIRuntimeAdapter) RuntimeOwnedBy(ctx context.Context, runtimeID string, pluginID string) (bool, error) {
	if a.manager == nil {
		return false, fmt.Errorf("hostapi: runtime store not configured")
	}
	inst, err := a.manager.Get(ctx, domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return false, err
	}
	return inst.PluginID == domain.PluginID(pluginID), nil
}

func (a *HostAPIRuntimeAdapter) PluginExists(ctx context.Context, pluginID string) (bool, error) {
	if a.plugins == nil {
		return false, fmt.Errorf("hostapi: plugin registry not configured")
	}
	_, err := a.plugins.Get(ctx, domain.PluginID(pluginID))
	return err == nil, nil
}

type HostAPIGenerationReader struct {
	manager *runtime.Manager
}

func NewHostAPIGenerationReader(manager *runtime.Manager) *HostAPIGenerationReader {
	return &HostAPIGenerationReader{manager: manager}
}

func (r *HostAPIGenerationReader) CurrentGeneration(ctx context.Context, runtimeID domain.RuntimeInstanceID) (uint64, error) {
	gen, err := r.manager.GetCurrentGeneration(runtimeID)
	if err != nil {
		return 0, err
	}
	return uint64(gen), nil
}

func (r *HostAPIGenerationReader) AllocateGeneration(ctx context.Context, runtimeID domain.RuntimeInstanceID) (uint64, error) {
	gen, err := r.manager.AllocateGeneration(runtimeID)
	if err != nil {
		return 0, err
	}
	return uint64(gen), nil
}

type HostAPITopologyAdapter struct {
	topology *runtime.TopologyStore
}

func NewHostAPITopologyAdapter(topology *runtime.TopologyStore) *HostAPITopologyAdapter {
	return &HostAPITopologyAdapter{topology: topology}
}

func (a *HostAPITopologyAdapter) ServiceBelongsToRuntime(ctx context.Context, runtimeID string, serviceID string) error {
	snap, err := a.topology.GetTopologySnapshot(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return fmt.Errorf("get topology: %w", err)
	}
	for _, svc := range snap.Services {
		if string(svc.ServiceID) == serviceID {
			return nil
		}
	}
	return fmt.Errorf("service %q not found in runtime %q topology", serviceID, runtimeID)
}
