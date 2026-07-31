package commit

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
)

type PublishJournal struct {
	ID            string `gorm:"column:id;primaryKey;type:text" json:"id"`
	ArtifactID    string `gorm:"column:artifact_id;type:text" json:"artifactId"`
	AttemptID     string `gorm:"column:attempt_id;type:text" json:"attemptId"`
	TaskID        string `gorm:"column:task_id;type:text" json:"taskId"`
	TaskActionID  string `gorm:"column:task_action_id;type:text" json:"taskActionId"`
	StagingPath   string `gorm:"column:staging_path;type:text" json:"stagingPath"`
	FinalPath     string `gorm:"column:final_path;type:text" json:"finalPath"`
	ContentHash   string `gorm:"column:content_hash;type:text" json:"contentHash"`
	StorageKey    string `gorm:"column:storage_key;type:text" json:"storageKey"`
	JournalStatus string `gorm:"column:journal_status;type:text" json:"journalStatus"`
	ErrorMessage  string `gorm:"column:error_message;type:text" json:"errorMessage"`
	CreatedAt     string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt     string `gorm:"column:updated_at;type:text" json:"updatedAt"`
	CompletedAt   string `gorm:"column:completed_at;type:text" json:"completedAt"`
}

func (PublishJournal) TableName() string {
	return "desktop_pet_generation_artifact_publish_journal"
}

const (
	JournalStatusStaging   = "staging"
	JournalStatusRenamed   = "renamed"
	JournalStatusPersisted = "persisted"
	JournalStatusFailed    = "failed"
)

var ErrJournalNotFound = errors.New("publish journal not found")

type JournalRepository interface {
	Create(tx *gorm.DB, journal *PublishJournal) error
	GetByArtifactID(artifactID string) (*PublishJournal, error)
	UpdateStatus(tx *gorm.DB, id string, status string, errMsg string) error
	ListByStatus(status string) ([]PublishJournal, error)
}

type journalRepository struct {
	db *gorm.DB
}

func NewJournalRepository(db *gorm.DB) JournalRepository {
	return &journalRepository{db: db}
}

func (r *journalRepository) Create(tx *gorm.DB, journal *PublishJournal) error {
	if tx == nil {
		tx = r.db
	}
	if journal.ID == "" {
		journal.ID = generateUUID()
	}
	if journal.CreatedAt == "" {
		journal.CreatedAt = nowRFC3339()
	}
	if journal.UpdatedAt == "" {
		journal.UpdatedAt = nowRFC3339()
	}
	if journal.JournalStatus == "" {
		journal.JournalStatus = JournalStatusStaging
	}
	if err := tx.Create(journal).Error; err != nil {
		return fmt.Errorf("create publish journal: %w", err)
	}
	return nil
}

func (r *journalRepository) GetByArtifactID(artifactID string) (*PublishJournal, error) {
	var journal PublishJournal
	err := r.db.Where("artifact_id = ?", artifactID).First(&journal).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrJournalNotFound
		}
		return nil, err
	}
	return &journal, nil
}

func (r *journalRepository) UpdateStatus(tx *gorm.DB, id string, status string, errMsg string) error {
	if tx == nil {
		tx = r.db
	}
	updates := map[string]interface{}{
		"journal_status": status,
		"error_message":  errMsg,
		"updated_at":     nowRFC3339(),
	}
	if status == JournalStatusPersisted {
		updates["completed_at"] = nowRFC3339()
	}
	result := tx.Model(&PublishJournal{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrJournalNotFound
	}
	return nil
}

func (r *journalRepository) ListByStatus(status string) ([]PublishJournal, error) {
	var journals []PublishJournal
	err := r.db.Where("journal_status = ?", status).Order("created_at ASC").Find(&journals).Error
	if err != nil {
		return nil, err
	}
	return journals, nil
}
