package eventbridge

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/extension/kernel/event"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

const (
	runtimeProducerID = "runtime-session-kernel"
)

type RuntimeSessionPayload struct {
	RuntimeSessionID runtimeidentity.RuntimeSessionID `json:"runtimeSessionId"`

	UserID    runtimeidentity.UserID    `json:"userId"`
	DeviceID  runtimeidentity.DeviceID  `json:"deviceId"`
	RuntimeID runtimeidentity.RuntimeID `json:"runtimeId"`
	Platform  runtimeidentity.Platform  `json:"platform,omitempty"`

	ConnectionGeneration int64 `json:"connectionGeneration"`

	Status protocol.SessionStatus `json:"status"`

	RuntimeVersion         string `json:"runtimeVersion,omitempty"`
	RuntimeContractVersion string `json:"runtimeContractVersion,omitempty"`

	CapabilitiesHash string `json:"capabilitiesHash,omitempty"`

	LastAppliedStateRevision     int64 `json:"lastAppliedStateRevision,omitempty"`
	LastProcessedCommandSequence int64 `json:"lastProcessedCommandSequence,omitempty"`
	LastEventSequence            int64 `json:"lastEventSequence,omitempty"`

	Reconnect bool   `json:"reconnect"`
	Reason    string `json:"reason,omitempty"`

	OccurredAt time.Time `json:"occurredAt"`
}

type RuntimeSessionEventPublisher struct {
	publisher *Publisher
}

func NewRuntimeSessionEventPublisher(publisher *Publisher) *RuntimeSessionEventPublisher {
	return &RuntimeSessionEventPublisher{publisher: publisher}
}

func (p *RuntimeSessionEventPublisher) PublishTx(
	ctx context.Context,
	tx *sql.Tx,
	domainEvent deviceruntime.SessionDomainEvent,
) error {
	typeID, ok := runtimeEventTypeID(domainEvent.Type)
	if !ok {
		return fmt.Errorf("eventbridge: unknown runtime session event type: %s", domainEvent.Type)
	}
	payload := buildRuntimePayload(domainEvent)
	opts := runtimeOptions(domainEvent)
	_, err := p.publisher.PublishTx(ctx, tx, typeID, eventVersion, payload, opts)
	return err
}

func runtimeEventTypeID(t deviceruntime.SessionDomainEventType) (event.EventTypeID, bool) {
	switch t {
	case deviceruntime.SessionEventAcquired:
		return rtSessionAcquired, true
	case deviceruntime.SessionEventReady:
		return rtSessionReady, true
	case deviceruntime.SessionEventSuperseded:
		return rtSessionSuperseded, true
	case deviceruntime.SessionEventDisconnected:
		return rtSessionDisconnected, true
	case deviceruntime.SessionEventClosed:
		return rtSessionClosed, true
	case deviceruntime.SessionEventExpired:
		return rtSessionExpired, true
	}
	return "", false
}

func buildRuntimePayload(e deviceruntime.SessionDomainEvent) RuntimeSessionPayload {
	return RuntimeSessionPayload{
		RuntimeSessionID:             e.Session.ID,
		UserID:                       e.Session.UserID,
		DeviceID:                     e.Session.DeviceID,
		RuntimeID:                    e.Session.RuntimeID,
		Platform:                     e.Session.Platform,
		ConnectionGeneration:         e.Session.ConnectionGeneration,
		Status:                       e.Session.Status,
		RuntimeVersion:               e.Session.RuntimeVersion,
		RuntimeContractVersion:       e.Session.RuntimeContractVersion,
		CapabilitiesHash:             e.Session.CapabilitiesHash,
		LastAppliedStateRevision:     e.Session.LastAppliedStateRevision,
		LastProcessedCommandSequence: e.Session.LastProcessedCommandSequence,
		LastEventSequence:            e.Session.LastEventSequence,
		Reconnect:                    e.Reconnect,
		Reason:                       e.Reason,
		OccurredAt:                   e.OccurredAt,
	}
}

func runtimeOptions(e deviceruntime.SessionDomainEvent) event.PublishOptions {
	return event.PublishOptions{
		Domain:           event.EventDomainRuntime,
		ProducerType:     event.EventProducerTypeRuntime,
		ProducerID:       runtimeProducerID,
		AggregateType:    "runtime_session",
		AggregateID:      e.Session.ID.String(),
		AggregateVersion: &e.Session.Revision,
		PartitionKey:     runtimePartitionKey(e),
		OrderingKey:      e.Session.ID.String(),
		IdempotencyKey:   runtimeIdempotencyKey(e),
	}
}

func runtimePartitionKey(e deviceruntime.SessionDomainEvent) string {
	if e.Session.UserID != "" {
		return e.Session.UserID.String()
	}
	return "system"
}

func runtimeIdempotencyKey(e deviceruntime.SessionDomainEvent) string {
	return e.Session.ID.String() + ":" + string(e.Type) + ":" + strconv.FormatInt(e.Session.Revision, 10)
}
