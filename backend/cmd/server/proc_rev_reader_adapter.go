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
