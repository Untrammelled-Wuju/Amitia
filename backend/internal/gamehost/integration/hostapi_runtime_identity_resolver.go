package integration

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/hostapi"
	"github.com/u-ai/backend/internal/gamehost/registry"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type HostAPIRuntimeIdentityResolver struct {
	manager  *runtime.Manager
	plugins  *registry.Registry
	topology *runtime.TopologyStore
}

func NewHostAPIRuntimeIdentityResolver(
	manager *runtime.Manager,
	plugins *registry.Registry,
	topology *runtime.TopologyStore,
) *HostAPIRuntimeIdentityResolver {
	return &HostAPIRuntimeIdentityResolver{
		manager:  manager,
		plugins:  plugins,
		topology: topology,
	}
}

func (r *HostAPIRuntimeIdentityResolver) RuntimeInfo(
	ctx context.Context,
	runtimeID string,
	serviceID string,
) (extensionID string, moduleID string, runtimeType string, generation int64, err error) {
	if r.manager == nil {
		return "", "", "", 0, fmt.Errorf("hostapi resolver: runtime manager not configured")
	}
	if r.plugins == nil {
		return "", "", "", 0, fmt.Errorf("hostapi resolver: plugin registry not configured")
	}

	inst, err := r.manager.Get(ctx, domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return "", "", "", 0, fmt.Errorf("resolve runtime: %w", err)
	}

	plugin, err := r.plugins.Get(ctx, inst.PluginID)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("resolve plugin: %w", err)
	}

	gen, err := r.manager.GetCurrentGeneration(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return "", "", "", 0, fmt.Errorf("resolve generation: %w", err)
	}

	modID, err := r.resolveModuleID(ctx, runtimeID, serviceID)
	if err != nil {
		return "", "", "", 0, err
	}

	return plugin.ExtensionID, modID, "gamehost", gen, nil
}

func (r *HostAPIRuntimeIdentityResolver) resolveModuleID(ctx context.Context, runtimeID string, serviceID string) (string, error) {
	if r.topology == nil {
		return "", fmt.Errorf("hostapi resolver: topology store not configured")
	}
	if serviceID == "" {
		return "", fmt.Errorf("hostapi resolver: service id is required")
	}

	moduleID, err := r.topology.ResolveModuleID(domain.RuntimeInstanceID(runtimeID), domain.ServiceID(serviceID))
	if err != nil {
		return "", fmt.Errorf("resolve module binding: %w", err)
	}
	if moduleID == "" {
		return "", fmt.Errorf("hostapi resolver: empty module binding for service %q", serviceID)
	}
	return moduleID, nil
}

var _ hostapi.RuntimeResolver = (*HostAPIRuntimeIdentityResolver)(nil)
