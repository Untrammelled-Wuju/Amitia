package main

import (
	"github.com/u-ai/backend/internal/desktoppet/processing"
)

type processingRevisionReaderAdapter struct {
	repo processing.Repository
}

func (a *processingRevisionReaderAdapter) GetProcessingRevision(revisionID string) (*processing.ProcessingRevision, error) {
	return a.repo.GetRevision(revisionID)
}

func (a *processingRevisionReaderAdapter) GetProcessingArtifacts(revisionID string) ([]processing.ProcessingArtifactRecord, error) {
	return a.repo.ListArtifactsByRevision(revisionID)
}

func (a *processingRevisionReaderAdapter) GetProcessingTask(taskID string) (*processing.ProcessingTask, error) {
	return a.repo.GetProcessingTask(taskID)
}

func (a *processingRevisionReaderAdapter) GetProcessingAction(taskID, actionKey string) (*processing.ProcessingAction, error) {
	return a.repo.GetProcessingActionByActionKey(taskID, actionKey)
}
