package delivery

import (
	"context"
	"time"

	applog "github.com/u-ai/backend/log"
)

type Worker struct {
	store     *SQLiteDeliveryStore
	adapters  []ChannelAdapter
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
		Interval:  1 * time.Second,
	}
}

func NewWorker(store *SQLiteDeliveryStore, adapters []ChannelAdapter, cfg WorkerConfig) *Worker {
	return &Worker{
		store:     store,
		adapters:  adapters,
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
	w.store.ReleaseExpiredClaims()

	intents, err := w.store.ClaimNextIntents(w.batchSize)
	if err != nil {
		applog.Error("delivery worker claim failed", "error", err)
		return
	}

	for _, intent := range intents {
		if err := w.deliver(ctx, intent); err != nil {
			w.store.MarkFailed(intent.ID, err.Error())
			applog.Error("delivery worker send failed", "id", intent.ID, "channel", intent.Channel, "error", err)
			continue
		}
	}
}

func (w *Worker) deliver(ctx context.Context, intent DeliveryIntent) error {
	adapter := w.findAdapter(intent.Channel)
	if adapter == nil {
		w.store.MarkFailed(intent.ID, "no channel adapter for "+intent.Channel)
		return nil
	}

	if err := adapter.Deliver(intent); err != nil {
		return err
	}

	w.store.MarkSent(intent.ID)
	return nil
}

func (w *Worker) findAdapter(channel string) ChannelAdapter {
	for _, a := range w.adapters {
		if a.Name() == channel {
			return a
		}
	}
	return nil
}
