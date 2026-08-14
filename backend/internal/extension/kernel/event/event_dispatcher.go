package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

type DeliveryStore interface {
	CreateDeliveryTx(ctx context.Context, tx *sql.Tx, delivery Delivery) error
	CreateDelivery(ctx context.Context, delivery Delivery) error
	ClaimNextDeliveries(ctx context.Context, owner string, leaseTTL time.Duration, limit int) ([]Delivery, error)
	RenewDeliveryLease(ctx context.Context, deliveryID, owner string, leaseTTL time.Duration) error
	ReleaseDeliveryLease(ctx context.Context, deliveryID string) error
	ReleaseExpiredDeliveryLeases(ctx context.Context) (int, error)
	UpdateDeliveryStatus(ctx context.Context, deliveryID string, status DeliveryStatus, errorCode, errorMessage string) error
	GetDelivery(ctx context.Context, deliveryID string) (Delivery, error)
	ListDeliveries(ctx context.Context, filter DeliveryFilter, limit, offset int) ([]Delivery, error)
	ListDeliveriesByEvent(ctx context.Context, eventID string) ([]Delivery, error)
	ListDeliveriesBySubscription(ctx context.Context, subscriptionID string, limit, offset int) ([]Delivery, error)
	CountDeliveriesByStatus(ctx context.Context, status DeliveryStatus) (int, error)
	CancelPendingByExtension(ctx context.Context, extensionID, reason string) (int, error)
	CancelPendingBySubscription(ctx context.Context, subscriptionID, reason string) (int, error)
}

type DeliveryFilter struct {
	ExtensionID    string
	SubscriptionID string
	Status         DeliveryStatus
	EventID        string
}

type DeadLetterStore interface {
	CreateDeadLetter(ctx context.Context, record DeadLetterRecord) error
	GetDeadLetter(ctx context.Context, deadLetterID string) (DeadLetterRecord, error)
	ListDeadLetters(ctx context.Context, filter DeadLetterFilter, limit, offset int) ([]DeadLetterRecord, error)
	MarkReplayed(ctx context.Context, deadLetterID string) error
	MarkDiscarded(ctx context.Context, deadLetterID string) error
	ListByExtension(ctx context.Context, extensionID string, limit, offset int) ([]DeadLetterRecord, error)
}

type DeadLetterFilter struct {
	ExtensionID    string
	SubscriptionID string
	Reason         DeadLetterReason
	Status         DeadLetterStatus
}

type DeliveryPlanner struct {
	subscriptionRegistry *EventSubscriptionRegistry
	schemaRegistry       EventTypeRegistry
	deliveryStore        DeliveryStore
	deadLetterStore      DeadLetterStore
	outboxStore          OutboxStore
	sequenceAllocator    *SequenceAllocator
	traceRecorder        *EventTraceRecorder
}

func NewDeliveryPlanner(
	subscriptionRegistry *EventSubscriptionRegistry,
	schemaRegistry EventTypeRegistry,
	deliveryStore DeliveryStore,
	deadLetterStore DeadLetterStore,
	outboxStore OutboxStore,
	traceRecorder *EventTraceRecorder,
) *DeliveryPlanner {
	return &DeliveryPlanner{
		subscriptionRegistry: subscriptionRegistry,
		schemaRegistry:       schemaRegistry,
		deliveryStore:        deliveryStore,
		deadLetterStore:      deadLetterStore,
		outboxStore:          outboxStore,
		sequenceAllocator:    NewSequenceAllocator(),
		traceRecorder:        traceRecorder,
	}
}

func (p *DeliveryPlanner) PlanDeliveries(ctx context.Context, record OutboxRecord) error {
	envelope := outboxToEnvelope(record)
	subs, err := p.subscriptionRegistry.ResolveForDelivery(ctx, envelope)
	if err != nil {
		return err
	}
	if len(subs) == 0 {
		return p.outboxStore.MarkDispatched(ctx, record.OutboxID)
	}
	for _, sub := range subs {
		if !sub.Effective.IsActive() {
			delivery := Delivery{
				DeliveryID:             newDeliveryID(),
				EventID:                envelope.EventID,
				SubscriptionID:         sub.Definition.ContributionID,
				ExtensionID:            sub.Definition.ExtensionID,
				ModuleID:               sub.Definition.ModuleID,
				Status:                 DeliveryStatusSkipped,
				PartitionKey:           envelope.PartitionKey,
				OrderingKey:            envelope.OrderingKey,
				MaxAttempts:            sub.Definition.RetryPolicy.MaxAttempts,
				AvailableAt:            time.Now().UTC(),
				ErrorCode:              "subscription_inactive",
				ErrorMessage:           sub.Effective.DenyReason(),
				SubscriptionGeneration: sub.Definition.Generation,
				TargetGeneration:       sub.Definition.Generation,
				ProducerGeneration:     envelope.ProducerGeneration,
				CreatedAt:              time.Now().UTC(),
				UpdatedAt:              time.Now().UTC(),
			}
			now := time.Now().UTC()
			delivery.FinishedAt = &now
			_ = p.deliveryStore.CreateDelivery(ctx, delivery)
			continue
		}
		grantedPermissions := p.extractGrantedPermissions(sub)
		projection, err := sub.Projector.Project(envelope.Payload, sub.Definition.Projection, grantedPermissions)
		if err != nil {
			return fmt.Errorf("event: projection failed for %s: %w", sub.Definition.ContributionID, err)
		}
		sequence := int64(0)
		if sub.Definition.OrderingRequirement == OrderingPerPartition && envelope.PartitionKey != "" {
			sequence = p.sequenceAllocator.Next(envelope.PartitionKey)
		}
		delivery := Delivery{
			DeliveryID:             newDeliveryID(),
			EventID:                envelope.EventID,
			SubscriptionID:         sub.Definition.ContributionID,
			ExtensionID:            sub.Definition.ExtensionID,
			ModuleID:               sub.Definition.ModuleID,
			Status:                 DeliveryStatusPending,
			PartitionKey:           envelope.PartitionKey,
			OrderingKey:            envelope.OrderingKey,
			Sequence:               sequence,
			Attempt:                0,
			MaxAttempts:            sub.Definition.RetryPolicy.MaxAttempts,
			AvailableAt:            time.Now().UTC(),
			ScopeSnapshotID:        envelope.ScopeSnapshotID,
			PermissionSnapshotID:   envelope.PermissionSnapshotID,
			ProjectedPayloadHash:   projection.Hash,
			SubscriptionGeneration: sub.Definition.Generation,
			TargetGeneration:       sub.Definition.Generation,
			ProducerGeneration:     envelope.ProducerGeneration,
			CreatedAt:              time.Now().UTC(),
			UpdatedAt:              time.Now().UTC(),
		}
		if err := p.deliveryStore.CreateDelivery(ctx, delivery); err != nil {
			return fmt.Errorf("event: create delivery: %w", err)
		}
	}
	return p.outboxStore.MarkDispatched(ctx, record.OutboxID)
}

func (p *DeliveryPlanner) extractGrantedPermissions(sub *ResolvedSubscription) map[string]bool {
	result := make(map[string]bool)
	for _, req := range sub.Definition.PermissionRequirements {
		result[req.Permission] = true
	}
	return result
}

func outboxToEnvelope(record OutboxRecord) EventEnvelope {
	return EventEnvelope{
		EventID:              record.EventID,
		EventTypeID:          record.EventTypeID,
		EventVersion:         record.EventVersion,
		ProducerID:           record.ProducerID,
		ProducerType:         record.ProducerType,
		ProducerGeneration:   record.ProducerGeneration,
		Domain:               record.Domain,
		CausationID:          record.CausationID,
		AggregateType:        record.AggregateType,
		AggregateID:          record.AggregateID,
		AggregateVersion:     record.AggregateVersion,
		PartitionKey:         record.PartitionKey,
		OrderingKey:          record.OrderingKey,
		IdempotencyKey:       record.IdempotencyKey,
		ScopeSnapshotID:      record.ScopeSnapshotID,
		PermissionSnapshotID: record.PermissionSnapshotID,
		TraceID:              record.TraceID,
		OperationID:          record.OperationID,
		ParentEventID:        record.ParentEventID,
		Depth:                record.Depth,
		OccurredAt:           record.OccurredAt,
		Payload:              record.Payload,
		Metadata:             record.Metadata,
		PayloadHash:          record.PayloadHash,
		DefinitionHash:       record.DefinitionHash,
	}
}

type DispatcherConfig struct {
	BatchSize               int
	LeaseTTL                time.Duration
	PollInterval            time.Duration
	GlobalConcurrency       int
	PerExtensionConcurrency int
	PerPartitionConcurrency int
	MaxQueueLength          int
}

func DefaultDispatcherConfig() DispatcherConfig {
	return DispatcherConfig{
		BatchSize:               100,
		LeaseTTL:                60 * time.Second,
		PollInterval:            500 * time.Millisecond,
		GlobalConcurrency:       32,
		PerExtensionConcurrency: 8,
		PerPartitionConcurrency: 1,
		MaxQueueLength:          10000,
	}
}

type DeliveryHandler func(ctx context.Context, delivery Delivery, envelope EventEnvelope, sub *ResolvedSubscription) error

type Dispatcher struct {
	outboxStore          OutboxStore
	deliveryStore        DeliveryStore
	deadLetterStore      DeadLetterStore
	subscriptionRegistry *EventSubscriptionRegistry
	schemaRegistry       EventTypeRegistry
	ordering             *OrderingCoordinator
	circuitRegistry      *CircuitBreakerRegistry
	traceRecorder        *EventTraceRecorder
	generationResolver   GenerationResolver
	config               DispatcherConfig
	handler              DeliveryHandler
	mu                   sync.Mutex
	stopCh               chan struct{}
	stopped              bool
	wg                   sync.WaitGroup
}

func NewDispatcher(
	outboxStore OutboxStore,
	deliveryStore DeliveryStore,
	deadLetterStore DeadLetterStore,
	subscriptionRegistry *EventSubscriptionRegistry,
	schemaRegistry EventTypeRegistry,
	traceRecorder *EventTraceRecorder,
	config DispatcherConfig,
	handler DeliveryHandler,
) *Dispatcher {
	return &Dispatcher{
		outboxStore:          outboxStore,
		deliveryStore:        deliveryStore,
		deadLetterStore:      deadLetterStore,
		subscriptionRegistry: subscriptionRegistry,
		schemaRegistry:       schemaRegistry,
		ordering:             NewOrderingCoordinator(config.PerPartitionConcurrency),
		circuitRegistry:      NewCircuitBreakerRegistry(DefaultCircuitConfig()),
		traceRecorder:        traceRecorder,
		config:               config,
		handler:              handler,
		stopCh:               make(chan struct{}),
	}
}

func (d *Dispatcher) Start(ctx context.Context) {
	d.wg.Add(2)
	go d.runOutboxDispatcher(ctx)
	go d.runDeliveryDispatcher(ctx)
}

func (d *Dispatcher) Stop() {
	d.mu.Lock()
	if d.stopped {
		d.mu.Unlock()
		return
	}
	d.stopped = true
	close(d.stopCh)
	d.mu.Unlock()
	d.wg.Wait()
}

func (d *Dispatcher) runOutboxDispatcher(ctx context.Context) {
	defer d.wg.Done()
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = d.outboxStore.ReleaseExpiredLeases(ctx)
			records, err := d.outboxStore.ClaimNext(ctx, "outbox-dispatcher", d.config.LeaseTTL, d.config.BatchSize)
			if err != nil {
				continue
			}
			for _, record := range records {
				planner := NewDeliveryPlanner(d.subscriptionRegistry, d.schemaRegistry, d.deliveryStore, d.deadLetterStore, d.outboxStore, d.traceRecorder)
				if err := planner.PlanDeliveries(ctx, record); err != nil {
					_ = d.outboxStore.MarkFailed(ctx, record.OutboxID, "dispatch_error", err.Error())
				}
			}
		}
	}
}

func (d *Dispatcher) runDeliveryDispatcher(ctx context.Context) {
	defer d.wg.Done()
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = d.deliveryStore.ReleaseExpiredDeliveryLeases(ctx)
			deliveries, err := d.deliveryStore.ClaimNextDeliveries(ctx, "delivery-dispatcher", d.config.LeaseTTL, d.config.BatchSize)
			if err != nil {
				continue
			}
			sem := make(chan struct{}, d.config.GlobalConcurrency)
			for _, delivery := range deliveries {
				sem <- struct{}{}
				d.wg.Add(1)
				go func(del Delivery) {
					defer d.wg.Done()
					defer func() { <-sem }()
					d.executeDelivery(ctx, del)
				}(delivery)
			}
		}
	}
}

func (d *Dispatcher) executeDelivery(ctx context.Context, delivery Delivery) {
	sub, ok := d.subscriptionRegistry.Get(ctx, delivery.SubscriptionID)
	if !ok {
		_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, DeliveryStatusSkipped, "handler_not_found", "subscription not found")
		return
	}
	if sub.Definition.Generation != delivery.SubscriptionGeneration {
		_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, DeliveryStatusCancelled, "cancel_stale_subscription",
			fmt.Sprintf("subscription generation %d != delivery generation %d", sub.Definition.Generation, delivery.SubscriptionGeneration))
		return
	}
	if delivery.TargetGeneration != 0 && sub.Definition.Generation != delivery.TargetGeneration {
		_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, DeliveryStatusCancelled, "cancel_stale_target",
			fmt.Sprintf("target generation %d != delivery target generation %d", sub.Definition.Generation, delivery.TargetGeneration))
		return
	}
	outbox, err := d.outboxStore.GetByEventID(ctx, delivery.EventID)
	if err != nil {
		_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, DeliveryStatusRetryWait, "outbox_not_found", err.Error())
		return
	}
	envelope := outboxToEnvelope(outbox)
	if delivery.ProducerGeneration != 0 && d.generationResolver != nil {
		producerID := envelope.ProducerExtensionID
		if producerID == "" {
			producerID = envelope.ProducerID
		}
		if producerID != "" {
			currentProducerGen, genErr := d.generationResolver.CurrentGeneration(ctx, producerID)
			if genErr != nil {
				inv := d.traceRecorder.StartInvocation(envelope.OperationID, delivery.EventID, delivery.DeliveryID, delivery.SubscriptionID, delivery.Attempt)
				d.traceRecorder.FinishInvocation(inv.InvocationID, "failed", "generation_check_error", genErr.Error())
				_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, DeliveryStatusRetryWait, "generation_check_error", genErr.Error())
				return
			}
			if currentProducerGen > 0 && currentProducerGen != delivery.ProducerGeneration {
				_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, DeliveryStatusCancelled, "cancel_stale_producer",
					fmt.Sprintf("producer generation %d != delivery producer generation %d", currentProducerGen, delivery.ProducerGeneration))
				return
			}
		}
	}
	circuit := d.circuitRegistry.GetOrCreate(delivery.SubscriptionID)
	allowed, state := circuit.Allow()
	if !allowed {
		_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, DeliveryStatusSkipped, "circuit_open", fmt.Sprintf("circuit %s", state))
		d.subscriptionRegistry.MarkCircuitState(delivery.SubscriptionID, state)
		return
	}
	if !sub.Effective.IsActive() {
		_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, DeliveryStatusSkipped, "subscription_inactive", sub.Effective.DenyReason())
		return
	}
	if envelope.ScopeSnapshotID == "" && !envelope.IsFromHost() {
		_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, DeliveryStatusSkipped, "scope_snapshot_missing", "envelope has no scope snapshot")
		return
	}
	if delivery.PartitionKey != "" {
		if !d.ordering.AcquireSlot(delivery.PartitionKey, delivery.DeliveryID) {
			_ = d.deliveryStore.ReleaseDeliveryLease(ctx, delivery.DeliveryID)
			return
		}
		defer d.ordering.ReleaseSlot(delivery.PartitionKey, delivery.DeliveryID)
	}
	timeout := sub.Definition.Timeout
	if timeout == 0 {
		timeout = 5 * time.Second
	}
	callCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	delivery.Start("delivery-dispatcher")
	inv := d.traceRecorder.StartInvocation("", delivery.EventID, delivery.DeliveryID, delivery.SubscriptionID, delivery.Attempt)
	err = d.handler(callCtx, delivery, envelope, sub)
	if err == nil {
		delivery.Succeed()
		circuit.RecordSuccess()
		d.traceRecorder.FinishInvocation(inv.InvocationID, "succeeded", "", "")
		_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, DeliveryStatusSucceeded, "", "")
		return
	}
	code := ErrorCode(err)
	circuit.RecordFailure(code)
	d.traceRecorder.FinishInvocation(inv.InvocationID, "failed", code, err.Error())
	delivery.Fail(code, err.Error(), sub.Definition.RetryPolicy)
	if delivery.Status == DeliveryStatusDeadLetter {
		dlRecord := NewDeadLetterRecord(delivery, envelope, sub.Definition, DeadLetterMaxAttempts)
		_ = d.deadLetterStore.CreateDeadLetter(ctx, dlRecord)
	}
	_ = d.deliveryStore.UpdateDeliveryStatus(ctx, delivery.DeliveryID, delivery.Status, delivery.ErrorCode, delivery.ErrorMessage)
}

func (d *Dispatcher) ResetCircuit(subscriptionID string) {
	d.circuitRegistry.Reset(subscriptionID)
	d.subscriptionRegistry.MarkCircuitState(subscriptionID, CircuitClosed)
}

func (d *Dispatcher) SetHandler(handler DeliveryHandler) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handler = handler
}

func (d *Dispatcher) SetGenerationResolver(resolver GenerationResolver) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.generationResolver = resolver
}

func (d *Dispatcher) LookupCircuitState(subscriptionID string) CircuitState {
	cb, ok := d.circuitRegistry.Get(subscriptionID)
	if !ok {
		return CircuitClosed
	}
	return cb.State()
}

func (d *Dispatcher) CircuitStats() map[string]CircuitStats {
	return d.circuitRegistry.All()
}

var _ = errors.New
var _ = json.Marshal
