//go:build ios
// +build ios

package nativebridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	IOSBridgeProtocolVersion = 1
)

type IOSBridge struct {
	mu         sync.RWMutex
	session    relaySession
	generation atomic.Uint64
	hostHealth Health
	healthMu   sync.RWMutex
}

func NewIOSBridge() *IOSBridge {
	return &IOSBridge{
		hostHealth: HealthUnknown,
	}
}

func (b *IOSBridge) Execute(ctx context.Context, req Request) (Response, error) {
	b.mu.RLock()
	session := b.session
	b.mu.RUnlock()

	if session == nil {
		return Response{
			ProtocolVersion: req.ProtocolVersion,
			RequestID:       req.RequestID,
			Status:          "error",
			Error: &Error{
				Code:    ErrBridgeDisconnected,
				Message: "ios native host is not connected",
			},
		}, &bridgeError{Code: ErrBridgeDisconnected, Message: "ios native host is not connected"}
	}

	if req.RequestID == "" {
		return Response{
			ProtocolVersion: req.ProtocolVersion,
			RequestID:       req.RequestID,
			Status:          "error",
			Error: &Error{
				Code:    "INVALID_ARGUMENT",
				Message: "missing requestId",
			},
		}, nil
	}

	resp, err := session.SendRequest(ctx, req)
	if err != nil {
		return Response{
			ProtocolVersion: req.ProtocolVersion,
			RequestID:       req.RequestID,
			Status:          "error",
			Error: &Error{
				Code:    ErrBridgeDisconnected,
				Message: err.Error(),
			},
		}, &bridgeError{Code: ErrBridgeDisconnected, Message: err.Error()}
	}
	return resp, nil
}

func (b *IOSBridge) Health(_ context.Context) Health {
	b.healthMu.RLock()
	defer b.healthMu.RUnlock()
	return b.hostHealth
}

func (b *IOSBridge) AttachSession(session relaySession) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.session = session
	b.healthMu.Lock()
	if session != nil {
		b.hostHealth = HealthReady
	} else {
		b.hostHealth = HealthUnhealthy
	}
	b.healthMu.Unlock()
	b.generation.Add(1)
}

func (b *IOSBridge) AttachRelaySession(transport RelayTransport) {
	session := newRelaySession(transport)
	b.AttachSession(session)
}

func (b *IOSBridge) DetachSession() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.session = nil
	b.healthMu.Lock()
	b.hostHealth = HealthUnhealthy
	b.healthMu.Unlock()
	b.generation.Add(1)
}

func (b *IOSBridge) Generation() uint64 {
	return b.generation.Load()
}

func (b *IOSBridge) SessionAttached() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.session != nil
}

func (b *IOSBridge) SetHostHealth(h Health) {
	b.healthMu.Lock()
	defer b.healthMu.Unlock()
	b.hostHealth = h
}

func (b *IOSBridge) HandleRelayEnvelope(payload []byte) error {
	var env RelayEnvelope
	if err := json.Unmarshal(payload, &env); err != nil {
		return fmt.Errorf("decode relay envelope: %w", err)
	}

	switch env.Type {
	case "native_bridge.response", "native_bridge.request":
		b.mu.RLock()
		session := b.session
		b.mu.RUnlock()
		if session == nil {
			return fmt.Errorf("no active relay session")
		}
		if pSession, ok := session.(*productionRelaySession); ok {
			pSession.handleIncomingEnvelope(env)
		}
		return nil
	default:
		return fmt.Errorf("unknown relay envelope type: %s", env.Type)
	}
}
