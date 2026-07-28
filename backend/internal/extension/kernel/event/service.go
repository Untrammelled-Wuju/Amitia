package event

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"
)

type ServiceConfig struct {
	DB              *sql.DB
	MaxDepth        int
	MaxChainLength  int
	MaxSubscribers  int
	TraceMaxEntries int
	Dispatcher      DispatcherConfig
}

func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		MaxDepth:        8,
		MaxChainLength:  32,
		MaxSubscribers:  64,
		TraceMaxEntries: 10000,
		Dispatcher:      DefaultDispatcherConfig(),
	}
}

func (c ServiceConfig) WithDB(db *sql.DB) ServiceConfig {
	c.DB = db
	return c
}

type Service struct {
	mu                 sync.RWMutex
	config             ServiceConfig
	db                 *sql.DB
	schemaRegistry     *EventSchemaRegistry
	outboxRepo         *OutboxRepository
	deliveryStore      *SQLiteDeliveryStore
	deadLetterStore    *SQLiteDeadLetterStore
	subscriptionRepo   *SQLiteSubscriptionRepository
	loopGuard          *LoopGuard
	traceRecorder      *EventTraceRecorder
	subscriptionReg    *EventSubscriptionRegistry
	publisher          *EventPublisher
	dispatcher         *Dispatcher
	handler            DeliveryHandler
	started            bool
	stopCancel         context.CancelFunc
	wg                 sync.WaitGroup
}

func NewService(cfg ServiceConfig) (*Service, error) {
	if cfg.DB == nil {
		return nil, fmt.Errorf("event: db required")
	}
	if cfg.MaxDepth <= 0 {
		cfg.MaxDepth = 8
	}
	if cfg.MaxSubscribers <= 0 {
		cfg.MaxSubscribers = 64
	}
	if cfg.TraceMaxEntries <= 0 {
		cfg.TraceMaxEntries = 10000
	}

	schemaRegistry := NewEventSchemaRegistry()
	outboxRepo := NewOutboxRepository(cfg.DB)
	deliveryStore := NewSQLiteDeliveryStore(cfg.DB)
	deadLetterStore := NewSQLiteDeadLetterStore(cfg.DB)
	subscriptionRepo := NewSQLiteSubscriptionRepository(cfg.DB)
	loopGuard := NewLoopGuard(cfg.MaxDepth, cfg.MaxChainLength)
	traceRecorder := NewEventTraceRecorder(cfg.TraceMaxEntries)
	subscriptionReg := NewEventSubscriptionRegistry(schemaRegistry, cfg.MaxSubscribers)
	subscriptionReg.SetRepository(subscriptionRepo)

	svc := &Service{
		config:          cfg,
		db:              cfg.DB,
		schemaRegistry:  schemaRegistry,
		outboxRepo:      outboxRepo,
		deliveryStore:   deliveryStore,
		deadLetterStore: deadLetterStore,
		subscriptionRepo: subscriptionRepo,
		loopGuard:       loopGuard,
		traceRecorder:   traceRecorder,
		subscriptionReg: subscriptionReg,
		handler:         defaultDeliveryHandler,
	}

	svc.publisher = NewEventPublisher(schemaRegistry, outboxRepo, cfg.DB, loopGuard, cfg.MaxDepth)
	svc.dispatcher = NewDispatcher(
		outboxRepo, deliveryStore, deadLetterStore,
		subscriptionReg, schemaRegistry, traceRecorder,
		cfg.Dispatcher, svc.handler,
	)

	return svc, nil
}

func (s *Service) RegisterDefaultEventTypes(ctx context.Context) error {
	for _, def := range DefaultHostEventTypes() {
		if err := s.schemaRegistry.RegisterEventType(ctx, def); err != nil {
			return fmt.Errorf("event: register %s: %w", def.EventTypeID, err)
		}
	}
	for _, def := range InternalOnlyEventTypes() {
		if err := s.schemaRegistry.RegisterEventType(ctx, def); err != nil {
			return fmt.Errorf("event: register %s: %w", def.EventTypeID, err)
		}
	}
	return nil
}

func (s *Service) RegisterEventType(ctx context.Context, def EventTypeDefinition) error {
	return s.schemaRegistry.RegisterEventType(ctx, def)
}

func (s *Service) ListEventTypes(ctx context.Context) ([]EventTypeDefinition, error) {
	return s.schemaRegistry.ListEventTypes(ctx)
}

func (s *Service) GetEventType(ctx context.Context, typeID EventTypeID, version int) (EventTypeDefinition, error) {
	return s.schemaRegistry.GetEventType(ctx, typeID, version)
}

func (s *Service) Publish(ctx context.Context, typeID EventTypeID, version int, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return s.publisher.Publish(ctx, typeID, version, payload, opts)
}

func (s *Service) PublishTx(ctx context.Context, tx *sql.Tx, typeID EventTypeID, version int, payload json.RawMessage, opts PublishOptions) (PublishResult, error) {
	return s.publisher.PublishTx(ctx, tx, typeID, version, payload, opts)
}

func (s *Service) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, nil)
}

func (s *Service) RegisterSubscription(ctx context.Context, def EventSubscriptionDefinition) error {
	return s.subscriptionReg.Register(ctx, def)
}

func (s *Service) RegisterSubscriptions(ctx context.Context, defs []EventSubscriptionDefinition) error {
	return s.subscriptionReg.RegisterBatch(ctx, defs)
}

func (s *Service) UnregisterSubscription(ctx context.Context, contributionID string) error {
	return s.subscriptionReg.Unregister(ctx, contributionID)
}

func (s *Service) RemoveSubscriptionsByExtension(ctx context.Context, extensionID string) error {
	return s.subscriptionReg.RemoveByExtension(ctx, extensionID)
}

func (s *Service) DeactivateSubscriptionsByExtension(ctx context.Context, extensionID string) error {
	return s.subscriptionReg.DeactivateByExtension(ctx, extensionID)
}

func (s *Service) UpdateExtensionGeneration(ctx context.Context, extensionID string, generation int64, defs []EventSubscriptionDefinition) error {
	if err := s.subscriptionReg.UpdateGeneration(ctx, extensionID, generation, defs); err != nil {
		return err
	}
	_, _ = s.deliveryStore.CancelPendingByExtension(ctx, extensionID, "cancelled_stale_generation")
	return nil
}

func (s *Service) ListSubscriptionsByExtension(ctx context.Context, extensionID string) []*ResolvedSubscription {
	return s.subscriptionReg.ListByExtension(ctx, extensionID)
}

func (s *Service) ListSubscriptionsByType(ctx context.Context, typeID EventTypeID) []*ResolvedSubscription {
	return s.subscriptionReg.ListByType(ctx, typeID)
}

func (s *Service) GetSubscription(ctx context.Context, contributionID string) (*ResolvedSubscription, bool) {
	return s.subscriptionReg.Get(ctx, contributionID)
}

func (s *Service) GetDelivery(ctx context.Context, deliveryID string) (Delivery, error) {
	return s.deliveryStore.GetDelivery(ctx, deliveryID)
}

func (s *Service) ListDeliveries(ctx context.Context, filter DeliveryFilter, limit, offset int) ([]Delivery, error) {
	return s.deliveryStore.ListDeliveries(ctx, filter, limit, offset)
}

func (s *Service) ListDeliveriesByEvent(ctx context.Context, eventID string) ([]Delivery, error) {
	return s.deliveryStore.ListDeliveriesByEvent(ctx, eventID)
}

func (s *Service) ListDeliveriesBySubscription(ctx context.Context, subscriptionID string, limit, offset int) ([]Delivery, error) {
	return s.deliveryStore.ListDeliveriesBySubscription(ctx, subscriptionID, limit, offset)
}

func (s *Service) CancelDeliveriesByExtension(ctx context.Context, extensionID, reason string) (int, error) {
	return s.deliveryStore.CancelPendingByExtension(ctx, extensionID, reason)
}

func (s *Service) CancelDeliveriesBySubscription(ctx context.Context, subscriptionID, reason string) (int, error) {
	return s.deliveryStore.CancelPendingBySubscription(ctx, subscriptionID, reason)
}

func (s *Service) GetDeadLetter(ctx context.Context, deadLetterID string) (DeadLetterRecord, error) {
	return s.deadLetterStore.GetDeadLetter(ctx, deadLetterID)
}

func (s *Service) ListDeadLetters(ctx context.Context, filter DeadLetterFilter, limit, offset int) ([]DeadLetterRecord, error) {
	return s.deadLetterStore.ListDeadLetters(ctx, filter, limit, offset)
}

func (s *Service) ListDeadLettersByExtension(ctx context.Context, extensionID string, limit, offset int) ([]DeadLetterRecord, error) {
	return s.deadLetterStore.ListByExtension(ctx, extensionID, limit, offset)
}

func (s *Service) ReplayDeadLetter(ctx context.Context, req ReplayRequest) error {
	if err := req.Validate(); err != nil {
		return err
	}
	record, err := s.deadLetterStore.GetDeadLetter(ctx, req.DeadLetterID)
	if err != nil {
		return err
	}
	if record.Status == DeadLetterStatusDiscarded {
		return fmt.Errorf("%w: dead letter discarded", ErrReplayDenied)
	}
	sub, ok := s.subscriptionReg.Get(ctx, record.SubscriptionID)
	if !ok && req.Strategy != ReplayDiscard && req.NewSubscriptionID == "" {
		return fmt.Errorf("%w: subscription %s not found", ErrSubscriptionNotFound, record.SubscriptionID)
	}
	if req.NewSubscriptionID != "" {
		sub, ok = s.subscriptionReg.Get(ctx, req.NewSubscriptionID)
		if !ok {
			return fmt.Errorf("%w: new subscription %s not found", ErrSubscriptionNotFound, req.NewSubscriptionID)
		}
	}
	switch req.Strategy {
	case ReplayDiscard:
		return s.deadLetterStore.MarkDiscarded(ctx, req.DeadLetterID)
	case ReplaySameSubscription, ReplayAfterRepair, ReplayToNewGeneration:
		if sub != nil && !sub.Effective.IsActive() {
			return fmt.Errorf("%w: subscription inactive: %s", ErrReplayDenied, sub.Effective.DenyReason())
		}
		delivery := Delivery{
			DeliveryID:             newDeliveryID(),
			EventID:                record.EventID,
			SubscriptionID:         record.SubscriptionID,
			ExtensionID:            record.ExtensionID,
			ModuleID:               record.ModuleID,
			Status:                 DeliveryStatusPending,
			PartitionKey:           record.PartitionKey,
			OrderingKey:            record.OrderingKey,
			MaxAttempts:            sub.Definition.RetryPolicy.MaxAttempts,
			AvailableAt:            time.Now().UTC(),
			ScopeSnapshotID:        record.ScopeSnapshotID,
			PermissionSnapshotID:   record.PermissionSnapshotID,
			SubscriptionGeneration: sub.Definition.Generation,
			TargetGeneration:       sub.Definition.Generation,
			CreatedAt:              time.Now().UTC(),
			UpdatedAt:              time.Now().UTC(),
		}
		if err := s.deliveryStore.CreateDelivery(ctx, delivery); err != nil {
			return err
		}
		return s.deadLetterStore.MarkReplayed(ctx, req.DeadLetterID)
	default:
		return fmt.Errorf("%w: unknown replay strategy %s", ErrInvalidEvent, req.Strategy)
	}
}

func (s *Service) DiscardDeadLetter(ctx context.Context, deadLetterID string) error {
	return s.deadLetterStore.MarkDiscarded(ctx, deadLetterID)
}

func (s *Service) GetOutboxRecord(ctx context.Context, outboxID string) (OutboxRecord, error) {
	return s.outboxRepo.Get(ctx, outboxID)
}

func (s *Service) GetOutboxByEventID(ctx context.Context, eventID string) (OutboxRecord, error) {
	return s.outboxRepo.GetByEventID(ctx, eventID)
}

func (s *Service) ListOutboxByExtension(ctx context.Context, extensionID string, limit, offset int) ([]OutboxRecord, error) {
	return s.outboxRepo.ListByExtension(ctx, extensionID, limit, offset)
}

func (s *Service) ListOutboxByStatus(ctx context.Context, status OutboxStatus, limit, offset int) ([]OutboxRecord, error) {
	return s.outboxRepo.ListByStatus(ctx, status, limit, offset)
}

func (s *Service) QueryAudit(filter AuditFilter) []EventAuditEntry {
	return s.traceRecorder.QueryAudit(filter)
}

func (s *Service) CircuitStats() map[string]CircuitStats {
	return s.dispatcher.CircuitStats()
}

func (s *Service) ResetCircuit(subscriptionID string) {
	s.dispatcher.ResetCircuit(subscriptionID)
}

func (s *Service) SetDeliveryHandler(handler DeliveryHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handler = handler
	s.dispatcher.SetHandler(handler)
}

func (s *Service) SetEffectiveResolver(resolver EffectiveResolver) {
	s.subscriptionReg.SetEffectiveResolver(resolver)
}

func (s *Service) GetSubscriptionRepository() *SQLiteSubscriptionRepository {
	return s.subscriptionRepo
}

func (s *Service) GetSubscriptionRegistry() *EventSubscriptionRegistry {
	return s.subscriptionReg
}

func (s *Service) GetDispatcher() *Dispatcher {
	return s.dispatcher
}

func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()

	if err := s.subscriptionReg.LoadFromRepository(ctx); err != nil {
		log.Printf("[event] failed to load subscriptions from repository: %v", err)
	}
	s.subscriptionReg.RebuildEffectiveStates(ctx)

	dispatchCtx, cancel := context.WithCancel(ctx)
	s.stopCancel = cancel
	s.dispatcher.Start(dispatchCtx)
	log.Printf("[event] service started")
	return nil
}

func (s *Service) Stop() {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return
	}
	s.started = false
	s.mu.Unlock()
	if s.stopCancel != nil {
		s.stopCancel()
	}
	s.dispatcher.Stop()
	log.Printf("[event] service stopped")
}

func (s *Service) Stats(ctx context.Context) (ServiceStats, error) {
	stats := ServiceStats{}
	var err error
	stats.PendingOutbox, err = s.outboxRepo.CountByStatus(ctx, OutboxStatusPending)
	if err != nil {
		return stats, err
	}
	stats.DispatchingOutbox, err = s.outboxRepo.CountByStatus(ctx, OutboxStatusDispatching)
	if err != nil {
		return stats, err
	}
	stats.DispatchedOutbox, err = s.outboxRepo.CountByStatus(ctx, OutboxStatusDispatched)
	if err != nil {
		return stats, err
	}
	stats.DeadLetterOutbox, err = s.outboxRepo.CountByStatus(ctx, OutboxStatusDeadLetter)
	if err != nil {
		return stats, err
	}
	stats.PendingDeliveries, err = s.deliveryStore.CountDeliveriesByStatus(ctx, DeliveryStatusPending)
	if err != nil {
		return stats, err
	}
	stats.LeasedDeliveries, err = s.deliveryStore.CountDeliveriesByStatus(ctx, DeliveryStatusLeased)
	if err != nil {
		return stats, err
	}
	stats.SucceededDeliveries, err = s.deliveryStore.CountDeliveriesByStatus(ctx, DeliveryStatusSucceeded)
	if err != nil {
		return stats, err
	}
	stats.FailedDeliveries, err = s.deliveryStore.CountDeliveriesByStatus(ctx, DeliveryStatusFailed)
	if err != nil {
		return stats, err
	}
	stats.RetryWaitDeliveries, err = s.deliveryStore.CountDeliveriesByStatus(ctx, DeliveryStatusRetryWait)
	if err != nil {
		return stats, err
	}
	stats.DeadLetterDeliveries, err = s.deliveryStore.CountDeliveriesByStatus(ctx, DeliveryStatusDeadLetter)
	if err != nil {
		return stats, err
	}
	stats.CancelledDeliveries, err = s.deliveryStore.CountDeliveriesByStatus(ctx, DeliveryStatusCancelled)
	if err != nil {
		return stats, err
	}
	stats.SkippedDeliveries, err = s.deliveryStore.CountDeliveriesByStatus(ctx, DeliveryStatusSkipped)
	if err != nil {
		return stats, err
	}
	stats.ActiveSubscriptions = s.subscriptionReg.Count()
	stats.Circuits = s.dispatcher.CircuitStats()
	return stats, nil
}

type ServiceStats struct {
	PendingOutbox         int                       `json:"pendingOutbox"`
	DispatchingOutbox     int                       `json:"dispatchingOutbox"`
	DispatchedOutbox      int                       `json:"dispatchedOutbox"`
	DeadLetterOutbox      int                       `json:"deadLetterOutbox"`
	PendingDeliveries     int                       `json:"pendingDeliveries"`
	LeasedDeliveries      int                       `json:"leasedDeliveries"`
	SucceededDeliveries   int                       `json:"succeededDeliveries"`
	FailedDeliveries      int                       `json:"failedDeliveries"`
	RetryWaitDeliveries   int                       `json:"retryWaitDeliveries"`
	DeadLetterDeliveries  int                       `json:"deadLetterDeliveries"`
	CancelledDeliveries   int                       `json:"cancelledDeliveries"`
	SkippedDeliveries     int                       `json:"skippedDeliveries"`
	ActiveSubscriptions   int                       `json:"activeSubscriptions"`
	Circuits              map[string]CircuitStats   `json:"circuits"`
}

func (s *Service) CleanupOldRecords(ctx context.Context, maxAge time.Duration) error {
	before := time.Now().UTC().Add(-maxAge)
	_, err := s.outboxRepo.DeleteOlderThan(ctx, before, OutboxStatusDispatched)
	return err
}

func defaultDeliveryHandler(ctx context.Context, delivery Delivery, envelope EventEnvelope, sub *ResolvedSubscription) error {
	if sub.Definition.RuntimeBinding.Entry == "" {
		return nil
	}
	return nil
}
