package protocol

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

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
		cursor ResumeCursor,
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
}

type ReconcilePort interface {
	Reconcile(
		ctx context.Context,
		request ReconcileRequest,
	) (ReconcileDecision, error)
}
