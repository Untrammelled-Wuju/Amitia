package deviceruntime

import (
	"context"
	"database/sql"
	"strconv"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type SessionDomainEventType string

const (
	SessionEventAcquired     SessionDomainEventType = "acquired"
	SessionEventReady        SessionDomainEventType = "ready"
	SessionEventSuperseded   SessionDomainEventType = "superseded"
	SessionEventDisconnected SessionDomainEventType = "disconnected"
	SessionEventClosed       SessionDomainEventType = "closed"
	SessionEventExpired      SessionDomainEventType = "expired"
)

type SessionDomainEvent struct {
	Type            SessionDomainEventType
	Session         RuntimeSession
	PreviousSession *RuntimeSession
	Reason          string
	Reconnect       bool
	OccurredAt      time.Time
}

type SessionEventPublisher interface {
	PublishTx(
		ctx context.Context,
		tx *sql.Tx,
		event SessionDomainEvent,
	) error
}

type SessionTx interface {
	SessionStore
	RawTx() *sql.Tx
}

type SessionUnitOfWork interface {
	WithinTx(
		ctx context.Context,
		fn func(SessionTx) error,
	) error
}

func RuntimeSessionIDFromEvent(event SessionDomainEvent) runtimeidentity.RuntimeSessionID {
	return event.Session.ID
}

func SessionEventPartitionKey(event SessionDomainEvent) string {
	if event.Session.UserID != "" {
		return event.Session.UserID.String()
	}
	return "system"
}

func SessionEventOrderingKey(event SessionDomainEvent) string {
	return event.Session.ID.String()
}

func SessionEventAggregateVersion(event SessionDomainEvent) *int64 {
	return &event.Session.Revision
}

func BuildSessionEventIDempotency(event SessionDomainEvent) string {
	return event.Session.ID.String() + ":" + string(event.Type) + ":" + revString(event.Session.Revision)
}

func revString(r int64) string {
	return strconv.FormatInt(r, 10)
}
