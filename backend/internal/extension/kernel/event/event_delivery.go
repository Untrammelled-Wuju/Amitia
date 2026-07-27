package event

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type DeliveryStatus string

const (
	DeliveryStatusPending     DeliveryStatus = "pending"
	DeliveryStatusLeased       DeliveryStatus = "leased"
	DeliveryStatusDelivering    DeliveryStatus = "delivering"
	DeliveryStatusSucceeded     DeliveryStatus = "succeeded"
	DeliveryStatusRetryWait     DeliveryStatus = "retry_wait"
	DeliveryStatusFailed        DeliveryStatus = "failed"
	DeliveryStatusDeadLetter    DeliveryStatus = "dead_letter"
	DeliveryStatusCancelled     DeliveryStatus = "cancelled"
	DeliveryStatusSkipped       DeliveryStatus = "skipped"
)

type Delivery struct {
	DeliveryID             string
	EventID                string
	SubscriptionID         string
	ExtensionID            string
	ModuleID               string
	Status                 DeliveryStatus
	PartitionKey           string
	OrderingKey            string
	Sequence               int64
	Attempt                int
	MaxAttempts            int
	AvailableAt            time.Time
	LeaseOwner             string
	LeaseExpiresAt         *time.Time
	RuntimeInstanceID      string
	ScopeSnapshotID        string
	PermissionSnapshotID   string
	ProjectedPayloadHash   string
	StartedAt              *time.Time
	FinishedAt             *time.Time
	ErrorCode              string
	ErrorMessage           string
	CreatedAt              time.Time
	UpdatedAt              time.Time
}

func NewDelivery(eventID string, sub EventSubscriptionDefinition, sequence int64) Delivery {
	now := time.Now().UTC()
	return Delivery{
		DeliveryID:           newDeliveryID(),
		EventID:              eventID,
		SubscriptionID:       sub.ContributionID,
		ExtensionID:          sub.ExtensionID,
		ModuleID:             sub.ModuleID,
		Status:               DeliveryStatusPending,
		PartitionKey:         "",
		OrderingKey:          "",
		Sequence:             sequence,
		Attempt:              0,
		MaxAttempts:          sub.RetryPolicy.MaxAttempts,
		AvailableAt:          now,
		ScopeSnapshotID:      "",
		PermissionSnapshotID: "",
		CreatedAt:            now,
		UpdatedAt:            now,
	}
}

func newDeliveryID() string {
	return fmt.Sprintf("del-%s", uuid.NewString())
}

func (d *Delivery) Lease(owner string, ttl time.Duration) {
	now := time.Now().UTC()
	d.Status = DeliveryStatusLeased
	d.LeaseOwner = owner
	expires := now.Add(ttl)
	d.LeaseExpiresAt = &expires
	d.UpdatedAt = now
}

func (d *Delivery) RenewLease(ttl time.Duration) {
	if d.LeaseOwner == "" {
		return
	}
	now := time.Now().UTC()
	expires := now.Add(ttl)
	d.LeaseExpiresAt = &expires
	d.UpdatedAt = now
}

func (d *Delivery) Release() {
	now := time.Now().UTC()
	d.LeaseOwner = ""
	d.LeaseExpiresAt = nil
	if d.Status == DeliveryStatusLeased || d.Status == DeliveryStatusDelivering {
		d.Status = DeliveryStatusPending
	}
	d.UpdatedAt = now
}

func (d *Delivery) Start(runtimeInstanceID string) {
	now := time.Now().UTC()
	d.Status = DeliveryStatusDelivering
	d.Attempt++
	d.RuntimeInstanceID = runtimeInstanceID
	d.StartedAt = &now
	d.UpdatedAt = now
}

func (d *Delivery) Succeed() {
	now := time.Now().UTC()
	d.Status = DeliveryStatusSucceeded
	d.FinishedAt = &now
	d.ErrorCode = ""
	d.ErrorMessage = ""
	d.LeaseOwner = ""
	d.LeaseExpiresAt = nil
	d.UpdatedAt = now
}

func (d *Delivery) Fail(code, message string, retryPolicy RetryPolicy) {
	now := time.Now().UTC()
	d.ErrorCode = code
	d.ErrorMessage = message
	d.LeaseOwner = ""
	d.LeaseExpiresAt = nil
	if retryPolicy.ShouldRetry(d.Attempt, code) {
		d.Status = DeliveryStatusRetryWait
		d.AvailableAt = now.Add(retryPolicy.ComputeBackoff(d.Attempt))
	} else {
		d.Status = DeliveryStatusDeadLetter
		d.FinishedAt = &now
	}
	d.UpdatedAt = now
}

func (d *Delivery) DeadLetter(code, message string) {
	now := time.Now().UTC()
	d.Status = DeliveryStatusDeadLetter
	d.ErrorCode = code
	d.ErrorMessage = message
	d.FinishedAt = &now
	d.LeaseOwner = ""
	d.LeaseExpiresAt = nil
	d.UpdatedAt = now
}

func (d *Delivery) Cancel(reason string) {
	now := time.Now().UTC()
	d.Status = DeliveryStatusCancelled
	d.ErrorCode = "cancelled"
	d.ErrorMessage = reason
	d.FinishedAt = &now
	d.LeaseOwner = ""
	d.LeaseExpiresAt = nil
	d.UpdatedAt = now
}

func (d *Delivery) Skip(reason string) {
	now := time.Now().UTC()
	d.Status = DeliveryStatusSkipped
	d.ErrorCode = "skipped"
	d.ErrorMessage = reason
	d.FinishedAt = &now
	d.UpdatedAt = now
}

func (d *Delivery) IsLeaseExpired() bool {
	if d.LeaseExpiresAt == nil {
		return false
	}
	return time.Now().UTC().After(*d.LeaseExpiresAt)
}

func (d *Delivery) IsAvailable() bool {
	now := time.Now().UTC()
	switch d.Status {
	case DeliveryStatusPending:
		return true
	case DeliveryStatusRetryWait:
		return now.After(d.AvailableAt) || now.Equal(d.AvailableAt)
	default:
		return false
	}
}

func (d *Delivery) Elapsed() time.Duration {
	if d.StartedAt == nil {
		return 0
	}
	if d.FinishedAt != nil {
		return d.FinishedAt.Sub(*d.StartedAt)
	}
	return time.Since(*d.StartedAt)
}

type DeliveryAttempt struct {
	AttemptID        string
	DeliveryID       string
	Attempt          int
	StartedAt        time.Time
	FinishedAt       *time.Time
	ErrorCode        string
	ErrorMessage     string
	RuntimeInstanceID string
	Duration         time.Duration
}
