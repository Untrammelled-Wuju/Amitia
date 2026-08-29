// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package installation

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/coordinator"
	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
	"github.com/u-ai/backend/internal/desktoppet/installation/device"
	"github.com/u-ai/backend/log"
)

const (
	desiredOutboxPollInterval = 2 * time.Second
	desiredOutboxBatchSize    = 50
)

type DesiredStateOutboxWorker struct {
	repo      RepositoryV2
	publisher coordinator.RuntimeDesiredStatePublisher
	mu        sync.Mutex
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	running   atomic.Bool
}

func NewDesiredStateOutboxWorker(repo RepositoryV2, publisher coordinator.RuntimeDesiredStatePublisher) *DesiredStateOutboxWorker {
	return &DesiredStateOutboxWorker{repo: repo, publisher: publisher}
}

func (w *DesiredStateOutboxWorker) Start(ctx context.Context) error {
	if w == nil || w.repo == nil || w.publisher == nil {
		return fmt.Errorf("desired outbox worker: dependencies are not configured")
	}
	w.mu.Lock()
	if w.running.Load() {
		w.mu.Unlock()
		return nil
	}
	workerCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.running.Store(true)
	w.wg.Add(1)
	w.mu.Unlock()
	go func() {
		defer w.wg.Done()
		defer w.running.Store(false)
		if err := w.run(workerCtx); err != nil && workerCtx.Err() == nil {
			log.Error("installation desired outbox worker stopped: ", err)
		}
	}()
	return nil
}

func (w *DesiredStateOutboxWorker) Stop() {
	if w == nil {
		return
	}
	w.mu.Lock()
	cancel := w.cancel
	w.cancel = nil
	w.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	w.wg.Wait()
}

func (w *DesiredStateOutboxWorker) IsRunning() bool {
	return w != nil && w.running.Load()
}

func (w *DesiredStateOutboxWorker) Run(ctx context.Context) error {
	if w == nil || w.repo == nil || w.publisher == nil {
		return fmt.Errorf("desired outbox worker: dependencies are not configured")
	}
	if !w.running.CompareAndSwap(false, true) {
		return nil
	}
	defer w.running.Store(false)
	return w.run(ctx)
}

func (w *DesiredStateOutboxWorker) run(ctx context.Context) error {
	if err := w.processBatch(ctx); err != nil && ctx.Err() == nil {
		log.Warn("installation desired outbox initial batch failed: ", err)
	}
	ticker := time.NewTicker(desiredOutboxPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := w.processBatch(ctx); err != nil && ctx.Err() == nil {
				log.Warn("installation desired outbox batch failed: ", err)
			}
		}
	}
}

func (w *DesiredStateOutboxWorker) processBatch(ctx context.Context) error {
	events, err := w.repo.ListPendingOutboxEvents(desiredOutboxBatchSize)
	if err != nil {
		return err
	}
	for _, event := range events {
		if event == nil {
			continue
		}
		if err := w.processEvent(ctx, event); err != nil {
			if markErr := w.markRetry(ctx, event, err); markErr != nil {
				return fmt.Errorf("desired outbox worker: publish failed (%v), retry persistence failed: %w", err, markErr)
			}
			continue
		}
		if err := w.repo.Transaction(ctx, func(tx RepositoryV2) error {
			return tx.MarkOutboxEventPublished(tx.DB(), event.EventID)
		}); err != nil {
			return err
		}
	}
	return nil
}

func (w *DesiredStateOutboxWorker) processEvent(ctx context.Context, event *desired.DesiredStateOutboxEvent) error {
	var snapshot coordinator.DesiredStateSnapshot
	if err := json.Unmarshal([]byte(event.PayloadJSON), &snapshot); err != nil {
		return fmt.Errorf("decode desired outbox payload: %w", err)
	}
	if snapshot.UserID == "" {
		snapshot.UserID = event.UserID
	}
	if snapshot.DeviceID == "" {
		snapshot.DeviceID = event.DeviceID
	}
	if snapshot.RuntimeID == "" {
		snapshot.RuntimeID = event.RuntimeID
	}
	if snapshot.InstallationID == "" {
		snapshot.InstallationID = event.InstallationID
	}
	if snapshot.DesiredRevision == 0 {
		snapshot.DesiredRevision = event.DesiredRevision
	}
	if snapshot.DesiredHash == "" {
		snapshot.DesiredHash = event.DesiredHash
	}
	return w.publisher.PublishDesiredState(ctx, device.DeviceContext{
		UserID: snapshot.UserID, DeviceID: snapshot.DeviceID, RuntimeID: snapshot.RuntimeID, Source: "desired_outbox",
	}, &snapshot)
}

func (w *DesiredStateOutboxWorker) markRetry(ctx context.Context, event *desired.DesiredStateOutboxEvent, cause error) error {
	attempt := event.AttemptCount + 1
	delay := time.Duration(attempt) * 2 * time.Second
	if delay > time.Minute {
		delay = time.Minute
	}
	return w.repo.Transaction(ctx, func(tx RepositoryV2) error {
		result := tx.DB().Model(&desired.DesiredStateOutboxEvent{}).
			Where("event_id = ? AND status = ?", event.EventID, "pending").
			Updates(map[string]interface{}{
				"attempt_count": attempt,
				"available_at":  time.Now().UTC().Add(delay).Format("2006-01-02 15:04:05"),
				"last_error":    cause.Error(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return fmt.Errorf("desired outbox worker: retry CAS affected %d rows", result.RowsAffected)
		}
		return nil
	})
}
