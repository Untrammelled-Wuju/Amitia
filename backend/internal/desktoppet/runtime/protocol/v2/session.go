package v2

import (
	"time"

	"github.com/u-ai/backend/internal/deviceruntime/protocol"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type SessionStatus = protocol.SessionStatus

const (
	SessionStatusRegistering = protocol.SessionStatusRegistering
	SessionStatusSyncing     = protocol.SessionStatusSyncing
	SessionStatusReady       = protocol.SessionStatusReady
	SessionStatusDegraded    = protocol.SessionStatusDegraded
	SessionStatusClosing     = protocol.SessionStatusClosing
	SessionStatusClosed      = protocol.SessionStatusClosed
	SessionStatusSuperseded  = protocol.SessionStatusSuperseded
)

type RuntimeSession struct {
	ID string `gorm:"column:id;primaryKey;type:text" json:"id"`

	UserID    runtimeidentity.UserID    `gorm:"column:user_id;type:text;not null" json:"userId"`
	DeviceID  runtimeidentity.DeviceID  `gorm:"column:device_id;type:text;not null" json:"deviceId"`
	RuntimeID runtimeidentity.RuntimeID `gorm:"column:runtime_id;type:text;not null" json:"runtimeId"`

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

func (s *RuntimeSession) RuntimeIdentity() runtimeidentity.Identity {
	return runtimeidentity.Identity{
		UserID:           s.UserID,
		DeviceID:         s.DeviceID,
		RuntimeID:        s.RuntimeID,
		RuntimeSessionID: runtimeidentity.ParseRuntimeSessionID(s.ID),
	}
}

func (s *RuntimeSession) ProtocolSessionIdentity() protocol.SessionIdentity {
	return protocol.SessionIdentity{
		UserID:           s.UserID,
		DeviceID:         s.DeviceID,
		RuntimeID:        s.RuntimeID,
		RuntimeSessionID: runtimeidentity.ParseRuntimeSessionID(s.ID),
	}
}

func (s *RuntimeSession) ResumeCursor() protocol.SessionCursor {
	return protocol.SessionCursor{
		ConnectionGeneration:         s.ConnectionGeneration,
		LastAppliedStateRevision:     s.LastAppliedDesiredRevision,
		LastProcessedCommandSequence: s.LastProcessedCommandSequence,
		LastEventSequence:            s.LastEventSequence,
	}
}

type HelloPayload struct {
	RuntimeVersion               string                    `json:"runtimeVersion"`
	RuntimeContractVersion       string                    `json:"runtimeContractVersion"`
	DeviceID                     runtimeidentity.DeviceID  `json:"deviceId"`
	RuntimeID                    runtimeidentity.RuntimeID `json:"runtimeId"`
	Capabilities                 []string                  `json:"runtimeCapabilities"`
	LastAppliedDesiredRevision   int64                     `json:"lastAppliedDesiredRevision"`
	LastProcessedCommandSequence int64                     `json:"lastProcessedCommandSequence"`
	LastEventSequence            int64                     `json:"lastEventSequence"`
	ActualStateHash              string                    `json:"actualStateHash,omitempty"`
}

func (p HelloPayload) ToDeviceRuntimeHello() protocol.HelloPayload {
	return protocol.HelloPayload{
		RuntimeVersion:               p.RuntimeVersion,
		RuntimeContractVersion:       p.RuntimeContractVersion,
		DeviceID:                     p.DeviceID,
		RuntimeID:                    p.RuntimeID,
		RuntimeCapabilities:          p.Capabilities,
		LastAppliedStateRevision:     p.LastAppliedDesiredRevision,
		LastProcessedCommandSequence: p.LastProcessedCommandSequence,
		LastEventSequence:            p.LastEventSequence,
		ActualStateHash:              p.ActualStateHash,
	}
}

func (p HelloPayload) ResumeCursor(connectionGeneration int64) protocol.SessionCursor {
	return protocol.SessionCursor{
		ConnectionGeneration:         connectionGeneration,
		LastAppliedStateRevision:     p.LastAppliedDesiredRevision,
		LastProcessedCommandSequence: p.LastProcessedCommandSequence,
		LastEventSequence:            p.LastEventSequence,
		ActualStateHash:              p.ActualStateHash,
	}
}

type HelloAckPayload struct {
	Accepted                           bool                             `json:"accepted"`
	SessionID                          runtimeidentity.RuntimeSessionID `json:"sessionId,omitempty"`
	ServerTime                         time.Time                        `json:"serverTime"`
	DesiredRevision                    int64                            `json:"currentDesiredRevision"`
	ResumeMode                         string                           `json:"resumeMode,omitempty"`
	ServerLastAppliedDesiredRevision   int64                            `json:"serverLastAppliedDesiredRevision"`
	ServerLastProcessedCommandSequence int64                            `json:"serverLastProcessedCommandSequence"`
	LastCommittedClientEventSequence   int64                            `json:"lastCommittedClientEventSequence"`
	HeartbeatIntervalMs                int                              `json:"heartbeatIntervalMs,omitempty"`
	HeartbeatTimeoutMs                 int                              `json:"heartbeatTimeoutMs,omitempty"`
	MaxMessageBytes                    int64                            `json:"maxMessageBytes,omitempty"`
	ErrorCode                          string                           `json:"errorCode,omitempty"`
	ErrorMessage                       string                           `json:"errorMessage,omitempty"`
}

func (p HelloAckPayload) DeviceRuntimeHelloAck() protocol.HelloAckPayload {
	return protocol.HelloAckPayload{
		Accepted:   p.Accepted,
		SessionID:  p.SessionID,
		ServerTime: p.ServerTime,
		ResumeMode: protocol.ResumeMode(p.ResumeMode),
	}
}
