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

var _ control.HandshakeResetter = (*EmergencyReadyAdapter)(nil)
