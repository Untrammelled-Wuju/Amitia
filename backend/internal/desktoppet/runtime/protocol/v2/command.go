package v2

import (
	"encoding/json"
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type CommandType string

const (
	CommandTypeSyncDesiredState CommandType = "runtime.command.sync_desired_state"
	CommandTypeEnsureAbsent     CommandType = "runtime.command.ensure_absent"
	CommandTypeReloadRelease    CommandType = "runtime.command.reload_release"
	CommandTypePlayAction       CommandType = "runtime.command.play_action"
	CommandTypeStopAction       CommandType = "runtime.command.stop_action"
	CommandTypePauseAction      CommandType = "runtime.command.pause_action"
	CommandTypeResumeAction     CommandType = "runtime.command.resume_action"
	CommandTypeRecenterOnce     CommandType = "runtime.command.recenter_once"
)

func (c CommandType) IsDurable() bool {
	switch c {
	case CommandTypeSyncDesiredState, CommandTypeEnsureAbsent, CommandTypeReloadRelease:
		return true
	}
	return false
}

func (c CommandType) IsEphemeral() bool {
	return !c.IsDurable()
}

type CommandStatus string

const (
	CommandStatusCreated             CommandStatus = "created"
	CommandStatusQueued              CommandStatus = "queued"
	CommandStatusDispatching         CommandStatus = "dispatching"
	CommandStatusTransportDispatched CommandStatus = "transport_dispatched"
	CommandStatusRuntimeReceived     CommandStatus = "runtime_received"
	CommandStatusRuntimeAccepted     CommandStatus = "runtime_accepted"
	CommandStatusRendererAccepted    CommandStatus = "renderer_accepted"
	CommandStatusPlaybackStarted     CommandStatus = "playback_started"
	CommandStatusCompleted           CommandStatus = "completed"
	CommandStatusFailedRetryable     CommandStatus = "failed_retryable"
	CommandStatusFailedTerminal      CommandStatus = "failed_terminal"
	CommandStatusExpired             CommandStatus = "expired"
	CommandStatusCancelRequested     CommandStatus = "cancel_requested"
	CommandStatusCancelled           CommandStatus = "cancelled"
	CommandStatusSuperseded          CommandStatus = "superseded"
)

func (s CommandStatus) IsTerminal() bool {
	switch s {
	case CommandStatusCompleted, CommandStatusFailedTerminal, CommandStatusExpired,
		CommandStatusCancelled, CommandStatusSuperseded:
		return true
	}
	return false
}

func (s CommandStatus) IsRunning() bool {
	switch s {
	case CommandStatusDispatching, CommandStatusTransportDispatched,
		CommandStatusRuntimeReceived, CommandStatusRuntimeAccepted,
		CommandStatusRendererAccepted, CommandStatusPlaybackStarted:
		return true
	}
	return false
}

func (s CommandStatus) CanRetry() bool {
	switch s {
	case CommandStatusCreated, CommandStatusQueued, CommandStatusFailedRetryable:
		return true
	}
	return false
}

type RuntimeCommand struct {
	ID string `gorm:"column:id;primaryKey;type:text" json:"id"`

	UserID    string `gorm:"column:user_id;type:text;not null" json:"userId"`
	DeviceID  string `gorm:"column:device_id;type:text;not null" json:"deviceId"`
	RuntimeID string `gorm:"column:runtime_id;type:text;not null" json:"runtimeId"`

	RuntimeSessionID string `gorm:"column:runtime_session_id;type:text" json:"runtimeSessionId"`

	CommandType string `gorm:"column:command_type;type:text;not null" json:"commandType"`
	Durability  string `gorm:"column:durability;type:text;not null" json:"durability"`

	DeviceSequence int64 `gorm:"column:device_sequence;type:integer" json:"deviceSequence"`

	IdempotencyKey string `gorm:"column:idempotency_key;type:text" json:"idempotencyKey"`
	CoalesceKey    string `gorm:"column:coalesce_key;type:text" json:"coalesceKey"`

	PayloadJSON          string `gorm:"column:payload_json;type:text" json:"payloadJSON"`
	PayloadHash          string `gorm:"column:payload_hash;type:text" json:"payloadHash"`
	PayloadSchemaVersion int    `gorm:"column:payload_schema_version;type:integer" json:"payloadSchemaVersion"`

	Status string `gorm:"column:status;type:text;not null" json:"status"`

	DesiredRevision  int64 `gorm:"column:desired_revision;type:integer" json:"desiredRevision"`
	SettingsRevision int64 `gorm:"column:settings_revision;type:integer" json:"settingsRevision"`

	InstallationID string `gorm:"column:installation_id;type:text" json:"installationId"`
	PetID          string `gorm:"column:pet_id;type:text" json:"petId"`
	ReleaseID      string `gorm:"column:release_id;type:text" json:"releaseId"`

	ExpiresAt string `gorm:"column:expires_at;type:text" json:"expiresAt,omitempty"`

	LastAttemptID string `gorm:"column:last_attempt_id;type:text" json:"lastAttemptId,omitempty"`

	SupersededByCommandID string `gorm:"column:superseded_by_command_id;type:text" json:"supersededByCommandId,omitempty"`

	ErrorCode    string `gorm:"column:error_code;type:text" json:"errorCode,omitempty"`
	ErrorMessage string `gorm:"column:error_message;type:text" json:"errorMessage,omitempty"`

	CreatedAt   string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt   string `gorm:"column:updated_at;type:text" json:"updatedAt"`
	CompletedAt string `gorm:"column:completed_at;type:text" json:"completedAt,omitempty"`
}

func (RuntimeCommand) TableName() string {
	return "desktop_pet_runtime_commands_v2"
}

func (c *RuntimeCommand) IsTerminal() bool {
	return CommandStatus(c.Status).IsTerminal()
}

func (c *RuntimeCommand) IsDurable() bool {
	return CommandType(c.CommandType).IsDurable()
}

type SyncDesiredStatePayload struct {
	DesiredRevision        int64           `json:"desiredRevision"`
	DesiredHash            string          `json:"desiredHash"`
	EnsureAbsent           bool            `json:"ensureAbsent"`
	InstallationID         string          `json:"installationId"`
	PetID                  string          `json:"petId"`
	CharacterID            string          `json:"characterId"`
	ReleaseID              string          `json:"releaseId"`
	ReleaseVersion         string          `json:"releaseVersion"`
	ContentRootHash        string          `json:"contentRootHash"`
	ManifestHash           string          `json:"manifestHash"`
	RuntimeContractVersion string          `json:"runtimeContractVersion"`
	DefaultActionKey       string          `json:"defaultActionKey"`
	SettingsRevision       int64           `json:"settingsRevision"`
	SettingsSnapshot       json.RawMessage `json:"settingsSnapshot,omitempty"`
	ResourceSnapshot       json.RawMessage `json:"resourceSnapshot,omitempty"`
}

type PlayActionPayload struct {
	ActionKey        string  `json:"actionKey"`
	ActionSpecHash   string  `json:"actionSpecHash,omitempty"`
	PlaybackMode     string  `json:"playbackMode"`
	Priority         int     `json:"priority"`
	QueuePolicy      string  `json:"queuePolicy"`
	Interruptible    bool    `json:"interruptible"`
	ReturnTo         string  `json:"returnTo,omitempty"`
	PlaybackRate     float64 `json:"playbackRate"`
	MinimumPlayMs    int64   `json:"minimumPlayMs,omitempty"`
	MaximumPlayMs    int64   `json:"maximumPlayMs,omitempty"`
	CompletionPolicy string  `json:"completionPolicy,omitempty"`
	DecisionID       string  `json:"decisionId,omitempty"`
	Semantic         string  `json:"semantic,omitempty"`
	ReasonCode       string  `json:"reasonCode,omitempty"`
}

type CommandAckPayload struct {
	CommandID        string    `json:"commandId"`
	CommandSequence  int64     `json:"commandSequence"`
	Status           string    `json:"status"`
	PayloadHash      string    `json:"payloadHash,omitempty"`
	RejectReason     string    `json:"rejectReason,omitempty"`
	RejectErrorCode  string    `json:"rejectErrorCode,omitempty"`
	EstimatedStartMs int64     `json:"estimatedStartMs,omitempty"`
	RuntimeSessionID string    `json:"runtimeSessionId"`
	ReceivedAt       time.Time `json:"receivedAt"`
}

func (p CommandAckPayload) ToDeviceRuntimeCommandAck() protocol.CommandAckPayload {
	return protocol.CommandAckPayload{
		CommandID:        p.CommandID,
		CommandSequence:  p.CommandSequence,
		Status:           p.Status,
		PayloadHash:      p.PayloadHash,
		RejectReason:     p.RejectReason,
		RejectErrorCode:  p.RejectErrorCode,
		EstimatedStartMs: p.EstimatedStartMs,
		RuntimeSessionID: runtimeidentity.ParseRuntimeSessionID(p.RuntimeSessionID),
		ReceivedAt:       p.ReceivedAt,
	}
}

type CommandDispatchPayload struct {
	CommandID        string          `json:"commandId"`
	CommandType      string          `json:"commandType"`
	CommandSequence  int64           `json:"commandSequence"`
	DesiredRevision  int64           `json:"desiredRevision"`
	SettingsRevision int64           `json:"settingsRevision"`
	InstallationID   string          `json:"installationId"`
	PetID            string          `json:"petId"`
	ReleaseID        string          `json:"releaseId"`
	Payload          json.RawMessage `json:"payload,omitempty"`
}

func (p CommandDispatchPayload) ToDeviceRuntimeCommand() protocol.CommandPayload {
	return protocol.CommandPayload{
		CommandID:       p.CommandID,
		CommandName:     p.CommandType,
		CommandSequence: p.CommandSequence,
		Payload:         p.Payload,
	}
}
