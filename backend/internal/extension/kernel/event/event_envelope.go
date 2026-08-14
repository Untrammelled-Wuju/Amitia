package event

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

type EventProducerType string

const (
	EventProducerTypeSystem             EventProducerType = "system"
	EventProducerTypeExtension          EventProducerType = "extension"
	EventProducerTypeDevice             EventProducerType = "device"
	EventProducerTypeRuntime            EventProducerType = "runtime"
	EventProducerTypeTask               EventProducerType = "task"
	EventProducerTypeCapabilityProvider EventProducerType = "capability_provider"
	EventProducerTypeSync               EventProducerType = "sync"
)

func (t EventProducerType) String() string {
	return string(t)
}

func (t EventProducerType) IsValid() bool {
	switch t {
	case EventProducerTypeSystem, EventProducerTypeExtension, EventProducerTypeDevice,
		EventProducerTypeRuntime, EventProducerTypeTask, EventProducerTypeCapabilityProvider,
		EventProducerTypeSync:
		return true
	}
	return false
}

func ParseEventProducerType(raw string) EventProducerType {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case "host":
		return EventProducerTypeSystem
	case "system":
		return EventProducerTypeSystem
	case "extension":
		return EventProducerTypeExtension
	case "device":
		return EventProducerTypeDevice
	case "runtime":
		return EventProducerTypeRuntime
	case "task":
		return EventProducerTypeTask
	case "capability_provider":
		return EventProducerTypeCapabilityProvider
	case "sync":
		return EventProducerTypeSync
	}
	return EventProducerTypeSystem
}

func ParseProducerType(raw string) EventProducerType {
	return ParseEventProducerType(raw)
}

// EventEnvelope 是后续 Device/Task/Sync 可靠事件的公共 envelope 基线。
// 后续不得另建独立的 envelope 类型作为跨设备事件权威结构。
type EventEnvelope struct {
	EventID              string
	EventTypeID          EventTypeID
	EventVersion         int
	ProducerID           string
	ProducerType         EventProducerType
	ProducerGeneration   int64
	Domain               EventDomain
	CausationID          string
	AggregateType        string
	AggregateID          string
	AggregateVersion     *int64
	PartitionKey         string
	OrderingKey          string
	IdempotencyKey       string
	ScopeSnapshotID      string
	PermissionSnapshotID string
	CharacterID          string
	ConversationID       string
	ProducerExtensionID  string
	ProducerModuleID     string
	TraceID              string
	OperationID          string
	ParentEventID        *string
	Depth                int
	OccurredAt           time.Time
	PublishedAt          time.Time
	Payload              json.RawMessage
	Metadata             json.RawMessage
	PayloadHash          string
	DefinitionHash       string
}

func NewEventEnvelope(typeID EventTypeID, version int, producerID string, producerType EventProducerType, payload json.RawMessage) EventEnvelope {
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
		return BuildIdempotencyKey(string(e.EventTypeID), e.AggregateID, e.PayloadHash, fmt.Sprintf("%d", e.AggregateVersionOrZero()))
	}
	return BuildIdempotencyKey(string(e.EventTypeID), e.PayloadHash, fmt.Sprintf("%d", e.OccurredAt.UnixNano()))
}

func (e EventEnvelope) AggregateVersionOrZero() int64 {
	if e.AggregateVersion == nil {
		return 0
	}
	return *e.AggregateVersion
}

func (e EventEnvelope) WithProducer(producerID string, producerType EventProducerType, generation int64) EventEnvelope {
	e.ProducerID = producerID
	e.ProducerType = producerType
	e.ProducerGeneration = generation
	return e
}

func (e EventEnvelope) WithProducerString(producerID, producerType string, generation int64) EventEnvelope {
	return e.WithProducer(producerID, ParseProducerType(producerType), generation)
}

func (e EventEnvelope) WithProducerDetail(extensionID, moduleID string, generation int64) EventEnvelope {
	e.ProducerExtensionID = extensionID
	e.ProducerModuleID = moduleID
	e.ProducerGeneration = generation
	if e.ProducerID == "" {
		e.ProducerID = extensionID
	}
	if e.ProducerType == "" {
		e.ProducerType = EventProducerTypeExtension
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
	return e.WithCausation("", parentEventID, parentDepth)
}

func (e EventEnvelope) WithCausation(causationID, parentEventID string, parentDepth int) EventEnvelope {
	e.CausationID = strings.TrimSpace(causationID)
	if parentEventID != "" {
		e.ParentEventID = &parentEventID
		e.Depth = parentDepth + 1
	}
	return e
}

func (e EventEnvelope) WithDomain(domain EventDomain) EventEnvelope {
	e.Domain = domain
	return e
}

func (e EventEnvelope) EffectiveDomain() EventDomain {
	if e.Domain != "" {
		if e.Domain.IsValid() {
			return e.Domain
		}
	}
	if domain := e.EventTypeID.Domain(); domain != "" {
		return domain
	}
	switch e.ProducerType {
	case EventProducerTypeExtension:
		return EventDomainExtension
	case EventProducerTypeDevice:
		return EventDomainDevice
	case EventProducerTypeRuntime:
		return EventDomainRuntime
	case EventProducerTypeTask:
		return EventDomainTask
	case EventProducerTypeCapabilityProvider:
		return EventDomainCapabilityProvider
	case EventProducerTypeSync:
		return EventDomainSync
	}
	return EventDomainSystem
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
	if e.ProducerType != "" && !e.ProducerType.IsValid() {
		return fmt.Errorf("event: invalid producer type: %s", e.ProducerType)
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
	if e.Domain != "" && !e.Domain.IsValid() {
		return fmt.Errorf("event: invalid domain: %s", e.Domain)
	}
	return nil
}

func (e EventEnvelope) IsFromExtension(extensionID string) bool {
	return e.ProducerType == EventProducerTypeExtension && e.ProducerID == extensionID
}

func (e EventEnvelope) IsFromHost() bool {
	normalized := ParseEventProducerType(e.ProducerType.String())
	return normalized == EventProducerTypeSystem
}

func (e EventEnvelope) IsFromSystem() bool {
	normalized := ParseEventProducerType(e.ProducerType.String())
	return normalized == EventProducerTypeSystem
}

func (e EventEnvelope) ChainKey() string {
	if e.TraceID != "" {
		return e.TraceID
	}
	return e.EventID
}
