package nativebridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
)

const (
	AndroidBridgeProtocolVersion = 1
)

type bridgeError struct {
	Code    string
	Message string
}

func (e *bridgeError) Error() string {
	return e.Code + ": " + e.Message
}

type relaySession interface {
	SendRequest(ctx context.Context, req Request) (Response, error)
}

type AndroidTransportBridge struct {
	mu          sync.RWMutex
	session     relaySession
	generation  atomic.Uint64
	hostHealth  Health
	healthMu    sync.RWMutex
}

func NewAndroidTransportBridge() *AndroidTransportBridge {
	return &AndroidTransportBridge{
		hostHealth: HealthUnknown,
	}
}

func (b *AndroidTransportBridge) Execute(ctx context.Context, req Request) (Response, error) {
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
				Message: "android native host is not connected",
			},
		}, &bridgeError{Code: ErrBridgeDisconnected, Message: "android native host is not connected"}
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

func (b *AndroidTransportBridge) Health(_ context.Context) Health {
	b.healthMu.RLock()
	defer b.healthMu.RUnlock()
	if b.session == nil {
		return HealthUnhealthy
	}
	return b.hostHealth
}

func (b *AndroidTransportBridge) SessionAttached() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.session != nil
}

func (b *AndroidTransportBridge) AttachSession(session relaySession) {
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

func (b *AndroidTransportBridge) AttachRelaySession(transport RelayTransport) {
	session := newRelaySession(transport)
	b.AttachSession(session)
}

func (b *AndroidTransportBridge) DetachSession() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.session = nil
	b.healthMu.Lock()
	b.hostHealth = HealthUnhealthy
	b.healthMu.Unlock()
	b.generation.Add(1)
}

func (b *AndroidTransportBridge) Generation() uint64 {
	return b.generation.Load()
}

func (b *AndroidTransportBridge) SetHostHealth(h Health) {
	b.healthMu.Lock()
	defer b.healthMu.Unlock()
	b.hostHealth = h
}

func (b *AndroidTransportBridge) HandleRelayEnvelope(payload []byte) error {
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
