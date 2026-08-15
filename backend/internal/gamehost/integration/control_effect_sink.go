package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/gamehost/control"
	"github.com/u-ai/backend/internal/gamehost/domain"
	"github.com/u-ai/backend/internal/gamehost/ipc"
	"github.com/u-ai/backend/pkg/gameplugin/protocol"
)

const SinkDispatchMethod = "control.sink.dispatch"

const DefaultSinkDispatchTimeout = 30 * time.Second

type SinkEffectDispatchPayload struct {
	SinkID    string          `json:"sinkId"`
	Service   string          `json:"serviceId"`
	Payload   json.RawMessage `json:"payload"`
	OutputID  string          `json:"outputId"`
	Epoch     uint64          `json:"epoch"`
	Generation uint64         `json:"generation"`
}

type SinkEffectCommitResult struct {
	Accepted  bool   `json:"accepted"`
	Committed bool   `json:"committed"`
	EffectID  string `json:"effectId,omitempty"`
	Generation uint64 `json:"generation,omitempty"`
	ErrorCode string `json:"errorCode,omitempty"`
	Message   string `json:"message,omitempty"`
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

	outputID := fmt.Sprintf("effect-%s-%d", s.sinkID, time.Now().UnixNano())
	dispatch := SinkEffectDispatchPayload{
		SinkID:     s.sinkID,
		Service:    string(serviceID),
		Payload:    payload,
		OutputID:   outputID,
		Epoch:      permit.OutputEpoch,
		Generation: permit.Generation,
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
		Type:     protocol.MessageTypeRequest,
		ID:       fmt.Sprintf("sink-dispatch-%s-%s", runtimeID, s.sinkID),
		Method:   SinkDispatchMethod,
		Payload:  dispatchBytes,
	}

	resp, err := s.controlPlane.SendRequest(ctx, peer, envelope, DefaultSinkDispatchTimeout)
	if err != nil {
		return fmt.Errorf("sink dispatch failed: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("sink dispatch rejected: %s - %s", resp.Error.Code, resp.Error.Message)
	}

	var commit SinkEffectCommitResult
	if len(resp.Payload) > 0 {
		if err := json.Unmarshal(resp.Payload, &commit); err == nil {
			if !commit.Committed {
				if commit.ErrorCode != "" {
					return fmt.Errorf("effect commit failed: %s - %s", commit.ErrorCode, commit.Message)
				}
				return fmt.Errorf("effect not committed by plugin")
			}
		}
	}

	return nil
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
