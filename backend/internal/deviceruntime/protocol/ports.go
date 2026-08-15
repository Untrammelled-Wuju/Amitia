package protocol

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type PresenceSnapshot struct {
	UserID               runtimeidentity.UserID
	DeviceID             runtimeidentity.DeviceID
	RuntimeID            runtimeidentity.RuntimeID
	RuntimeSessionID     runtimeidentity.RuntimeSessionID
	Platform             runtimeidentity.Platform
	ConnectionGeneration int64
	At                   time.Time
}

type SendPort interface {
	SendEnvelope(
		ctx context.Context,
		envelope *Envelope,
	) error
}

type SessionLifecyclePort interface {
	Acquire(
		ctx context.Context,
		identity SessionIdentity,
		cursor SessionCursor,
		runtimeVersion string,
		contractVersion RuntimeContractVersion,
		capabilities []string,
	) (
		sessionID runtimeidentity.RuntimeSessionID,
		decision ResumeDecision,
		err error,
	)

	Heartbeat(
		ctx context.Context,
		sessionID runtimeidentity.RuntimeSessionID,
		at time.Time,
	) error

	Close(
		ctx context.Context,
		sessionID runtimeidentity.RuntimeSessionID,
		reason string,
	) error

	SessionReady(
		ctx context.Context,
		snapshot PresenceSnapshot,
	) error

	SessionDisconnected(
		ctx context.Context,
		snapshot PresenceSnapshot,
		reason string,
	) error
}

type ReconcilePort interface {
	Reconcile(
		ctx context.Context,
		request ReconcileRequest,
	) (ReconcileDecision, error)
}
