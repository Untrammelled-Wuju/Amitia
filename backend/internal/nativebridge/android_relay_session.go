package nativebridge

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const (
	relaySendTimeout = 30 * time.Second
)

type RelayTransport interface {
	Send(payload []byte) error
}

type productionRelaySession struct {
	transport   RelayTransport
	mu          sync.Mutex
	pending     map[string]chan RelayEnvelope
	generation  uint64
	connectedAt time.Time
}

type RelayEnvelope struct {
	Type       string          `json:"type"`
	Platform   string          `json:"platform,omitempty"`
	Generation uint64          `json:"generation"`
	RequestID  string          `json:"requestId,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

type NativeBridgeEvent struct {
	Domain    string         `json:"domain"`
	Event     string         `json:"event"`
	Timestamp string         `json:"timestamp"`
	Data      map[string]any `json:"data"`
}

type NativeEventSink interface {
	PublishNativeEvent(ctx context.Context, platform string, generation uint64, payload json.RawMessage) error
}

func newRelaySession(transport RelayTransport) *productionRelaySession {
	return &productionRelaySession{
		transport:   transport,
		pending:     make(map[string]chan RelayEnvelope),
		generation:  1,
		connectedAt: time.Now(),
	}
}

func (s *productionRelaySession) SendRequest(ctx context.Context, req Request) (Response, error) {
	if s.transport == nil {
		return Response{
			ProtocolVersion: req.ProtocolVersion,
			RequestID:       req.RequestID,
			Status:          "error",
			Error: &Error{
				Code:    ErrBridgeDisconnected,
				Message: "relay transport not available",
			},
		}, fmt.Errorf("relay transport not available")
	}

	payload, err := json.Marshal(req)
	if err != nil {
		return Response{
			ProtocolVersion: req.ProtocolVersion,
			RequestID:       req.RequestID,
			Status:          "error",
			Error: &Error{
				Code:    "ENCODE_ERROR",
				Message: err.Error(),
			},
		}, err
	}

	env := RelayEnvelope{
		Type:       "native_bridge.request",
		Generation: s.generation,
		RequestID:  req.RequestID,
		Payload:    payload,
	}

	envData, err := json.Marshal(env)
	if err != nil {
		return Response{
			ProtocolVersion: req.ProtocolVersion,
			RequestID:       req.RequestID,
			Status:          "error",
			Error: &Error{
				Code:    "ENCODE_ERROR",
				Message: err.Error(),
			},
		}, err
	}

	respChan := make(chan RelayEnvelope, 1)
	s.mu.Lock()
	s.pending[req.RequestID] = respChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pending, req.RequestID)
		s.mu.Unlock()
	}()

	if err := s.transport.Send(envData); err != nil {
		return Response{
			ProtocolVersion: req.ProtocolVersion,
			RequestID:       req.RequestID,
			Status:          "error",
			Error: &Error{
				Code:    ErrBridgeDisconnected,
				Message: err.Error(),
			},
		}, err
	}

	sendCtx, cancel := context.WithTimeout(ctx, relaySendTimeout)
	defer cancel()

	select {
	case <-sendCtx.Done():
		return Response{
			ProtocolVersion: req.ProtocolVersion,
			RequestID:       req.RequestID,
			Status:          "error",
			Error: &Error{
				Code:    "TIMEOUT",
				Message: "relay request timed out",
			},
		}, sendCtx.Err()
	case resp, ok := <-respChan:
		if !ok || len(resp.Payload) == 0 {
			return Response{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "error",
				Error: &Error{
					Code:    "INVALID_RESPONSE",
					Message: "empty or invalid relay response",
				},
			}, fmt.Errorf("empty or invalid relay response")
		}
		var response Response
		if err := json.Unmarshal(resp.Payload, &response); err != nil {
			return Response{
				ProtocolVersion: req.ProtocolVersion,
				RequestID:       req.RequestID,
				Status:          "error",
				Error: &Error{
					Code:    "INVALID_RESPONSE",
					Message: "failed to decode response: " + err.Error(),
				},
			}, fmt.Errorf("failed to decode response: %w", err)
		}
		return response, nil
	}
}

func (s *productionRelaySession) handleIncomingEnvelope(env RelayEnvelope) {
	switch env.Type {
	case "native_bridge.response", "native_bridge.request":
		if env.RequestID == "" {
			return
		}
		s.mu.Lock()
		ch, ok := s.pending[env.RequestID]
		s.mu.Unlock()
		if ok {
			select {
			case ch <- env:
			default:
			}
		}
	}
}

func (s *productionRelaySession) generationValue() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.generation
}
