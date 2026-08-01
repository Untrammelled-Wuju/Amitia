package installation

import (
	"context"
	"errors"
	"time"

	"github.com/u-ai/backend/internal/desktoppet/installation/desired"
)

type OutboxEventRepo interface {
	ListPendingOutboxEvents(limit int) ([]*desired.DesiredStateOutboxEvent, error)
	MarkOutboxEventPublished(tx Execer, eventID string) error
	MarkOutboxEventFailed(tx Execer, eventID, errorMsg string) error
	RequeueOutboxEventsBefore(tx Execer, availableBefore string) error
}

type Execer interface {
	Exec(query string, args ...interface{}) (Result, error)
}

type Result interface {
	RowsAffected() (int64, error)
}

type DesiredStatePublisher interface {
	PublishDesiredState(ctx context.Context, userID, deviceID, runtimeID string, snapshot *desired.DeviceDesiredSnapshot) error
}

type OutboxProcessor struct {
	eventRepo OutboxEventRepo
	publisher DesiredStatePublisher
	now       func() time.Time
}

func NewOutboxProcessor(eventRepo OutboxEventRepo, publisher DesiredStatePublisher) *OutboxProcessor {
	return &OutboxProcessor{
		eventRepo: eventRepo,
		publisher: publisher,
		now:        time.Now,
	}
}

func (p *OutboxProcessor) ProcessPendingBatch(ctx context.Context, limit int) (processed int, err error) {
	events, err := p.eventRepo.ListPendingOutboxEvents(limit)
	if err != nil {
		return 0, err
	}
	for _, ev := range events {
		if err := p.processEvent(ctx, ev); err != nil {
			continue
		}
		processed++
	}
	return processed, nil
}

func (p *OutboxProcessor) processEvent(ctx context.Context, ev *desired.DesiredStateOutboxEvent) error {
	if ev == nil {
		return errors.New("outbox processor: nil event")
	}
	snapshot := &desired.DeviceDesiredSnapshot{
		DesiredRevision: ev.DesiredRevision,
		DesiredHash:     ev.DesiredHash,
		InstallationID:  ev.InstallationID,
		UserID:          ev.UserID,
		DeviceID:        ev.DeviceID,
		RuntimeID:       ev.RuntimeID,
	}
	if err := p.publisher.PublishDesiredState(ctx, ev.UserID, ev.DeviceID, ev.RuntimeID, snapshot); err != nil {
		return err
	}
	return nil
}

func (p *OutboxProcessor) RequeueExpired() error {
	if err := p.eventRepo.RequeueOutboxEventsBefore(nil, p.now().Add(-5*time.Minute).Format("2006-01-02 15:04:05")); err != nil {
		return err
	}
	return nil
}

var _ OutboxEventRepo = (*outboxEventRepoAdapter)(nil)

type outboxEventRepoAdapter struct {
	repo OutboxEventRepo
}

func (a *outboxEventRepoAdapter) ListPendingOutboxEvents(limit int) ([]*desired.DesiredStateOutboxEvent, error) {
	return a.repo.ListPendingOutboxEvents(limit)
}

func (a *outboxEventRepoAdapter) MarkOutboxEventPublished(tx Execer, eventID string) error {
	return a.repo.MarkOutboxEventPublished(tx, eventID)
}

func (a *outboxEventRepoAdapter) MarkOutboxEventFailed(tx Execer, eventID, errorMsg string) error {
	return a.repo.MarkOutboxEventFailed(tx, eventID, errorMsg)
}

func (a *outboxEventRepoAdapter) RequeueOutboxEventsBefore(tx Execer, availableBefore string) error {
	return a.repo.RequeueOutboxEventsBefore(tx, availableBefore)
}
