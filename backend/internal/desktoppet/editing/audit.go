package editing

import (
	"context"

	"github.com/u-ai/backend/log"
)

type AuditOutbox struct {
	repo Repository
}

func NewAuditOutbox(repo Repository) *AuditOutbox {
	return &AuditOutbox{repo: repo}
}

func (a *AuditOutbox) Log(ctx context.Context, eventType, spaceID, actionKey, editSessionID, jobID, baseRevisionID, candidateRevisionID, previousActiveRevisionID, newActiveRevisionID, reason string) error {
	entry := &EditAuditLog{
		ID:                       generateID("audit"),
		EventType:                eventType,
		SpaceID:                  spaceID,
		ActionKey:                actionKey,
		EditSessionID:            editSessionID,
		JobID:                    jobID,
		BaseRevisionID:           baseRevisionID,
		CandidateRevisionID:      candidateRevisionID,
		PreviousActiveRevisionID: previousActiveRevisionID,
		NewActiveRevisionID:      newActiveRevisionID,
		Reason:                   reason,
		OccurredAt:               nowUTC(),
	}
	if err := a.repo.CreateAuditLog(entry); err != nil {
		log.Logger.Errorf("editing audit log write failed: type=%s user=%s action=%s err=%v",
			eventType, spaceID, actionKey, err)
	}
	return nil
}
