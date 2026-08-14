package deviceruntime

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type PresenceSnapshot struct {
	UserID           runtimeidentity.UserID
	DeviceID         runtimeidentity.DeviceID
	RuntimeID        runtimeidentity.RuntimeID
	RuntimeSessionID runtimeidentity.RuntimeSessionID
	Platform         runtimeidentity.Platform

	ConnectionGeneration int64
	At                   time.Time
}

type PresencePort interface {
	SessionReady(ctx context.Context, snapshot PresenceSnapshot) error

	Heartbeat(ctx context.Context, snapshot PresenceSnapshot) error

	SessionDisconnected(ctx context.Context, snapshot PresenceSnapshot, reason string) error
}

type NoopPresencePort struct{}

func (n NoopPresencePort) SessionReady(ctx context.Context, snapshot PresenceSnapshot) error {
	return nil
}

func (n NoopPresencePort) Heartbeat(ctx context.Context, snapshot PresenceSnapshot) error {
	return nil
}

func (n NoopPresencePort) SessionDisconnected(ctx context.Context, snapshot PresenceSnapshot, reason string) error {
	return nil
}

var _ PresencePort = NoopPresencePort{}
