package event

import (
	"context"
	"database/sql"
	"encoding/json"
)

type DurableEventPublisher interface {
	Publish(
		ctx context.Context,
		typeID EventTypeID,
		version int,
		payload json.RawMessage,
		opts PublishOptions,
	) (PublishResult, error)

	PublishTx(
		ctx context.Context,
		tx *sql.Tx,
		typeID EventTypeID,
		version int,
		payload json.RawMessage,
		opts PublishOptions,
	) (PublishResult, error)
}

type DurableEventReader interface {
	GetOutboxRecord(
		ctx context.Context,
		outboxID string,
	) (OutboxRecord, error)

	GetOutboxByEventID(
		ctx context.Context,
		eventID string,
	) (OutboxRecord, error)

	GetDelivery(
		ctx context.Context,
		deliveryID string,
	) (Delivery, error)
}

type DurableReplayPort interface {
	ReplayDeadLetter(
		ctx context.Context,
		req ReplayRequest,
	) error

	DiscardDeadLetter(
		ctx context.Context,
		deadLetterID string,
	) error
}

type DurableEventInput struct {
	TypeID           EventTypeID
	Version          int
	Domain           EventDomain
	ProducerID       string
	ProducerType     EventProducerType
	Payload          json.RawMessage
	AggregateType    string
	AggregateID      string
	AggregateVersion *int64
	PartitionKey     string
	OrderingKey      string
	IdempotencyKey   string
	TraceID          string
	OperationID      string
	CausationID      string
	ParentEventID    string
	ParentDepth      int
	Metadata         json.RawMessage
}

func BuildDurableEvent(input DurableEventInput) EventEnvelope {
	env := NewEventEnvelope(input.TypeID, input.Version, input.ProducerID, input.ProducerType, input.Payload)
	if input.Domain != "" {
		env = env.WithDomain(input.Domain)
	}
	if input.AggregateType != "" || input.AggregateID != "" {
		env = env.WithAggregate(input.AggregateType, input.AggregateID, input.AggregateVersion)
	}
	if input.PartitionKey != "" {
		env = env.WithPartition(input.PartitionKey, input.OrderingKey)
	}
	if input.TraceID != "" || input.OperationID != "" {
		env = env.WithTrace(input.TraceID, input.OperationID)
	}
	if input.CausationID != "" || input.ParentEventID != "" {
		env = env.WithCausation(input.CausationID, input.ParentEventID, input.ParentDepth)
	}
	if len(input.Metadata) > 0 {
		env = env.WithMetadata(input.Metadata)
	}
	if input.IdempotencyKey != "" {
		env.IdempotencyKey = input.IdempotencyKey
	}
	return env
}
