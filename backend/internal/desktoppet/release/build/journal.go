package build

import (
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/desktoppet/release"
)

type PublishJournalManager struct {
	repo release.ReleaseRepository
}

func NewPublishJournalManager(repo release.ReleaseRepository) *PublishJournalManager {
	return &PublishJournalManager{repo: repo}
}

func (m *PublishJournalManager) CreateJournal(operationID, releaseID, petID string) (*release.ReleasePublishJournal, error) {
	journal := &release.ReleasePublishJournal{
		ID:            uuid.NewString(),
		OperationID:   operationID,
		ReleaseID:     releaseID,
		PetID:         petID,
		OperationKind: string(release.JournalOperationBuild),
		Stage:         release.JournalStageSnapshotCreated,
		CreatedAt:     formatTimestamp(time.Now()),
		UpdatedAt:     formatTimestamp(time.Now()),
	}
	if err := m.repo.CreatePublishJournal(journal); err != nil {
		return nil, err
	}
	return journal, nil
}

func (m *PublishJournalManager) CreateImportJournal(operationID, releaseID, petID string) (*release.ReleasePublishJournal, error) {
	journal := &release.ReleasePublishJournal{
		ID:            uuid.NewString(),
		OperationID:   operationID,
		ReleaseID:     releaseID,
		PetID:         petID,
		OperationKind: string(release.JournalOperationImport),
		Stage:         release.ImportJournalStageCreated,
		CreatedAt:     formatTimestamp(time.Now()),
		UpdatedAt:     formatTimestamp(time.Now()),
	}
	if err := m.repo.CreatePublishJournal(journal); err != nil {
		return nil, err
	}
	return journal, nil
}

func (m *PublishJournalManager) UpdateStage(journal *release.ReleasePublishJournal, stage, contentRootHash, stagingPath, publishedPath string) error {
	journal.Stage = stage
	if contentRootHash != "" {
		journal.ContentRootHash = contentRootHash
	}
	if stagingPath != "" {
		journal.StagingPath = stagingPath
	}
	if publishedPath != "" {
		journal.PublishedPath = publishedPath
	}
	journal.UpdatedAt = formatTimestamp(time.Now())
	return m.repo.UpdatePublishJournal(journal)
}

func (m *PublishJournalManager) MarkFailed(journal *release.ReleasePublishJournal, errMsg string) error {
	journal.Stage = release.JournalStageFailed
	journal.ErrorMessage = errMsg
	journal.UpdatedAt = formatTimestamp(time.Now())
	return m.repo.UpdatePublishJournal(journal)
}

func (m *PublishJournalManager) GetByOperation(operationID string) (*release.ReleasePublishJournal, error) {
	return m.repo.GetPublishJournalByOperation(operationID)
}
