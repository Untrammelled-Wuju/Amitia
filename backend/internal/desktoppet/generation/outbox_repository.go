package generation

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

var (
	ErrOutboxEntryNotFound = NewGenerationError("GEN_OUTBOX_ENTRY_NOT_FOUND", "outbox entry not found", nil)
)

type OutboxRepository interface {
	CreateTx(tx *gorm.DB, entry *GenerationOutboxEntry) error
	ListPending(limit int) ([]GenerationOutboxEntry, error)
	MarkProcessing(id string) (bool, error)
	MarkCompleted(id string) error
	MarkFailed(id string, errMsg string) error
	IncrementRetry(id string, errMsg string) error
}

type outboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) CreateTx(tx *gorm.DB, entry *GenerationOutboxEntry) error {
	if tx == nil {
		tx = r.db
	}
	if entry.ID == "" {
		entry.ID = generateUUID()
	}
	if entry.Status == "" {
		entry.Status = string(OutboxStatusPending)
	}
	if entry.MaxRetries == 0 {
		entry.MaxRetries = 3
	}
	now := nowRFC3339()
	if entry.CreatedAt == "" {
		entry.CreatedAt = now
	}
	if entry.UpdatedAt == "" {
		entry.UpdatedAt = now
	}
	if err := tx.Create(entry).Error; err != nil {
		return fmt.Errorf("create outbox entry: %w", err)
	}
	return nil
}

func (r *outboxRepository) ListPending(limit int) ([]GenerationOutboxEntry, error) {
	if limit <= 0 {
		limit = 50
	}
	now := nowRFC3339()
	var entries []GenerationOutboxEntry
	err := r.db.Where("status = ? AND (next_retry_at = '' OR next_retry_at <= ?)", string(OutboxStatusPending), now).
		Order("created_at ASC").
		Limit(limit).
		Find(&entries).Error
	if err != nil {
		return nil, fmt.Errorf("list pending outbox entries: %w", err)
	}
	return entries, nil
}

func (r *outboxRepository) MarkProcessing(id string) (bool, error) {
	if id == "" {
		return false, fmt.Errorf("outbox entry id is required")
	}
	now := nowRFC3339()
	result := r.db.Model(&GenerationOutboxEntry{}).
		Where("id = ? AND status = ?", id, string(OutboxStatusPending)).
		Updates(map[string]interface{}{
			"status":     string(OutboxStatusProcessing),
			"updated_at": now,
		})
	if result.Error != nil {
		return false, fmt.Errorf("mark outbox processing: %w", result.Error)
	}
	return result.RowsAffected > 0, nil
}

func (r *outboxRepository) MarkCompleted(id string) error {
	if id == "" {
		return fmt.Errorf("outbox entry id is required")
	}
	now := nowRFC3339()
	result := r.db.Model(&GenerationOutboxEntry{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":       string(OutboxStatusCompleted),
			"processed_at": now,
			"updated_at":   now,
		})
	if result.Error != nil {
		return fmt.Errorf("mark outbox completed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrOutboxEntryNotFound
	}
	return nil
}

func (r *outboxRepository) MarkFailed(id string, errMsg string) error {
	if id == "" {
		return fmt.Errorf("outbox entry id is required")
	}
	now := nowRFC3339()
	result := r.db.Model(&GenerationOutboxEntry{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        string(OutboxStatusFailed),
			"error_message": errMsg,
			"processed_at":  now,
			"updated_at":    now,
		})
	if result.Error != nil {
		return fmt.Errorf("mark outbox failed: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrOutboxEntryNotFound
	}
	return nil
}

func (r *outboxRepository) IncrementRetry(id string, errMsg string) error {
	if id == "" {
		return fmt.Errorf("outbox entry id is required")
	}
	var entry GenerationOutboxEntry
	err := r.db.Where("id = ?", id).First(&entry).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrOutboxEntryNotFound
		}
		return fmt.Errorf("get outbox entry for retry: %w", err)
	}
	newRetryCount := entry.RetryCount + 1
	if newRetryCount >= entry.MaxRetries {
		return r.MarkFailed(id, errMsg)
	}
	backoff := computeOutboxBackoff(newRetryCount)
	nextRetryAt := time.Now().Add(backoff).UTC().Format(time.RFC3339)
	now := nowRFC3339()
	result := r.db.Model(&GenerationOutboxEntry{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"status":        string(OutboxStatusPending),
			"retry_count":   newRetryCount,
			"error_message": errMsg,
			"next_retry_at": nextRetryAt,
			"updated_at":    now,
		})
	if result.Error != nil {
		return fmt.Errorf("increment outbox retry: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrOutboxEntryNotFound
	}
	return nil
}

func computeOutboxBackoff(retryCount int) time.Duration {
	base := 5 * time.Second
	max := 5 * time.Minute
	shift := uint(retryCount)
	if shift > 10 {
		shift = 10
	}
	backoff := base * time.Duration(1<<shift)
	if backoff > max {
		backoff = max
	}
	if backoff < base {
		backoff = base
	}
	return backoff
}
