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

func NewOutboxDispatcher(store OutboxStore, deadStore DeadLetterStore, publisher OutboxPublisher) *OutboxDispatcher {
	return &OutboxDispatcher{
		store:       store,
		deadStore:   deadStore,
		publisher:   publisher,
		interval:    5 * time.Second,
		concurrency: 3,
		stopCh:      make(chan struct{}),
	}
}

func (d *OutboxDispatcher) Start(ctx context.Context) {
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
}

func NewDeadLetterRecoveryWorker(deadStore DeadLetterStore, publisher OutboxPublisher) *DeadLetterRecoveryWorker {
	return &DeadLetterRecoveryWorker{
		deadStore: deadStore,
		publisher: publisher,
		interval:  30 * time.Second,
	}
}

func (w *DeadLetterRecoveryWorker) Start(ctx context.Context) {
	go w.recoveryLoop(ctx)
}

func (w *DeadLetterRecoveryWorker) recoveryLoop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.recoverPending(ctx)
		}
	}
}

func (w *DeadLetterRecoveryWorker) recoverPending(ctx context.Context) {
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
