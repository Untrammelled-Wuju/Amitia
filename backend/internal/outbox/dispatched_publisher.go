package outbox

import (
	"fmt"
	"sync"

	applog "github.com/u-ai/backend/log"
)

type DispatchedPublisher struct {
	mu         sync.RWMutex
	handlers   map[string]Publisher
	fallback   Publisher
}

func NewDispatchedPublisher(fallback Publisher) *DispatchedPublisher {
	return &DispatchedPublisher{
		handlers:   make(map[string]Publisher),
		fallback:   fallback,
	}
}

func (d *DispatchedPublisher) Register(eventType string, publisher Publisher) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.handlers[eventType] = publisher
}

func (d *DispatchedPublisher) Publish(record OutboxRecord) error {
	d.mu.RLock()
	handler, ok := d.handlers[record.EventType]
	d.mu.RUnlock()

	if ok {
		return handler.Publish(record)
	}

	if d.fallback != nil {
		if _, isLogOnly := d.fallback.(*logPublisher); isLogOnly {
			applog.Warn("outbox event has no registered publisher and log-only fallback is insufficient", "eventType", record.EventType, "id", record.ID)
			return fmt.Errorf("outbox: event type %q is not registered with a real publisher", record.EventType)
		}
		return d.fallback.Publish(record)
	}

	return fmt.Errorf("outbox: no publisher registered for event type %q", record.EventType)
}

type logPublisher struct{}

func (l *logPublisher) Publish(record OutboxRecord) error {
	applog.Info("outbox event dispatched", "id", record.ID, "type", record.EventType, "aggregate", record.AggregateID)
	return nil
}

func LogOnlyPublisher() Publisher {
	return &logPublisher{}
}

func IsLogPublisher(p Publisher) bool {
	_, ok := p.(*logPublisher)
	return ok
}
