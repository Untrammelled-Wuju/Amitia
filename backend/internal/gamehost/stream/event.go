package stream

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/gamehost/domain"
)

type EventEnvelope struct {
	ID          string
	TypeID      string
	Version     int
	PluginID    domain.PluginID
	RuntimeID   domain.RuntimeInstanceID
	ServiceID   domain.ServiceID
	Method      string
	Payload     json.RawMessage
	Metadata    map[string]json.RawMessage
	TraceID     string
	OccurredAt   int64
}

type PublishEventOption func(*publishEventConfig)

type publishEventConfig struct {
	producerType  string
	aggregateType string
	aggregateID   string
	partitionKey  string
	orderingKey   string
	parentEventID string
	parentDepth   int
}

func WithProducerType(t string) PublishEventOption {
	return func(c *publishEventConfig) {
		c.producerType = t
	}
}

func WithPartitionKey(key string) PublishEventOption {
	return func(c *publishEventConfig) {
		c.partitionKey = key
	}
}

func WithParentEvent(parentEventID string, parentDepth int) PublishEventOption {
	return func(c *publishEventConfig) {
		c.parentEventID = parentEventID
		c.parentDepth = parentDepth
	}
}

type EventPublisher interface {
	PublishEvent(
		ctx context.Context,
		event EventEnvelope,
		opts ...PublishEventOption,
	) error
}
