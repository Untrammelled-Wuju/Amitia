package eventbridge

import (
	"context"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

const (
	rtSessionAcquired     event.EventTypeID = "runtime.session.acquired"
	rtSessionReady        event.EventTypeID = "runtime.session.ready"
	rtSessionSuperseded   event.EventTypeID = "runtime.session.superseded"
	rtSessionDisconnected event.EventTypeID = "runtime.session.disconnected"
	rtSessionClosed       event.EventTypeID = "runtime.session.closed"
	rtSessionExpired      event.EventTypeID = "runtime.session.expired"

	devicePresenceReady        event.EventTypeID = "device.presence.ready"
	devicePresenceDisconnected event.EventTypeID = "device.presence.disconnected"

	providerRegistered   event.EventTypeID = "capability_provider.registered"
	providerUpdated      event.EventTypeID = "capability_provider.updated"
	providerUnregistered event.EventTypeID = "capability_provider.unregistered"

	providerInstanceRegistered          event.EventTypeID = "capability_provider.instance_registered"
	providerInstanceUpdated             event.EventTypeID = "capability_provider.instance_updated"
	providerInstanceUnregistered        event.EventTypeID = "capability_provider.instance_unregistered"
	providerInstanceAvailabilityChanged event.EventTypeID = "capability_provider.instance_availability_changed"
	providerInstanceHealthChanged       event.EventTypeID = "capability_provider.instance_health_changed"

	taskRunCreated                        event.EventTypeID = "task.run.created"
	taskRunQueued                         event.EventTypeID = "task.run.queued"
	taskExecutionTargetBound              event.EventTypeID = "task.execution_target_bound"
	taskExecutionConnectionBindingChanged event.EventTypeID = "task.execution_connection_binding_changed"
	taskExecutionAttemptStarted           event.EventTypeID = "task.execution_attempt_started"
	taskRunRunning                        event.EventTypeID = "task.run.running"
	taskRunSucceeded                      event.EventTypeID = "task.run.succeeded"
	taskRunFailed                         event.EventTypeID = "task.run.failed"
	taskRunCancelled                      event.EventTypeID = "task.run.cancelled"
	taskRunPaused                         event.EventTypeID = "task.run.paused"
	taskRunTimedOut                       event.EventTypeID = "task.run.timed_out"
	taskRunRecoveryRequired               event.EventTypeID = "task.run.recovery_required"
)

const eventVersion = 1

const (
	maxPayloadBytes  int64 = 64 << 10
	maxMetadataBytes int64 = 4 << 10
)

var producerPolicy = event.EventProducerPolicy{
	MaxPayloadBytes:  maxPayloadBytes,
	MaxMetadataBytes: maxMetadataBytes,
}

var deliveryPolicy = event.EventDeliveryPolicy{
	OrderingRequirement: event.OrderingPerAggregate,
}

func RegisterCloudFoundationEventTypes(
	ctx context.Context,
	registry event.EventTypeRegistry,
) error {
	types := []event.EventTypeID{
		rtSessionAcquired,
		rtSessionReady,
		rtSessionSuperseded,
		rtSessionDisconnected,
		rtSessionClosed,
		rtSessionExpired,
		devicePresenceReady,
		devicePresenceDisconnected,
		providerRegistered,
		providerUpdated,
		providerUnregistered,
		providerInstanceRegistered,
		providerInstanceUpdated,
		providerInstanceUnregistered,
		providerInstanceAvailabilityChanged,
		providerInstanceHealthChanged,
		taskRunCreated,
		taskRunQueued,
		taskExecutionTargetBound,
		taskExecutionConnectionBindingChanged,
		taskExecutionAttemptStarted,
		taskRunRunning,
		taskRunSucceeded,
		taskRunFailed,
		taskRunCancelled,
		taskRunPaused,
		taskRunTimedOut,
		taskRunRecoveryRequired,
	}
	for _, id := range types {
		if err := registry.RegisterEventType(ctx, event.EventTypeDefinition{
			EventTypeID:      id,
			Version:          eventVersion,
			Description:      string(id),
			ProducerPolicy:   producerPolicy,
			DeliveryPolicy:   deliveryPolicy,
			MaxPayloadBytes:  maxPayloadBytes,
			MaxMetadataBytes: maxMetadataBytes,
		}); err != nil {
			return err
		}
	}
	return nil
}
