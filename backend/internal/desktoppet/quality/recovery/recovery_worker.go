// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/log"
)

type RecoveryWorker struct {
	repo         quality.QualityRepository
	pollInterval time.Duration

	mu      sync.Mutex
	cancel  context.CancelFunc
	wg      sync.WaitGroup
	running bool
	runID   uint64
}

func NewRecoveryWorker(repo quality.QualityRepository) *RecoveryWorker {
	return &RecoveryWorker{
		repo:         repo,
		pollInterval: 30 * time.Second,
	}
}

func (w *RecoveryWorker) Start(ctx context.Context) {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	w.cancel = cancel
	w.running = true
	w.runID++
	runID := w.runID
	w.wg.Add(1)
	w.mu.Unlock()

	go func(id uint64) {
		defer w.wg.Done()
		defer func() {
			w.mu.Lock()
			if w.runID == id {
				w.cancel = nil
				w.running = false
			}
			w.mu.Unlock()
		}()
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-runCtx.Done():
				return
			case <-ticker.C:
				if _, err := w.RecoverStuckEvaluations(runCtx); err != nil && runCtx.Err() == nil {
					log.Logger.Errorf("recover stuck evaluations failed: %v", err)
				}
			}
		}
	}(runID)
}

func (w *RecoveryWorker) Stop() {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return
	}
	cancel := w.cancel
	runID := w.runID
	w.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	w.wg.Wait()

	w.mu.Lock()
	if w.runID == runID {
		w.cancel = nil
		w.running = false
	}
	w.mu.Unlock()
}

func (w *RecoveryWorker) RecoverStuckEvaluations(ctx context.Context) (int, error) {
	evals, err := w.repo.ListEvaluationsByStatus(ctx, string(quality.EvalRunning))
	if err != nil {
		return 0, err
	}

	recovered := 0
	now := time.Now().UTC()
	for _, eval := range evals {
		if ctx.Err() != nil {
			return recovered, ctx.Err()
		}

		var leaseExpires time.Time
		if eval.LeaseExpiresAt != "" {
			if t, parseErr := time.Parse(time.RFC3339, eval.LeaseExpiresAt); parseErr == nil {
				leaseExpires = t
			}
		}
		if !leaseExpires.IsZero() && now.Before(leaseExpires) {
			continue
		}

		ok, recoverErr := w.repo.RecoverExpiredEvaluation(ctx, eval.ID, eval.ExecutionID, now)
		if recoverErr != nil {
			log.Logger.Errorf("recover stuck evaluation %s failed: %v", eval.ID, recoverErr)
			continue
		}
		if !ok {
			// The execution owner or lease changed after the snapshot was read.
			// Another worker/heartbeat won the race, so this recovery is stale.
			continue
		}
		recovered++
		log.Logger.Infof("recovered stuck evaluation: %s", eval.ID)
	}

	return recovered, nil
}

type OutboxPublisher struct {
	repo           quality.QualityRepository
	eventPublisher quality.EventPublisher
	flushMu        sync.Mutex
}

func NewOutboxPublisher(repo quality.QualityRepository, eventPublisher quality.EventPublisher) *OutboxPublisher {
	return &OutboxPublisher{
		repo:           repo,
		eventPublisher: eventPublisher,
	}
}

func (p *OutboxPublisher) PublishEvent(ctx context.Context, event quality.QualityOutboxEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	record := &quality.QualityOutboxEventRecord{
		EventType:   event.EventType,
		PayloadJSON: string(payload),
		Status:      "pending",
	}
	return p.repo.CreateOutboxEvent(ctx, record)
}

func (p *OutboxPublisher) Flush(ctx context.Context) error {
	// processEvaluation completions and the periodic retry ticker can flush at
	// the same time. Serialize the local dispatcher so one pending record is not
	// published twice by this process before MarkOutboxEventPublished lands.
	p.flushMu.Lock()
	defer p.flushMu.Unlock()

	events, err := p.repo.ListPendingOutboxEvents(ctx, 100)
	if err != nil {
		return err
	}

	for _, record := range events {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		var event quality.QualityOutboxEvent
		if err := json.Unmarshal([]byte(record.PayloadJSON), &event); err != nil {
			log.Logger.Errorf("unmarshal quality outbox event %s payload failed: %v", record.ID, err)
			continue
		}
		if event.EventType == "" {
			event.EventType = record.EventType
		}

		qualityEvent := qualityEventFromOutbox(event)
		if err := p.eventPublisher.PublishQualityEvent(ctx, qualityEvent); err != nil {
			log.Logger.Errorf("publish quality outbox event %s failed: %v", record.ID, err)
			continue
		}

		if err := p.repo.MarkOutboxEventPublished(ctx, record.ID); err != nil {
			log.Logger.Errorf("mark quality outbox event %s published failed: %v", record.ID, err)
		}
	}

	return nil
}

func qualityEventFromOutbox(event quality.QualityOutboxEvent) quality.QualityEvent {
	stage := "quality_event"
	progress := 0
	switch event.EventType {
	case quality.OutboxEventEvaluationCreated:
		stage = "evaluation_created"
	case quality.OutboxEventEvaluationStarted:
		stage = "evaluation_started"
	case quality.OutboxEventEvaluationCompleted:
		stage = "evaluation_completed"
		progress = 100
	case quality.OutboxEventEvaluationFailed:
		stage = "evaluation_failed"
	case quality.OutboxEventGateUpdated:
		stage = "gate_updated"
	case quality.OutboxEventGateStale:
		stage = "gate_stale"
	}
	return quality.QualityEvent{
		JobID:            event.ExecutionID,
		ProcessingTaskID: event.ProcessingTaskID,
		ActionKey:        event.ActionKey,
		EvaluationID:     event.EvaluationID,
		Stage:            stage,
		Progress:         progress,
		Status:           event.Status,
		Message:          event.Verdict,
	}
}
