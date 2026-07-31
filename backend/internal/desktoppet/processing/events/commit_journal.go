package events

import "gorm.io/gorm"

type CommitJournal struct {
	ID                   string `gorm:"column:id;primaryKey" json:"id"`
	CommitID             string `gorm:"column:commit_id" json:"commitId"`
	ProcessingAttemptID  string `gorm:"column:processing_attempt_id" json:"processingAttemptId"`
	ProcessingRevisionID string `gorm:"column:processing_revision_id" json:"processingRevisionId"`
	SourceManifestID     string `gorm:"column:source_manifest_id" json:"sourceManifestId"`
	Status               string `gorm:"column:status" json:"status"`
	StagingPath          string `gorm:"column:staging_path" json:"stagingPath"`
	FinalPath            string `gorm:"column:final_path" json:"finalPath"`
	ContentRootHash      string `gorm:"column:content_root_hash" json:"contentRootHash"`
	PipelineResultHash   string `gorm:"column:pipeline_result_hash" json:"pipelineResultHash"`
	CreatedAt            string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt            string `gorm:"column:updated_at" json:"updatedAt"`
	LastError            string `gorm:"column:last_error;default:''" json:"lastError"`
}

func (CommitJournal) TableName() string { return "desktop_pet_processing_commit_journals" }

const (
	CommitJournalStatusCreated          = "created"
	CommitJournalStatusStagingPrepared  = "staging_prepared"
	CommitJournalStatusRevisionRecorded = "revision_recorded"
	CommitJournalStatusFilesPublished   = "files_published"
	CommitJournalStatusRecordsCommitted = "records_committed"
	CommitJournalStatusEventCommitted   = "event_committed"
	CommitJournalStatusCompleted        = "completed"
	CommitJournalStatusFailedRetryable  = "failed_retryable"
	CommitJournalStatusFailedTerminal   = "failed_terminal"
)

type CommitJournalRepository interface {
	Create(tx *gorm.DB, journal *CommitJournal) error
	GetByCommitID(commitID string) (*CommitJournal, error)
	GetByAttemptID(attemptID string) (*CommitJournal, error)
	UpdateStatus(tx *gorm.DB, commitID, status, lastError string) error
	UpdateRevisionID(tx *gorm.DB, commitID, revisionID string) error
	UpdatePaths(tx *gorm.DB, commitID, stagingPath, finalPath, contentRootHash string) error
	ListByStatus(status string, limit int) ([]CommitJournal, error)
}
