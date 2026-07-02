package interaction

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OutboxRecordModel struct {
	ID          string    `gorm:"primaryKey;column:id"`
	AggregateID string    `gorm:"column:aggregate_id;index"`
	EventType   string    `gorm:"column:event_type;index"`
	Payload     string    `gorm:"column:payload;type:text"`
	Status      string    `gorm:"column:status;index"`
	RetryCount  int       `gorm:"column:retry_count"`
	MaxRetries  int       `gorm:"column:max_retries"`
	NextRetryAt time.Time `gorm:"column:next_retry_at"`
	LeasedUntil time.Time `gorm:"column:leased_until;index"`
	LastError   string    `gorm:"column:last_error"`
	CreatedAt   time.Time `gorm:"column:created_at;index"`
}

func (OutboxRecordModel) TableName() string {
	return "interaction_outbox_records"
}

type SQLiteOutboxStore struct {
	db *gorm.DB
}

func NewSQLiteOutboxStore(db *gorm.DB) *SQLiteOutboxStore {
	return &SQLiteOutboxStore{db: db}
}

func (s *SQLiteOutboxStore) InitSchema() error {
	return s.db.AutoMigrate(&OutboxRecordModel{})
}

func (s *SQLiteOutboxStore) Append(record *OutboxRecord) (string, error) {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	if record.Status == "" {
		record.Status = OutboxStatusPending
	}
	if record.MaxRetries == 0 {
		record.MaxRetries = DefaultMaxRetries
	}
	if record.NextRetryAt.IsZero() {
		record.NextRetryAt = record.CreatedAt
	}
	payloadStr := ""
	if len(record.Payload) > 0 {
		payloadStr = string(record.Payload)
	}
	model := OutboxRecordModel{
		ID:          record.ID,
		AggregateID: record.AggregateID,
		EventType:   record.EventType,
		Payload:     payloadStr,
		Status:      string(record.Status),
		RetryCount:  record.RetryCount,
		MaxRetries:  record.MaxRetries,
		NextRetryAt: record.NextRetryAt,
		LeasedUntil: record.LeasedUntil,
		LastError:   record.LastError,
		CreatedAt:   record.CreatedAt,
	}
	return record.ID, s.db.Create(&model).Error
}

func (s *SQLiteOutboxStore) MarkPublished(id string) error {
	result := s.db.Model(&OutboxRecordModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       OutboxStatusPublished,
		"leased_until": time.Time{},
	})
	if result.RowsAffected == 0 {
		return errors.New("outbox: record not found")
	}
	return result.Error
}

func (s *SQLiteOutboxStore) MarkFailed(id string, errMsg string) error {
	result := s.db.Model(&OutboxRecordModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":        OutboxStatusFailed,
		"last_error":    errMsg,
		"retry_count":   gorm.Expr("retry_count + 1"),
		"next_retry_at": time.Now().Add(DefaultRetryBackoff),
		"leased_until":  time.Time{},
	})
	if result.RowsAffected == 0 {
		return errors.New("outbox: record not found")
	}
	return result.Error
}

func (s *SQLiteOutboxStore) MarkDead(id string) error {
	result := s.db.Model(&OutboxRecordModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":       OutboxStatusDead,
		"leased_until": time.Time{},
	})
	if result.RowsAffected == 0 {
		return errors.New("outbox: record not found")
	}
	return result.Error
}

func (s *SQLiteOutboxStore) ListPending() ([]OutboxRecord, error) {
	now := time.Now()
	var models []OutboxRecordModel
	err := s.db.Where("status IN ? OR (status = ? AND next_retry_at <= ? AND retry_count < max_retries)",
		[]string{string(OutboxStatusPending)},
		string(OutboxStatusFailed), now,
	).Find(&models).Error
	if err != nil {
		return nil, err
	}
	result := make([]OutboxRecord, 0, len(models))
	for _, m := range models {
		result = append(result, modelToOutboxRecord(m))
	}
	return result, nil
}

func (s *SQLiteOutboxStore) LeasePending(limit int, leaseUntil time.Time) ([]OutboxRecord, error) {
	if limit <= 0 {
		return nil, nil
	}
	now := time.Now()
	leased := make([]OutboxRecord, 0, limit)
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var models []OutboxRecordModel
		err := tx.Where("status = ? OR (status = ? AND next_retry_at <= ? AND retry_count < max_retries)",
			string(OutboxStatusPending),
			string(OutboxStatusFailed),
			now,
		).Order("created_at ASC").Limit(limit).Find(&models).Error
		if err != nil {
			return err
		}
		for _, model := range models {
			result := tx.Model(&OutboxRecordModel{}).
				Where("id = ? AND (status = ? OR (status = ? AND next_retry_at <= ? AND retry_count < max_retries))",
					model.ID,
					string(OutboxStatusPending),
					string(OutboxStatusFailed),
					now,
				).
				Updates(map[string]interface{}{
					"status":       OutboxStatusProcessing,
					"leased_until": leaseUntil,
				})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				continue
			}
			model.Status = string(OutboxStatusProcessing)
			model.LeasedUntil = leaseUntil
			leased = append(leased, modelToOutboxRecord(model))
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return leased, nil
}

func (s *SQLiteOutboxStore) ReleaseExpiredLeases(now time.Time) error {
	return s.db.Model(&OutboxRecordModel{}).
		Where("status = ? AND leased_until <= ?", string(OutboxStatusProcessing), now).
		Updates(map[string]interface{}{
			"status":       OutboxStatusPending,
			"leased_until": time.Time{},
		}).Error
}

func (s *SQLiteOutboxStore) Get(id string) (*OutboxRecord, error) {
	var model OutboxRecordModel
	err := s.db.Where("id = ?", id).First(&model).Error
	if err != nil {
		return nil, err
	}
	rec := modelToOutboxRecord(model)
	return &rec, nil
}

func modelToOutboxRecord(m OutboxRecordModel) OutboxRecord {
	return OutboxRecord{
		ID:          m.ID,
		AggregateID: m.AggregateID,
		EventType:   m.EventType,
		Payload:     []byte(m.Payload),
		Status:      OutboxStatus(m.Status),
		RetryCount:  m.RetryCount,
		MaxRetries:  m.MaxRetries,
		NextRetryAt: m.NextRetryAt,
		LeasedUntil: m.LeasedUntil,
		LastError:   m.LastError,
		CreatedAt:   m.CreatedAt,
	}
}

var _ OutboxStore = (*SQLiteOutboxStore)(nil)
