package worker

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/release"
	"github.com/u-ai/backend/log"
)

type EventOutboxDispatcher struct {
	repo          release.ReleaseRepository
	sink          EventSink
	batchSize     int
	checkInterval time.Duration
	stopCh        chan struct{}
	wg            sync.WaitGroup
	mu            sync.Mutex
	running       bool
}

type EventSink interface {
	Deliver(ctx context.Context, eventType, aggregateID string, payload []byte) error
}

func NewEventOutboxDispatcher(
	repo release.ReleaseRepository,
	sink EventSink,
) *EventOutboxDispatcher {
	return &EventOutboxDispatcher{
		repo:          repo,
		sink:          sink,
		batchSize:     50,
		checkInterval: 5 * time.Second,
		stopCh:        make(chan struct{}),
	}
}

func (d *EventOutboxDispatcher) Start(ctx context.Context) {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return
	}
	d.running = true
	d.stopCh = make(chan struct{})
	d.mu.Unlock()

	d.wg.Add(1)
	go d.run(ctx)
	log.Logger.Info("Event outbox dispatcher started")
}

func (d *EventOutboxDispatcher) Stop() {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return
	}
	d.running = false
	close(d.stopCh)
	d.mu.Unlock()
	d.wg.Wait()
	log.Logger.Info("Event outbox dispatcher stopped")
}

func (d *EventOutboxDispatcher) run(ctx context.Context) {
	defer d.wg.Done()
	ticker := time.NewTicker(d.checkInterval)
	defer ticker.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := d.dispatchPending(ctx); err != nil {
				log.Logger.Warnf("Event outbox dispatch failed: %v", err)
			}
		}
	}
}

func (d *EventOutboxDispatcher) dispatchPending(ctx context.Context) error {
	events, err := d.repo.ListPendingOutboxEvents(d.batchSize)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event.Status != "pending" {
			continue
		}
		if event.AvailableAt != "" {
			availableAt, parseErr := time.Parse("2006-01-02 15:04:05", event.AvailableAt)
			if parseErr == nil && time.Now().Before(availableAt) {
				continue
			}
		}
		if err := d.dispatchEvent(ctx, event); err != nil {
			log.Logger.Warnf("Failed to dispatch event %s: %v", event.EventID, err)
		}
	}
	return nil
}

func (d *EventOutboxDispatcher) dispatchEvent(ctx context.Context, event *release.ReleaseEventOutbox) error {
	event.AttemptCount++

	if d.sink != nil {
		err := d.sink.Deliver(ctx, event.EventType, event.AggregateID, []byte(event.PayloadJSON))
		if err != nil {
			event.LastError = err.Error()
			event.Status = "failed"
			event.AvailableAt = formatOutboxTimestamp(time.Now().Add(exponentialBackoff(event.AttemptCount)))
			d.repo.UpdateOutboxEvent(event)
			return err
		}
	}

	event.Status = "published"
	event.PublishedAt = formatOutboxTimestamp(time.Now())
	event.LastError = ""
	if err := d.repo.UpdateOutboxEvent(event); err != nil {
		log.Logger.Warnf("Failed to update outbox event %s: %v", event.EventID, err)
		return err
	}
	return nil
}

func exponentialBackoff(attempt int) time.Duration {
	if attempt <= 0 {
		attempt = 1
	}
	if attempt > 10 {
		attempt = 10
	}
	seconds := 1 << (attempt - 1)
	return time.Duration(seconds) * time.Second
}

type NoOpEventSink struct{}

func (s *NoOpEventSink) Deliver(ctx context.Context, eventType, aggregateID string, payload []byte) error {
	log.Logger.Debugf("NoOpEventSink: event=%s aggregate=%s payload=%d bytes", eventType, aggregateID, len(payload))
	return nil
}

func formatOutboxTimestamp(t time.Time) string {
	return t.Format("2006-01-02 15:04:05")
}

type InMemoryEventSink struct {
	events []CapturedEvent
	mu     sync.Mutex
}

type CapturedEvent struct {
	EventType    string
	AggregateID  string
	PayloadJSON  string
	DeliveredAt  string
}

func NewInMemoryEventSink() *InMemoryEventSink {
	return &InMemoryEventSink{
		events: make([]CapturedEvent, 0),
	}
}

func (s *InMemoryEventSink) Deliver(ctx context.Context, eventType, aggregateID string, payload []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, CapturedEvent{
		EventType:   eventType,
		AggregateID: aggregateID,
		PayloadJSON: string(payload),
		DeliveredAt: formatOutboxTimestamp(time.Now()),
	})
	return nil
}

func (s *InMemoryEventSink) GetEvents() []CapturedEvent {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]CapturedEvent, len(s.events))
	copy(result, s.events)
	return result
}

func SerializePayload(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(data)
}
