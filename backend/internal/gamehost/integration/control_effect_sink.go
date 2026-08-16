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
	"github.com/u-ai/backend/pkg/gameplugin/protocol/contracts"
)

const SinkDispatchMethod = contracts.MethodSinkDispatch

const DefaultSinkDispatchTimeout = 30 * time.Second

type SinkEffectDispatchPayload = contracts.SinkEffectDispatchPayload

type SinkEffectCommitResult = contracts.SinkEffectCommitResult

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

	outputID := permit.OutputID
	if outputID == "" {
		return fmt.Errorf("effect commit failed: permit missing outputId")
	}
	dispatch := SinkEffectDispatchPayload{
		SinkID:     s.sinkID,
		ServiceID:  string(serviceID),
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
		ID:       outputID,
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

	if len(resp.Payload) == 0 {
		return fmt.Errorf("effect commit failed: empty response payload")
	}

	var commit SinkEffectCommitResult
	if err := json.Unmarshal(resp.Payload, &commit); err != nil {
		return fmt.Errorf("effect commit failed: invalid response payload: %w", err)
	}

	if !commit.Accepted {
		return fmt.Errorf("effect not accepted by plugin")
	}

	if !commit.Committed {
		if commit.ErrorCode != "" {
			return fmt.Errorf("effect commit failed: %s - %s", commit.ErrorCode, commit.Message)
		}
		return fmt.Errorf("effect not committed by plugin")
	}

	if commit.EffectID == "" {
		return fmt.Errorf("effect commit failed: effectId is required and must not be empty")
	}

	if commit.EffectID != outputID {
		return fmt.Errorf("effect commit mismatch: expected %s, got %s", outputID, commit.EffectID)
	}

	if commit.Generation == 0 {
		return fmt.Errorf("effect commit failed: generation is required and must not be zero")
	}

	if commit.Generation != permit.Generation {
		return fmt.Errorf("effect commit generation mismatch: expected %d, got %d", permit.Generation, commit.Generation)
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
