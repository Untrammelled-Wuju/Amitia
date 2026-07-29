package kernel

import (
	"time"

	"github.com/google/uuid"
)

const (
	defaultEventTTL = 5 * time.Minute
	dialogEventTTL  = 35 * time.Second
)

type SSEEventEnvelope struct {
	EventType   string                 `json:"eventType"`
	RequestID   string                 `json:"requestId"`
	SessionID   string                 `json:"sessionId"`
	ExtensionID string                 `json:"extensionId"`
	Payload     map[string]interface{} `json:"payload"`
	ExpiresAt   string                 `json:"expiresAt,omitempty"`
	Timestamp   string                 `json:"timestamp"`
}

func NewEventEnvelope(eventType string, extensionID string, payload map[string]interface{}, ttl time.Duration) SSEEventEnvelope {
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
