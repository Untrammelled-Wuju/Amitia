// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package contracts

import (
	"encoding/json"
	"time"
)

const (
	SchemaVersion              = 1
	ProtocolMin                = "1.0"
	ProtocolMax                = "1.0"
	DefaultHeartbeatIntervalMs = 10000
	DefaultHeartbeatTimeoutMs  = 30000
	MaxMessageBytes            = 1048576
	RegisterTimeoutSeconds     = 10
)

const (
	CapPetWindow                 = "pet.window"
	CapPetAnimationFrameSequence = "pet.animation.frame_sequence"
	CapPetSettings               = "pet.settings"
	CapPetRecenter               = "pet.recenter"
	CapPetClickThrough           = "pet.click_through"
	CapPetInteractionEvents      = "pet.interaction_events"
)

type MessageKind string

const (
	KindControl MessageKind = "control"
	KindCommand MessageKind = "command"
	KindResult  MessageKind = "result"
	KindEvent   MessageKind = "event"
)

const (
	MsgRuntimeRegister   = "runtime.register"
	MsgRuntimeWelcome    = "runtime.welcome"
	MsgRuntimeHeartbeat  = "runtime.heartbeat"
	MsgRuntimeStateProbe = "runtime.state_probe"
	MsgRuntimeSync       = "runtime.sync"
	MsgRuntimeSyncResult = "runtime.sync_result"
	MsgControlShutdown   = "control.shutdown"
	MsgControlSuperseded = "control.superseded"

	MsgPetSpawn                = "pet.spawn"
	MsgPetDestroy              = "pet.destroy"
	MsgPetShow                 = "pet.show"
	MsgPetHide                 = "pet.hide"
	MsgPetPlayAction           = "pet.play_action"
	MsgPetUpdateSettings       = "pet.update_settings"
	MsgPetRecenter             = "pet.recenter"
	MsgPetDefaultActionChanged = "pet.default_action_changed"

	MsgRuntimeResult = "runtime.result"
	MsgRuntimeEvent  = "runtime.event"
)

type ResultStatus string

const (
	ResultAccepted  ResultStatus = "accepted"
	ResultApplied   ResultStatus = "applied"
	ResultRejected  ResultStatus = "rejected"
	ResultFailed    ResultStatus = "failed"
	ResultDuplicate ResultStatus = "duplicate"
	ResultExpired   ResultStatus = "expired"
	ResultCancelled ResultStatus = "cancelled"
)

type CommandDurability string

const (
	DurabilityDurable           CommandDurability = "durable"
	DurabilityDurableCoalescing CommandDurability = "durable_coalescing"
	DurabilityDurableImmediate  CommandDurability = "durable_immediate"
	DurabilityEphemeral         CommandDurability = "ephemeral"
	DurabilityEphemeralControl  CommandDurability = "ephemeral_control"
	DurabilityReconcile         CommandDurability = "reconcile"
)

type CommandStatus string

const (
	CmdPending      CommandStatus = "pending"
	CmdSent         CommandStatus = "sent"
	CmdAcknowledged CommandStatus = "acknowledged"
	CmdApplied      CommandStatus = "applied"
	CmdRejected     CommandStatus = "rejected"
	CmdFailed       CommandStatus = "failed"
	CmdExpired      CommandStatus = "expired"
	CmdCancelled    CommandStatus = "cancelled"
	CmdSuperseded   CommandStatus = "superseded"
)

type RuntimeMessage struct {
	SchemaVersion   int             `json:"schemaVersion"`
	ProtocolVersion string          `json:"protocolVersion"`
	Kind            MessageKind     `json:"kind"`
	Name            string          `json:"name"`
	MessageID       string          `json:"messageId"`
	RequestID       string          `json:"requestId,omitempty"`
	CommandID       string          `json:"commandId,omitempty"`
	CorrelationID   string          `json:"correlationId,omitempty"`
	CausationID     string          `json:"causationId,omitempty"`
	IdempotencyKey  string          `json:"idempotencyKey,omitempty"`
	RuntimeID       string          `json:"runtimeId,omitempty"`
	SessionID       string          `json:"sessionId,omitempty"`
	UserID          string          `json:"userId,omitempty"`
	InstallationID  string          `json:"installationId,omitempty"`
	PetInstanceID   string          `json:"petInstanceId,omitempty"`
	Sequence        uint64          `json:"sequence,omitempty"`
	SentAt          time.Time       `json:"sentAt"`
	DeadlineAt      *time.Time      `json:"deadlineAt,omitempty"`
	Payload         json.RawMessage `json:"payload,omitempty"`
}

type RegisterPayload struct {
	RuntimeID                    string   `json:"runtimeId"`
	DeviceID                     string   `json:"deviceId"`
	ProcessInstanceID            string   `json:"processInstanceId"`
	AppVersion                   string   `json:"appVersion"`
	Platform                     string   `json:"platform"`
	Arch                         string   `json:"arch"`
	ProtocolMin                  string   `json:"protocolMin"`
	ProtocolMax                  string   `json:"protocolMax"`
	Capabilities                 []string `json:"capabilities"`
	ResumeSessionID              string   `json:"resumeSessionId"`
	LastAppliedDesiredRevision   int64    `json:"lastAppliedDesiredRevision"`
	LastProcessedCommandSequence uint64   `json:"lastProcessedCommandSequence"`
	ChallengeResponse            string   `json:"challengeResponse"`
	BootstrapTicket              string   `json:"bootstrapTicket"`
}

type WelcomePayload struct {
	SessionID           string    `json:"sessionId"`
	SelectedProtocol    string    `json:"selectedProtocol"`
	BackendInstanceID   string    `json:"backendInstanceId"`
	HeartbeatIntervalMs int       `json:"heartbeatIntervalMs"`
	HeartbeatTimeoutMs  int       `json:"heartbeatTimeoutMs"`
	MaxMessageBytes     int       `json:"maxMessageBytes"`
	FullSyncRequired    bool      `json:"fullSyncRequired"`
	ServerTime          time.Time `json:"serverTime"`
}

type PetInstanceSummary struct {
	PetInstanceID    string  `json:"petInstanceId"`
	InstallationID   string  `json:"installationId"`
	Visible          bool    `json:"visible"`
	CurrentActionKey string  `json:"currentActionKey"`
	PositionX        int     `json:"positionX"`
	PositionY        int     `json:"positionY"`
	ScreenID         string  `json:"screenId"`
	Scale            float64 `json:"scale"`
}

type HeartbeatPayload struct {
	RendererHealthy            bool                 `json:"rendererHealthy"`
	PetInstances               []PetInstanceSummary `json:"petInstances"`
	LastAppliedDesiredRevision int64                `json:"lastAppliedDesiredRevision"`
	QueueDepth                 int                  `json:"queueDepth"`
	MemoryUsageMB              int                  `json:"memoryUsageMB"`
	ErrorSummary               string               `json:"errorSummary"`
}

type InstallationSnapshot struct {
	InstallationID   string `json:"installationId"`
	CharacterID      string `json:"characterId"`
	PackageID        string `json:"packageId"`
	PackageVersion   string `json:"packageVersion"`
	InstallRoot      string `json:"installRoot"`
	ManifestPath     string `json:"manifestPath"`
	PackageHash      string `json:"packageHash"`
	DefaultActionKey string `json:"defaultActionKey"`
	CanvasWidth      int    `json:"canvasWidth"`
	CanvasHeight     int    `json:"canvasHeight"`
}

type SettingsSnapshot struct {
	Revision         int64   `json:"revision"`
	AlwaysOnTop      bool    `json:"alwaysOnTop"`
	Scale            float64 `json:"scale"`
	PositionX        int     `json:"positionX"`
	PositionY        int     `json:"positionY"`
	ScreenID         string  `json:"screenId"`
	ClickThroughMode string  `json:"clickThroughMode"`
	SoundEnabled     bool    `json:"soundEnabled"`
}

type SpawnPayload struct {
	DesiredRevision int64                `json:"desiredRevision"`
	Installation    InstallationSnapshot `json:"installation"`
	Settings        SettingsSnapshot     `json:"settings"`
}

type DestroyPayload struct {
	DesiredRevision int64  `json:"desiredRevision"`
	Reason          string `json:"reason"`
}

type ShowPayload struct {
	DesiredRevision int64 `json:"desiredRevision"`
}

type HidePayload struct {
	DesiredRevision int64 `json:"desiredRevision"`
}

type PlayActionPayload struct {
	ActionKey        string    `json:"actionKey"`
	ActionSpecHash   string    `json:"actionSpecHash"`
	PriorityOverride int       `json:"priorityOverride"`
	ExpiresAt        time.Time `json:"expiresAt"`
}

type UpdateSettingsPayload struct {
	SettingsRevision int64            `json:"settingsRevision"`
	Settings         SettingsSnapshot `json:"settings"`
}

type RecenterPayload struct {
	SettingsRevision int64  `json:"settingsRevision"`
	ScreenID         string `json:"screenId"`
}

type DefaultActionChangedPayload struct {
	ActionKey      string `json:"actionKey"`
	ActionSpecHash string `json:"actionSpecHash"`
	Revision       int64  `json:"revision"`
}

type StateProbePayload struct {
	IncludeInstances bool `json:"includeInstances"`
	IncludeHealth    bool `json:"includeHealth"`
}

type SyncPayload struct {
	DesiredRevision int64         `json:"desiredRevision"`
	EnsureAbsent    bool          `json:"ensureAbsent"`
	DesiredPet      *SpawnPayload `json:"desiredPet,omitempty"`
}

type SyncResultPayload struct {
	AppliedRevision   int64                `json:"appliedRevision"`
	Instances         []PetInstanceSummary `json:"instances"`
	DestroyedStaleIds []string             `json:"destroyedStaleIds"`
	Warnings          []string             `json:"warnings"`
}

type ShutdownPayload struct {
	Deadline time.Time `json:"deadline"`
	Reason   string    `json:"reason"`
}

type SupersededPayload struct {
	NewSessionID string `json:"newSessionId"`
	Reason       string `json:"reason"`
}

type ResultPayload struct {
	CommandID         string              `json:"commandId"`
	Status            ResultStatus        `json:"status"`
	ErrorCode         string              `json:"errorCode"`
	ErrorMessage      string              `json:"errorMessage"`
	AppliedRevision   int64               `json:"appliedRevision"`
	ActualState       *PetInstanceSummary `json:"actualState,omitempty"`
	AcceptedAction    string              `json:"acceptedAction,omitempty"`
	PlaybackRequestID string              `json:"playbackRequestId,omitempty"`
	PreviousResult    json.RawMessage     `json:"previousResult,omitempty"`
}

type EventPayload struct {
	EventType     string          `json:"eventType"`
	PetInstanceID string          `json:"petInstanceId"`
	Data          json.RawMessage `json:"data,omitempty"`
}

type DesiredRuntimeSnapshot struct {
	DesiredRevision int64         `json:"desiredRevision"`
	EnsureAbsent    bool          `json:"ensureAbsent"`
	DesiredPet      *SpawnPayload `json:"desiredPet,omitempty"`
	GeneratedAt     time.Time     `json:"generatedAt"`
}

type RuntimeStateSnapshot struct {
	RuntimeID  string               `json:"runtimeId"`
	SessionID  string               `json:"sessionId"`
	Instances  []PetInstanceSummary `json:"instances"`
	Health     string               `json:"health"`
	ObservedAt time.Time            `json:"observedAt"`
}

func IsValidMessageKind(kind MessageKind) bool {
	switch kind {
	case KindControl, KindCommand, KindResult, KindEvent:
		return true
	}
	return false
}

func IsValidResultStatus(status ResultStatus) bool {
	switch status {
	case ResultAccepted, ResultApplied, ResultRejected, ResultFailed, ResultDuplicate, ResultExpired, ResultCancelled:
		return true
	}
	return false
}

func IsValidCommandDurability(d CommandDurability) bool {
	switch d {
	case DurabilityDurable, DurabilityDurableCoalescing, DurabilityDurableImmediate,
		DurabilityEphemeral, DurabilityEphemeralControl, DurabilityReconcile:
		return true
	}
	return false
}

func IsValidCommandStatus(s CommandStatus) bool {
	switch s {
	case CmdPending, CmdSent, CmdAcknowledged, CmdApplied, CmdRejected, CmdFailed, CmdExpired, CmdCancelled, CmdSuperseded:
		return true
	}
	return false
}
