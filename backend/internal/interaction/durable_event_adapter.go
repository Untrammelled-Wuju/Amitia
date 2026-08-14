package interaction

import (
	"encoding/json"

	"github.com/u-ai/backend/internal/extension/kernel/event"
)

type DurableEventAdapterOptions struct {
	TypeID        event.EventTypeID
	Version       int
	ProducerID    string
	ProducerType  event.EventProducerType
	Payload       json.RawMessage
	Metadata      json.RawMessage
	AggregateType string
	AggregateID   string
}

func BuildDurableEventInput(
	envelope EventEnvelope,
	opts DurableEventAdapterOptions,
) event.DurableEventInput {
	input := event.DurableEventInput{
		TypeID:        opts.TypeID,
		Version:       opts.Version,
		ProducerID:    opts.ProducerID,
		ProducerType:  opts.ProducerType,
		Payload:       opts.Payload,
		Metadata:      opts.Metadata,
		AggregateType: opts.AggregateType,
		AggregateID:   opts.AggregateID,
		Domain:        event.EventDomainInteraction,
	}
	if envelope.Causation.CorrelationID != "" {
		input.TraceID = envelope.Causation.CorrelationID
	}
	if envelope.Causation.CausationID != "" {
		input.CausationID = envelope.Causation.CausationID
	}
	if envelope.Causation.ParentEventID != "" {
		input.ParentEventID = envelope.Causation.ParentEventID
	}
	if envelope.IdempotencyKey != "" {
		input.IdempotencyKey = envelope.IdempotencyKey
	}
	if envelope.StateVersion != 0 && envelope.Causation.CorrelationID != "" {
		v := envelope.StateVersion
		input.AggregateVersion = &v
	}
	return input
}
