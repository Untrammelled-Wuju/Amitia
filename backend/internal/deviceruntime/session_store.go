package deviceruntime

import (
	"context"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type SessionStore interface {
	Create(ctx context.Context, session RuntimeSession) error

	Get(ctx context.Context, sessionID runtimeidentity.RuntimeSessionID) (RuntimeSession, error)

	GetActiveByRuntime(
		ctx context.Context,
		userID runtimeidentity.UserID,
		deviceID runtimeidentity.DeviceID,
		runtimeID runtimeidentity.RuntimeID,
	) (RuntimeSession, error)

	Update(ctx context.Context, session RuntimeSession) error

	ListActive(ctx context.Context) ([]RuntimeSession, error)

	CloseActiveOnStartup(ctx context.Context, at time.Time, reason string) error

	UpdateHeartbeat(
		ctx context.Context,
		sessionID runtimeidentity.RuntimeSessionID,
		generation int64,
		at time.Time,
		expiresAt time.Time,
	) error

	UpdateCursor(
		ctx context.Context,
		sessionID runtimeidentity.RuntimeSessionID,
		generation int64,
		cursor protocol.ResumeCursor,
		at time.Time,
	) error

	UpdateStatus(
		ctx context.Context,
		sessionID runtimeidentity.RuntimeSessionID,
		generation int64,
		status protocol.SessionStatus,
		at time.Time,
	) error

	Close(
		ctx context.Context,
		sessionID runtimeidentity.RuntimeSessionID,
		generation int64,
		reason string,
		at time.Time,
	) error

	ReplaceForReconnect(
		ctx context.Context,
		expectedGeneration int64,
		updated RuntimeSession,
	) error
}
