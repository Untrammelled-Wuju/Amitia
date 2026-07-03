package outbox

import (
	"context"
	"time"

	applog "github.com/u-ai/backend/log"
)

type Publisher interface {
	Publish(record OutboxRecord) error
}

type PublisherFunc func(record OutboxRecord) error

func (f PublisherFunc) Publish(record OutboxRecord) error {
	return f(record)
}

type Worker struct {
	store     *SQLiteOutboxStore
	publisher Publisher
	batchSize int
	interval  time.Duration
	done      chan struct{}
}

type WorkerConfig struct {
	BatchSize int
	Interval  time.Duration
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		BatchSize: 10,
		Interval:  5 * time.Second,
	}
}

func NewWorker(store *SQLiteOutboxStore, publisher Publisher, cfg WorkerConfig) *Worker {
	return &Worker{
		store:     store,
		publisher: publisher,
		batchSize: cfg.BatchSize,
		interval:  cfg.Interval,
		done:      make(chan struct{}),
	}
}

func (w *Worker) Start(ctx context.Context) {
	go w.loop(ctx)
}

func (w *Worker) Stop() {
	close(w.done)
}

func (w *Worker) loop(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.done:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.processBatch(ctx)
		}
	}
}

func (w *Worker) processBatch(ctx context.Context) {
	w.store.ReleaseExpiredLeases()

	owner := "outbox-worker"
	records, err := w.store.ClaimNext(w.batchSize, owner)
	if err != nil {
		applog.Error("outbox worker claim failed", "error", err)
		return
	}

	for _, record := range records {
		if err := w.publisher.Publish(record); err != nil {
			w.store.MarkFailed(record.ID, record.LeaseToken, err.Error())
			applog.Error("outbox worker publish failed", "id", record.ID, "error", err)
			continue
		}
		w.store.MarkPublished(record.ID, record.LeaseToken)
	}
}
