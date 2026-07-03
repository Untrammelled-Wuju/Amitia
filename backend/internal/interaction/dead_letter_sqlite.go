package interaction

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type DeadLetterRecordModel struct {
	ID          string    `gorm:"primaryKey;column:id"`
	OutboxID    string    `gorm:"column:outbox_id;index"`
	EventType   string    `gorm:"column:event_type;index"`
	AggregateID string    `gorm:"column:aggregate_id;index"`
	Payload     string    `gorm:"column:payload;type:text"`
	LastError   string    `gorm:"column:last_error"`
	RetryCount  int       `gorm:"column:retry_count"`
	Status      string    `gorm:"column:status;index"`
	CreatedAt   time.Time `gorm:"column:created_at;index"`
	ReplayedAt  time.Time `gorm:"column:replayed_at"`
}

func (DeadLetterRecordModel) TableName() string {
	return "interaction_dead_letter_records"
}

type SQLiteDeadLetterStore struct {
	db *gorm.DB
}

func NewSQLiteDeadLetterStore(db *gorm.DB) *SQLiteDeadLetterStore {
	return &SQLiteDeadLetterStore{db: db}
}

func (s *SQLiteDeadLetterStore) InitSchema() error {
	return s.db.AutoMigrate(&DeadLetterRecordModel{})
}

func (s *SQLiteDeadLetterStore) Append(record *DeadLetterRecord) (string, error) {
	if record.ID == "" {
		record.ID = uuid.New().String()
	}
	if record.CreatedAt.IsZero() {
		record.CreatedAt = time.Now()
	}
	if record.Status == "" {
		record.Status = DeadLetterStatusPending
	}
	model := DeadLetterRecordModel{
		ID:          record.ID,
		OutboxID:    record.OutboxID,
		EventType:   record.EventType,
		AggregateID: record.AggregateID,
		Payload:     string(record.Payload),
		LastError:   record.LastError,
		RetryCount:  record.RetryCount,
		Status:      string(record.Status),
		CreatedAt:   record.CreatedAt,
		ReplayedAt:  record.ReplayedAt,
	}
	return record.ID, s.db.Create(&model).Error
}

func (s *SQLiteDeadLetterStore) Get(id string) (*DeadLetterRecord, error) {
	var model DeadLetterRecordModel
	err := s.db.Where("id = ?", id).First(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("dead_letter: record not found")
		}
		return nil, err
	}
	rec := modelToDeadLetterRecord(model)
	return &rec, nil
}

func (s *SQLiteDeadLetterStore) ListPending() ([]DeadLetterRecord, error) {
	var models []DeadLetterRecordModel
	err := s.db.Where("status = ?", string(DeadLetterStatusPending)).Order("created_at ASC").Find(&models).Error
	if err != nil {
		return nil, err
	}
	result := make([]DeadLetterRecord, 0, len(models))
	for _, model := range models {
		result = append(result, modelToDeadLetterRecord(model))
	}
	return result, nil
}

func (s *SQLiteDeadLetterStore) MarkReplaying(id string) error {
	result := s.db.Model(&DeadLetterRecordModel{}).
		Where("id = ? AND status = ?", id, string(DeadLetterStatusPending)).
		Update("status", string(DeadLetterStatusReplaying))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return nil
	}
	var count int64
	if err := s.db.Model(&DeadLetterRecordModel{}).Where("id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return errors.New("dead_letter: record not found")
	}
	return errors.New("dead_letter: record not in pending state")
}

func (s *SQLiteDeadLetterStore) MarkReplayed(id string) error {
	result := s.db.Model(&DeadLetterRecordModel{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":      string(DeadLetterStatusReplayed),
		"replayed_at": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("dead_letter: record not found")
	}
	return nil
}

func (s *SQLiteDeadLetterStore) MarkArchived(id string) error {
	result := s.db.Model(&DeadLetterRecordModel{}).Where("id = ?", id).Update("status", string(DeadLetterStatusArchived))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return errors.New("dead_letter: record not found")
	}
	return nil
}

func modelToDeadLetterRecord(m DeadLetterRecordModel) DeadLetterRecord {
	return DeadLetterRecord{
		ID:          m.ID,
		OutboxID:    m.OutboxID,
		EventType:   m.EventType,
		AggregateID: m.AggregateID,
		Payload:     []byte(m.Payload),
		LastError:   m.LastError,
		RetryCount:  m.RetryCount,
		Status:      DeadLetterStatus(m.Status),
		CreatedAt:   m.CreatedAt,
		ReplayedAt:  m.ReplayedAt,
	}
}

var _ DeadLetterStore = (*SQLiteDeadLetterStore)(nil)
