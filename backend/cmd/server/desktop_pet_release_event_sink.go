package main

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	kernelevent "github.com/u-ai/backend/internal/extension/kernel/event"
)

// desktopPetReleaseEventSink bridges the desktop-pet release outbox into the
// host's durable event service. The release outbox remains the single producer
// authority; a row is marked published only after the host event service has
// durably accepted the event into its own outbox.
type desktopPetReleaseEventSink struct {
	service *kernelevent.Service
}

func newDesktopPetReleaseEventSink(service *kernelevent.Service) *desktopPetReleaseEventSink {
	return &desktopPetReleaseEventSink{service: service}
}

func (s *desktopPetReleaseEventSink) Deliver(ctx context.Context, eventType, aggregateID string, payload []byte) error {
	if s == nil || s.service == nil {
		return errors.New("kernel event service unavailable")
	}
	def := desktopPetReleaseEventType(eventType)
	if err := s.service.RegisterEventType(ctx, def); err != nil {
		return err
	}
	_, err := s.service.Publish(
		ctx,
		kernelevent.EventTypeID(eventType),
		1,
		json.RawMessage(payload),
		kernelevent.PublishOptions{
			ProducerID:    "desktop_pet.release",
			ProducerType:  kernelevent.EventProducerTypeSystem,
			Domain:        kernelevent.EventDomainSystem,
			AggregateType: "desktop_pet_release",
			AggregateID:   aggregateID,
			PartitionKey:  aggregateID,
			OrderingKey:   aggregateID,
		},
	)
	return err
}

func desktopPetReleaseEventType(eventType string) kernelevent.EventTypeDefinition {
	const maxPayload = int64(256 * 1024)
	const maxMetadata = int64(32 * 1024)
	return kernelevent.EventTypeDefinition{
		EventTypeID:      kernelevent.EventTypeID(eventType),
		Version:          1,
		Description:      "Durable desktop-pet release lifecycle event",
		MaxPayloadBytes:  maxPayload,
		MaxMetadataBytes: maxMetadata,
		RiskLevel:        kernelevent.RiskLevelLow,
		ProducerPolicy: kernelevent.EventProducerPolicy{
			AllowedProducers:   []string{kernelevent.EventProducerTypeSystem.String()},
			RequireSystemTrust: true,
			MaxPayloadBytes:    maxPayload,
			MaxMetadataBytes:   maxMetadata,
		},
		SubscriberPolicy: kernelevent.EventSubscriberPolicy{
			AllowThirdParty: false,
			MaxSubscribers:  32,
			RequireApproval: true,
		},
		DeliveryPolicy: kernelevent.EventDeliveryPolicy{
			Timeout:             5 * time.Second,
			MaxAttempts:         5,
			InitialBackoff:      time.Second,
			MaxBackoff:          30 * time.Second,
			BackoffMultiplier:   2,
			JitterFactor:        0.2,
			OrderingRequirement: kernelevent.OrderingPerAggregate,
			MaxInFlight:         4,
		},
		OrderingPolicy:  kernelevent.OrderingPerAggregate,
		RetentionPolicy: kernelevent.EventRetentionPolicy{MaxAge: 30 * 24 * time.Hour, MaxDeliveryCount: 5},
	}
}
