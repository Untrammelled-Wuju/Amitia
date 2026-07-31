package revisioncommit

import (
	"errors"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/editing"
	"github.com/u-ai/backend/internal/desktoppet/editing/baseline"
	"gorm.io/gorm"
)

type BridgeRequestSnapshot struct {
	JournalID   string `gorm:"column:journal_id;primaryKey" json:"journalId"`
	RequestJSON string `gorm:"column:request_json" json:"requestJson"`
	CreatedAt   string `gorm:"column:created_at" json:"createdAt"`
}

func (BridgeRequestSnapshot) TableName() string {
	return "desktop_pet_bridge_request_snapshots"
}

type BridgeJournalRepository interface {
	Create(journal *editing.RevisionBridgeJournal) error
	GetByProcessingRevision(processingRevisionID string) (*editing.RevisionBridgeJournal, error)
	GetByID(id string) (*editing.RevisionBridgeJournal, error)
	UpdateStatus(id, status, lastError string) error
	UpdateActionRevisionID(id, actionRevisionID string) error
	IncrementRetryCount(id string) error
	ListByStatus(status string) ([]editing.RevisionBridgeJournal, error)
	ListPending() ([]editing.RevisionBridgeJournal, error)
	SaveRequestSnapshot(snap *BridgeRequestSnapshot) error
	GetRequestSnapshot(journalID string) (*BridgeRequestSnapshot, error)
}

type bridgeJournalRepository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) BridgeJournalRepository {
	return &bridgeJournalRepository{db: db}
}

func (r *bridgeJournalRepository) Create(journal *editing.RevisionBridgeJournal) error {
	return r.db.Create(journal).Error
}

func (r *bridgeJournalRepository) GetByProcessingRevision(processingRevisionID string) (*editing.RevisionBridgeJournal, error) {
	var journal editing.RevisionBridgeJournal
	err := r.db.Where("processing_revision_id = ?", processingRevisionID).First(&journal).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &journal, nil
}

func (r *bridgeJournalRepository) GetByID(id string) (*editing.RevisionBridgeJournal, error) {
	var journal editing.RevisionBridgeJournal
	err := r.db.Where("id = ?", id).First(&journal).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, editing.ErrRevisionNotFound
		}
		return nil, err
	}
	return &journal, nil
}

func (r *bridgeJournalRepository) UpdateStatus(id, status, lastError string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	updates := map[string]any{
		"status":     status,
		"updated_at": now,
	}
	if lastError != "" {
		updates["last_error"] = lastError
	}
	return r.db.Model(&editing.RevisionBridgeJournal{}).Where("id = ?", id).Updates(updates).Error
}

func (r *bridgeJournalRepository) UpdateActionRevisionID(id, actionRevisionID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&editing.RevisionBridgeJournal{}).Where("id = ?", id).Updates(map[string]any{
		"action_revision_id": actionRevisionID,
		"updated_at":         now,
	}).Error
}

func (r *bridgeJournalRepository) IncrementRetryCount(id string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	return r.db.Model(&editing.RevisionBridgeJournal{}).Where("id = ?", id).Updates(map[string]any{
		"retry_count": gorm.Expr("retry_count + 1"),
		"updated_at":  now,
	}).Error
}

func (r *bridgeJournalRepository) ListByStatus(status string) ([]editing.RevisionBridgeJournal, error) {
	var journals []editing.RevisionBridgeJournal
	err := r.db.Where("status = ?", status).Find(&journals).Error
	return journals, err
}

func (r *bridgeJournalRepository) ListPending() ([]editing.RevisionBridgeJournal, error) {
	var journals []editing.RevisionBridgeJournal
	err := r.db.Where("status NOT IN ?", []string{baseline.BridgeStatusCompleted, baseline.BridgeStatusFailed}).Find(&journals).Error
	return journals, err
}

func (r *bridgeJournalRepository) SaveRequestSnapshot(snap *BridgeRequestSnapshot) error {
	var existing BridgeRequestSnapshot
	err := r.db.Where("journal_id = ?", snap.JournalID).First(&existing).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return r.db.Create(snap).Error
		}
		return err
	}
	return r.db.Model(&BridgeRequestSnapshot{}).Where("journal_id = ?", snap.JournalID).Updates(map[string]any{
		"request_json": snap.RequestJSON,
	}).Error
}

func (r *bridgeJournalRepository) GetRequestSnapshot(journalID string) (*BridgeRequestSnapshot, error) {
	var snap BridgeRequestSnapshot
	err := r.db.Where("journal_id = ?", journalID).First(&snap).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &snap, nil
}
