package capability

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type hostRegistryDeviceSessionResolver struct {
	registry *host_registry.Registry
}

func NewHostRegistryDeviceSessionResolver(registry *host_registry.Registry) DeviceSessionResolver {
	return &hostRegistryDeviceSessionResolver{registry: registry}
}

func (r *hostRegistryDeviceSessionResolver) ResolveActiveSession(
	ctx context.Context,
	userID runtimeidentity.UserID,
	deviceID runtimeidentity.DeviceID,
	runtimeID runtimeidentity.RuntimeID,
) (runtimeidentity.RuntimeSessionID, bool) {
	if r.registry == nil {
		return "", false
	}
	entry, err := r.registry.FindRuntimeEntry(ctx, userID, deviceID, runtimeID)
	if err != nil || entry == nil {
		return "", false
	}
	if entry.RuntimeSessionID == "" {
		return "", false
	}
	return entry.RuntimeSessionID, true
}
