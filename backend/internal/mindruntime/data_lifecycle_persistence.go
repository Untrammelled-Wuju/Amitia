package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strings"
	"time"
)

func generateTombstoneID(targetID, targetType string) string {
	raw := fmt.Sprintf("%s:%s:%d", targetType, targetID, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("tomb_%x", hash[:12])
}

func generateItemToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func normalizeTargetID(targetID string) string {
	return strings.ToLower(strings.TrimSpace(targetID))
}

func tombstoneToModel(t DeletionTombstone) DeletionTombstoneModel {
	return DeletionTombstoneModel{
		ID:               t.ID,
		TargetID:         t.TargetID,
		TargetType:       t.TargetType,
		Scope:            string(t.Scope),
		Status:           string(t.Status),
		ItemsCount:       t.ItemsCount,
		CleanedCount:     t.CleanedCount,
		FailedCount:      t.FailedCount,
		RequestedAt:      t.RequestedAt,
		BlockedUntil:     t.BlockedUntil,
		CompletedAt:      t.CompletedAt,
		RetrievalBlocked: t.RetrievalBlocked,
	}
}

func outboxItemToModel(item OutboxCleanupItem) OutboxCleanupItemModel {
	return OutboxCleanupItemModel{
		ID:          item.ID,
		Storage:     item.Storage,
		TargetID:    item.TargetID,
		TargetKind:  item.TargetKind,
		Status:      string(item.Status),
		Attempts:    item.Attempts,
		MaxAttempts: item.MaxAttempts,
		NextRetryAt: item.NextRetryAt,
		LeaseOwner:  item.LeaseOwner,
		LeaseToken:  item.LeaseToken,
		LeasedUntil: item.LeasedUntil,
		LastError:   item.LastError,
		CleanedAt:   item.CleanedAt,
	}
}

func recalculationTaskToModel(task RecalculationTask) RecalculationTaskModel {
	return RecalculationTaskModel{
		ID:           task.ID,
		TriggerType:  task.TriggerType,
		TargetID:     task.TargetID,
		AffectedZone: task.AffectedZone,
		Priority:     task.Priority,
		CreatedAt:    task.CreatedAt,
		Status:       string(task.Status),
		Description:  task.Description,
		Attempts:     task.Attempts,
		MaxAttempts:  task.MaxAttempts,
		NextRetryAt:  task.NextRetryAt,
		LeaseOwner:   task.LeaseOwner,
		LeaseToken:   task.LeaseToken,
		LeasedUntil:  task.LeasedUntil,
		LastError:    task.LastError,
		CompletedAt:  task.CompletedAt,
	}
}

func outboxItemFromModel(m OutboxCleanupItemModel) OutboxCleanupItem {
	return OutboxCleanupItem{
		ID:          m.ID,
		Storage:     m.Storage,
		TargetID:    m.TargetID,
		TargetKind:  m.TargetKind,
		Status:      CleanupItemStatus(m.Status),
		Attempts:    m.Attempts,
		MaxAttempts: m.MaxAttempts,
		NextRetryAt: m.NextRetryAt,
		LeaseOwner:  m.LeaseOwner,
		LeaseToken:  m.LeaseToken,
		LeasedUntil: m.LeasedUntil,
		LastError:   m.LastError,
		CleanedAt:   m.CleanedAt,
	}
}

func recalculationTaskFromModel(m RecalculationTaskModel) RecalculationTask {
	return RecalculationTask{
		ID:           m.ID,
		TriggerType:  m.TriggerType,
		TargetID:     m.TargetID,
		AffectedZone: m.AffectedZone,
		Priority:     m.Priority,
		CreatedAt:    m.CreatedAt,
		Status:       RecalculationTaskStatus(m.Status),
		Description:  m.Description,
		Attempts:     m.Attempts,
		MaxAttempts:  m.MaxAttempts,
		NextRetryAt:  m.NextRetryAt,
		LeaseOwner:   m.LeaseOwner,
		LeaseToken:   m.LeaseToken,
		LeasedUntil:  m.LeasedUntil,
		LastError:    m.LastError,
		CompletedAt:  m.CompletedAt,
	}
}

func (c *DataLifecycleCoordinator) loadPersistedState() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	var tombstoneModels []DeletionTombstoneModel
	if err := c.db.Find(&tombstoneModels).Error; err != nil {
		return err
	}
	for _, tm := range tombstoneModels {
		c.tombstones[tm.ID] = DeletionTombstone{
			ID:               tm.ID,
			TargetID:         tm.TargetID,
			TargetType:       tm.TargetType,
			Scope:            DeletionScope(tm.Scope),
			Status:           DeletionStatus(tm.Status),
			ItemsCount:       tm.ItemsCount,
			CleanedCount:     tm.CleanedCount,
			FailedCount:      tm.FailedCount,
			RequestedAt:      tm.RequestedAt,
			BlockedUntil:     tm.BlockedUntil,
			CompletedAt:      tm.CompletedAt,
			RetrievalBlocked: tm.RetrievalBlocked,
		}
	}

	var outboxModels []OutboxCleanupItemModel
	if err := c.db.Find(&outboxModels).Error; err == nil {
		for _, om := range outboxModels {
			c.outbox = append(c.outbox, outboxItemFromModel(om))
		}
	}

	var recalcModels []RecalculationTaskModel
	if err := c.db.Find(&recalcModels).Error; err == nil {
		for _, rm := range recalcModels {
			c.recalcTasks = append(c.recalcTasks, recalculationTaskFromModel(rm))
		}
	}

	return nil
}

func (c *DataLifecycleCoordinator) persistTombstoneLocked(t DeletionTombstone) error {
	if c.db == nil {
		return nil
	}
	return c.db.Save(tombstoneToModel(t)).Error
}

func (c *DataLifecycleCoordinator) persistOutboxItemLocked(item OutboxCleanupItem) error {
	if c.db == nil {
		return nil
	}
	return c.db.Save(outboxItemToModel(item)).Error
}

func (c *DataLifecycleCoordinator) persistRecalculationTasksLocked(tasks []RecalculationTask) error {
	if c.db == nil {
		return nil
	}
	for _, task := range tasks {
		if err := c.db.Save(recalculationTaskToModel(task)).Error; err != nil {
			return err
		}
	}
	return nil
}

type DeletionTombstoneModel struct {
	ID               string     `gorm:"primaryKey;column:id" json:"id"`
	TargetID         string     `gorm:"column:target_id;index" json:"targetId"`
	TargetType       string     `gorm:"column:target_type" json:"targetType"`
	Scope            string     `gorm:"column:scope" json:"scope"`
	Status           string     `gorm:"column:status;index" json:"status"`
	ItemsCount       int        `gorm:"column:items_count" json:"itemsCount"`
	CleanedCount     int        `gorm:"column:cleaned_count" json:"cleanedCount"`
	FailedCount      int        `gorm:"column:failed_count" json:"failedCount"`
	RequestedAt      time.Time  `gorm:"column:requested_at" json:"requestedAt"`
	BlockedUntil     time.Time  `gorm:"column:blocked_until" json:"blockedUntil"`
	CompletedAt      *time.Time `gorm:"column:completed_at" json:"completedAt,omitempty"`
	RetrievalBlocked bool       `gorm:"column:retrieval_blocked;default:false" json:"retrievalBlocked"`
}

func (DeletionTombstoneModel) TableName() string { return "deletion_tombstones" }

type OutboxCleanupItemModel struct {
	ID          string     `gorm:"primaryKey;column:id" json:"id"`
	Storage     string     `gorm:"column:storage;index" json:"storage"`
	TargetID    string     `gorm:"column:target_id;index" json:"targetId"`
	TargetKind  string     `gorm:"column:target_kind" json:"targetKind"`
	Status      string     `gorm:"column:status;index" json:"status"`
	Attempts    int        `gorm:"column:attempts" json:"attempts"`
	MaxAttempts int        `gorm:"column:max_attempts;default:5" json:"maxAttempts"`
	NextRetryAt time.Time  `gorm:"column:next_retry_at" json:"nextRetryAt"`
	LeaseOwner  string     `gorm:"column:lease_owner" json:"leaseOwner"`
	LeaseToken  string     `gorm:"column:lease_token" json:"leaseToken"`
	LeasedUntil time.Time  `gorm:"column:leased_until;index" json:"leasedUntil"`
	LastError   string     `gorm:"column:last_error" json:"lastError,omitempty"`
	CleanedAt   *time.Time `gorm:"column:cleaned_at" json:"cleanedAt,omitempty"`
}

func (OutboxCleanupItemModel) TableName() string { return "data_lifecycle_outbox_cleanup_items" }

type RecalculationTaskModel struct {
	ID           string     `gorm:"primaryKey;column:id" json:"id"`
	TriggerType  string     `gorm:"column:trigger_type;index" json:"triggerType"`
	TargetID     string     `gorm:"column:target_id;index" json:"targetId"`
	AffectedZone string     `gorm:"column:affected_zone" json:"affectedZone"`
	Priority     int        `gorm:"column:priority" json:"priority"`
	CreatedAt    time.Time  `gorm:"column:created_at" json:"createdAt"`
	Status       string     `gorm:"column:status;index" json:"status"`
	Description  string     `gorm:"column:description" json:"description"`
	Attempts     int        `gorm:"column:attempts" json:"attempts"`
	MaxAttempts  int        `gorm:"column:max_attempts;default:3" json:"maxAttempts"`
	NextRetryAt  time.Time  `gorm:"column:next_retry_at" json:"nextRetryAt"`
	LeaseOwner   string     `gorm:"column:lease_owner" json:"leaseOwner"`
	LeaseToken   string     `gorm:"column:lease_token" json:"leaseToken"`
	LeasedUntil  time.Time  `gorm:"column:leased_until;index" json:"leasedUntil"`
	LastError    string     `gorm:"column:last_error" json:"lastError,omitempty"`
	CompletedAt  *time.Time `gorm:"column:completed_at" json:"completedAt,omitempty"`
}

func (RecalculationTaskModel) TableName() string { return "data_lifecycle_recalculation_tasks" }
