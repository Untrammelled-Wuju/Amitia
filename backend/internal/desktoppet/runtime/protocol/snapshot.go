// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package protocol

import "time"

type RuntimeStateSnapshotV2 struct {
	SnapshotVersion             int       `json:"snapshotVersion"`
	RuntimeInstanceID           string    `json:"runtimeInstanceId"`
	InstallationID              string    `json:"installationId"`
	PetID                       string    `json:"petId"`
	ReleaseID                   string    `json:"releaseId"`
	Connected                   bool      `json:"connected"`
	WindowVisible               bool      `json:"windowVisible"`
	WindowX                     float64   `json:"windowX"`
	WindowY                     float64   `json:"windowY"`
	DisplayID                   string    `json:"displayId"`
	CurrentActionKey            string    `json:"currentActionKey"`
	PlaybackID                  string    `json:"playbackId"`
	PlaybackPhase               string    `json:"playbackPhase"`
	FrameIndex                  int       `json:"frameIndex"`
	CycleIndex                  int       `json:"cycleIndex"`
	LastAppliedDesiredRevision  int64     `json:"lastAppliedDesiredRevision"`
	LastReceivedCommandSequence int64     `json:"lastReceivedCommandSequence"`
	LastSentEventSequence       int64     `json:"lastSentEventSequence"`
	CapturedAt                  time.Time `json:"capturedAt"`
}

type OutboxPriority int

const (
	OutboxPriorityMustRetain OutboxPriority = 0
	OutboxPriorityMergeable  OutboxPriority = 1
	OutboxPriorityDroppable  OutboxPriority = 2
)

func OutboxPriorityForEvent(eventType string) OutboxPriority {
	switch RuntimeEventType(eventType) {
	case EvtRuntimeCommandAcknowledged, EvtRuntimeCommandRejected:
		return OutboxPriorityMustRetain
	case EvtPlaybackActionCompleted, EvtPlaybackActionInterrupted, EvtPlaybackActionFailed:
		return OutboxPriorityMustRetain
	case EvtRuntimeStateSnapshot:
		return OutboxPriorityMergeable
	case EvtWindowMoved, EvtDesktopPetHoverMoved:
		return OutboxPriorityMergeable
	case EvtRuntimeHeartbeat:
		return OutboxPriorityMergeable
	default:
		return OutboxPriorityDroppable
	}
}
