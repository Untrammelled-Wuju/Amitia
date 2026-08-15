package integration

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const SinkDispatchMethod = "control.sink.dispatch"

type SinkEffectDispatchPayload struct {
	SinkID  string          `json:"sinkId"`
	Service string          `json:"serviceId"`
	Payload json.RawMessage `json:"payload"`
}

type ProtocolControlEffectSink struct {
	runtimeID    domain.RuntimeInstanceID
	pluginID     domain.PluginID
	sinkID       string
	connReg      *ipc.ConnectionRegistry
	controlPlane ipc.ControlPlane
}

func NewProtocolControlEffectSink(
	runtimeID domain.RuntimeInstanceID,
	pluginID domain.PluginID,
	sinkID string,
	connReg *ipc.ConnectionRegistry,
	controlPlane ipc.ControlPlane,
) *ProtocolControlEffectSink {
	return &ProtocolControlEffectSink{
		runtimeID:    runtimeID,
		pluginID:     pluginID,
		sinkID:       sinkID,
		connReg:      connReg,
		controlPlane: controlPlane,
	}
}

func (s *ProtocolControlEffectSink) ExecuteAuthorized(
	ctx context.Context,
	runtimeID domain.RuntimeInstanceID,
	serviceID domain.ServiceID,
	pluginID domain.PluginID,
	permit control.OutputPermit,
	payload []byte,
) error {
	if s.controlPlane == nil {
		return fmt.Errorf("control plane unavailable for effect delivery")
	}
	if s.connReg == nil {
		return fmt.Errorf("connection registry unavailable for effect delivery")
	}

	dispatch := SinkEffectDispatchPayload{
		SinkID:  s.sinkID,
		Service: string(serviceID),
		Payload: payload,
	}
	dispatchBytes, err := json.Marshal(dispatch)
	if err != nil {
		return fmt.Errorf("marshal sink dispatch: %w", err)
	}

	peer := ipc.Peer{
		RuntimeID:  runtimeID,
		PluginID:   pluginID,
		ServiceID:  serviceID,
		Generation: int64(permit.Generation),
	}

	envelope := protocol.Envelope{
		Protocol: protocol.ProtocolVersion,
		Type:     protocol.MessageTypeNotification,
		ID:       fmt.Sprintf("sink-dispatch-%s-%s", runtimeID, s.sinkID),
		Method:   SinkDispatchMethod,
		Payload:  dispatchBytes,
	}

	return s.controlPlane.Send(ctx, peer, envelope)
}

type ProtocolControlEffectSinkFactory struct {
	connReg      *ipc.ConnectionRegistry
	controlPlane ipc.ControlPlane
}

func NewProtocolControlEffectSinkFactory(
	connReg *ipc.ConnectionRegistry,
	controlPlane ipc.ControlPlane,
) *ProtocolControlEffectSinkFactory {
	return &ProtocolControlEffectSinkFactory{
		connReg:      connReg,
		controlPlane: controlPlane,
	}
}

func (f *ProtocolControlEffectSinkFactory) CreateSink(runtimeID domain.RuntimeInstanceID, pluginID domain.PluginID, sinkID string) control.ControlEffectSink {
	return NewProtocolControlEffectSink(runtimeID, pluginID, sinkID, f.connReg, f.controlPlane)
}

var _ control.ControlEffectSink = (*ProtocolControlEffectSink)(nil)
