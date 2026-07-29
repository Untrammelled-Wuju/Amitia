package event

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type DeadLetterReason string

const (
	DeadLetterMaxAttempts         DeadLetterReason = "max_attempts_exceeded"
	DeadLetterPermanentError      DeadLetterReason = "permanent_error"
	DeadLetterHandlerNotFound     DeadLetterReason = "handler_not_found"
	DeadLetterSubscriptionInvalid DeadLetterReason = "subscription_invalid"
	DeadLetterPermissionRevoked   DeadLetterReason = "permission_revoked"
	DeadLetterScopeInvalid        DeadLetterReason = "scope_invalid"
	DeadLetterExtensionDisabled   DeadLetterReason = "extension_disabled"
	DeadLetterCircuitOpen         DeadLetterReason = "circuit_open"
	DeadLetterManualDiscard       DeadLetterReason = "manual_discard"
)

type DeadLetterRecord struct {
	DeadLetterID         string
	EventID              string
	DeliveryID           string
	SubscriptionID       string
	ExtensionID          string
	ModuleID             string
	EventTypeID          EventTypeID
	EventVersion         int
	Reason               DeadLetterReason
	ErrorCode            string
	ErrorMessage         string
	Attempts             int
	PartitionKey         string
	OrderingKey          string
	PayloadHash          string
	ProjectedPayloadHash string
	DefinitionHash       string
	ScopeSnapshotID      string
	PermissionSnapshotID string
	RuntimeInstanceID    string
	TraceID              string
	OperationID          string
	OriginEvent          json.RawMessage
	SubscriptionSnapshot json.RawMessage
	CreatedAt            time.Time
	ReplayCount          int
	LastReplayAt         *time.Time
	Status               DeadLetterStatus
}

type DeadLetterStatus string

const (
	DeadLetterStatusPending   DeadLetterStatus = "pending"
	DeadLetterStatusReplayed  DeadLetterStatus = "replayed"
	DeadLetterStatusDiscarded DeadLetterStatus = "discarded"
)

func NewDeadLetterRecord(delivery Delivery, envelope EventEnvelope, sub EventSubscriptionDefinition, reason DeadLetterReason) DeadLetterRecord {
	now := time.Now().UTC()
	return DeadLetterRecord{
		DeadLetterID:         newDeadLetterID(),
		EventID:              envelope.EventID,
		DeliveryID:           delivery.DeliveryID,
		SubscriptionID:       sub.ContributionID,
		ExtensionID:          sub.ExtensionID,
		ModuleID:             sub.ModuleID,
		EventTypeID:          envelope.EventTypeID,
		EventVersion:         envelope.EventVersion,
		Reason:               reason,
		ErrorCode:            delivery.ErrorCode,
		ErrorMessage:         delivery.ErrorMessage,
		Attempts:             delivery.Attempt,
		PartitionKey:         delivery.PartitionKey,
		OrderingKey:          delivery.OrderingKey,
		PayloadHash:          envelope.PayloadHash,
		ProjectedPayloadHash: delivery.ProjectedPayloadHash,
		DefinitionHash:       sub.DefinitionHash,
		ScopeSnapshotID:      delivery.ScopeSnapshotID,
		PermissionSnapshotID: delivery.PermissionSnapshotID,
		RuntimeInstanceID:    delivery.RuntimeInstanceID,
		TraceID:              envelope.TraceID,
		OperationID:          envelope.OperationID,
		CreatedAt:            now,
		Status:               DeadLetterStatusPending,
	}
}

func newDeadLetterID() string {
	return fmt.Sprintf("dl-%s", uuid.NewString())
}

func (r *DeadLetterRecord) MarkReplayed() {
	now := time.Now().UTC()
	r.ReplayCount++
	r.LastReplayAt = &now
	r.Status = DeadLetterStatusReplayed
}

func (r *DeadLetterRecord) MarkDiscarded() {
	r.Status = DeadLetterStatusDiscarded
}

type ReplayRequest struct {
	DeadLetterID      string
	Strategy          ReplayStrategy
	NewSubscriptionID string
	RequestedBy       string
	Reason            string
}

type ReplayStrategy string

const (
	ReplaySameSubscription ReplayStrategy = "replay_same_subscription"
	ReplayAfterRepair      ReplayStrategy = "replay_after_repair"
	ReplayToNewGeneration  ReplayStrategy = "replay_to_new_generation"
	ReplayDiscard          ReplayStrategy = "discard"
)

func (r ReplayRequest) Validate() error {
	if r.DeadLetterID == "" {
		return ErrDeadLetterNotFound
	}
	if r.Strategy == "" {
		return ErrInvalidEvent
	}
	switch r.Strategy {
	case ReplaySameSubscription, ReplayAfterRepair, ReplayToNewGeneration, ReplayDiscard:
		return nil
	default:
		return ErrInvalidEvent
	}
}

func IsReplayAllowed(requestedBy string, isSystem bool) bool {
	return isSystem || requestedBy == "user" || requestedBy == "system_repair" || requestedBy == "developer_console"
}
