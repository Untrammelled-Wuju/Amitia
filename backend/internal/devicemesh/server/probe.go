package server

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	protocol "github.com/u-ai/backend/internal/devicemesh/protocol"
	runtimeprotocol "github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type ProbeService struct {
	hub *ConnectionHub
}

func NewProbeService(hub *ConnectionHub) *ProbeService {
	return &ProbeService{hub: hub}
}

func (s *ProbeService) ProbeRuntime(ctx context.Context, spaceID runtimeidentity.SpaceID, deviceID runtimeidentity.DeviceID, runtimeID runtimeidentity.RuntimeID) (time.Duration, error) {
	conn, ok := s.hub.GetByRuntime(spaceID, deviceID, runtimeID)
	if !ok || conn == nil {
		return 0, fmt.Errorf("mesh: no active connection for runtime")
	}

	pingID := uuid.New().String()
	ping := runtimeprotocol.PingPayload{Time: time.Now().UTC()}
	payloadBytes, _ := json.Marshal(ping)

	env := runtimeprotocol.Envelope{
		EnvelopeVersion:      protocol.EnvelopeVersion,
		Protocol:             protocol.ProtocolName,
		MessageType:          runtimeprotocol.MessageTypePing,
		MessageID:            pingID,
		SpaceID:              spaceID,
		DeviceID:             deviceID,
		RuntimeID:            runtimeID,
		RuntimeSessionID:     conn.SessionID,
		ConnectionGeneration: conn.Generation,
		Sequence:             0,
		PayloadSchemaVersion: 1,
		PayloadHash:          runtimeprotocol.ComputePayloadHash(payloadBytes),
		SentAt:               time.Now().UTC(),
		Payload:              payloadBytes,
	}

	data, err := json.Marshal(env)
	if err != nil {
		return 0, fmt.Errorf("marshal ping: %w", err)
	}

	if err := conn.Send(data); err != nil {
		return 0, fmt.Errorf("send ping: %w", err)
	}

	start := time.Now()
	timeout := time.NewTimer(protocol.ProbeTimeoutSeconds * time.Second)
	defer timeout.Stop()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout.C:
			return 0, fmt.Errorf("mesh: probe timeout")
		case <-ticker.C:
			if conn.LastPongAt.After(start) {
				return time.Since(start), nil
			}
		case <-ctx.Done():
			return 0, ctx.Err()
		}
	}
}

func (s *ProbeService) CloseAll() {
	hub := s.hub
	if hub != nil {
		hub.CloseAll()
	}
}
