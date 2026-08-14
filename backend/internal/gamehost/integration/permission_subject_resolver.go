package integration

import (
	"context"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/permission"
	"github.com/u-ai/backend/internal/gamehost/registry"
	"github.com/u-ai/backend/internal/gamehost/runtime"
)

type GameHostPermissionSubjectResolver struct {
	runtimeManager *runtime.Manager
	pluginRegistry *registry.Registry
}

func NewGameHostPermissionSubjectResolver(
	runtimeManager *runtime.Manager,
	pluginRegistry *registry.Registry,
) *GameHostPermissionSubjectResolver {
	return &GameHostPermissionSubjectResolver{
		runtimeManager: runtimeManager,
		pluginRegistry: pluginRegistry,
	}
}

func (r *GameHostPermissionSubjectResolver) ResolveExtensionID(pluginID string) (string, bool) {
	if pluginID == "" {
		return "", false
	}
	ctx := context.Background()
	plugin, err := r.pluginRegistry.Get(ctx, domain.PluginID(pluginID))
	if err != nil {
		return "", false
	}
	return plugin.ExtensionID, true
}

func (r *GameHostPermissionSubjectResolver) RuntimeExists(runtimeID string) (string, domain.RuntimeState, error) {
	if runtimeID == "" {
		return "", "", fmt.Errorf("runtime id is required")
	}
	rt, err := r.runtimeManager.GetRuntime(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return "", "", err
	}
	return string(rt.PluginID), rt.State, nil
}

func (r *GameHostPermissionSubjectResolver) ServiceExists(runtimeID string, serviceID string) (string, error) {
	if runtimeID == "" || serviceID == "" {
		return "", fmt.Errorf("runtime id and service id are required")
	}
	rt, err := r.runtimeManager.GetRuntime(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return "", err
	}
	return string(rt.PluginID), nil
}

func (r *GameHostPermissionSubjectResolver) GetRuntimeState(runtimeID string) (domain.RuntimeState, error) {
	if runtimeID == "" {
		return "", fmt.Errorf("runtime id is required")
	}
	rt, err := r.runtimeManager.GetRuntime(domain.RuntimeInstanceID(runtimeID))
	if err != nil {
		return "", err
	}
	return rt.State, nil
}

var _ permission.SubjectResolver = (*GameHostPermissionSubjectResolver)(nil)
