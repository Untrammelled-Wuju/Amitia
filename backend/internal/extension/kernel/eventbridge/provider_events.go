package eventbridge

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/internal/extension/kernel/event"
)

const (
	providerProducerID = "capability-provider-kernel"
)

type ProviderEventEmitter struct {
	publisher *Publisher
}

func NewProviderEventEmitter(publisher *Publisher) *ProviderEventEmitter {
	return &ProviderEventEmitter{publisher: publisher}
}

func (e *ProviderEventEmitter) ProviderRegistered(ctx context.Context, payload capability.ProviderRegisteredPayload) error {
	return e.publish(ctx, providerRegistered, payload.Revision, payload, definitionOptions(payload.ProviderID, payload.Revision))
}

func (e *ProviderEventEmitter) ProviderUpdated(ctx context.Context, payload capability.ProviderUpdatedPayload) error {
	return e.publish(ctx, providerUpdated, payload.Revision, payload, definitionOptions(payload.ProviderID, payload.Revision))
}

func (e *ProviderEventEmitter) ProviderUnregistered(ctx context.Context, payload capability.ProviderUnregisteredPayload) error {
	return e.publish(ctx, providerUnregistered, 0, payload, event.PublishOptions{
		Domain:        event.EventDomainCapabilityProvider,
		ProducerType:  event.EventProducerTypeCapabilityProvider,
		ProducerID:    providerProducerID,
		AggregateType: "capability_provider",
		AggregateID:   payload.ProviderID.String(),
		OrderingKey:   payload.ProviderID.String(),
	})
}

func (e *ProviderEventEmitter) ProviderInstanceRegistered(ctx context.Context, payload capability.ProviderInstanceEventPayload) error {
	return e.publish(ctx, providerInstanceRegistered, payload.Revision, payload, instanceOptions(payload))
}

func (e *ProviderEventEmitter) ProviderInstanceUpdated(ctx context.Context, payload capability.ProviderInstanceEventPayload) error {
	return e.publish(ctx, providerInstanceUpdated, payload.Revision, payload, instanceOptions(payload))
}

func (e *ProviderEventEmitter) ProviderInstanceUnregistered(ctx context.Context, payload capability.ProviderInstanceEventPayload) error {
	return e.publish(ctx, providerInstanceUnregistered, 0, payload, instanceOptions(payload))
}

func (e *ProviderEventEmitter) ProviderInstanceAvailabilityChanged(ctx context.Context, payload capability.ProviderInstanceAvailabilityChangedPayload) error {
	return e.publish(ctx, providerInstanceAvailabilityChanged, payload.Revision, payload, instanceOptions(payload.ProviderInstanceEventPayload))
}

func (e *ProviderEventEmitter) ProviderInstanceHealthChanged(ctx context.Context, payload capability.ProviderInstanceHealthChangedPayload) error {
	return e.publish(ctx, providerInstanceHealthChanged, payload.Revision, payload, instanceOptions(payload.ProviderInstanceEventPayload))
}

func (e *ProviderEventEmitter) publish(ctx context.Context, typeID event.EventTypeID, revision int64, payload any, opts event.PublishOptions) error {
	_, err := e.publisher.Publish(ctx, typeID, eventVersion, payload, opts)
	return err
}

func definitionOptions(providerID capability.ProviderID, revision int64) event.PublishOptions {
	var rev *int64
	if revision > 0 {
		r := revision
		rev = &r
	}
	return event.PublishOptions{
		Domain:           event.EventDomainCapabilityProvider,
		ProducerType:     event.EventProducerTypeCapabilityProvider,
		ProducerID:       providerProducerID,
		AggregateType:    "capability_provider",
		AggregateID:      providerID.String(),
		AggregateVersion: rev,
		PartitionKey:     "system",
		OrderingKey:      providerID.String(),
	}
}

func instanceOptions(payload capability.ProviderInstanceEventPayload) event.PublishOptions {
	var rev *int64
	if payload.Revision > 0 {
		r := payload.Revision
		rev = &r
	}
	return event.PublishOptions{
		Domain:           event.EventDomainCapabilityProvider,
		ProducerType:     event.EventProducerTypeCapabilityProvider,
		ProducerID:       providerProducerID,
		AggregateType:    "capability_provider_instance",
		AggregateID:      payload.ProviderInstanceID.String(),
		AggregateVersion: rev,
		PartitionKey:     partitionKeyForProvider(payload.UserID),
		OrderingKey:      payload.ProviderInstanceID.String(),
	}
}

func partitionKeyForProvider(userID interface{ String() string }) string {
	if userID != nil && userID.String() != "" {
		return userID.String()
	}
	return "system"
}
