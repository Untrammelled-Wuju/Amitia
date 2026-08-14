package integration

import (
	"context"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/handshake"
	"github.com/u-ai/backend/internal/gamehost/ipc"
)

type EmergencyReadyAdapter struct {
	readyGate *handshake.ReadyGate
	registry  *ipc.ConnectionRegistry
}

func NewEmergencyReadyAdapter(readyGate *handshake.ReadyGate, registry *ipc.ConnectionRegistry) *EmergencyReadyAdapter {
	return &EmergencyReadyAdapter{
		readyGate: readyGate,
		registry:  registry,
	}
}

func (a *EmergencyReadyAdapter) ClearRuntimeReady(ctx context.Context, runtimeID domain.RuntimeInstanceID) error {
	if a.readyGate == nil {
		return nil
	}
	if a.registry != nil {
		conns := a.registry.ListByRuntime(runtimeID)
		for _, conn := range conns {
			a.readyGate.Remove(string(conn.ID))
		}
	}
	return nil
}

func (a *EmergencyReadyAdapter) CountRuntimeReady(runtimeID domain.RuntimeInstanceID) int {
	if a.registry == nil || a.readyGate == nil {
		return 0
	}
	count := 0
	for _, conn := range a.registry.ListByRuntime(runtimeID) {
		if a.readyGate.IsReady(string(conn.ID)) {
			count++
		}
	}
	return count
}

var _ control.HandshakeResetter = (*EmergencyReadyAdapter)(nil)
