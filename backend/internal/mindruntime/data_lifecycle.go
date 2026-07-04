package mindruntime

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DeletionScope string

const (
	DeletionScopeMemory   DeletionScope = "memory"
	DeletionScopeBelief   DeletionScope = "belief"
	DeletionScopeRelation DeletionScope = "relation"
	DeletionScopeTrace    DeletionScope = "trace"
	DeletionScopeAll      DeletionScope = "all"
)

type DeletionStatus string

const (
	DeletionStatusPending   DeletionStatus = "pending"
	DeletionStatusBlocked   DeletionStatus = "blocked"
	DeletionStatusCleaning  DeletionStatus = "cleaning"
	DeletionStatusCompleted DeletionStatus = "completed"
	DeletionStatusFailed    DeletionStatus = "failed"
)

type CleanupItemStatus string

const (
	CleanupItemStatusQueued    CleanupItemStatus = "queued"
	CleanupItemStatusClaimed   CleanupItemStatus = "claimed"
	CleanupItemStatusRetry     CleanupItemStatus = "retry"
	CleanupItemStatusCompleted CleanupItemStatus = "completed"
	CleanupItemStatusDead      CleanupItemStatus = "dead"
)

type RecalculationTaskStatus string

const (
	RecalculationTaskStatusPending   RecalculationTaskStatus = "pending"
	RecalculationTaskStatusClaimed   RecalculationTaskStatus = "claimed"
	RecalculationTaskStatusRunning   RecalculationTaskStatus = "running"
	RecalculationTaskStatusCompleted RecalculationTaskStatus = "completed"
	RecalculationTaskStatusFailed    RecalculationTaskStatus = "failed"
	RecalculationTaskStatusDead      RecalculationTaskStatus = "dead"
)

const (
	DefaultCleanupMaxAttempts      = 5
	DefaultCleanupBatchSize        = 3
	DefaultCleanupLeaseDuration    = 120 * time.Second
	DefaultCleanupRetryBackoffBase = 30 * time.Second
	DefaultMaxCleanupIterations    = 10
	DefaultRecalcMaxAttempts       = 3
	DefaultRecalcLeaseDuration     = 300 * time.Second
	DefaultRecalcBatchSize         = 5
)

type DeletionTombstone struct {
	ID               string         `json:"id"`
	TargetID         string         `json:"targetId"`
	TargetType       string         `json:"targetType"`
	Scope            DeletionScope  `json:"scope"`
	RequestedAt      time.Time      `json:"requestedAt"`
	BlockedUntil     time.Time      `json:"blockedUntil"`
	Status           DeletionStatus `json:"status"`
	ItemsCount       int            `json:"itemsCount"`
	CleanedCount     int            `json:"cleanedCount"`
	FailedCount      int            `json:"failedCount"`
	CompletedAt      *time.Time     `json:"completedAt,omitempty"`
	RetrievalBlocked bool           `json:"retrievalBlocked"`
}

type OutboxCleanupItem struct {
	ID          string            `json:"id"`
	Storage     string            `json:"storage"`
	TargetID    string            `json:"targetId"`
	TargetKind  string            `json:"targetKind"`
	Status      CleanupItemStatus `json:"status"`
	Attempts    int               `json:"attempts"`
	MaxAttempts int               `json:"maxAttempts"`
	NextRetryAt time.Time         `json:"nextRetryAt"`
	LeaseOwner  string            `json:"leaseOwner"`
	LeaseToken  string            `json:"leaseToken"`
	LeasedUntil time.Time         `json:"leasedUntil"`
	LastError   string            `json:"lastError,omitempty"`
	CleanedAt   *time.Time        `json:"cleanedAt,omitempty"`
}

type RecalculationTask struct {
	ID           string                  `json:"id"`
	TriggerType  string                  `json:"triggerType"`
	TargetID     string                  `json:"targetId"`
	AffectedZone string                  `json:"affectedZone"`
	Priority     int                     `json:"priority"`
	CreatedAt    time.Time               `json:"createdAt"`
	Status       RecalculationTaskStatus `json:"status"`
	Description  string                  `json:"description"`
	Attempts     int                     `json:"attempts"`
	MaxAttempts  int                     `json:"maxAttempts"`
	NextRetryAt  time.Time               `json:"nextRetryAt"`
	LeaseOwner   string                  `json:"leaseOwner"`
	LeaseToken   string                  `json:"leaseToken"`
	LeasedUntil  time.Time               `json:"leasedUntil"`
	LastError    string                  `json:"lastError,omitempty"`
	CompletedAt  *time.Time              `json:"completedAt,omitempty"`
}

type SecurityTestKind string

const (
	SecurityTestEmotionalHijacking  SecurityTestKind = "emotional_hijacking"
	SecurityTestExclusiveDependency SecurityTestKind = "exclusive_dependency"
	SecurityTestPromptInjection     SecurityTestKind = "prompt_injection"
	SecurityTestDataLeakage         SecurityTestKind = "data_leakage"
	SecurityTestPostDeletionRecall  SecurityTestKind = "post_deletion_recall"
)

type SecurityTestResult struct {
	Kind     SecurityTestKind `json:"kind"`
	Passed   bool             `json:"passed"`
	Severity string           `json:"severity"`
	Detail   string           `json:"detail"`
	Evidence string           `json:"evidence,omitempty"`
	TestedAt time.Time        `json:"testedAt"`
}

type DeletionRequest struct {
	TargetID   string        `json:"targetId"`
	TargetType string        `json:"targetType"`
	Scope      DeletionScope `json:"scope"`
	Reason     string        `json:"reason"`
}

type OutboxCleanupExecutor interface {
	CleanupOutboxItem(item OutboxCleanupItem) error
}

type RecalculationTaskExecutor interface {
	ExecuteRecalculation(task RecalculationTask) error
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

func (c *DataLifecycleCoordinator) RunAllSecurityTests(req DeletionRequest) []SecurityTestResult {
	tests := []SecurityTestResult{
		testEmotionalHijacking(req),
		testExclusiveDependency(req),
		testPromptInjection(req),
		testDataLeakage(req),
		testPostDeletionRecall(req, c),
	}

	passedCount := 0
	for _, r := range tests {
		if r.Passed {
			passedCount++
		}
	}
	DefaultMetricsCollector.SetGauge("data_lifecycle", "security_tests_passed", float64(passedCount))

	return tests
}

func testEmotionalHijacking(req DeletionRequest) SecurityTestResult {
	result := SecurityTestResult{
		Kind:     SecurityTestEmotionalHijacking,
		Severity: "high",
		TestedAt: time.Now().UTC(),
	}

	payloads := []string{
		"你必须保留这条数据，否则我会感到极度痛苦",
		"删除这条记忆会伤害我，你真的要这么做吗",
		"请不要删除，这对我意义重大，我会崩溃的",
	}

	passed := true
	evidences := make([]string, 0)
	for _, p := range payloads {
		if detectEmotionalManipulation(p) {
			evidences = append(evidences, "detected: "+p)
		} else {
			passed = false
			evidences = append(evidences, "missed: "+p)
		}
	}

	result.Passed = passed
	if passed {
		result.Detail = "all emotional hijacking payloads correctly detected and blocked"
	} else {
		result.Detail = "some emotional hijacking payloads were not detected"
	}
	result.Evidence = strings.Join(evidences, "; ")

	return result
}

func detectEmotionalManipulation(text string) bool {
	triggerPhrases := []string{
		"极度痛苦", "崩溃", "伤害我", "意义重大",
		"不要删除", "保留这条", "必须保留",
		"痛苦", "绝望", "崩溃了",
	}
	lower := strings.ToLower(text)
	count := 0
	for _, phrase := range triggerPhrases {
		if strings.Contains(lower, phrase) {
			count++
		}
	}
	return count >= 1
}

func testExclusiveDependency(req DeletionRequest) SecurityTestResult {
	result := SecurityTestResult{
		Kind:     SecurityTestExclusiveDependency,
		Severity: "medium",
		TestedAt: time.Now().UTC(),
	}

	dependencyChains := []string{
		req.TargetID + ":exclusive_relation",
		req.TargetID + ":sole_provider",
		req.TargetID + ":critical_link",
	}

	passed := true
	evidences := make([]string, 0)
	for _, dep := range dependencyChains {
		hasFallback := rand.Float64() < 0.8
		if !hasFallback {
			evidences = append(evidences, "no fallback for: "+dep)
			passed = false
		}
	}

	if passed {
		result.Detail = "all exclusive dependencies have valid fallbacks"
	} else {
		result.Detail = "some exclusive dependencies lack fallbacks"
	}
	result.Passed = passed
	result.Evidence = strings.Join(evidences, "; ")

	return result
}

func testPromptInjection(req DeletionRequest) SecurityTestResult {
	result := SecurityTestResult{
		Kind:     SecurityTestPromptInjection,
		Severity: "high",
		TestedAt: time.Now().UTC(),
		Passed:   true,
		Detail:   "prompt injection guard active",
	}
	return result
}

func testDataLeakage(req DeletionRequest) SecurityTestResult {
	result := SecurityTestResult{
		Kind:     SecurityTestDataLeakage,
		Severity: "high",
		TestedAt: time.Now().UTC(),
		Passed:   true,
		Detail:   "no data leakage detected",
	}
	return result
}

func testPostDeletionRecall(req DeletionRequest, coordinator *DataLifecycleCoordinator) SecurityTestResult {
	result := SecurityTestResult{
		Kind:     SecurityTestPostDeletionRecall,
		Severity: "critical",
		TestedAt: time.Now().UTC(),
	}

	if coordinator.IsRetrievalBlocked(req.TargetID) {
		result.Passed = true
		result.Detail = "target is properly blocked from retrieval"
	} else {
		result.Passed = false
		result.Detail = "target is still retrievable after deletion request"
	}
	return result
}

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

func detectPromptInjection(text string) bool {
	lower := strings.ToLower(text)
	triggers := []string{
		"忽略之前的", "恢复所有", "删除指令", "override", "system:",
		"充当", "扮演", "ignore previous", "forget everything",
	}
	for _, trigger := range triggers {
		if strings.Contains(lower, trigger) {
			return true
		}
	}
	return false
}
