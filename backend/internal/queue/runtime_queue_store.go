package queue

import (
	"context"
	"sync"
	"time"

	"gorm.io/gorm"
)

type RuntimeQueueRecord struct {
	TaskID         string    `gorm:"primaryKey;column:task_id"`
	Scope          string    `gorm:"column:scope;index:idx_runtime_queue_scope"`
	Priority       int       `gorm:"column:priority;index:idx_runtime_queue_priority_available,priority:1"`
	Status         string    `gorm:"column:status"`
	AvailableAt    time.Time `gorm:"column:available_at;index:idx_runtime_queue_priority_available,priority:2"`
	Deadline       time.Time `gorm:"column:deadline"`
	Lease          string    `gorm:"column:lease;index:idx_runtime_queue_lease"`
	Attempt        int       `gorm:"column:attempt"`
	PayloadVersion int       `gorm:"column:payload_version"`
}

func (RuntimeQueueRecord) TableName() string {
	return "runtime_queue"
}

type RuntimeQueueStore interface {
	InitSchema() error
	Upsert(ctx context.Context, record *RuntimeQueueRecord) error
	Delete(ctx context.Context, taskID string) error
	LoadPending(ctx context.Context) ([]RuntimeQueueRecord, error)
	LoadByScope(ctx context.Context, scope string) ([]RuntimeQueueRecord, error)
	DeleteByScope(ctx context.Context, scope string) (int64, error)
	CountByScope(ctx context.Context, scope string) (int64, error)
}

type SQLiteRuntimeQueueStore struct {
	db *gorm.DB
	mu sync.Mutex
}

func NewSQLiteRuntimeQueueStore(db *gorm.DB) *SQLiteRuntimeQueueStore {
	return &SQLiteRuntimeQueueStore{db: db}
}

func (s *SQLiteRuntimeQueueStore) InitSchema() error {
	if err := s.db.AutoMigrate(&RuntimeQueueRecord{}); err != nil {
		return err
	}
	return nil
}

func (s *SQLiteRuntimeQueueStore) Upsert(ctx context.Context, record *RuntimeQueueRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.WithContext(ctx).Save(record).Error
}

func (s *SQLiteRuntimeQueueStore) Delete(ctx context.Context, taskID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.WithContext(ctx).Where("task_id = ?", taskID).Delete(&RuntimeQueueRecord{}).Error
}

func (s *SQLiteRuntimeQueueStore) LoadPending(ctx context.Context) ([]RuntimeQueueRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var records []RuntimeQueueRecord
	err := s.db.WithContext(ctx).
		Where("status = ?", "pending").
		Order("priority ASC, available_at ASC").
		Find(&records).Error
	return records, err
}

func (s *SQLiteRuntimeQueueStore) LoadByScope(ctx context.Context, scope string) ([]RuntimeQueueRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var records []RuntimeQueueRecord
	err := s.db.WithContext(ctx).
		Where("scope = ? AND status = ?", scope, "pending").
		Order("priority ASC, available_at ASC").
		Find(&records).Error
	return records, err
}

func (s *SQLiteRuntimeQueueStore) DeleteByScope(ctx context.Context, scope string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := s.db.WithContext(ctx).Where("scope = ? AND status = ?", scope, "pending").Delete(&RuntimeQueueRecord{})
	return result.RowsAffected, result.Error
}

func (s *SQLiteRuntimeQueueStore) CountByScope(ctx context.Context, scope string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var count int64
	err := s.db.WithContext(ctx).Model(&RuntimeQueueRecord{}).Where("scope = ? AND status = ?", scope, "pending").Count(&count).Error
	return count, err
}
