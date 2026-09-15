package securityaudit

import (
	"crypto/rand"
	"encoding/base64"
	"gorm.io/gorm"
	"time"
)

type AuditEvent struct {
	EventID       string `gorm:"column:event_id;primaryKey;size:64" json:"eventId"`
	EventType     string `gorm:"column:event_type;size:100;not null;index:idx_audit_type" json:"eventType"`
	Severity      string `gorm:"column:severity;size:20;not null;default:info" json:"severity"`
	Outcome       string `gorm:"column:outcome;size:20;not null" json:"outcome"`
	SpaceID       string `gorm:"column:space_id;size:80;index:idx_audit_space" json:"spaceId"`
	DeviceID      string `gorm:"column:device_id;size:100;index:idx_audit_device" json:"deviceId"`
	RuntimeID     string `gorm:"column:runtime_id;size:100" json:"runtimeId"`
	SessionID     string `gorm:"column:session_id;size:100;index:idx_audit_session" json:"sessionId"`
	PrincipalType string `gorm:"column:principal_type;size:40" json:"principalType"`
	AuthMethod    string `gorm:"column:auth_method;size:40" json:"authMethod"`
	IPAddress     string `gorm:"column:ip_address;size:50" json:"ipAddress"`
	UserAgent     string `gorm:"column:user_agent;size:500" json:"userAgent"`
	ReasonCode    string `gorm:"column:reason_code;size:100" json:"reasonCode"`
	DetailsJSON   string `gorm:"column:details_json" json:"detailsJson"`
	OccurredAt    string `gorm:"column:occurred_at;not null;index:idx_audit_time" json:"occurredAt"`
}

func (AuditEvent) TableName() string { return "security_audit_events" }

type Repository struct{ db *gorm.DB }

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }
func (r *Repository) Insert(event *AuditEvent) error {
	if event.EventID == "" {
		event.EventID = newEventID()
	}
	if event.OccurredAt == "" {
		event.OccurredAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	return r.db.Create(event).Error
}
func (r *Repository) ListSpaceEvents(spaceID string, limit int, cursor string) ([]AuditEvent, error) {
	var events []AuditEvent
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := r.db.Where("space_id = ?", spaceID).Order("occurred_at DESC").Limit(limit)
	if cursor != "" {
		q = q.Where("occurred_at < ?", cursor)
	}
	err := q.Find(&events).Error
	return events, err
}
func (r *Repository) ListSpaceEventsByType(spaceID, eventType string, limit int) ([]AuditEvent, error) {
	var events []AuditEvent
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	err := r.db.Where("space_id = ? AND event_type = ?", spaceID, eventType).Order("occurred_at DESC").Limit(limit).Find(&events).Error
	return events, err
}
func (r *Repository) EnsureTable() error { return r.db.AutoMigrate(&AuditEvent{}) }
func newEventID() string {
	b := make([]byte, 18)
	_, _ = rand.Read(b)
	return "ae_" + base64.RawURLEncoding.EncodeToString(b)
}
