package protocol

import "github.com/u-ai/backend/internal/runtimeidentity"

type SessionIdentity struct {
	UserID           runtimeidentity.UserID
	DeviceID         runtimeidentity.DeviceID
	RuntimeID        runtimeidentity.RuntimeID
	RuntimeSessionID runtimeidentity.RuntimeSessionID
}

type SessionCursor struct {
	ConnectionGeneration         int64 `json:"connectionGeneration"`
	LastProcessedCommandSequence int64 `json:"lastProcessedCommandSequence"`
	LastEventSequence            int64 `json:"lastEventSequence"`
	LastAppliedStateRevision     int64 `json:"lastAppliedStateRevision"`
}
