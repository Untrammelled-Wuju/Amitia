package v2

import "github.com/u-ai/backend/internal/deviceruntime/protocol"

const (
	EventRuntimeConnected    = protocol.RuntimeEventConnected
	EventRuntimeDisconnected = protocol.RuntimeEventDisconnected
	EventRuntimeHeartbeat    = protocol.RuntimeEventHeartbeat

	EventPointerClicked       = "runtime.pointer.clicked"
	EventPointerDoubleClicked = "runtime.pointer.double_clicked"
	EventPointerHovered       = "runtime.pointer.hovered"

	EventDragStarted   = "runtime.drag.started"
	EventDragMoved     = "runtime.drag.moved"
	EventDragCompleted = "runtime.drag.completed"
	EventDragCancelled = "runtime.drag.cancelled"

	EventPlaybackCommandAccepted   = "runtime.playback.command_accepted"
	EventPlaybackActionStarted     = "runtime.playback.action_started"
	EventPlaybackActionFirstCycle  = "runtime.playback.action_first_cycle"
	EventPlaybackActionHolding     = "runtime.playback.action_holding"
	EventPlaybackActionCompleted   = "runtime.playback.action_completed"
	EventPlaybackActionInterrupted = "runtime.playback.action_interrupted"
	EventPlaybackActionFailed      = "runtime.playback.action_failed"

	EventStateDesiredApplied  = "runtime.state.desired_applied"
	EventStateDesiredRejected = "runtime.state.desired_rejected"
	EventStateSnapshot        = "runtime.state.snapshot"

	EventHealthChanged = "runtime.health.changed"

	EventWindowShown          = "window.shown"
	EventWindowHidden         = "window.hidden"
	EventWindowMoved          = "window.moved"
	EventWindowDisplayChanged = "window.display_changed"

	EventCommandAcknowledged = "runtime.command.acknowledged"
	EventCommandRejected     = "runtime.command.rejected"
	EventCommandTimeout      = "runtime.command.timeout"
)

func IsEventType(t string) bool {
	switch t {
	case EventRuntimeConnected, EventRuntimeDisconnected, EventRuntimeHeartbeat,
		EventPointerClicked, EventPointerDoubleClicked, EventPointerHovered,
		EventDragStarted, EventDragMoved, EventDragCompleted, EventDragCancelled,
		EventPlaybackCommandAccepted, EventPlaybackActionStarted, EventPlaybackActionFirstCycle, EventPlaybackActionHolding,
		EventPlaybackActionCompleted, EventPlaybackActionInterrupted, EventPlaybackActionFailed,
		EventStateDesiredApplied, EventStateDesiredRejected, EventStateSnapshot,
		EventHealthChanged, EventWindowShown, EventWindowHidden, EventWindowMoved,
		EventWindowDisplayChanged, EventCommandAcknowledged, EventCommandRejected, EventCommandTimeout:
		return true
	}
	return false
}

type TriggerSource string

const (
	TriggerSourceRuntimeCommand   TriggerSource = "runtime_command"
	TriggerSourceBehavior         TriggerSource = "behavior"
	TriggerSourceLocalIdle        TriggerSource = "local_idle"
	TriggerSourceLocalInteraction TriggerSource = "local_interaction"
	TriggerSourceSystemRecovery   TriggerSource = "system_recovery"
)

type CompletionReason string

const (
	CompletionReasonNaturalEnd  CompletionReason = "natural_end"
	CompletionReasonMaxDuration CompletionReason = "max_duration_reached"
	CompletionReasonExpired     CompletionReason = "expired"
)

type InterruptReason string

const (
	InterruptReasonHigherPriorityAction InterruptReason = "higher_priority_action"
	InterruptReasonReplacedByCommand    InterruptReason = "replaced_by_command"
	InterruptReasonPackageSwitch        InterruptReason = "package_switch"
	InterruptReasonResourceFailure      InterruptReason = "resource_failure"
	InterruptReasonWindowDestroyed      InterruptReason = "window_destroyed"
	InterruptReasonRuntimeReconnect     InterruptReason = "runtime_reconnect"
	InterruptReasonRuntimeStop          InterruptReason = "runtime_stop"
	InterruptReasonUserDisable          InterruptReason = "user_disable"
	InterruptReasonMaxDurationReached   InterruptReason = "max_duration_reached"
	InterruptReasonCommandCancelled     InterruptReason = "command_cancelled"
)

type PlaybackEvent struct {
	Type                         string `json:"type"`
	PlaybackInstanceID           string `json:"playbackInstanceId,omitempty"`
	CommandID                    string `json:"commandId,omitempty"`
	ActionKey                    string `json:"actionKey,omitempty"`
	TriggerSource                string `json:"triggerSource,omitempty"`
	FrameIndex                   int    `json:"frameIndex,omitempty"`
	CycleIndex                   int    `json:"cycleIndex,omitempty"`
	StartedAt                    string `json:"startedAt,omitempty"`
	CompletedAt                  string `json:"completedAt,omitempty"`
	HoldingAt                    string `json:"holdingAt,omitempty"`
	InterruptedAt                string `json:"interruptedAt,omitempty"`
	FailedAt                     string `json:"failedAt,omitempty"`
	InterruptReason              string `json:"interruptReason,omitempty"`
	ReplacedByCommandID          string `json:"replacedByCommandId,omitempty"`
	ReplacedByPlaybackInstanceID string `json:"replacedByPlaybackInstanceId,omitempty"`
	CompletionReason             string `json:"completionReason,omitempty"`
	CycleCount                   int    `json:"cycleCount,omitempty"`
	ReturnTarget                 string `json:"returnTarget,omitempty"`
	ErrorCode                    string `json:"errorCode,omitempty"`
	ErrorMessage                 string `json:"errorMessage,omitempty"`
	ResourcePathHash             string `json:"resourcePathHash,omitempty"`
	Recoverable                  bool   `json:"recoverable,omitempty"`
	PlayedMs                     int64  `json:"playedMs,omitempty"`
	OccurredAt                   string `json:"occurredAt"`
}

type SyncRejectedPayload struct {
	DesiredRevision int64  `json:"desiredRevision"`
	ErrorCode       string `json:"errorCode"`
	ErrorMessage    string `json:"errorMessage,omitempty"`
	RejectedAt      string `json:"rejectedAt"`
}

type DesiredAppliedPayload struct {
	DesiredRevision  int64  `json:"desiredRevision"`
	DesiredHash      string `json:"desiredHash"`
	SettingsRevision int64  `json:"settingsRevision"`
	InstallationID   string `json:"installationId"`
	ReleaseID        string `json:"releaseId"`
	ActualStateHash  string `json:"actualStateHash"`
	AppliedAt        string `json:"appliedAt"`
}

type StateSnapshotPayload struct {
	ConnectionGeneration         int64  `json:"connectionGeneration"`
	EventSequence                int64  `json:"eventSequence"`
	ActualStateHash              string `json:"actualStateHash"`
	InstanceStatus               string `json:"instanceStatus"`
	WindowStatus                 string `json:"windowStatus"`
	RendererStatus               string `json:"rendererStatus"`
	PlaybackStatus               string `json:"playbackStatus"`
	AppliedDesiredRevision       int64  `json:"appliedDesiredRevision"`
	AppliedDesiredHash           string `json:"appliedDesiredHash,omitempty"`
	AppliedSettingsRevision      int64  `json:"appliedSettingsRevision"`
	InstallationID               string `json:"installationId"`
	PetID                        string `json:"petId"`
	ReleaseID                    string `json:"releaseId"`
	StableActionKey              string `json:"stableActionKey"`
	CurrentActionKey             string `json:"currentActionKey"`
	PlaybackInstanceID           string `json:"playbackInstanceId,omitempty"`
	CurrentCommandID             string `json:"currentCommandId,omitempty"`
	LastProcessedCommandSequence int64  `json:"lastProcessedCommandSequence"`
	CapturedAt                   string `json:"capturedAt"`
}

type HealthChangedPayload struct {
	PreviousStatus string `json:"previousStatus"`
	CurrentStatus  string `json:"currentStatus"`
	Reason         string `json:"reason,omitempty"`
	ChangedAt      string `json:"changedAt"`
}

type EventRecord struct {
	ID               string `gorm:"column:id;primaryKey;type:text" json:"id"`
	EventType        string `gorm:"column:event_type;type:text;not null" json:"eventType"`
	Payload          []byte `gorm:"column:payload;type:blob" json:"payload"`
	PayloadHash      string `gorm:"column:payload_hash;type:text" json:"payloadHash"`
	Source           string `gorm:"column:source;type:text" json:"source"`
	RuntimeSessionID string `gorm:"column:runtime_session_id;type:text;not null" json:"runtimeSessionId"`
	CommandID        string `gorm:"column:command_id;type:text" json:"commandId"`
	Sequence         int64  `gorm:"column:sequence;type:integer;not null" json:"sequence"`
	OccurredAt       string `gorm:"column:occurred_at;type:text" json:"occurredAt"`
	Delivered        int    `gorm:"column:delivered;type:integer" json:"delivered"`
	DeliveredAt      string `gorm:"column:delivered_at;type:text" json:"deliveredAt"`
	InsertedAt       string `gorm:"column:inserted_at;type:text" json:"insertedAt"`
}

func (EventRecord) TableName() string {
	return "desktop_pet_runtime_event_records"
}
