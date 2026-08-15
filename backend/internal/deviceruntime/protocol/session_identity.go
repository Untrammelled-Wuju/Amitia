package protocol

import "github.com/u-ai/backend/internal/runtimeidentity"

type SessionIdentity struct {
	UserID           runtimeidentity.UserID
	DeviceID         runtimeidentity.DeviceID
	RuntimeID        runtimeidentity.RuntimeID
	RuntimeSessionID runtimeidentity.RuntimeSessionID
}

type SessionCursor struct {
	ConnectionGeneration         int64  `json:"connectionGeneration"`
	LastAppliedStateRevision     int64  `json:"lastAppliedStateRevision"`
	LastProcessedCommandSequence int64  `json:"lastProcessedCommandSequence"`
	LastEventSequence            int64  `json:"lastEventSequence"`
	ActualStateHash              string `json:"actualStateHash,omitempty"`
}

func (c SessionCursor) IsZero() bool {
	return c.ConnectionGeneration == 0 && c.LastAppliedStateRevision == 0 &&
		c.LastProcessedCommandSequence == 0 && c.LastEventSequence == 0 && c.ActualStateHash == ""
}

func (c SessionCursor) Normalize() SessionCursor {
	if c.ConnectionGeneration < 0 {
		c.ConnectionGeneration = 0
	}
	if c.LastAppliedStateRevision < 0 {
		c.LastAppliedStateRevision = 0
	}
	if c.LastProcessedCommandSequence < 0 {
		c.LastProcessedCommandSequence = 0
	}
	if c.LastEventSequence < 0 {
		c.LastEventSequence = 0
	}
	return c
}
