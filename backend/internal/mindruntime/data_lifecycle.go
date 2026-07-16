package mindruntime

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

type DataLifecycleCoordinator struct {
	db                   *gorm.DB
	mu                   sync.RWMutex
	tombstones           map[string]DeletionTombstone
	outbox               []OutboxCleanupItem
	recalcTasks          []RecalculationTask
	lastClean            time.Time
	cleanupExecutor      OutboxCleanupExecutor
	recalcExecutor       RecalculationTaskExecutor
	cleanupBatchSize     int
	recalcBatchSize      int
	cleanupLeaseDuration time.Duration
	recalcLeaseDuration  time.Duration
}

var DefaultDataLifecycleCoordinator = NewDataLifecycleCoordinator(nil)

func NewDataLifecycleCoordinator(db *gorm.DB) *DataLifecycleCoordinator {
	return &DataLifecycleCoordinator{
		db:                   db,
		tombstones:           make(map[string]DeletionTombstone),
		outbox:               make([]OutboxCleanupItem, 0),
		recalcTasks:          make([]RecalculationTask, 0),
		cleanupBatchSize:     DefaultCleanupBatchSize,
		recalcBatchSize:      DefaultRecalcBatchSize,
		cleanupLeaseDuration: DefaultCleanupLeaseDuration,
		recalcLeaseDuration:  DefaultRecalcLeaseDuration,
	}
}

func (c *DataLifecycleCoordinator) SetOutboxCleanupExecutor(executor OutboxCleanupExecutor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cleanupExecutor = executor
}

func (c *DataLifecycleCoordinator) SetRecalculationTaskExecutor(executor RecalculationTaskExecutor) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recalcExecutor = executor
}

func (c *DataLifecycleCoordinator) InitSchema() error {
	if c.db == nil {
		return fmt.Errorf("data_lifecycle: db is nil, cannot init schema")
	}
	if err := c.db.AutoMigrate(&DeletionTombstoneModel{}, &OutboxCleanupItemModel{}, &RecalculationTaskModel{}); err != nil {
		return err
	}
	return c.loadPersistedState()
}

func (c *DataLifecycleCoordinator) RequestDeletion(req DeletionRequest) (DeletionTombstone, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := generateTombstoneID(req.TargetID, req.TargetType)

	now := time.Now().UTC()
	tombstone := DeletionTombstone{
		ID:               id,
		TargetID:         req.TargetID,
		TargetType:       req.TargetType,
		Scope:            req.Scope,
		RequestedAt:      now,
		BlockedUntil:     now,
		Status:           DeletionStatusBlocked,
		ItemsCount:       0,
		CleanedCount:     0,
		FailedCount:      0,
		RetrievalBlocked: true,
	}

	items := c.scheduleOutboxCleanupLocked(tombstone)
	tombstone.ItemsCount = len(items)

	recalcTasks := c.buildRecalculationTasksLocked(tombstone)

	if c.db != nil {
		err := c.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Save(tombstoneToModel(tombstone)).Error; err != nil {
				return fmt.Errorf("persist tombstone: %w", err)
			}
			for _, item := range items {
				if err := tx.Save(outboxItemToModel(item)).Error; err != nil {
					return fmt.Errorf("persist outbox item %s: %w", item.ID, err)
				}
			}
			for _, task := range recalcTasks {
				if err := tx.Save(recalculationTaskToModel(task)).Error; err != nil {
					return fmt.Errorf("persist recalc task %s: %w", task.ID, err)
				}
			}
			return nil
		})
		if err != nil {
			return tombstone, err
		}
	}

	c.tombstones[id] = tombstone
	for _, item := range items {
		c.outbox = append(c.outbox, item)
	}
	c.recalcTasks = append(c.recalcTasks, recalcTasks...)

	DefaultMetricsCollector.IncrementCounter("data_lifecycle", "deletion_requests", 1)
	DefaultMetricsCollector.IncrementCounter("data_lifecycle", "recalc_tasks_generated", int64(len(recalcTasks)))

	return tombstone, nil
}

func (c *DataLifecycleCoordinator) IsRetrievalBlocked(targetID string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	id := normalizeTargetID(targetID)
	for _, t := range c.tombstones {
		if normalizeTargetID(t.TargetID) == id && t.RetrievalBlocked {
			return true
		}
	}
	return false
}

func (c *DataLifecycleCoordinator) BlockedEntityIDs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	ids := make([]string, 0, len(c.tombstones))
	for _, t := range c.tombstones {
		if t.RetrievalBlocked && t.TargetType != "" && t.TargetType != "character" {
			ids = append(ids, t.TargetID)
		}
	}
	return ids
}

func (c *DataLifecycleCoordinator) BlockedEntityIDsByType(targetType string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	targetType = strings.TrimSpace(targetType)
	ids := make([]string, 0, len(c.tombstones))
	for _, t := range c.tombstones {
		if t.RetrievalBlocked && strings.TrimSpace(t.TargetType) == targetType {
			ids = append(ids, t.TargetID)
		}
	}
	return ids
}

func (c *DataLifecycleCoordinator) GetTombstone(targetID string) (DeletionTombstone, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	id := normalizeTargetID(targetID)
	for _, t := range c.tombstones {
		if normalizeTargetID(t.TargetID) == id {
			return t, true
		}
	}
	return DeletionTombstone{}, false
}

func (c *DataLifecycleCoordinator) MarkDeletionComplete(targetID string) (DeletionTombstone, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	id := normalizeTargetID(targetID)
	for key, t := range c.tombstones {
		if normalizeTargetID(t.TargetID) == id && t.Status != DeletionStatusCompleted {
			now := time.Now().UTC()
			t.Status = DeletionStatusCompleted
			t.CompletedAt = &now
			c.tombstones[key] = t
			_ = c.persistTombstoneLocked(t)
			return t, true
		}
	}
	return DeletionTombstone{}, false
}

func (c *DataLifecycleCoordinator) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tombstones := len(c.tombstones)
	outboxItems := len(c.outbox)
	recalcTasks := len(c.recalcTasks)
	completed := 0
	failed := 0
	for _, t := range c.tombstones {
		if t.Status == DeletionStatusCompleted {
			completed++
		}
		if t.Status == DeletionStatusFailed {
			failed++
		}
	}
	return map[string]interface{}{
		"tombstones":  tombstones,
		"outboxItems": outboxItems,
		"recalcTasks": recalcTasks,
		"completed":   completed,
		"failed":      failed,
	}
}

func (c *DataLifecycleCoordinator) Reset() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.tombstones = make(map[string]DeletionTombstone)
	c.outbox = make([]OutboxCleanupItem, 0)
	c.recalcTasks = make([]RecalculationTask, 0)
	c.lastClean = time.Time{}
}

func (c *DataLifecycleCoordinator) GetOutboxItems() []OutboxCleanupItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := make([]OutboxCleanupItem, len(c.outbox))
	copy(items, c.outbox)
	return items
}

func (c *DataLifecycleCoordinator) GetRecalculationTasks() []RecalculationTask {
	c.mu.RLock()
	defer c.mu.RUnlock()

	tasks := make([]RecalculationTask, len(c.recalcTasks))
	copy(tasks, c.recalcTasks)
	return tasks
}
