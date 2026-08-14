package v2

import (
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/deviceruntime"
	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

func DesktopPetSessionIDFactory() runtimeidentity.RuntimeSessionID {
	return runtimeidentity.RuntimeSessionID("rtsessv2_" + uuid.NewString())
}

func HelloToAcquireRequest(
	hello HelloPayload,
	userID runtimeidentity.UserID,
	platform runtimeidentity.Platform,
	now time.Time,
) deviceruntime.AcquireRequest {
	caps := hello.Capabilities
	return deviceruntime.AcquireRequest{
		Identity: protocol.SessionIdentity{
			UserID:    userID,
			DeviceID:  hello.DeviceID,
			RuntimeID: hello.RuntimeID,
		},
		Platform:               platform,
		RuntimeVersion:         hello.RuntimeVersion,
		RuntimeContractVersion: hello.RuntimeContractVersion,
		Capabilities:           caps,
		Cursor: protocol.ResumeCursor{
			LastAppliedStateRevision:     hello.LastAppliedDesiredRevision,
			LastProcessedCommandSequence: hello.LastProcessedCommandSequence,
			LastEventSequence:            hello.LastEventSequence,
			ActualStateHash:              hello.ActualStateHash,
		},
		Now: now,
	}
}

func SessionResultToHelloAck(
	result deviceruntime.AcquireResult,
	desiredRevision int64,
) *HelloAckPayload {
	return &HelloAckPayload{
		Accepted:        true,
		SessionID:       result.Session.ID,
		ServerTime:      result.Session.CreatedAt,
		DesiredRevision: desiredRevision,
		ResumeMode:      string(result.Resume.Mode),
	}
}

func PresenceSnapshotFromSession(session deviceruntime.RuntimeSession) deviceruntime.PresenceSnapshot {
	return deviceruntime.PresenceSnapshot{
		UserID:               session.UserID,
		DeviceID:             session.DeviceID,
		RuntimeID:            session.RuntimeID,
		RuntimeSessionID:     session.ID,
		Platform:             session.Platform,
		ConnectionGeneration: session.ConnectionGeneration,
		At:                   session.LastHeartbeatAt,
	}
}
