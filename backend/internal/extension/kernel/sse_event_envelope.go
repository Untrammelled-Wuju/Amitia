// Package kernel provides the Extension Kernel services.
//
// This file contains SSE (Server-Sent Events) transport types for communicating
// with connected UI endpoints.
//
// IMPORTANT: SSEEventEnvelope is a transport-level envelope for UI communication.
// It is NOT the same as event.EventEnvelope from the kernel Durable Event system
// (event.Service / event.Outbox). SSE events are ephemeral and are not
// persisted in the durable event outbox.
package kernel

import (
	"time"

	"github.com/google/uuid"
)

const (
	defaultEventTTL = 5 * time.Minute
	dialogEventTTL  = 35 * time.Second
)

// SSEEventEnvelope is an ephemeral UI transport envelope for SSE communication
// with connected desktop/web UI endpoints.
//
// This is NOT extension/kernel/event.EventEnvelope (the durable kernel event)
// and is NOT persisted in the durable event outbox. It is used solely for
// real-time UI notification/dialog/navigate actions over SSE.
//
// Do NOT confuse with:
//   - event.EventEnvelope: durable kernel event stored in outbox
//   - GameHost stream.EventEnvelope: GameHost protocol transport envelope
type SSEEventEnvelope struct {
	EventType   string                 `json:"eventType"`
	RequestID   string                 `json:"requestId"`
	SessionID   string                 `json:"sessionId"`
	ExtensionID string                 `json:"extensionId"`
	Payload     map[string]interface{} `json:"payload"`
	ExpiresAt   string                 `json:"expiresAt,omitempty"`
	Timestamp   string                 `json:"timestamp"`
}

// NewSSEEventEnvelope creates a new ephemeral SSE transport envelope for UI communication.
//
// The returned envelope is intended for real-time UI delivery only and is NOT stored
// in the kernel durable event outbox. Use a separate path for durable events.
//
// SessionID "ui-host" here is a transport field and does NOT represent a
// RuntimeSessionID or Device Session ID.
func NewSSEEventEnvelope(eventType string, extensionID string, payload map[string]interface{}, ttl time.Duration) SSEEventEnvelope {
	now := time.Now().UTC()
	return SSEEventEnvelope{
		EventType:   eventType,
		RequestID:   uuid.NewString(),
		SessionID:   "ui-host",
		ExtensionID: extensionID,
		Payload:     payload,
		ExpiresAt:   now.Add(ttl).Format(time.RFC3339),
		Timestamp:   now.Format(time.RFC3339),
	}
}

// NewEventEnvelope is a backwards-compatible alias for NewSSEEventEnvelope.
// New code should use NewSSEEventEnvelope directly.
//
// Deprecated: Use NewSSEEventEnvelope instead.
func NewEventEnvelope(eventType string, extensionID string, payload map[string]interface{}, ttl time.Duration) SSEEventEnvelope {
	return NewSSEEventEnvelope(eventType, extensionID, payload, ttl)
}

func (e SSEEventEnvelope) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"eventType":   e.EventType,
		"requestId":   e.RequestID,
		"sessionId":   e.SessionID,
		"extensionId": e.ExtensionID,
		"payload":     e.Payload,
		"timestamp":   e.Timestamp,
	}
	if e.ExpiresAt != "" {
		m["expiresAt"] = e.ExpiresAt
	}
	return m
}
