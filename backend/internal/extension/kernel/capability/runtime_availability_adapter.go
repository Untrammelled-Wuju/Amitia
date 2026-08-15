package capability

import (
	"context"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

// RuntimeAvailabilityAdapter adapts RuntimeStateService to the RuntimeAvailabilityPort interface.
// This allows the capability resolver to distinguish device_offline from capability_not_registered.
type RuntimeAvailabilityAdapter struct {
	runtimeState RuntimeStateServiceAvailer
}

// RuntimeStateServiceAvailer is the minimal interface the adapter needs.
type RuntimeStateServiceAvailer interface {
	IsRuntimeOnline(ctx context.Context, runtimeID runtimeidentity.RuntimeID) (bool, error)
	IsDeviceOffline(ctx context.Context, deviceID runtimeidentity.DeviceID) (bool, error)
}

func NewRuntimeAvailabilityAdapter(runtimeState RuntimeStateServiceAvailer) *RuntimeAvailabilityAdapter {
	return &RuntimeAvailabilityAdapter{runtimeState: runtimeState}
}

func (a *RuntimeAvailabilityAdapter) IsRuntimeOnline(ctx context.Context, runtimeID runtimeidentity.RuntimeID) (bool, error) {
	if a.runtimeState == nil {
		return true, nil
	}
	return a.runtimeState.IsRuntimeOnline(ctx, runtimeID)
}

func (a *RuntimeAvailabilityAdapter) IsDeviceOffline(ctx context.Context, deviceID runtimeidentity.DeviceID) (bool, error) {
	if a.runtimeState == nil {
		return false, nil
	}
	return a.runtimeState.IsDeviceOffline(ctx, deviceID)
}
