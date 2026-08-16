package delivery

import (
	"context"
	"time"

	applog "github.com/u-ai/backend/log"
)

type Worker struct {
	store        *SQLiteDeliveryStore
	resolver     ChannelResolver
	availability ProviderAvailabilityChecker
	batchSize    int
	interval     time.Duration
	done         chan struct{}
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

func NewWorker(store *SQLiteDeliveryStore, resolver ChannelResolver, cfg WorkerConfig) *Worker {
	return &Worker{
		store:        store,
		resolver:     resolver,
		availability: noopAvailabilityChecker{},
		batchSize:    cfg.BatchSize,
		interval:     cfg.Interval,
		done:         make(chan struct{}),
	}
}

func NewWorkerWithAvailability(store *SQLiteDeliveryStore, resolver ChannelResolver, availability ProviderAvailabilityChecker, cfg WorkerConfig) *Worker {
	return &Worker{
		store:        store,
		resolver:     resolver,
		availability: availability,
		batchSize:    cfg.BatchSize,
		interval:     cfg.Interval,
		done:         make(chan struct{}),
	}
}

type noopAvailabilityChecker struct{}

func (noopAvailabilityChecker) IsProviderAvailable(providerInstanceID string) bool { return true }
func (noopAvailabilityChecker) MarkProviderUnavailable(providerInstanceID string, reason string) {
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
			if err2 := w.store.MarkFailed(intent.ID, intent.LeaseToken, err.Error()); err2 != nil {
				applog.Error("delivery worker mark failed error", "id", intent.ID, "error", err2)
			}
			applog.Error("delivery worker send failed", "id", intent.ID, "channel", intent.Channel, "error", err)
			continue
		}
	}
}

func (w *Worker) deliver(ctx context.Context, intent DeliveryIntent) error {
	adapter := w.resolver.Resolve(intent.Channel)
	if adapter == nil {
		if err := w.store.MarkFailed(intent.ID, intent.LeaseToken, "no channel adapter for "+intent.Channel); err != nil {
			applog.Error("delivery worker mark failed (no adapter) error", "id", intent.ID, "error", err)
		}
		return nil
	}

	providerID := adapter.ProviderInstanceID()
	if !w.availability.IsProviderAvailable(providerID) {
		if err := w.store.MarkFailed(intent.ID, intent.LeaseToken, "channel provider unavailable: "+providerID); err != nil {
			applog.Error("delivery worker mark failed (provider unavailable) error", "id", intent.ID, "error", err)
		}
		applog.Warn("delivery worker skip: provider unavailable", "id", intent.ID, "channel", intent.Channel, "provider", providerID)
		return nil
	}

	if err := adapter.Deliver(intent); err != nil {
		w.availability.MarkProviderUnavailable(providerID, "delivery failed: "+err.Error())
		return err
	}

	if err := w.store.MarkSent(intent.ID, intent.LeaseToken); err != nil {
		applog.Error("delivery worker mark sent failed", "id", intent.ID, "channel", intent.Channel, "error", err)
	}
	return nil
}
