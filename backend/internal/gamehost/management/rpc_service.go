package management

import (
	"context"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

type RPCInvoker interface {
	SendCustomRPC(ctx context.Context, runtimeID, serviceID, method string, payload []byte, timeout time.Duration) (*protocol.Envelope, error)
}

type RuntimeGenerationReader interface {
	GetCurrentGeneration(runtimeID domain.RuntimeInstanceID) (int64, error)
}

type ControlPlaneRPCInvoker struct {
	controlPlane ipc.ControlPlane
	topology     RuntimeTopologyReader
	registry     PluginRegistryReader
	generation   RuntimeGenerationReader
}

func NewControlPlaneRPCInvoker(cp ipc.ControlPlane, topo RuntimeTopologyReader, reg PluginRegistryReader, generation ...RuntimeGenerationReader) *ControlPlaneRPCInvoker {
	var gen RuntimeGenerationReader
	if len(generation) > 0 {
		gen = generation[0]
	}
	return &ControlPlaneRPCInvoker{controlPlane: cp, topology: topo, registry: reg, generation: gen}
}

func (v *ControlPlaneRPCInvoker) SendCustomRPC(ctx context.Context, runtimeID, serviceID, method string, payload []byte, timeout time.Duration) (*protocol.Envelope, error) {
	if v.controlPlane == nil {
		return nil, ErrControlPlaneUnavailable
	}
	if v.topology == nil {
		return nil, ErrTopologyUnavailable
	}

	rtID := domain.RuntimeInstanceID(runtimeID)
	svcID := domain.ServiceID(serviceID)

	snap, err := v.topology.GetTopologySnapshot(string(rtID))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRuntimeNotFound, err)
	}

	var targetPluginID string
	var found bool
	for _, svc := range snap.Services {
		if string(svc.ServiceID) == string(svcID) && string(svc.RuntimeID) == string(runtimeID) {
			targetPluginID = string(svc.PluginID)
			found = true
			break
		}
	}
	if !found {
		return nil, ErrServiceNotFound
	}
	if v.generation == nil {
		return nil, ErrGenerationUnavailable
	}
	generation, err := v.generation.GetCurrentGeneration(rtID)
	if err != nil || generation <= 0 {
		return nil, fmt.Errorf("%w: %v", ErrGenerationUnavailable, err)
	}

	envelope := protocol.Envelope{
		Protocol:   protocol.ProtocolVersion,
		Type:       protocol.MessageTypeRequest,
		ID:         fmt.Sprintf("mgmt-rpc-%d", time.Now().UnixNano()),
		Method:     method,
		Payload:    payload,
		PluginID:   targetPluginID,
		RuntimeID:  runtimeID,
		ServiceID:  serviceID,
		Generation: uint64(generation),
	}

	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	return v.controlPlane.SendRequest(ctx, ipc.Peer{
		PluginID:   domain.PluginID(targetPluginID),
		RuntimeID:  rtID,
		ServiceID:  svcID,
		Generation: generation,
	}, envelope, timeout)
}

var _ RPCInvoker = (*ControlPlaneRPCInvoker)(nil)

var (
	ErrControlPlaneUnavailable = fmt.Errorf("management RPC: control plane unavailable")
	ErrTopologyUnavailable     = fmt.Errorf("management RPC: topology unavailable")
	ErrRuntimeNotFound         = fmt.Errorf("management RPC: runtime not found")
	ErrServiceNotFound         = fmt.Errorf("management RPC: service not found")
	ErrGenerationUnavailable   = fmt.Errorf("management RPC: runtime generation unavailable")
)
