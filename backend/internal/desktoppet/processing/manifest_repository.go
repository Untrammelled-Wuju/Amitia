package processing

import (
	"context"
	"errors"

	"github.com/u-ai/backend/internal/desktoppet/processing/source"
	"gorm.io/gorm"
)

type manifestStore struct {
	db *gorm.DB
}

func NewManifestStore(db *gorm.DB) source.ManifestStore {
	return &manifestStore{db: db}
}

func (s *manifestStore) Create(ctx context.Context, record *source.ProcessingSourceManifestRecord) error {
	if record == nil {
		return errors.New("manifest store: record is nil")
	}
	return s.db.WithContext(ctx).Create(record).Error
}

func (s *manifestStore) CreateInTx(ctx context.Context, tx *gorm.DB, record *source.ProcessingSourceManifestRecord) error {
	if tx == nil {
		return errors.New("manifest store: transaction is nil")
	}
	if record == nil {
		return errors.New("manifest store: record is nil")
	}
	return tx.WithContext(ctx).Create(record).Error
}

func (s *manifestStore) GetByProcessingAction(ctx context.Context, processingActionID string) (*source.ProcessingSourceManifestRecord, error) {
	var record source.ProcessingSourceManifestRecord
	err := s.db.WithContext(ctx).
		Where("processing_action_id = ?", processingActionID).
		First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *manifestStore) GetByID(ctx context.Context, id string) (*source.ProcessingSourceManifestRecord, error) {
	var record source.ProcessingSourceManifestRecord
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (s *manifestStore) ListByProcessingTask(ctx context.Context, processingTaskID string) ([]source.ProcessingSourceManifestRecord, error) {
	var records []source.ProcessingSourceManifestRecord
	err := s.db.WithContext(ctx).
		Where("processing_task_id = ?", processingTaskID).
		Order("action_key ASC").
		Find(&records).Error
	if err != nil {
		return nil, err
	}
	return records, nil
}
