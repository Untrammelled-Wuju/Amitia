package events

import (
	"fmt"

	"gorm.io/gorm"
)

type outboxRepository struct {
	db *gorm.DB
}

func NewOutboxRepository(db *gorm.DB) OutboxRepository {
	return &outboxRepository{db: db}
}

func (r *outboxRepository) CreateOutbox(tx *gorm.DB, record *OutboxRecord) error {
	if record == nil {
		return fmt.Errorf("outbox: record is nil")
	}
	return tx.Create(record).Error
}

func (r *outboxRepository) ListPendingOutbox(limit int) ([]OutboxRecord, error) {
	var records []OutboxRecord
	err := r.db.Where("status = ?", OutboxStatusPending).
		Order("created_at ASC").
		Limit(limit).
		Find(&records).Error
	if records == nil {
		records = []OutboxRecord{}
	}
	return records, err
}

func (r *outboxRepository) MarkPublished(outboxID string) error {
	return r.db.Model(&OutboxRecord{}).
		Where("id = ?", outboxID).
		Updates(map[string]interface{}{
			"status":       OutboxStatusPublished,
			"published_at": "",
		}).Error
}

func (r *outboxRepository) MarkFailed(outboxID string, errMsg string) error {
	return r.db.Model(&OutboxRecord{}).
		Where("id = ?", outboxID).
		Updates(map[string]interface{}{
			"status": OutboxStatusFailed,
			"error":  errMsg,
		}).Error
}

type commitJournalRepository struct {
	db *gorm.DB
}

func NewCommitJournalRepository(db *gorm.DB) CommitJournalRepository {
	return &commitJournalRepository{db: db}
}

func (r *commitJournalRepository) Create(tx *gorm.DB, journal *CommitJournal) error {
	if journal == nil {
		return fmt.Errorf("commit journal: journal is nil")
	}
	return tx.Create(journal).Error
}

func (r *commitJournalRepository) GetByCommitID(commitID string) (*CommitJournal, error) {
	var journal CommitJournal
	err := r.db.Where("commit_id = ?", commitID).First(&journal).Error
	if err != nil {
		return nil, err
	}
	return &journal, nil
}

func (r *commitJournalRepository) GetByAttemptID(attemptID string) (*CommitJournal, error) {
	var journal CommitJournal
	err := r.db.Where("processing_attempt_id = ?", attemptID).First(&journal).Error
	if err != nil {
		return nil, err
	}
	return &journal, nil
}

func (r *commitJournalRepository) UpdateStatus(tx *gorm.DB, commitID, status, lastError string) error {
	return tx.Model(&CommitJournal{}).
		Where("commit_id = ?", commitID).
		Updates(map[string]interface{}{
			"status":     status,
			"last_error": lastError,
		}).Error
}

func (r *commitJournalRepository) UpdateRevisionID(tx *gorm.DB, commitID, revisionID string) error {
	return tx.Model(&CommitJournal{}).
		Where("commit_id = ?", commitID).
		Update("processing_revision_id", revisionID).Error
}

func (r *commitJournalRepository) UpdatePaths(tx *gorm.DB, commitID, stagingPath, finalPath, contentRootHash string) error {
	return tx.Model(&CommitJournal{}).
		Where("commit_id = ?", commitID).
		Updates(map[string]interface{}{
			"staging_path":      stagingPath,
			"final_path":        finalPath,
			"content_root_hash": contentRootHash,
		}).Error
}

func (r *commitJournalRepository) ListByStatus(status string, limit int) ([]CommitJournal, error) {
	var journals []CommitJournal
	err := r.db.Where("status = ?", status).
		Order("created_at ASC").
		Limit(limit).
		Find(&journals).Error
	if journals == nil {
		journals = []CommitJournal{}
	}
	return journals, err
}
