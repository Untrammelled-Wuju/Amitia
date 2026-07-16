package mindruntime

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (c *DataLifecycleCoordinator) ReleaseExpiredRecalcLeases() {
	if c.db == nil {
		return
	}
	now := time.Now().UTC()
	c.db.Model(&RecalculationTaskModel{}).
		Where("status = ? AND leased_until <= ?", string(RecalculationTaskStatusClaimed), now).
		Updates(map[string]interface{}{
			"status":       RecalculationTaskStatusPending,
			"lease_owner":  "",
			"lease_token":  "",
			"leased_until": time.Time{},
		})
}

func (c *DataLifecycleCoordinator) ExecuteRecalculationTasks() ([]RecalculationTask, error) {
	if c.db != nil {
		c.ReleaseExpiredRecalcLeases()
	}
	tasks, err := c.LeaseRecalcBatch()
	if err != nil {
		return nil, fmt.Errorf("lease recalc batch: %w", err)
	}
	if len(tasks) == 0 {
		return nil, nil
	}
	executor := c.getRecalcExecutorLocked()
	results := make([]RecalculationTask, 0, len(tasks))
	var execErrs []error
	for i := range tasks {
		task := tasks[i]
		if executor == nil {
			err := fmt.Errorf("data_lifecycle: recalc executor is not configured")
			c.MarkRecalcFailed(task.ID, task.LeaseOwner, task.LeaseToken, err)
			execErrs = append(execErrs, err)
			continue
		}
		execErr := executor.ExecuteRecalculation(task)
		if execErr != nil {
			wrapped := fmt.Errorf("data_lifecycle: recalc %s/%s zone=%s: %w", task.TriggerType, task.TargetID, task.AffectedZone, execErr)
			c.MarkRecalcFailed(task.ID, task.LeaseOwner, task.LeaseToken, wrapped)
			execErrs = append(execErrs, wrapped)
		} else {
			c.MarkRecalcCompleted(task.ID, task.LeaseOwner, task.LeaseToken)
		}
	}
	results = append(results, tasks...)
	c.persistRecalcResults(tasks)

	DefaultMetricsCollector.IncrementCounter("data_lifecycle", "recalc_executions", 1)

	return results, errors.Join(execErrs...)
}

func (c *DataLifecycleCoordinator) getRecalcExecutorLocked() RecalculationTaskExecutor {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.recalcExecutor
}

func (c *DataLifecycleCoordinator) LeaseRecalcBatch() ([]RecalculationTask, error) {
	if c.db == nil {
		return nil, nil
	}
	now := time.Now().UTC()
	leaseOwner := uuid.New().String()
	leaseToken := generateItemToken()
	leaseUntil := now.Add(c.recalcLeaseDuration)

	leased := make([]RecalculationTask, 0, c.recalcBatchSize)

	err := c.db.Transaction(func(tx *gorm.DB) error {
		var models []RecalculationTaskModel
		err := tx.Where(
			"status IN ? AND next_retry_at <= ? AND (lease_owner = '' OR leased_until <= ?)",
			[]string{string(RecalculationTaskStatusPending), string(RecalculationTaskStatusFailed)},
			now, now,
		).Order("priority ASC, next_retry_at ASC").Limit(c.recalcBatchSize).Find(&models).Error
		if err != nil {
			return err
		}
		for _, model := range models {
			result := tx.Model(&RecalculationTaskModel{}).
				Where("id = ? AND status IN ? AND (lease_owner = '' OR leased_until <= ?)",
					model.ID,
					[]string{string(RecalculationTaskStatusPending), string(RecalculationTaskStatusFailed)},
					now,
				).
				Updates(map[string]interface{}{
					"status":       RecalculationTaskStatusClaimed,
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
			model.Status = string(RecalculationTaskStatusClaimed)
			model.LeaseOwner = leaseOwner
			model.LeaseToken = leaseToken
			model.LeasedUntil = leaseUntil
			leased = append(leased, recalculationTaskFromModel(model))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return leased, nil
}

func (c *DataLifecycleCoordinator) MarkRecalcCompleted(taskID string, leaseOwner string, leaseToken string) {
	if c.db == nil {
		return
	}
	now := time.Now().UTC()
	c.db.Model(&RecalculationTaskModel{}).
		Where("id = ? AND lease_owner = ? AND lease_token = ? AND status = ?",
			taskID, leaseOwner, leaseToken, string(RecalculationTaskStatusClaimed)).
		Updates(map[string]interface{}{
			"status":       RecalculationTaskStatusCompleted,
			"completed_at": now,
			"lease_owner":  "",
			"lease_token":  "",
			"leased_until": time.Time{},
		})
}

func (c *DataLifecycleCoordinator) MarkRecalcFailed(taskID string, leaseOwner string, leaseToken string, err error) {
	if c.db == nil {
		return
	}
	var model RecalculationTaskModel
	if txErr := c.db.Where("id = ? AND lease_owner = ? AND lease_token = ?",
		taskID, leaseOwner, leaseToken).First(&model).Error; txErr != nil {
		return
	}
	attempts := model.Attempts + 1
	maxAttempts := model.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = DefaultRecalcMaxAttempts
	}
	now := time.Now().UTC()
	if attempts >= maxAttempts {
		c.db.Model(&RecalculationTaskModel{}).
			Where("id = ? AND lease_owner = ? AND lease_token = ?", taskID, leaseOwner, leaseToken).
			Updates(map[string]interface{}{
				"status":       RecalculationTaskStatusDead,
				"attempts":     attempts,
				"last_error":   err.Error(),
				"lease_owner":  "",
				"lease_token":  "",
				"leased_until": time.Time{},
			})
		return
	}
	nextRetry := now.Add(time.Minute * time.Duration(attempts*2))
	c.db.Model(&RecalculationTaskModel{}).
		Where("id = ? AND lease_owner = ? AND lease_token = ?", taskID, leaseOwner, leaseToken).
		Updates(map[string]interface{}{
			"status":        RecalculationTaskStatusFailed,
			"attempts":      attempts,
			"next_retry_at": nextRetry,
			"last_error":    err.Error(),
			"lease_owner":   "",
			"lease_token":   "",
			"leased_until":  time.Time{},
		})
}

func (c *DataLifecycleCoordinator) persistRecalcResults(tasks []RecalculationTask) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, task := range tasks {
		for i := range c.recalcTasks {
			if c.recalcTasks[i].ID == task.ID {
				c.recalcTasks[i] = task
				break
			}
		}
	}
}

func (c *DataLifecycleCoordinator) loadPersistedOutboxItemsLocked() {
	if c.db == nil {
		return
	}
	known := make(map[string]int, len(c.outbox))
	for i := range c.outbox {
		known[c.outbox[i].ID] = i
	}
	var models []OutboxCleanupItemModel
	if err := c.db.Find(&models).Error; err != nil {
		return
	}
	for _, model := range models {
		item := outboxItemFromModel(model)
		if idx, ok := known[item.ID]; ok {
			c.outbox[idx] = item
			continue
		}
		c.outbox = append(c.outbox, item)
	}
}

func (c *DataLifecycleCoordinator) buildRecalculationTasksLocked(tombstone DeletionTombstone) []RecalculationTask {
	tasks := make([]RecalculationTask, 0)
	now := time.Now().UTC()

	if tombstone.Scope == DeletionScopeAll || tombstone.Scope == DeletionScopeBelief {
		task := RecalculationTask{
			ID:           fmt.Sprintf("recalc_%s_belief", tombstone.ID),
			TriggerType:  "deletion",
			TargetID:     tombstone.TargetID,
			AffectedZone: "belief",
			Priority:     1,
			CreatedAt:    now,
			Status:       RecalculationTaskStatusPending,
			MaxAttempts:  DefaultRecalcMaxAttempts,
			NextRetryAt:  now,
			Description:  fmt.Sprintf("recalculate belief resolution after deletion of %s/%s", tombstone.TargetType, tombstone.TargetID),
		}
		tasks = append(tasks, task)
	}

	if tombstone.Scope == DeletionScopeAll || tombstone.Scope == DeletionScopeRelation {
		task := RecalculationTask{
			ID:           fmt.Sprintf("recalc_%s_relation", tombstone.ID),
			TriggerType:  "deletion",
			TargetID:     tombstone.TargetID,
			AffectedZone: "relationship",
			Priority:     2,
			CreatedAt:    now,
			Status:       RecalculationTaskStatusPending,
			MaxAttempts:  DefaultRecalcMaxAttempts,
			NextRetryAt:  now,
			Description:  fmt.Sprintf("recalculate relationship narrative after deletion of %s/%s", tombstone.TargetType, tombstone.TargetID),
		}
		tasks = append(tasks, task)
	}

	if tombstone.Scope == DeletionScopeAll || tombstone.Scope == DeletionScopeMemory {
		task := RecalculationTask{
			ID:           fmt.Sprintf("recalc_%s_memory", tombstone.ID),
			TriggerType:  "deletion",
			TargetID:     tombstone.TargetID,
			AffectedZone: "memory",
			Priority:     3,
			CreatedAt:    now,
			Status:       RecalculationTaskStatusPending,
			MaxAttempts:  DefaultRecalcMaxAttempts,
			NextRetryAt:  now,
			Description:  fmt.Sprintf("recalculate memory summaries after deletion of %s/%s", tombstone.TargetType, tombstone.TargetID),
		}
		tasks = append(tasks, task)
	}

	return tasks
}

func (c *DataLifecycleCoordinator) GenerateRecalculationTasks(tombstone DeletionTombstone) []RecalculationTask {
	c.mu.Lock()
	defer c.mu.Unlock()

	tasks := c.buildRecalculationTasksLocked(tombstone)

	c.recalcTasks = append(c.recalcTasks, tasks...)
	_ = c.persistRecalculationTasksLocked(tasks)
	DefaultMetricsCollector.IncrementCounter("data_lifecycle", "recalc_tasks_generated", int64(len(tasks)))

	return tasks
}
