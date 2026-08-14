package host_registry

import (
	"context"
	"strconv"
	"time"
)

type PresenceDomainEventType string

const (
	PresenceEventReady        PresenceDomainEventType = "ready"
	PresenceEventDisconnected PresenceDomainEventType = "disconnected"
)

type PresenceDomainEvent struct {
	Type   PresenceDomainEventType
	Entry  RuntimeEntry
	Reason string
	At     time.Time
}

type PresenceEventSink interface {
	PresenceReady(ctx context.Context, event PresenceDomainEvent) error
	PresenceDisconnected(ctx context.Context, event PresenceDomainEvent) error
}

func PresenceEventPartitionKey(event PresenceDomainEvent) string {
	if event.Entry.UserID != "" {
		return event.Entry.UserID.String()
	}
	return "system"
}

func PresenceEventOrderingKey(event PresenceDomainEvent) string {
	return event.Entry.EntryID
}

func PresenceEventAggregateVersion(event PresenceDomainEvent) *int64 {
	return &event.Entry.Revision
}

func BuildPresenceEventIDempotency(event PresenceDomainEvent) string {
	return event.Entry.EntryID + ":" + string(event.Type) + ":" + strconv.FormatInt(event.Entry.Revision, 10)
}
