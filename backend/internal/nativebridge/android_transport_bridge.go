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

type AndroidTransportBridge struct {
	mu         sync.RWMutex
	session    *productionRelaySession
	generation atomic.Uint64
	hostHealth Health
	healthMu   sync.RWMutex
	evtSink    NativeEventSink
}

func NewAndroidTransportBridge() *AndroidTransportBridge {
	return &AndroidTransportBridge{
		hostHealth: HealthUnknown,
	}
}

func (b *AndroidTransportBridge) SetEventSink(sink NativeEventSink) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.evtSink = sink
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

func (b *AndroidTransportBridge) AttachRelaySession(transport RelayTransport) uint64 {
	b.mu.Lock()
	b.generation.Add(1)
	gen := b.generation.Load()
	session := newRelaySession(transport, gen)
	b.session = session
	b.healthMu.Lock()
	b.hostHealth = HealthReady
	b.healthMu.Unlock()
	b.mu.Unlock()
	return gen
}

func (b *AndroidTransportBridge) DetachRelaySession(expectedGeneration uint64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.generation.Load() != expectedGeneration {
		return
	}
	b.session = nil
	b.healthMu.Lock()
	b.hostHealth = HealthUnhealthy
	b.healthMu.Unlock()
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
		gen := b.generation.Load()
		b.mu.RUnlock()
		if session == nil {
			return fmt.Errorf("no active relay session")
		}
		if gen != env.Generation {
			return nil
		}
		session.handleIncomingEnvelope(env)
		return nil
	case "native_bridge.event":
		b.mu.RLock()
		sink := b.evtSink
		gen := b.generation.Load()
		b.mu.RUnlock()
		if gen != env.Generation {
			return nil
		}
		if sink != nil {
			return sink.PublishNativeEvent(context.Background(), "android", env.Generation, env.Payload)
		}
		return nil
	case "native_bridge.health":
		b.mu.RLock()
		gen := b.generation.Load()
		b.mu.RUnlock()
		if gen != env.Generation {
			return nil
		}
		return b.updateHostHealthFromEnvelope(env)
	default:
		return fmt.Errorf("unknown relay envelope type: %s", env.Type)
	}
}

func (b *AndroidTransportBridge) updateHostHealthFromEnvelope(env RelayEnvelope) error {
	if env.Payload == nil {
		return nil
	}
	var healthData struct {
		Generation uint64 `json:"generation"`
		Ready      bool   `json:"ready"`
		Foreground bool   `json:"foreground"`
	}
	if err := json.Unmarshal(env.Payload, &healthData); err != nil {
		return err
	}
	b.mu.RLock()
	gen := b.generation.Load()
	b.mu.RUnlock()
	if gen != healthData.Generation {
		return nil
	}
	b.healthMu.Lock()
	if healthData.Ready {
		b.hostHealth = HealthReady
	} else {
		b.hostHealth = HealthUnhealthy
	}
	b.healthMu.Unlock()
	return nil
}
