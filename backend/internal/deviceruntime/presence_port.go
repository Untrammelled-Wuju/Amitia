package deviceruntime

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type PresenceSnapshot = protocol.PresenceSnapshot

type NoopPresencePort struct{}

func (n NoopPresencePort) Acquire(
	ctx context.Context,
	identity protocol.SessionIdentity,
	cursor protocol.SessionCursor,
	runtimeVersion string,
	contractVersion protocol.RuntimeContractVersion,
	capabilities []string,
) (runtimeidentity.RuntimeSessionID, protocol.ResumeDecision, error) {
	return "", protocol.ResumeDecision{}, nil
}

func (n NoopPresencePort) Heartbeat(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	at time.Time,
) error {
	return nil
}

func (n NoopPresencePort) Close(
	ctx context.Context,
	sessionID runtimeidentity.RuntimeSessionID,
	reason string,
) error {
	return nil
}

func (n NoopPresencePort) SessionReady(ctx context.Context, snapshot protocol.PresenceSnapshot) error {
	return nil
}

func (n NoopPresencePort) SessionDisconnected(ctx context.Context, snapshot protocol.PresenceSnapshot, reason string) error {
	return nil
}

var _ protocol.SessionLifecyclePort = NoopPresencePort{}
