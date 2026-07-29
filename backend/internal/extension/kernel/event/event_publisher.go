package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PublishOptions struct {
	ProducerID           string
	ProducerType         string
	ProducerGeneration   int64
	ProducerExtensionID  string
	ProducerModuleID     string
	AggregateType        string
	AggregateID       string
	AggregateVersion  *int64
	PartitionKey      string
	OrderingKey       string
	ScopeSnapshotID   string
	PermissionSnapshotID string
	TraceID           string
	OperationID       string
	ParentEventID     string
	ParentDepth       int
	Metadata          json.RawMessage
}

type PublishResult struct {
	EventID      string
	OutboxID     string
	Accepted     bool
	RejectReason string
}

type EventPublisher struct {
	schemaRegistry EventTypeRegistry
	outbox         OutboxStore
	db             *sql.DB
	loopGuard      *LoopGuard
	maxDepth       int
}

func NewEventPublisher(schemaRegistry EventTypeRegistry, outbox OutboxStore, db *sql.DB, loopGuard *LoopGuard, maxDepth int) *EventPublisher {
	if maxDepth <= 0 {
		maxDepth = 8
	}
	return &EventPublisher{
		schemaRegistry: schemaRegistry,
		outbox:         outbox,
		db:             db,
		loopGuard:      loopGuard,
		maxDepth:       maxDepth,
	}
}

func (p *EventPublisher) Publish(ctx context.Context, typeID EventTypeID, version int, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	def, err := p.schemaRegistry.GetEventType(ctx, typeID, version)
	if err != nil {
		return PublishResult{}, err
	}
	if err := p.validateProducer(ctx, def, opts); err != nil {
		return PublishResult{}, err
	}
	if int64(len(payload)) > def.MaxPayloadBytes {
		return PublishResult{}, fmt.Errorf("%w: payload %d exceeds %d", ErrInvalidPayload, len(payload), def.MaxPayloadBytes)
	}
	if int64(len(opts.Metadata)) > def.MaxMetadataBytes {
		return PublishResult{}, fmt.Errorf("%w: metadata %d exceeds %d", ErrInvalidPayload, len(opts.Metadata), def.MaxMetadataBytes)
	}
	envelope := NewEventEnvelope(typeID, version, opts.ProducerID, opts.ProducerType, payload)
	envelope = envelope.WithProducer(opts.ProducerID, opts.ProducerType, opts.ProducerGeneration)
	if opts.ProducerExtensionID != "" {
		envelope = envelope.WithProducerDetail(opts.ProducerExtensionID, opts.ProducerModuleID, opts.ProducerGeneration)
	}
	if opts.AggregateType != "" || opts.AggregateID != "" {
		envelope = envelope.WithAggregate(opts.AggregateType, opts.AggregateID, opts.AggregateVersion)
	}
	if opts.PartitionKey != "" {
		envelope = envelope.WithPartition(opts.PartitionKey, opts.OrderingKey)
	}
	if opts.ScopeSnapshotID != "" || opts.PermissionSnapshotID != "" {
		envelope = envelope.WithScope(opts.ScopeSnapshotID, opts.PermissionSnapshotID)
	}
	envelope = envelope.WithTrace(opts.TraceID, opts.OperationID)
	if opts.ParentEventID != "" {
		envelope = envelope.WithParent(opts.ParentEventID, opts.ParentDepth)
	}
	if len(opts.Metadata) > 0 {
		envelope = envelope.WithMetadata(opts.Metadata)
	}
	envelope.DefinitionHash = def.DefinitionHash
	if def.DefinitionHash == "" {
		envelope.DefinitionHash = def.Hash()
	}
	if err := envelope.Validate(def, p.maxDepth); err != nil {
		return PublishResult{}, err
	}
	chainKey := envelope.ChainKey()
	if err := p.loopGuard.Enter(chainKey, string(typeID), opts.ProducerID, envelope.Depth, envelope.TraceID); err != nil {
		return PublishResult{}, err
	}
	defer p.loopGuard.Exit(chainKey)
	if err := p.loopGuard.DetectCycle(chainKey, string(typeID), opts.ProducerID, opts.AggregateID, envelope.IdempotencyKey); err != nil {
		return PublishResult{}, err
	}
	now := time.Now().UTC()
	envelope.PublishedAt = now
	outboxID := fmt.Sprintf("ob-%s", uuid.NewString())
	partitionKey := envelope.PartitionKey
	if partitionKey == "" && opts.AggregateID != "" {
		partitionKey = ComputePartitionKey(opts.AggregateType, opts.AggregateID)
	}
	record := OutboxRecord{
		OutboxID:             outboxID,
		EventID:              envelope.EventID,
		EventTypeID:          envelope.EventTypeID,
		EventVersion:         envelope.EventVersion,
		ProducerID:           envelope.ProducerID,
		ProducerType:         envelope.ProducerType,
		ProducerGeneration:   envelope.ProducerGeneration,
		AggregateType:        envelope.AggregateType,
		AggregateID:          envelope.AggregateID,
		AggregateVersion:     envelope.AggregateVersion,
		PartitionKey:         partitionKey,
		OrderingKey:          envelope.OrderingKey,
		IdempotencyKey:       envelope.IdempotencyKey,
		ScopeSnapshotID:      envelope.ScopeSnapshotID,
		PermissionSnapshotID: envelope.PermissionSnapshotID,
		TraceID:              envelope.TraceID,
		OperationID:         envelope.OperationID,
		ParentEventID:        envelope.ParentEventID,
		Depth:                envelope.Depth,
		OccurredAt:           envelope.OccurredAt,
		PublishedAt:          &now,
		Payload:              envelope.Payload,
		Metadata:             envelope.Metadata,
		PayloadHash:          envelope.PayloadHash,
		DefinitionHash:       envelope.DefinitionHash,
		Status:               OutboxStatusPending,
		AvailableAt:          now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := p.outbox.Enqueue(ctx, record); err != nil {
		return PublishResult{}, fmt.Errorf("event: enqueue outbox: %w", err)
	}
	return PublishResult{
		EventID:  envelope.EventID,
		OutboxID: outboxID,
		Accepted: true,
	}, nil
}

func (p *EventPublisher) PublishTx(ctx context.Context, tx *sql.Tx, typeID EventTypeID, version int, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	def, err := p.schemaRegistry.GetEventType(ctx, typeID, version)
	if err != nil {
		return PublishResult{}, err
	}
	if err := p.validateProducer(ctx, def, opts); err != nil {
		return PublishResult{}, err
	}
	if int64(len(payload)) > def.MaxPayloadBytes {
		return PublishResult{}, fmt.Errorf("%w: payload exceeds limit", ErrInvalidPayload)
	}
	envelope := NewEventEnvelope(typeID, version, opts.ProducerID, opts.ProducerType, payload)
	envelope = envelope.WithProducer(opts.ProducerID, opts.ProducerType, opts.ProducerGeneration)
	if opts.ProducerExtensionID != "" {
		envelope = envelope.WithProducerDetail(opts.ProducerExtensionID, opts.ProducerModuleID, opts.ProducerGeneration)
	}
	if opts.AggregateType != "" || opts.AggregateID != "" {
		envelope = envelope.WithAggregate(opts.AggregateType, opts.AggregateID, opts.AggregateVersion)
	}
	if opts.PartitionKey != "" {
		envelope = envelope.WithPartition(opts.PartitionKey, opts.OrderingKey)
	}
	if opts.ScopeSnapshotID != "" || opts.PermissionSnapshotID != "" {
		envelope = envelope.WithScope(opts.ScopeSnapshotID, opts.PermissionSnapshotID)
	}
	envelope = envelope.WithTrace(opts.TraceID, opts.OperationID)
	if opts.ParentEventID != "" {
		envelope = envelope.WithParent(opts.ParentEventID, opts.ParentDepth)
	}
	if len(opts.Metadata) > 0 {
		envelope = envelope.WithMetadata(opts.Metadata)
	}
	envelope.DefinitionHash = def.DefinitionHash
	if def.DefinitionHash == "" {
		envelope.DefinitionHash = def.Hash()
	}
	if err := envelope.Validate(def, p.maxDepth); err != nil {
		return PublishResult{}, err
	}
	chainKey := envelope.ChainKey()
	if err := p.loopGuard.Enter(chainKey, string(typeID), opts.ProducerID, envelope.Depth, envelope.TraceID); err != nil {
		return PublishResult{}, err
	}
	defer p.loopGuard.Exit(chainKey)
	now := time.Now().UTC()
	envelope.PublishedAt = now
	outboxID := fmt.Sprintf("ob-%s", uuid.NewString())
	partitionKey := envelope.PartitionKey
	if partitionKey == "" && opts.AggregateID != "" {
		partitionKey = ComputePartitionKey(opts.AggregateType, opts.AggregateID)
	}
	record := OutboxRecord{
		OutboxID:             outboxID,
		EventID:              envelope.EventID,
		EventTypeID:          envelope.EventTypeID,
		EventVersion:         envelope.EventVersion,
		ProducerID:           envelope.ProducerID,
		ProducerType:         envelope.ProducerType,
		ProducerGeneration:   envelope.ProducerGeneration,
		AggregateType:        envelope.AggregateType,
		AggregateID:          envelope.AggregateID,
		AggregateVersion:     envelope.AggregateVersion,
		PartitionKey:         partitionKey,
		OrderingKey:          envelope.OrderingKey,
		IdempotencyKey:       envelope.IdempotencyKey,
		ScopeSnapshotID:      envelope.ScopeSnapshotID,
		PermissionSnapshotID: envelope.PermissionSnapshotID,
		TraceID:              envelope.TraceID,
		OperationID:         envelope.OperationID,
		ParentEventID:        envelope.ParentEventID,
		Depth:                envelope.Depth,
		OccurredAt:           envelope.OccurredAt,
		PublishedAt:          &now,
		Payload:              envelope.Payload,
		Metadata:             envelope.Metadata,
		PayloadHash:          envelope.PayloadHash,
		DefinitionHash:       envelope.DefinitionHash,
		Status:               OutboxStatusPending,
		AvailableAt:          now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := p.outbox.EnqueueTx(ctx, tx, record); err != nil {
		return PublishResult{}, fmt.Errorf("event: enqueue outbox tx: %w", err)
	}
	return PublishResult{
		EventID:  envelope.EventID,
		OutboxID: outboxID,
		Accepted: true,
	}, nil
}

func (p *EventPublisher) validateProducer(ctx context.Context, def EventTypeDefinition, opts PublishOptions) error {
	if opts.ProducerID == "" {
		return errors.New("event: producer id required")
	}
	if opts.ProducerType == "" {
		return errors.New("event: producer type required")
	}
	if def.ProducerPolicy.RequireSystemTrust {
		isSystem := opts.ProducerType == "host" || opts.ProducerType == "system"
		if !isSystem {
			return fmt.Errorf("%w: %s requires system trust", ErrProducerDenied, def.EventTypeID)
		}
	}
	if def.ProducerPolicy.RequireNamespaceMatch && opts.ProducerType == "extension" {
		if !def.EventTypeID.IsExtensionNamespace(opts.ProducerID) && !def.EventTypeID.IsHostNamespace() {
			return fmt.Errorf("%w: extension %s cannot publish %s", ErrNamespaceDenied, opts.ProducerID, def.EventTypeID)
		}
	}
	if len(def.ProducerPolicy.AllowedProducers) > 0 {
		allowed := false
		for _, p := range def.ProducerPolicy.AllowedProducers {
			if p == opts.ProducerType || p == opts.ProducerID {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("%w: producer %s not allowed for %s", ErrProducerDenied, opts.ProducerID, def.EventTypeID)
		}
	}
	return nil
}
