package eventbridge

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/extension/kernel/host_registry"
)

const (
	deviceProducerID = "host-registry-kernel"
)

type DevicePresencePayload struct {
	EntryID   string `json:"entryId"`
	Kind      string `json:"kind"`
	UserID    string `json:"userId"`
	DeviceID  string `json:"deviceId"`
	RuntimeID string `json:"runtimeId"`
	Platform  string `json:"platform"`
	State     string `json:"state"`
	Reason    string `json:"reason,omitempty"`

	RuntimeSessionID     string `json:"runtimeSessionId,omitempty"`
	ConnectionGeneration int64  `json:"connectionGeneration,omitempty"`

	At string `json:"at"`
}

type DevicePresenceEventPublisher struct {
	publisher *Publisher
}

func NewDevicePresenceEventPublisher(publisher *Publisher) *DevicePresenceEventPublisher {
	return &DevicePresenceEventPublisher{publisher: publisher}
}

func (p *DevicePresenceEventPublisher) PresenceReady(
	ctx context.Context,
	domainEvent host_registry.PresenceDomainEvent,
) error {
	return p.publish(ctx, devicePresenceReady, domainEvent)
}

func (p *DevicePresenceEventPublisher) PresenceDisconnected(
	ctx context.Context,
	domainEvent host_registry.PresenceDomainEvent,
) error {
	return p.publish(ctx, devicePresenceDisconnected, domainEvent)
}

func (p *DevicePresenceEventPublisher) publish(
	ctx context.Context,
	typeID event.EventTypeID,
	domainEvent host_registry.PresenceDomainEvent,
) error {
	payload := buildDevicePayload(domainEvent)
	opts := deviceOptions(domainEvent)
	_, err := p.publisher.Publish(ctx, typeID, eventVersion, payload, opts)
	return err
}

func buildDevicePayload(e host_registry.PresenceDomainEvent) DevicePresencePayload {
	return DevicePresencePayload{
		EntryID:              e.Entry.EntryID,
		Kind:                 e.Entry.Kind.String(),
		UserID:               e.Entry.UserID.String(),
		DeviceID:             e.Entry.DeviceID.String(),
		RuntimeID:            e.Entry.RuntimeID.String(),
		Platform:             e.Entry.Platform.String(),
		State:                string(e.Entry.PresenceState),
		Reason:               e.Reason,
		RuntimeSessionID:     e.Entry.RuntimeSessionID.String(),
		ConnectionGeneration: e.Entry.ConnectionGeneration,
		At:                   e.At.Format(time.RFC3339Nano),
	}
}

func deviceOptions(e host_registry.PresenceDomainEvent) event.PublishOptions {
	var rev *int64
	if e.Entry.Revision > 0 {
		r := e.Entry.Revision
		rev = &r
	}
	return event.PublishOptions{
		Domain:           event.EventDomainDevice,
		ProducerType:     event.EventProducerTypeDevice,
		ProducerID:       deviceProducerID,
		AggregateType:    "device_presence",
		AggregateID:      e.Entry.EntryID,
		AggregateVersion: rev,
		PartitionKey:     devicePartitionKey(e),
		OrderingKey:      e.Entry.EntryID,
	}
}

func devicePartitionKey(e host_registry.PresenceDomainEvent) string {
	if e.Entry.UserID != "" {
		return e.Entry.UserID.String()
	}
	return "system"
}
