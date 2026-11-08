package protocol

import (
	"time"

	"github.com/u-ai/backend/internal/runtimeidentity"
)

type HelloPayload struct {
	RuntimeVersion               string                    `json:"runtimeVersion"`
	RuntimeContractVersion       string                    `json:"runtimeContractVersion"`
	DeviceID                     runtimeidentity.DeviceID  `json:"deviceId"`
	RuntimeID                    runtimeidentity.RuntimeID `json:"runtimeId"`
	RuntimeCapabilities          []string                  `json:"runtimeCapabilities"`
	LastAppliedStateRevision     int64                     `json:"lastAppliedStateRevision"`
	LastProcessedCommandSequence int64                     `json:"lastProcessedCommandSequence"`
	LastEventSequence            int64                     `json:"lastEventSequence"`
	ActualStateHash              string                    `json:"actualStateHash,omitempty"`
	CapabilityExposureRevision   int64                     `json:"capabilityExposureRevision"`
	ExposedCapabilities          []string                  `json:"exposedCapabilities"`
}

func (p HelloPayload) ResumeCursor(connectionGeneration int64) SessionCursor {
	return SessionCursor{
		ConnectionGeneration:         connectionGeneration,
		LastAppliedStateRevision:     p.LastAppliedStateRevision,
		LastProcessedCommandSequence: p.LastProcessedCommandSequence,
		LastEventSequence:            p.LastEventSequence,
		ActualStateHash:              p.ActualStateHash,
	}
}

type ResumeMode string

const (
	ResumeModeFresh  ResumeMode = "fresh"
	ResumeModeResume ResumeMode = "resume"
	ResumeModeFull   ResumeMode = "full"
)

type HelloAckPayload struct {
	Accepted   bool                             `json:"accepted"`
	SessionID  runtimeidentity.RuntimeSessionID `json:"sessionId,omitempty"`
	ServerTime time.Time                        `json:"serverTime"`
	ResumeMode ResumeMode                       `json:"resumeMode,omitempty"`
}
