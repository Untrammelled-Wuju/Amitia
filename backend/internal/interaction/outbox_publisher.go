package interaction

import (
	"context"
	"log"
	"sync"
	"time"
)

type OutboxDispatcher struct {
	store       OutboxStore
	deadStore   DeadLetterStore
	publisher   OutboxPublisher
	interval    time.Duration
	concurrency int
	mu          sync.Mutex
	running     bool
	stopCh      chan struct{}
	wg          sync.WaitGroup
}

type OutboxWorkerConfig struct {
	DispatcherInterval time.Duration
	RecoveryInterval   time.Duration
	Concurrency        int
}

type OutboxPublisherFunc func(record OutboxRecord) error

func (f OutboxPublisherFunc) Publish(record OutboxRecord) error {
	return f(record)
}

type OutboxRuntime struct {
	dispatcher *OutboxDispatcher
	recovery   *DeadLetterRecoveryWorker
}

func NewOutboxRuntime(store OutboxStore, deadStore DeadLetterStore, publisher OutboxPublisher, cfg OutboxWorkerConfig) *OutboxRuntime {
	return &OutboxRuntime{
		dispatcher: NewOutboxDispatcherWithConfig(store, deadStore, publisher, cfg),
		recovery:   NewDeadLetterRecoveryWorkerWithConfig(deadStore, publisher, cfg),
	}
}

func (r *OutboxRuntime) Start(ctx context.Context) {
	if r == nil {
		return
	}
	r.dispatcher.Start(ctx)
	r.recovery.Start(ctx)
}

func (r *OutboxRuntime) Stop() {
	if r == nil {
		return
	}
	r.dispatcher.Stop()
	r.recovery.Stop()
}

func NewOutboxDispatcher(store OutboxStore, deadStore DeadLetterStore, publisher OutboxPublisher) *OutboxDispatcher {
	return NewOutboxDispatcherWithConfig(store, deadStore, publisher, OutboxWorkerConfig{})
}

func NewOutboxDispatcherWithConfig(store OutboxStore, deadStore DeadLetterStore, publisher OutboxPublisher, cfg OutboxWorkerConfig) *OutboxDispatcher {
	interval := cfg.DispatcherInterval
	if interval <= 0 {
		interval = 5 * time.Second
	}
	concurrency := cfg.Concurrency
	if concurrency <= 0 {
		concurrency = 3
	}
	return &OutboxDispatcher{
		store:       store,
		deadStore:   deadStore,
		publisher:   publisher,
		interval:    interval,
		concurrency: concurrency,
		stopCh:      make(chan struct{}),
	}
}

func (d *OutboxDispatcher) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	d.running = true
	d.mu.Unlock()

	d.wg.Add(1)
	go d.dispatchLoop(ctx)
}

func (d *OutboxDispatcher) Stop() {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		d.wg.Wait()
		return
	}
	d.running = false
	d.mu.Unlock()
	close(d.stopCh)
	d.wg.Wait()
	d.stopCh = make(chan struct{})
}

func (d *OutboxDispatcher) dispatchLoop(ctx context.Context) {
	defer d.wg.Done()
	defer func() {
		d.mu.Lock()
		d.running = false
		d.mu.Unlock()
	}()
	d.flush(ctx)
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.flush(ctx)
		}
	}
}

func (d *OutboxDispatcher) flush(ctx context.Context) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
	if err := d.store.ReleaseExpiredLeases(time.Now()); err != nil {
		log.Printf("[outbox] release expired leases error: %v", err)
		return
	}
	pending, err := d.store.LeasePending(d.concurrency, time.Now().Add(DefaultOutboxLeaseDuration))
	if err != nil {
		log.Printf("[outbox] lease pending error: %v", err)
		return
	}
	if len(pending) == 0 {
		return
	}

	sem := make(chan struct{}, d.concurrency)
	var wg sync.WaitGroup

	for i := range pending {
		rec := pending[i]
		wg.Add(1)
		go func(r OutboxRecord) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			d.processRecord(r)
		}(rec)
	}
	wg.Wait()
}

func (d *OutboxDispatcher) processRecord(rec OutboxRecord) {
	err := d.publisher.Publish(rec)
	if err == nil {
		if markErr := d.store.MarkPublished(rec.ID); markErr != nil {
			log.Printf("[outbox] mark published error: %v", markErr)
		}
		return
	}

	if rec.RetryCount+1 >= rec.MaxRetries {
		if markErr := d.store.MarkDead(rec.ID); markErr != nil {
			log.Printf("[outbox] mark dead error: %v", markErr)
		}
		d.moveToDeadLetter(rec, err)
		return
	}

	if markErr := d.store.MarkFailed(rec.ID, err.Error()); markErr != nil {
		log.Printf("[outbox] mark failed error: %v", markErr)
	}
}

func (d *OutboxDispatcher) moveToDeadLetter(rec OutboxRecord, err error) {
	dl := &DeadLetterRecord{
		OutboxID:    rec.ID,
		EventType:   rec.EventType,
		AggregateID: rec.AggregateID,
		Payload:     rec.Payload,
		LastError:   err.Error(),
		RetryCount:  rec.RetryCount,
		Status:      DeadLetterStatusPending,
		CreatedAt:   time.Now(),
	}
	if _, appendErr := d.deadStore.Append(dl); appendErr != nil {
		log.Printf("[outbox] move to dead letter error: %v", appendErr)
	}
}

type DeadLetterRecoveryWorker struct {
	deadStore DeadLetterStore
	publisher OutboxPublisher
	interval  time.Duration
	mu        sync.Mutex
	running   bool
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

func NewDeadLetterRecoveryWorker(deadStore DeadLetterStore, publisher OutboxPublisher) *DeadLetterRecoveryWorker {
	return NewDeadLetterRecoveryWorkerWithConfig(deadStore, publisher, OutboxWorkerConfig{})
}

func NewDeadLetterRecoveryWorkerWithConfig(deadStore DeadLetterStore, publisher OutboxPublisher, cfg OutboxWorkerConfig) *DeadLetterRecoveryWorker {
	interval := cfg.RecoveryInterval
	if interval <= 0 {
		interval = 30 * time.Second
	}
	return &DeadLetterRecoveryWorker{
		deadStore: deadStore,
		publisher: publisher,
		interval:  interval,
		stopCh:    make(chan struct{}),
	}
}

func (w *DeadLetterRecoveryWorker) Start(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	w.running = true
	w.mu.Unlock()

	w.wg.Add(1)
	go w.recoveryLoop(ctx)
}

func (w *DeadLetterRecoveryWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		w.wg.Wait()
		return
	}
	w.running = false
	w.mu.Unlock()
	close(w.stopCh)
	w.wg.Wait()
	w.stopCh = make(chan struct{})
}

func (w *DeadLetterRecoveryWorker) recoveryLoop(ctx context.Context) {
	defer w.wg.Done()
	defer func() {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
	}()
	w.recoverPending(ctx)
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.recoverPending(ctx)
		}
	}
}

func (w *DeadLetterRecoveryWorker) recoverPending(ctx context.Context) {
	if ctx != nil {
		select {
		case <-ctx.Done():
			return
		default:
		}
	}
	pending, err := w.deadStore.ListPending()
	if err != nil {
		log.Printf("[dead_letter] list pending error: %v", err)
		return
	}
	for _, rec := range pending {
		if err := w.deadStore.MarkReplaying(rec.ID); err != nil {
			continue
		}
		outboxRec := OutboxRecord{
			ID:          rec.OutboxID,
			EventType:   rec.EventType,
			AggregateID: rec.AggregateID,
			Payload:     rec.Payload,
		}
		pubErr := w.publisher.Publish(outboxRec)
		if pubErr != nil {
			w.deadStore.MarkArchived(rec.ID)
			continue
		}
		w.deadStore.MarkReplayed(rec.ID)
	}
}
