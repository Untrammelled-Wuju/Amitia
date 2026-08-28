package revisioncommit

import (
	"errors"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/editing/baseline"
	"gorm.io/gorm"
)

type BridgeInboxRepository interface {
	Create(entry *editing.ActionRevisionBridgeInbox) error
	GetByEventID(eventID string) (*editing.ActionRevisionBridgeInbox, error)
	GetByProcessingRevision(processingRevisionID string) (*editing.ActionRevisionBridgeInbox, error)
	AcquireLease(id, owner string, leaseDuration time.Duration) (bool, error)
	UpdateStatus(id, status, lastError string) error
	IncrementAttemptCount(id string) error
	ListPending(maxCount int) ([]editing.ActionRevisionBridgeInbox, error)
	ListFailed(maxCount int) ([]editing.ActionRevisionBridgeInbox, error)
	MarkCompleted(id string) error
	MarkFailedTerminal(id, reason string) error
}

type bridgeInboxRepository struct {
	db *gorm.DB
}

func NewBridgeInboxRepository(db *gorm.DB) BridgeInboxRepository {
	return &bridgeInboxRepository{db: db}
}

func (r *bridgeInboxRepository) Create(entry *editing.ActionRevisionBridgeInbox) error {
	return r.db.Create(entry).Error
}

func (r *bridgeInboxRepository) GetByEventID(eventID string) (*editing.ActionRevisionBridgeInbox, error) {
	var entry editing.ActionRevisionBridgeInbox
	err := r.db.Where("event_id = ?", eventID).First(&entry).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entry, nil
}

func (r *bridgeInboxRepository) GetByProcessingRevision(processingRevisionID string) (*editing.ActionRevisionBridgeInbox, error) {
	var entry editing.ActionRevisionBridgeInbox
	err := r.db.Where("processing_revision_id = ?", processingRevisionID).First(&entry).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &entry, nil
}

func (r *bridgeInboxRepository) AcquireLease(id, owner string, leaseDuration time.Duration) (bool, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(leaseDuration).Format(time.RFC3339)
	result := r.db.Model(&editing.ActionRevisionBridgeInbox{}).
		Where("id = ? AND (status IN ? OR (status = ? AND lease_expires_at < ?))",
			id, []string{baseline.InboxStatusReceived, baseline.InboxStatusFailedRetryable}, baseline.InboxStatusProcessing, now.Format(time.RFC3339)).
		Updates(map[string]any{
			"status":           baseline.InboxStatusProcessing,
			"lease_owner":      owner,
			"lease_expires_at": expiresAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *bridgeInboxRepository) UpdateStatus(id, status, lastError string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]any{
		"status":     status,
		"updated_at": now,
	}
	if lastError != "" {
		updates["last_error"] = lastError
	}
	return r.db.Model(&editing.ActionRevisionBridgeInbox{}).Where("id = ?", id).Updates(updates).Error
}

func (r *bridgeInboxRepository) IncrementAttemptCount(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&editing.ActionRevisionBridgeInbox{}).Where("id = ?", id).Updates(map[string]any{
		"attempt_count": gorm.Expr("attempt_count + 1"),
		"updated_at":    now,
	}).Error
}

func (r *bridgeInboxRepository) ListPending(maxCount int) ([]editing.ActionRevisionBridgeInbox, error) {
	var entries []editing.ActionRevisionBridgeInbox
	now := time.Now().UTC().Format(time.RFC3339)
	err := r.db.Where(
		"status IN ? OR (status = ? AND lease_expires_at < ?)",
		[]string{baseline.InboxStatusReceived, baseline.InboxStatusFailedRetryable}, baseline.InboxStatusProcessing, now,
	).Limit(maxCount).Find(&entries).Error
	return entries, err
}

func (r *bridgeInboxRepository) ListFailed(maxCount int) ([]editing.ActionRevisionBridgeInbox, error) {
	var entries []editing.ActionRevisionBridgeInbox
	err := r.db.Where("status IN ?", []string{baseline.InboxStatusFailedRetryable, baseline.InboxStatusFailedTerminal}).
		Limit(maxCount).Find(&entries).Error
	return entries, err
}

func (r *bridgeInboxRepository) MarkCompleted(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&editing.ActionRevisionBridgeInbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":       baseline.InboxStatusCompleted,
		"processed_at": now,
	}).Error
}

func (r *bridgeInboxRepository) MarkFailedTerminal(id, reason string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&editing.ActionRevisionBridgeInbox{}).Where("id = ?", id).Updates(map[string]any{
		"status":       baseline.InboxStatusFailedTerminal,
		"last_error":   reason,
		"processed_at": now,
	}).Error
}

type OutboxRepository interface {
	ListPending(maxCount int) ([]editing.ActionRevisionEventOutboxRecord, error)
	AcquireLease(id, owner string, leaseDuration time.Duration) (bool, error)
	MarkPublished(id string) error
	MarkFailed(id, reason string) error
	IncrementAttemptCount(id string) error
}

type outboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) ListPending(maxCount int) ([]editing.ActionRevisionEventOutboxRecord, error) {
	var records []editing.ActionRevisionEventOutboxRecord
	now := time.Now().UTC().Format(time.RFC3339)
	err := r.db.Where("status IN ? AND available_at <= ? AND attempt_count < ?", []string{baseline.OutboxStatusPending, baseline.OutboxStatusFailed, baseline.OutboxStatusPublishing}, now, MaxOutboxRetryAttempts).
		Order("created_at ASC").Limit(maxCount).Find(&records).Error
	return records, err
}

func (r *outboxRepository) AcquireLease(id, owner string, leaseDuration time.Duration) (bool, error) {
	now := time.Now().UTC()
	expiresAt := now.Add(leaseDuration).Format(time.RFC3339)
	result := r.db.Model(&editing.ActionRevisionEventOutboxRecord{}).
		Where("id = ? AND status IN ? AND available_at <= ? AND attempt_count < ?", id, []string{baseline.OutboxStatusPending, baseline.OutboxStatusFailed, baseline.OutboxStatusPublishing}, now.Format(time.RFC3339), MaxOutboxRetryAttempts).
		Updates(map[string]any{
			"status":       baseline.OutboxStatusPublishing,
			"available_at": expiresAt,
		})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, nil
	}
	_ = owner // schema has no lease-owner column; available_at carries the lease expiry.
	return true, nil
}

func (r *outboxRepository) MarkPublished(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&editing.ActionRevisionEventOutboxRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status":       baseline.OutboxStatusPublished,
		"published_at": now,
	}).Error
}

func (r *outboxRepository) MarkFailed(id, reason string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&editing.ActionRevisionEventOutboxRecord{}).Where("id = ?", id).Updates(map[string]any{
		"status":       baseline.OutboxStatusFailed,
		"last_error":   reason,
		"available_at": now,
	}).Error
}

func (r *outboxRepository) IncrementAttemptCount(id string) error {
	return r.db.Model(&editing.ActionRevisionEventOutboxRecord{}).Where("id = ?", id).Updates(map[string]any{
		"attempt_count": gorm.Expr("attempt_count + 1"),
	}).Error
}

var _ = fmt.Sprintf
