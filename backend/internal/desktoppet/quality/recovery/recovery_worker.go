// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package recovery

import (
	"context"
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/quality"
	"github.com/u-ai/backend/log"
)

type RecoveryWorker struct {
	repo         quality.QualityRepository
	pollInterval time.Duration
	stopCh       chan struct{}
}

func NewRecoveryWorker(repo quality.QualityRepository) *RecoveryWorker {
	return &RecoveryWorker{
		repo:         repo,
		pollInterval: 30 * time.Second,
		stopCh:       make(chan struct{}),
	}
}

func (w *RecoveryWorker) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(w.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-w.stopCh:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := w.RecoverStuckEvaluations(ctx); err != nil {
					log.Logger.Errorf("recover stuck evaluations failed: %v", err)
				}
			}
		}
	}()
}

func (w *RecoveryWorker) Stop() {
	close(w.stopCh)
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

		if err := w.repo.UpdateEvaluationStatus(ctx, eval.ID, quality.EvalPending, "", ""); err != nil {
			log.Logger.Errorf("recover stuck evaluation %s failed: %v", eval.ID, err)
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
	events, err := p.repo.ListPendingOutboxEvents(ctx, 100)
	if err != nil {
		return err
	}

	for _, event := range events {
		var qualityEvent quality.QualityEvent
		if err := json.Unmarshal([]byte(event.PayloadJSON), &qualityEvent); err != nil {
			log.Logger.Errorf("unmarshal outbox event %s payload failed: %v", event.ID, err)
			continue
		}

		if err := p.eventPublisher.PublishQualityEvent(ctx, qualityEvent); err != nil {
			log.Logger.Errorf("publish outbox event %s failed: %v", event.ID, err)
			continue
		}

		if err := p.repo.MarkOutboxEventPublished(ctx, event.ID); err != nil {
			log.Logger.Errorf("mark outbox event %s published failed: %v", event.ID, err)
		}
	}

	return nil
}
