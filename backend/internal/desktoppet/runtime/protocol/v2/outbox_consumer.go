// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package v2

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/u-ai/backend/log"
	"gorm.io/gorm"
)

const (
	outboxPollInterval  = 5 * time.Second
	outboxClaimTTL      = 30 * time.Second
	outboxMaxAttempts   = 3
	outboxBatchSize     = 10
	outboxFailThreshold = 10 * time.Minute
)

type OutboxHandler func(ctx context.Context, event DomainEventOutbox) error

type OutboxConsumer struct {
	db           *gorm.DB
	handler      OutboxHandler
	pollInterval time.Duration
	stopCh       chan struct{}
	doneCh       chan struct{}
	lifecycleMu  sync.Mutex
	running      bool
	alive        atomic.Bool
}

func NewOutboxConsumer(db *gorm.DB, handler OutboxHandler) *OutboxConsumer {
	return &OutboxConsumer{
		db:           db,
		handler:      handler,
		pollInterval: outboxPollInterval,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
}

func (c *OutboxConsumer) Start(ctx context.Context) {
	if c == nil || c.db == nil || c.handler == nil {
		return
	}
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if c.running {
		return
	}
	c.stopCh = make(chan struct{})
	c.doneCh = make(chan struct{})
	c.running = true
	c.alive.Store(true)
	go c.run(ctx, c.stopCh, c.doneCh)
}

func (c *OutboxConsumer) Stop() {
	if c == nil {
		return
	}
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	if !c.running {
		return
	}
	close(c.stopCh)
	<-c.doneCh
	c.running = false
	c.alive.Store(false)
}

func (c *OutboxConsumer) IsRunning() bool {
	if c == nil {
		return false
	}
	c.lifecycleMu.Lock()
	defer c.lifecycleMu.Unlock()
	return c.running && c.alive.Load()
}

func (c *OutboxConsumer) run(ctx context.Context, stopCh <-chan struct{}, doneCh chan<- struct{}) {
	defer close(doneCh)
	defer c.alive.Store(false)
	ticker := time.NewTicker(c.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ctx.Done():
			return
		case <-ticker.C:
			c.processBatch(ctx)
		}
	}
}

func (c *OutboxConsumer) processBatch(ctx context.Context) {
	now := time.Now()
	events, err := c.claimPendingEvents(now)
	if err != nil {
		log.Error("outbox consumer claim failed: ", err)
		return
	}
	for _, event := range events {
		c.processEvent(ctx, event, now)
	}
}

func (c *OutboxConsumer) claimPendingEvents(now time.Time) ([]DomainEventOutbox, error) {
	claimExpiry := now.Add(outboxClaimTTL).Format("2006-01-02 15:04:05")
	nowStr := now.Format("2006-01-02 15:04:05")

	var events []DomainEventOutbox
	err := c.db.Where(
		"status = ? OR (status = ? AND (claim_expires_at < ? OR claim_expires_at = ''))",
		OutboxStatusPending, OutboxStatusClaimed, nowStr,
	).Order("inserted_at ASC").Limit(outboxBatchSize).Find(&events).Error
	if err != nil {
		return nil, err
	}

	var claimed []DomainEventOutbox
	for _, event := range events {
		result := c.db.Model(&DomainEventOutbox{}).Where(
			"id = ? AND (status = ? OR (status = ? AND (claim_expires_at < ? OR claim_expires_at = '')))",
			event.ID, OutboxStatusPending, OutboxStatusClaimed, nowStr,
		).Updates(map[string]interface{}{
			"status":           OutboxStatusClaimed,
			"claim_expires_at": claimExpiry,
			"updated_at":       nowStr,
		})
		if result.Error != nil || result.RowsAffected == 0 {
			continue
		}
		event.Status = OutboxStatusClaimed
		claimed = append(claimed, event)
	}
	return claimed, nil
}

func (c *OutboxConsumer) processEvent(ctx context.Context, event DomainEventOutbox, now time.Time) {
	if event.Attempt >= outboxMaxAttempts {
		if err := c.markFailed(event, "max attempts reached", now); err != nil {
			log.Error("outbox mark terminal failure failed: ", event.ID, " error: ", err)
		}
		return
	}

	handlerCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := c.handler(handlerCtx, event); err != nil {
		log.Error("outbox event handler failed: ", event.ID, " error: ", err)
		if markErr := c.markAttemptFailed(event, err, now); markErr != nil {
			log.Error("outbox persist retry failure failed: ", event.ID, " error: ", markErr)
		}
		return
	}

	if err := c.markSent(event, now); err != nil {
		log.Error("outbox mark sent failed: ", event.ID, " error: ", err)
	}
}

func (c *OutboxConsumer) markSent(event DomainEventOutbox, now time.Time) error {
	publishedAt := now.Format("2006-01-02 15:04:05")
	return c.db.Model(&DomainEventOutbox{}).Where("id = ?", event.ID).Updates(map[string]interface{}{
		"status":       OutboxStatusSent,
		"published_at": publishedAt,
		"updated_at":   publishedAt,
		"last_error":   "",
	}).Error
}

func (c *OutboxConsumer) markAttemptFailed(event DomainEventOutbox, handlerErr error, now time.Time) error {
	nowStr := now.Format("2006-01-02 15:04:05")
	nextAttempt := event.Attempt + 1
	updates := map[string]interface{}{
		"attempt":          nextAttempt,
		"last_error":       handlerErr.Error(),
		"updated_at":       nowStr,
		"claim_expires_at": "",
	}
	if nextAttempt >= outboxMaxAttempts {
		updates["status"] = OutboxStatusFailed
	} else {
		updates["status"] = OutboxStatusPending
	}
	return c.db.Model(&DomainEventOutbox{}).Where("id = ?", event.ID).Updates(updates).Error
}

func (c *OutboxConsumer) markFailed(event DomainEventOutbox, reason string, now time.Time) error {
	nowStr := now.Format("2006-01-02 15:04:05")
	return c.db.Model(&DomainEventOutbox{}).Where("id = ?", event.ID).Updates(map[string]interface{}{
		"status":     OutboxStatusFailed,
		"last_error": reason,
		"updated_at": nowStr,
	}).Error
}

func (c *OutboxConsumer) FailAbandonedEvents() (int64, error) {
	threshold := time.Now().Add(-outboxFailThreshold)
	thresholdStr := threshold.Format("2006-01-02 15:04:05")
	now := time.Now().Format("2006-01-02 15:04:05")

	var count int64
	err := c.db.Model(&DomainEventOutbox{}).Where(
		"status = ? AND inserted_at < ?",
		OutboxStatusPending, thresholdStr,
	).Count(&count).Error
	if err != nil {
		return 0, err
	}

	result := c.db.Model(&DomainEventOutbox{}).Where(
		"status = ? AND inserted_at < ?",
		OutboxStatusPending, thresholdStr,
	).Updates(map[string]interface{}{
		"status":     OutboxStatusFailed,
		"last_error": "expired",
		"updated_at": now,
	})
	return count, result.Error
}

type OutboxStats struct {
	Pending int64
	Claimed int64
	Sent    int64
	Failed  int64
}

func (c *OutboxConsumer) Stats() (*OutboxStats, error) {
	stats := &OutboxStats{}
	if err := c.db.Model(&DomainEventOutbox{}).
		Where("status = ?", OutboxStatusPending).Count(&stats.Pending).Error; err != nil {
		return nil, err
	}
	if err := c.db.Model(&DomainEventOutbox{}).
		Where("status = ?", OutboxStatusClaimed).Count(&stats.Claimed).Error; err != nil {
		return nil, err
	}
	if err := c.db.Model(&DomainEventOutbox{}).
		Where("status = ?", OutboxStatusSent).Count(&stats.Sent).Error; err != nil {
		return nil, err
	}
	if err := c.db.Model(&DomainEventOutbox{}).
		Where("status = ?", OutboxStatusFailed).Count(&stats.Failed).Error; err != nil {
		return nil, err
	}
	return stats, nil
}
