package javascript_main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type EventDeliveryAdapter struct {
	factory *RuntimeFactory
}

func NewEventDeliveryAdapter(factory *RuntimeFactory) *EventDeliveryAdapter {
	return &EventDeliveryAdapter{factory: factory}
}

func (a *EventDeliveryAdapter) HandleDelivery(ctx context.Context, delivery event.Delivery, envelope event.EventEnvelope, sub *event.ResolvedSubscription) error {
	if a.factory == nil {
		return fmt.Errorf("javascript_main: runtime factory not available")
	}

	hosts := a.factory.List()
	var targetHost *PluginHost
	for _, h := range hosts {
		if h.ExtensionID() == delivery.ExtensionID && h.ModuleID() == delivery.ModuleID && h.State() == HostStateReady {
			targetHost = h
			break
		}
	}
	if targetHost == nil {
		return fmt.Errorf("javascript_main: no ready runtime for extension %s module %s", delivery.ExtensionID, delivery.ModuleID)
	}

	entryName := sub.Definition.Entry
	if entryName == "" {
		return fmt.Errorf("javascript_main: subscription entry is empty")
	}

	handler, err := targetHost.Handlers().Get(HandlerTypeEvent, entryName)
	if err != nil {
		return fmt.Errorf("javascript_main: event handler not found for entry %s: %w", entryName, err)
	}

	input := EventDeliveryInput{
		EventID:        string(envelope.EventID),
		EventTypeID:    string(envelope.EventTypeID),
		EventVersion:   envelope.EventVersion,
		Payload:        envelope.Payload,
		Metadata:       envelope.Metadata,
		TraceID:        envelope.TraceID,
		OperationID:    envelope.OperationID,
		ParentEventID:  "",
		Depth:          envelope.Depth,
		OccurredAt:     envelope.OccurredAt,
		DeliveryID:     delivery.DeliveryID,
		SubscriptionID: delivery.SubscriptionID,
		Attempt:        delivery.Attempt,
	}
	if envelope.ParentEventID != nil {
		input.ParentEventID = *envelope.ParentEventID
	}

	timeout := sub.Definition.Timeout
	if timeout <= 0 {
		timeout = 5000
	}
	deadline := time.Now().Add(time.Duration(timeout) * time.Millisecond)

	invocationID := fmt.Sprintf("evt-%s-%d", delivery.DeliveryID, delivery.Attempt)
	result := targetHost.Dispatcher().Dispatch(ctx, HandlerTypeEvent, entryName, input, invocationID, deadline, handler)

	if result.Status == InvocationStatusSucceeded {
		return nil
	}
	if result.Error != "" {
		return fmt.Errorf("javascript_main: event handler failed: %s", result.Error)
	}
	return fmt.Errorf("javascript_main: event handler status: %s", result.Status)
}

type EventDeliveryInput struct {
	EventID        string          `json:"eventId"`
	EventTypeID    string          `json:"eventTypeId"`
	EventVersion   int             `json:"eventVersion"`
	Payload        json.RawMessage `json:"payload"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`
	TraceID        string          `json:"traceId,omitempty"`
	OperationID    string          `json:"operationId,omitempty"`
	ParentEventID  string          `json:"parentEventId,omitempty"`
	Depth          int             `json:"depth"`
	OccurredAt     time.Time       `json:"occurredAt"`
	DeliveryID     string          `json:"deliveryId"`
	SubscriptionID string          `json:"subscriptionId"`
	Attempt        int             `json:"attempt"`
}
