package mindruntime

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (c *DataLifecycleCoordinator) scheduleOutboxCleanupLocked(tombstone DeletionTombstone) []OutboxCleanupItem {
	storages := []string{"primary", "sqlite", "qdrant", "surrealdb", "cache", "summaries", "reflections", "traces"}
	items := make([]OutboxCleanupItem, 0, len(storages))
	now := time.Now().UTC()
	for _, storage := range storages {
		item := OutboxCleanupItem{
			ID:          fmt.Sprintf("outbox_%s_%s", tombstone.ID, storage),
			Storage:     storage,
			TargetID:    tombstone.TargetID,
			TargetKind:  tombstone.TargetType,
			Status:      CleanupItemStatusQueued,
			Attempts:    0,
			MaxAttempts: DefaultCleanupMaxAttempts,
			NextRetryAt: now,
		}
		items = append(items, item)
	}
	return items
}

func (c *DataLifecycleCoordinator) ReleaseExpiredCleanupLeases() {
	if c.db == nil {
		return
	}
	now := time.Now().UTC()
	c.db.Model(&OutboxCleanupItemModel{}).
		Where("status = ? AND leased_until <= ?", string(CleanupItemStatusClaimed), now).
		Updates(map[string]interface{}{
			"status":       CleanupItemStatusQueued,
			"lease_owner":  "",
			"lease_token":  "",
			"leased_until": time.Time{},
		})
}

func (c *DataLifecycleCoordinator) LeaseCleanupBatch() ([]OutboxCleanupItem, error) {
	if c.db == nil {
		return nil, nil
	}
	now := time.Now().UTC()
	leaseOwner := uuid.New().String()
	leaseToken := generateItemToken()
	leaseUntil := now.Add(c.cleanupLeaseDuration)

	leased := make([]OutboxCleanupItem, 0, c.cleanupBatchSize)

	err := c.db.Transaction(func(tx *gorm.DB) error {
		var models []OutboxCleanupItemModel
		err := tx.Where(
			"status IN ? AND next_retry_at <= ? AND (lease_owner = '' OR leased_until <= ?)",
			[]string{string(CleanupItemStatusQueued), string(CleanupItemStatusRetry)},
			now, now,
		).Order("next_retry_at ASC").Limit(c.cleanupBatchSize).Find(&models).Error
		if err != nil {
			return err
		}
		for _, model := range models {
			result := tx.Model(&OutboxCleanupItemModel{}).
				Where("id = ? AND status IN ? AND (lease_owner = '' OR leased_until <= ?)",
					model.ID,
					[]string{string(CleanupItemStatusQueued), string(CleanupItemStatusRetry)},
					now,
				).
				Updates(map[string]interface{}{
					"status":       CleanupItemStatusClaimed,
					"lease_owner":  leaseOwner,
					"lease_token":  leaseToken,
					"leased_until": leaseUntil,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}
			model.Status = string(CleanupItemStatusClaimed)
			model.LeaseOwner = leaseOwner
			model.LeaseToken = leaseToken
			model.LeasedUntil = leaseUntil
			leased = append(leased, outboxItemFromModel(model))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return leased, nil
}

func (c *DataLifecycleCoordinator) collectInMemoryItems() []OutboxCleanupItem {
	c.mu.RLock()
	defer c.mu.RUnlock()
	items := make([]OutboxCleanupItem, 0, c.cleanupBatchSize)
	for i := range c.outbox {
		item := &c.outbox[i]
		if item.Status != CleanupItemStatusQueued && item.Status != CleanupItemStatusRetry {
			continue
		}
		if item.MaxAttempts > 0 && item.Attempts >= item.MaxAttempts {
			item.Status = CleanupItemStatusDead
			continue
		}
		item.Status = CleanupItemStatusClaimed
		items = append(items, *item)

	}
	return items
}

func (c *DataLifecycleCoordinator) markInMemoryCleanupDone(itemID string, status CleanupItemStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now().UTC()
	for i := range c.outbox {
		if c.outbox[i].ID == itemID {
			c.outbox[i].Status = status
			c.outbox[i].CleanedAt = &now
			c.outbox[i].LastError = ""
			c.outbox[i].LeaseOwner = ""
			c.outbox[i].LeaseToken = ""
			c.outbox[i].LeasedUntil = time.Time{}
			return
		}
	}
}

func (c *DataLifecycleCoordinator) markInMemoryCleanupFailed(itemID string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.outbox {
		if c.outbox[i].ID == itemID {
			c.outbox[i].LastError = err.Error()
			c.outbox[i].LeaseOwner = ""
			c.outbox[i].LeaseToken = ""
			c.outbox[i].LeasedUntil = time.Time{}
			return
		}
	}
}

func (c *DataLifecycleCoordinator) ExecuteOutboxCleanup() ([]OutboxCleanupItem, error) {
	if c.db != nil {
		c.ReleaseExpiredCleanupLeases()
	}
	var allResults []OutboxCleanupItem
	var allErrs []error
	iter := 0
	for {
		iter++
		if iter > DefaultMaxCleanupIterations {
			break
		}
		items, err := c.LeaseCleanupBatch()
		if err != nil {
			return nil, fmt.Errorf("lease cleanup batch: %w", err)
		}
		if len(items) == 0 {
			if c.db == nil {
				items = c.collectInMemoryItems()
			}
			if len(items) == 0 {
				break
			}
		}
		executor := c.getCleanupExecutorLocked()
		results := make([]OutboxCleanupItem, 0, len(items))
		var batchErrs []error
		now := time.Now().UTC()
		for i := range items {
			item := items[i]
			if executor == nil {
				err := fmt.Errorf("data_lifecycle: cleanup executor is not configured for %s", item.Storage)
				c.MarkCleanupFailed(item.ID, item.LeaseOwner, item.LeaseToken, err)
				items[i].Attempts++
				items[i].LastError = err.Error()
				if items[i].MaxAttempts <= 0 {
					items[i].MaxAttempts = DefaultCleanupMaxAttempts
				}
				if items[i].Attempts >= items[i].MaxAttempts {
					items[i].Status = CleanupItemStatusDead
				} else {
					items[i].Status = CleanupItemStatusRetry
					items[i].NextRetryAt = now.Add(DefaultCleanupRetryBackoffBase * time.Duration(items[i].Attempts))
				}
				batchErrs = append(batchErrs, err)
				continue
			}
			cleanErr := executor.CleanupOutboxItem(item)
			if cleanErr != nil {
				wrapped := fmt.Errorf("data_lifecycle: cleanup %s/%s in %s: %w", item.TargetKind, item.TargetID, item.Storage, cleanErr)
				c.MarkCleanupFailed(item.ID, item.LeaseOwner, item.LeaseToken, wrapped)
				items[i].Attempts++
				items[i].LastError = wrapped.Error()
				if items[i].MaxAttempts <= 0 {
					items[i].MaxAttempts = DefaultCleanupMaxAttempts
				}
				if items[i].Attempts >= items[i].MaxAttempts {
					items[i].Status = CleanupItemStatusDead
				} else {
					items[i].Status = CleanupItemStatusRetry
					items[i].NextRetryAt = now.Add(DefaultCleanupRetryBackoffBase * time.Duration(items[i].Attempts))
				}
				batchErrs = append(batchErrs, wrapped)
			} else {
				c.MarkCleanupCompleted(item.ID, item.LeaseOwner, item.LeaseToken)
				items[i].Status = CleanupItemStatusCompleted
				items[i].LastError = ""
				items[i].CleanedAt = &now
			}
		}
		results = append(results, items...)
		c.persistOutboxResults(items)
		c.updateTombstoneAfterCleanup(items)
		allResults = append(allResults, results...)
		allErrs = append(allErrs, batchErrs...)
		if c.db == nil {
			break
		}
	}

	c.mu.Lock()
	c.lastClean = time.Now().UTC()
	c.mu.Unlock()

	DefaultMetricsCollector.IncrementCounter("data_lifecycle", "outbox_cleanups", 1)

	return allResults, errors.Join(allErrs...)
}

func (c *DataLifecycleCoordinator) getCleanupExecutorLocked() OutboxCleanupExecutor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cleanupExecutor
}

func (c *DataLifecycleCoordinator) MarkCleanupCompleted(itemID string, leaseOwner string, leaseToken string) {
	if c.db == nil {
		c.markInMemoryCleanupDone(itemID, CleanupItemStatusCompleted)
		return
	}
	now := time.Now().UTC()
	c.db.Model(&OutboxCleanupItemModel{}).
		Where("id = ? AND lease_owner = ? AND lease_token = ? AND status = ?",
			itemID, leaseOwner, leaseToken, string(CleanupItemStatusClaimed)).
		Updates(map[string]interface{}{
			"status":       CleanupItemStatusCompleted,
			"cleaned_at":   now,
			"last_error":   "",
			"lease_owner":  "",
			"lease_token":  "",
			"leased_until": time.Time{},
		})
}

func (c *DataLifecycleCoordinator) MarkCleanupFailed(itemID string, leaseOwner string, leaseToken string, err error) {
	if c.db == nil {
		c.markInMemoryCleanupFailed(itemID, err)
		return
	}
	var model OutboxCleanupItemModel
	if txErr := c.db.Where("id = ? AND lease_owner = ? AND lease_token = ?",
		itemID, leaseOwner, leaseToken).First(&model).Error; txErr != nil {
		return
	}
	attempts := model.Attempts + 1
	maxAttempts := model.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultCleanupMaxAttempts
	}
	now := time.Now().UTC()
	if attempts >= maxAttempts {
		c.db.Model(&OutboxCleanupItemModel{}).
			Where("id = ? AND lease_owner = ? AND lease_token = ?", itemID, leaseOwner, leaseToken).
			Updates(map[string]interface{}{
				"status":       CleanupItemStatusDead,
				"attempts":     attempts,
				"last_error":   err.Error(),
				"lease_owner":  "",
				"lease_token":  "",
				"leased_until": time.Time{},
			})
		c.moveCleanupToDeadItem(model, err)
		return
	}
	nextRetry := now.Add(DefaultCleanupRetryBackoffBase * time.Duration(attempts))
	c.db.Model(&OutboxCleanupItemModel{}).
		Where("id = ? AND lease_owner = ? AND lease_token = ?", itemID, leaseOwner, leaseToken).
		Updates(map[string]interface{}{
			"status":        CleanupItemStatusRetry,
			"attempts":      attempts,
			"next_retry_at": nextRetry,
			"last_error":    err.Error(),
			"lease_owner":   "",
			"lease_token":   "",
			"leased_until":  time.Time{},
		})
}

func (c *DataLifecycleCoordinator) moveCleanupToDeadItem(model OutboxCleanupItemModel, err error) {
	deadItem := OutboxCleanupItem{
		ID:          model.ID,
		Storage:     model.Storage,
		TargetID:    model.TargetID,
		TargetKind:  model.TargetKind,
		Status:      CleanupItemStatusDead,
		Attempts:    model.Attempts + 1,
		MaxAttempts: model.MaxAttempts,
		LastError:   err.Error(),
	}
	_ = deadItem
}

func (c *DataLifecycleCoordinator) persistOutboxResults(items []OutboxCleanupItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, item := range items {
		for i := range c.outbox {
			if c.outbox[i].ID == item.ID {
				c.outbox[i] = item
				break
			}
		}
	}
}

func (c *DataLifecycleCoordinator) updateTombstoneAfterCleanup(items []OutboxCleanupItem) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, item := range items {
		for key, tombstone := range c.tombstones {
			if tombstone.TargetID != item.TargetID || tombstone.TargetType != item.TargetKind {
				continue
			}
			itemsCount := 0
			cleanedCount := 0
			failedCount := 0
			for _, oi := range c.outbox {
				if oi.TargetID != item.TargetID || oi.TargetKind != item.TargetKind {
					continue
				}
				itemsCount++
				if oi.Status == CleanupItemStatusCompleted {
					cleanedCount++
				} else if oi.Status == CleanupItemStatusDead {
					failedCount++
				}
			}
			tombstone.ItemsCount = itemsCount
			tombstone.CleanedCount = cleanedCount
			tombstone.FailedCount = failedCount
			if itemsCount > 0 && cleanedCount == itemsCount && failedCount == 0 {
				now := time.Now().UTC()
				tombstone.Status = DeletionStatusCompleted
				tombstone.CompletedAt = &now
			} else if cleanedCount > 0 || failedCount > 0 {
				tombstone.Status = DeletionStatusCleaning
			}
			c.tombstones[key] = tombstone
			_ = c.persistTombstoneLocked(tombstone)
			break
		}
	}
}
