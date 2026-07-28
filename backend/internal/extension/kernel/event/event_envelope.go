package event

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type EventEnvelope struct {
	EventID             string
	EventTypeID         EventTypeID
	EventVersion        int
	ProducerID          string
	ProducerType        string
	ProducerGeneration  int64
	AggregateType       string
	AggregateID         string
	AggregateVersion    *int64
	PartitionKey        string
	OrderingKey         string
	IdempotencyKey      string
	ScopeSnapshotID     string
	PermissionSnapshotID string
	CharacterID         string
	ConversationID      string
	ProducerExtensionID string
	ProducerModuleID    string
	TraceID             string
	OperationID         string
	ParentEventID       *string
	Depth               int
	OccurredAt          time.Time
	PublishedAt         time.Time
	Payload             json.RawMessage
	Metadata            json.RawMessage
	PayloadHash         string
	DefinitionHash      string
}

func NewEventEnvelope(typeID EventTypeID, version int, producerID, producerType string, payload json.RawMessage) EventEnvelope {
	now := time.Now().UTC()
	env := EventEnvelope{
		EventID:      newEventID(),
		EventTypeID:  typeID,
		EventVersion: version,
		ProducerID:   producerID,
		ProducerType: producerType,
		OccurredAt:   now,
		Payload:      payload,
		Depth:        0,
	}
	env.PayloadHash = computePayloadHash(payload)
	env.IdempotencyKey = env.computeDefaultIdempotencyKey()
	return env
}

func newEventID() string {
	id := uuid.NewString()
	return fmt.Sprintf("evt-%s", id)
}

func computePayloadHash(payload json.RawMessage) string {
	if len(payload) == 0 {
		return ""
	}
	h := sha256.Sum256(payload)
	return hex.EncodeToString(h[:])
}

func (e EventEnvelope) computeDefaultIdempotencyKey() string {
	if e.AggregateID != "" {
		return fmt.Sprintf("%s:%s:%s:%d", e.EventTypeID, e.AggregateID, e.PayloadHash, e.AggregateVersionOrZero())
	}
	return fmt.Sprintf("%s:%s:%d", e.EventTypeID, e.PayloadHash, e.OccurredAt.UnixNano())
}

func (e EventEnvelope) AggregateVersionOrZero() int64 {
	if e.AggregateVersion == nil {
		return 0
	}
	return *e.AggregateVersion
}

func (e EventEnvelope) WithProducer(producerID, producerType string, generation int64) EventEnvelope {
	e.ProducerID = producerID
	e.ProducerType = producerType
	e.ProducerGeneration = generation
	return e
}

func (e EventEnvelope) WithProducerDetail(extensionID, moduleID string, generation int64) EventEnvelope {
	e.ProducerExtensionID = extensionID
	e.ProducerModuleID = moduleID
	e.ProducerGeneration = generation
	if e.ProducerID == "" {
		e.ProducerID = extensionID
	}
	if e.ProducerType == "" {
		e.ProducerType = "extension"
	}
	return e
}

func (e EventEnvelope) WithContext(characterID, conversationID string) EventEnvelope {
	e.CharacterID = characterID
	e.ConversationID = conversationID
	return e
}

func (e EventEnvelope) WithAggregate(aggType, aggID string, version *int64) EventEnvelope {
	e.AggregateType = aggType
	e.AggregateID = aggID
	e.AggregateVersion = version
	if e.PartitionKey == "" {
		e.PartitionKey = fmt.Sprintf("%s:%s", aggType, aggID)
	}
	e.IdempotencyKey = e.computeDefaultIdempotencyKey()
	return e
}

func (e EventEnvelope) WithPartition(partitionKey, orderingKey string) EventEnvelope {
	e.PartitionKey = partitionKey
	if orderingKey == "" {
		e.OrderingKey = partitionKey
	} else {
		e.OrderingKey = orderingKey
	}
	return e
}

func (e EventEnvelope) WithScope(scopeSnapshotID, permissionSnapshotID string) EventEnvelope {
	e.ScopeSnapshotID = scopeSnapshotID
	e.PermissionSnapshotID = permissionSnapshotID
	return e
}

func (e EventEnvelope) WithTrace(traceID, operationID string) EventEnvelope {
	if traceID == "" {
		traceID = uuid.NewString()
	}
	e.TraceID = traceID
	e.OperationID = operationID
	return e
}

func (e EventEnvelope) WithParent(parentEventID string, parentDepth int) EventEnvelope {
	e.ParentEventID = &parentEventID
	e.Depth = parentDepth + 1
	return e
}

func (e EventEnvelope) WithMetadata(metadata json.RawMessage) EventEnvelope {
	e.Metadata = metadata
	return e
}

func (e EventEnvelope) Validate(def EventTypeDefinition, maxDepth int) error {
	if e.EventID == "" {
		return errors.New("event: event id required")
	}
	if e.EventTypeID == "" {
		return errors.New("event: event type id required")
	}
	if e.EventTypeID != def.EventTypeID {
		return fmt.Errorf("event: type mismatch, expected %s, got %s", def.EventTypeID, e.EventTypeID)
	}
	if e.EventVersion <= 0 {
		return errors.New("event: event version must be positive")
	}
	if e.ProducerID == "" {
		return errors.New("event: producer id required")
	}
	if int64(len(e.Payload)) > def.MaxPayloadBytes {
		return fmt.Errorf("event: payload exceeds %d bytes", def.MaxPayloadBytes)
	}
	if int64(len(e.Metadata)) > def.MaxMetadataBytes {
		return fmt.Errorf("event: metadata exceeds %d bytes", def.MaxMetadataBytes)
	}
	if e.Depth > maxDepth {
		return fmt.Errorf("event: depth %d exceeds max %d", e.Depth, maxDepth)
	}
	if e.PayloadHash == "" {
		return errors.New("event: payload hash required")
	}
	return nil
}

func (e EventEnvelope) IsFromExtension(extensionID string) bool {
	return e.ProducerType == "extension" && e.ProducerID == extensionID
}

func (e EventEnvelope) IsFromHost() bool {
	return e.ProducerType == "host" || e.ProducerType == "system"
}

func (e EventEnvelope) ChainKey() string {
	if e.TraceID != "" {
		return e.TraceID
	}
	return e.EventID
}
