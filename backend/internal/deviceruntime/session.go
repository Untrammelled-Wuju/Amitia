package deviceruntime

import (
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type RuntimeSession struct {
	ID runtimeidentity.RuntimeSessionID

	UserID    runtimeidentity.UserID
	DeviceID  runtimeidentity.DeviceID
	RuntimeID runtimeidentity.RuntimeID
	Platform  runtimeidentity.Platform

	Status protocol.SessionStatus

	ConnectionGeneration int64

	RuntimeVersion         string
	RuntimeContractVersion string

	Capabilities     []string
	CapabilitiesHash string

	LastAppliedStateRevision     int64
	LastProcessedCommandSequence int64
	LastEventSequence            int64
	ActualStateHash              string

	CreatedAt       time.Time
	UpdatedAt       time.Time
	LastHeartbeatAt time.Time
	ExpiresAt       time.Time
	ClosedAt        *time.Time
	CloseReason     string

	Revision int64
}

func (s RuntimeSession) Identity() protocol.SessionIdentity {
	return protocol.SessionIdentity{
		UserID:           s.UserID,
		DeviceID:         s.DeviceID,
		RuntimeID:        s.RuntimeID,
		RuntimeSessionID: s.ID,
	}
}

func (s RuntimeSession) RuntimeIdentity() runtimeidentity.Identity {
	return runtimeidentity.Identity{
		UserID:           s.UserID,
		DeviceID:         s.DeviceID,
		RuntimeID:        s.RuntimeID,
		RuntimeSessionID: s.ID,
	}
}

func (s RuntimeSession) ResumeCursor() protocol.SessionCursor {
	return protocol.SessionCursor{
		ConnectionGeneration:         s.ConnectionGeneration,
		LastAppliedStateRevision:     s.LastAppliedStateRevision,
		LastProcessedCommandSequence: s.LastProcessedCommandSequence,
		LastEventSequence:            s.LastEventSequence,
		ActualStateHash:              s.ActualStateHash,
	}
}

func (s RuntimeSession) IsActive() bool {
	return s.Status.IsActive()
}

func (s RuntimeSession) IsExpiredAt(now time.Time) bool {
	if s.ExpiresAt.IsZero() {
		return false
	}
	return now.After(s.ExpiresAt)
}
