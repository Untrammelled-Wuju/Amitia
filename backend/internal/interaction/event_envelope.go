package interaction

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type EventEnvelopeVersion string

const (
	EventEnvelopeVersionV1 EventEnvelopeVersion = "event-envelope-v1"
)

type EventSource string

const (
	EventSourceUser      EventSource = "user"
	EventSourceAssistant EventSource = "assistant"
	EventSourceSystem    EventSource = "system"
	EventSourceTool      EventSource = "tool"
	EventSourceScheduler EventSource = "scheduler"
	EventSourceRuntime   EventSource = "runtime"
)

type EventStatus string

const (
	EventStatusPending    EventStatus = "pending"
	EventStatusProcessing EventStatus = "processing"
	EventStatusApplied    EventStatus = "applied"
	EventStatusFailed     EventStatus = "failed"
	EventStatusSuperseded EventStatus = "superseded"
)

var (
	ErrEventEnvelopeMissingType           = errors.New("interaction event envelope requires event_type")
	ErrEventEnvelopeMissingSource         = errors.New("interaction event envelope requires source")
	ErrEventEnvelopeMissingStatus         = errors.New("interaction event envelope requires status")
	ErrEventEnvelopeMissingIdempotencyKey = errors.New("interaction event envelope requires idempotency_key")
)

type EventCausation struct {
	CorrelationID string   `json:"correlationId,omitempty"`
	CausationID   string   `json:"causationId,omitempty"`
	ParentEventID string   `json:"parentEventId,omitempty"`
	Chain         []string `json:"chain,omitempty"`
}

func (c EventCausation) Normalize() EventCausation {
	c.CorrelationID = strings.TrimSpace(c.CorrelationID)
	c.CausationID = strings.TrimSpace(c.CausationID)
	c.ParentEventID = strings.TrimSpace(c.ParentEventID)
	if len(c.Chain) == 0 {
		return c
	}
	chain := make([]string, 0, len(c.Chain))
	for _, item := range c.Chain {
		normalized := strings.TrimSpace(item)
		if normalized == "" {
			continue
		}
		chain = append(chain, normalized)
	}
	c.Chain = chain
	return c
}

type EventEnvelope struct {
	Version        EventEnvelopeVersion `json:"version"`
	EventID        string               `json:"eventId,omitempty"`
	EventType      string               `json:"eventType"`
	Source         EventSource          `json:"source"`
	Status         EventStatus          `json:"status"`
	IdempotencyKey string               `json:"idempotencyKey"`
	Scope          InteractionScope     `json:"scope"`
	Causation      EventCausation       `json:"causation"`
	StateVersion   int64                `json:"stateVersion,omitempty"`
	OccurredAt     time.Time            `json:"occurredAt"`
}

func (e EventEnvelope) Normalize() EventEnvelope {
	e.Version = EventEnvelopeVersion(strings.TrimSpace(string(e.Version)))
	if e.Version == "" {
		e.Version = EventEnvelopeVersionV1
	}
	e.EventID = strings.TrimSpace(e.EventID)
	e.EventType = strings.TrimSpace(e.EventType)
	e.Source = EventSource(strings.ToLower(strings.TrimSpace(string(e.Source))))
	e.Status = EventStatus(strings.ToLower(strings.TrimSpace(string(e.Status))))
	e.IdempotencyKey = strings.TrimSpace(e.IdempotencyKey)
	e.Scope = e.Scope.Normalize()
	e.Causation = e.Causation.Normalize()
	return e
}

func (e EventEnvelope) Validate() error {
	e = e.Normalize()
	if e.EventType == "" {
		return ErrEventEnvelopeMissingType
	}
	if e.Source == "" {
		return ErrEventEnvelopeMissingSource
	}
	if e.Status == "" {
		return ErrEventEnvelopeMissingStatus
	}
	if e.IdempotencyKey == "" {
		return ErrEventEnvelopeMissingIdempotencyKey
	}
	return e.Scope.Validate()
}

func (e EventEnvelope) WithContext(ctx context.Context) context.Context {
	return e.Normalize().Scope.WithContext(ctx)
}

func BuildEventIdempotencyKey(parts ...string) string {
	return event.BuildIdempotencyKey(parts...)
}
