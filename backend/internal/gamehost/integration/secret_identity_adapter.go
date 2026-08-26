package integration

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/registry"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type SecretRuntimeIdentityReader struct {
	runtimes   *runtime.Manager
	topologies *runtime.TopologyStore
	pluginReg  *registry.Registry
}

func NewSecretRuntimeIdentityReader(
	runtimes *runtime.Manager,
	topologies *runtime.TopologyStore,
	pluginReg *registry.Registry,
) (*SecretRuntimeIdentityReader, error) {
	if runtimes == nil {
		return nil, fmt.Errorf("runtime manager is required")
	}
	if topologies == nil {
		return nil, fmt.Errorf("topology store is required")
	}
	if pluginReg == nil {
		return nil, fmt.Errorf("plugin registry is required")
	}
	return &SecretRuntimeIdentityReader{
		runtimes:   runtimes,
		topologies: topologies,
		pluginReg:  pluginReg,
	}, nil
}

func (r *SecretRuntimeIdentityReader) ResolveRuntime(
	ctx context.Context,
	runtimeID string,
) (
	pluginID string,
	extensionID string,
	state string,
	generation int64,
	err error,
) {
	if runtimeID == "" {
		return "", "", "", 0, fmt.Errorf("runtime id is required")
	}

	rt, err := r.runtimes.GetRuntime(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return "", "", "", 0, fmt.Errorf("resolve runtime %s: %w", runtimeID, err)
	}

	gen, err := r.runtimes.GetCurrentGeneration(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return "", "", "", 0, fmt.Errorf("resolve generation for runtime %s: %w", runtimeID, err)
	}

	plugin, err := r.pluginReg.Get(ctx, rt.PluginID)
	if err != nil {
		return "", "", "", 0, fmt.Errorf("resolve plugin %s: %w", rt.PluginID, err)
	}

	return string(rt.PluginID), plugin.ExtensionID, string(rt.State), gen, nil
}

func (r *SecretRuntimeIdentityReader) ResolveService(
	ctx context.Context,
	runtimeID string,
	serviceID string,
) (
	pluginID string,
	extensionID string,
	moduleID string,
	state string,
	err error,
) {
	if runtimeID == "" {
		return "", "", "", "", fmt.Errorf("runtime id is required")
	}
	if serviceID == "" {
		return "", "", "", "", fmt.Errorf("service id is required")
	}

	rt, err := r.runtimes.GetRuntime(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return "", "", "", "", fmt.Errorf("resolve runtime %s: %w", runtimeID, err)
	}

	moduleID, err = r.topologies.ResolveModuleID(domain.RuntimeInstanceID(runtimeID), domain.ServiceID(serviceID))
	if err != nil {
		return "", "", "", "", fmt.Errorf("resolve module binding for service %s in runtime %s: %w", serviceID, runtimeID, err)
	}
	if moduleID == "" {
		return "", "", "", "", fmt.Errorf("service %s has no kernel module binding", serviceID)
	}

	plugin, err := r.pluginReg.Get(ctx, rt.PluginID)
	if err != nil {
		return "", "", "", "", fmt.Errorf("resolve plugin %s: %w", rt.PluginID, err)
	}
	return string(rt.PluginID), plugin.ExtensionID, moduleID, string(rt.State), nil
}

func (r *SecretRuntimeIdentityReader) ExtensionEnabled(
	ctx context.Context,
	extensionID string,
) (bool, error) {
	if extensionID == "" {
		return false, fmt.Errorf("extension id is required")
	}
	plugins, err := r.pluginReg.ListByExtension(ctx, extensionID)
	if err != nil {
		return false, fmt.Errorf("list plugins for extension %s: %w", extensionID, err)
	}
	return len(plugins) > 0, nil
}
