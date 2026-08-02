package v2

import (
	"time"
)

type SessionStatus string

const (
	SessionStatusRegistering SessionStatus = "registering"
	SessionStatusSyncing     SessionStatus = "syncing"
	SessionStatusReady       SessionStatus = "ready"
	SessionStatusDegraded    SessionStatus = "degraded"
	SessionStatusClosing     SessionStatus = "closing"
	SessionStatusClosed      SessionStatus = "closed"
	SessionStatusSuperseded  SessionStatus = "superseded"
)

func (s SessionStatus) IsActive() bool {
	switch s {
	case SessionStatusRegistering, SessionStatusSyncing, SessionStatusReady, SessionStatusDegraded:
		return true
	}
	return false
}

func (s SessionStatus) IsTerminal() bool {
	switch s {
	case SessionStatusClosed, SessionStatusSuperseded:
		return true
	}
	return false
}

type RuntimeSession struct {
	ID string `gorm:"column:id;primaryKey;type:text" json:"id"`

	UserID    string `gorm:"column:user_id;type:text;not null" json:"userId"`
	DeviceID  string `gorm:"column:device_id;type:text;not null" json:"deviceId"`
	RuntimeID string `gorm:"column:runtime_id;type:text;not null" json:"runtimeId"`

	ConnectionGeneration int64 `gorm:"column:connection_generation;type:integer;not null" json:"connectionGeneration"`

	RuntimeVersion         string `gorm:"column:runtime_version;type:text" json:"runtimeVersion"`
	RuntimeContractVersion string `gorm:"column:runtime_contract_version;type:text" json:"runtimeContractVersion"`
	CapabilitiesJSON       string `gorm:"column:capabilities_json;type:text" json:"capabilitiesJSON"`
	CapabilitiesHash       string `gorm:"column:capabilities_hash;type:text" json:"capabilitiesHash"`

	LastAppliedDesiredRevision   int64 `gorm:"column:last_applied_desired_revision;type:integer" json:"lastAppliedDesiredRevision"`
	LastProcessedCommandSequence int64 `gorm:"column:last_processed_command_sequence;type:integer" json:"lastProcessedCommandSequence"`
	LastEventSequence            int64 `gorm:"column:last_event_sequence;type:integer" json:"lastEventSequence"`

	Status string `gorm:"column:status;type:text;not null" json:"status"`

	ConnectedAt     string `gorm:"column:connected_at;type:text" json:"connectedAt"`
	LastHeartbeatAt string `gorm:"column:last_heartbeat_at;type:text" json:"lastHeartbeatAt"`
	DisconnectedAt  string `gorm:"column:disconnected_at;type:text" json:"disconnectedAt"`
	SupersededAt    string `gorm:"column:superseded_at;type:text" json:"supersededAt"`
	SupersededBy    string `gorm:"column:superseded_by;type:text" json:"supersededBy"`
	CreatedAt       string `gorm:"column:created_at;type:text" json:"createdAt"`
	UpdatedAt       string `gorm:"column:updated_at;type:text" json:"updatedAt"`
}

func (RuntimeSession) TableName() string {
	return "desktop_pet_runtime_sessions"
}

func (s *RuntimeSession) IsActive() bool {
	return SessionStatus(s.Status).IsActive()
}

func (s *RuntimeSession) IsTerminal() bool {
	return SessionStatus(s.Status).IsTerminal()
}

type HelloPayload struct {
	RuntimeVersion               string   `json:"runtimeVersion"`
	RuntimeContractVersion       string   `json:"runtimeContractVersion"`
	DeviceID                     string   `json:"deviceId"`
	RuntimeID                    string   `json:"runtimeId"`
	Capabilities                 []string `json:"runtimeCapabilities"`
	LastAppliedDesiredRevision   int64    `json:"lastAppliedDesiredRevision"`
	LastProcessedCommandSequence int64    `json:"lastProcessedCommandSequence"`
	LastEventSequence            int64    `json:"lastEventSequence"`
	ActualStateHash              string   `json:"actualStateHash,omitempty"`
}

type HelloAckPayload struct {
	Accepted        bool      `json:"accepted"`
	SessionID       string    `json:"sessionId,omitempty"`
	ServerTime      time.Time `json:"serverTime"`
	DesiredRevision int64     `json:"currentDesiredRevision"`
	ResumeMode      string    `json:"resumeMode,omitempty"`
	ErrorCode       string    `json:"errorCode,omitempty"`
	ErrorMessage    string    `json:"errorMessage,omitempty"`
}
