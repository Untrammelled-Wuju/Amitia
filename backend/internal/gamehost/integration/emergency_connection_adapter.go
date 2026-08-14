package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
)

type EmergencyConnectionAdapter struct {
	registry  *ipc.ConnectionRegistry
	controlPlane ipc.ControlPlane
}

func NewEmergencyConnectionAdapter(registry *ipc.ConnectionRegistry, cp ipc.ControlPlane) *EmergencyConnectionAdapter {
	return &EmergencyConnectionAdapter{
		registry:     registry,
		controlPlane: cp,
	}
}

func (a *EmergencyConnectionAdapter) CloseRuntimeConnections(ctx context.Context, runtimeID domain.RuntimeInstanceID) (int, error) {
	if a.registry == nil || a.controlPlane == nil {
		return 0, nil
	}
	conns := a.registry.ListByRuntime(runtimeID)
	closed := 0
	for _, conn := range conns {
		if err := a.controlPlane.Detach(ctx, conn.ID); err != nil {
			continue
		}
		closed++
	}
	return closed, nil
}

var _ control.ConnectionCloser = (*EmergencyConnectionAdapter)(nil)

type EmergencyConnectionVerifier struct {
	registry *ipc.ConnectionRegistry
}

func NewEmergencyConnectionVerifier(registry *ipc.ConnectionRegistry) *EmergencyConnectionVerifier {
	return &EmergencyConnectionVerifier{registry: registry}
}

func (v *EmergencyConnectionVerifier) CountRuntimeConnections(runtimeID domain.RuntimeInstanceID) int {
	if v.registry == nil {
		return 0
	}
	return v.registry.CountByRuntime(runtimeID)
}

var _ control.ConnectionVerifier = (*EmergencyConnectionVerifier)(nil)
