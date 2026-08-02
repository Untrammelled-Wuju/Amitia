// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package protocol

const (
	ProtocolVersion1_0     = "1.0"
	CurrentProtocolVersion = ProtocolVersion1_0
	SnapshotVersionV2      = 2
	CurrentSnapshotVersion = SnapshotVersionV2
)

type RuntimeEventType string

const (
	EvtRuntimeConnected    RuntimeEventType = "runtime.connected"
	EvtRuntimeDisconnected RuntimeEventType = "runtime.disconnected"
	EvtRuntimeHeartbeat    RuntimeEventType = "runtime.heartbeat"

	EvtDesktopPetClicked       RuntimeEventType = "desktop.pet.clicked"
	EvtDesktopPetDoubleClicked RuntimeEventType = "desktop.pet.double_clicked"
	EvtDesktopPetHoverEntered  RuntimeEventType = "desktop.pet.hover.entered"
	EvtDesktopPetHoverMoved    RuntimeEventType = "desktop.pet.hover.moved"
	EvtDesktopPetHoverLeft     RuntimeEventType = "desktop.pet.hover.left"

	EvtDesktopPetDragStarted   RuntimeEventType = "desktop.pet.drag.started"
	EvtDesktopPetDragMoved     RuntimeEventType = "desktop.pet.drag.moved"
	EvtDesktopPetDragCompleted RuntimeEventType = "desktop.pet.drag.completed"
	EvtDesktopPetDragCancelled RuntimeEventType = "desktop.pet.drag.cancelled"

	EvtPlaybackActionRequested   RuntimeEventType = "playback.action.requested"
	EvtPlaybackActionStarted     RuntimeEventType = "playback.action.started"
	EvtPlaybackActionCompleted   RuntimeEventType = "playback.action.completed"
	EvtPlaybackActionInterrupted RuntimeEventType = "playback.action.interrupted"
	EvtPlaybackActionFailed      RuntimeEventType = "playback.action.failed"

	EvtWindowShown          RuntimeEventType = "window.shown"
	EvtWindowHidden         RuntimeEventType = "window.hidden"
	EvtWindowMoved          RuntimeEventType = "window.moved"
	EvtWindowDisplayChanged RuntimeEventType = "window.display_changed"

	EvtRuntimeStateSnapshot       RuntimeEventType = "runtime.state.snapshot"
	EvtRuntimeCommandAcknowledged RuntimeEventType = "runtime.command.acknowledged"
	EvtRuntimeCommandRejected     RuntimeEventType = "runtime.command.rejected"
)

var standardEventTypes = map[RuntimeEventType]struct{}{
	EvtRuntimeConnected:           {},
	EvtRuntimeDisconnected:        {},
	EvtRuntimeHeartbeat:           {},
	EvtDesktopPetClicked:          {},
	EvtDesktopPetDoubleClicked:    {},
	EvtDesktopPetHoverEntered:     {},
	EvtDesktopPetHoverMoved:       {},
	EvtDesktopPetHoverLeft:        {},
	EvtDesktopPetDragStarted:      {},
	EvtDesktopPetDragMoved:        {},
	EvtDesktopPetDragCompleted:    {},
	EvtDesktopPetDragCancelled:    {},
	EvtPlaybackActionRequested:    {},
	EvtPlaybackActionStarted:      {},
	EvtPlaybackActionCompleted:    {},
	EvtPlaybackActionInterrupted:  {},
	EvtPlaybackActionFailed:       {},
	EvtWindowShown:                {},
	EvtWindowHidden:               {},
	EvtWindowMoved:                {},
	EvtWindowDisplayChanged:       {},
	EvtRuntimeStateSnapshot:       {},
	EvtRuntimeCommandAcknowledged: {},
	EvtRuntimeCommandRejected:     {},
}

func IsValidEventType(t string) bool {
	_, ok := standardEventTypes[RuntimeEventType(t)]
	return ok
}

type CommandPhase string

const (
	PhaseCreated    CommandPhase = "created"
	PhaseQueued     CommandPhase = "queued"
	PhaseSent       CommandPhase = "sent"
	PhaseReceived   CommandPhase = "received"
	PhaseAccepted   CommandPhase = "accepted"
	PhaseStarted    CommandPhase = "started"
	PhaseCompleted  CommandPhase = "completed"
	PhaseRejected   CommandPhase = "rejected"
	PhaseFailed     CommandPhase = "failed"
	PhaseExpired    CommandPhase = "expired"
	PhaseSuperseded CommandPhase = "superseded"
)

type IdempotencyMode string

const (
	IdempotencyStatefulReplace IdempotencyMode = "stateful_replace"
	IdempotencyOnce            IdempotencyMode = "once"
	IdempotencyQuery           IdempotencyMode = "query"
)

type RejectReason string

const (
	RejectRuntimeNotReady      RejectReason = "runtime_not_ready"
	RejectInstallationMismatch RejectReason = "installation_mismatch"
	RejectReleaseMismatch      RejectReason = "release_mismatch"
	RejectActionNotFound       RejectReason = "action_not_found"
	RejectCommandExpired       RejectReason = "command_expired"
	RejectCommandOutOfOrder    RejectReason = "command_out_of_order"
	RejectUnsupportedCommand   RejectReason = "unsupported_command"
	RejectInvalidPayload       RejectReason = "invalid_payload"
	RejectWindowUnavailable    RejectReason = "window_unavailable"
	RejectRendererUnavailable  RejectReason = "renderer_unavailable"
	RejectCommandIDConflict    RejectReason = "command_id_conflict"
	RejectStaleDesiredRevision RejectReason = "stale_desired_revision"
)

type ResumeMode string

const (
	ResumeModeResume       ResumeMode = "resume"
	ResumeModeFullResync   ResumeMode = "full_resync"
	ResumeModeSessionReset ResumeMode = "session_reset"
)

type Capability string

const (
	CapAnimationV2          Capability = "animation.v2"
	CapPlaybackHold         Capability = "playback.hold"
	CapPlaybackPingPong     Capability = "playback.ping_pong"
	CapWindowClickThrough   Capability = "window.click-through.alpha"
	CapWindowMultiDisplay   Capability = "window.multi-display"
	CapRuntimeCommandAck    Capability = "runtime.command-ack"
	CapRuntimeSnapshotV2    Capability = "runtime.snapshot.v2"
	CapPetWindow            Capability = "pet.window"
	CapPetAnimationFrame    Capability = "pet.animation.frame_sequence"
	CapPetSettings          Capability = "pet.settings"
	CapPetRecenter          Capability = "pet.recenter"
	CapPetClickThrough      Capability = "pet.click_through"
	CapPetInteractionEvents Capability = "pet.interaction_events"
)
