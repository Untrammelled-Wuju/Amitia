package securityaudit

import (
	"time"

	"github.com/u-ai/backend/internal/accountsession"
	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Insert(event *accountsession.AuditEvent) error {
	if event.EventID == "" {
		event.EventID = accountsession.GeneratePublicID("ae_")
	}
	if event.OccurredAt == "" {
		event.OccurredAt = time.Now().UTC().Format(time.RFC3339)
	}
	return r.db.Create(event).Error
}

func (r *Repository) ListUserEvents(userID int64, limit int, cursor string) ([]accountsession.AuditEvent, error) {
	var events []accountsession.AuditEvent
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	query := r.db.Where("user_id = ?", userID).Order("occurred_at DESC").Limit(limit)
	if cursor != "" {
		query = query.Where("occurred_at < ?", cursor)
	}
	err := query.Find(&events).Error
	return events, err
}

func (r *Repository) ListUserEventsByType(userID int64, eventType string, limit int) ([]accountsession.AuditEvent, error) {
	var events []accountsession.AuditEvent
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	err := r.db.Where("user_id = ? AND event_type = ?", userID, eventType).
		Order("occurred_at DESC").Limit(limit).Find(&events).Error
	return events, err
}

func (r *Repository) EnsureTable() error {
	return r.db.AutoMigrate(&accountsession.AuditEvent{})
}
