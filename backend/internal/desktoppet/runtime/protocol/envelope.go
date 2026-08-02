// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package protocol

import (
	"encoding/json"
	"time"
)

type RuntimeEventEnvelope struct {
	ProtocolVersion   string          `json:"protocolVersion"`
	EventID           string          `json:"eventId"`
	EventType         string          `json:"eventType"`
	EventSequence     int64           `json:"eventSequence"`
	UserID            string          `json:"userId"`
	DeviceID          string          `json:"deviceId"`
	InstallationID    string          `json:"installationId"`
	PetID             string          `json:"petId"`
	ReleaseID         string          `json:"releaseId"`
	RuntimeInstanceID string          `json:"runtimeInstanceId"`
	CommandID         string          `json:"commandId,omitempty"`
	DesiredRevision   int64           `json:"desiredRevision,omitempty"`
	OccurredAt        time.Time       `json:"occurredAt"`
	SentAt            time.Time       `json:"sentAt"`
	Payload           json.RawMessage `json:"payload"`
}

type RuntimeCommandEnvelope struct {
	ProtocolVersion   string          `json:"protocolVersion"`
	CommandID         string          `json:"commandId"`
	CommandType       string          `json:"commandType"`
	CommandSequence   int64           `json:"commandSequence"`
	UserID            string          `json:"userId"`
	DeviceID          string          `json:"deviceId"`
	InstallationID    string          `json:"installationId"`
	PetID             string          `json:"petId"`
	ReleaseID         string          `json:"releaseId"`
	RuntimeInstanceID string          `json:"runtimeInstanceId"`
	DesiredRevision   int64           `json:"desiredRevision"`
	IssuedAt          time.Time       `json:"issuedAt"`
	ExpiresAt         time.Time       `json:"expiresAt"`
	IdempotencyMode   IdempotencyMode `json:"idempotencyMode"`
	OperationID       string          `json:"operationId,omitempty"`
	Payload           json.RawMessage `json:"payload"`
}

type RuntimeSessionContext struct {
	UserID            string `json:"userId"`
	DeviceID          string `json:"deviceId"`
	InstallationID    string `json:"installationId"`
	PetID             string `json:"petId"`
	ReleaseID         string `json:"releaseId"`
	RuntimeInstanceID string `json:"runtimeInstanceId"`
}

type CommandAckPayload struct {
	CommandID         string    `json:"commandId"`
	CommandSequence   int64     `json:"commandSequence"`
	Status            string    `json:"status"`
	RuntimeInstanceID string    `json:"runtimeInstanceId"`
	ReceivedAt        time.Time `json:"receivedAt"`
	RejectReason      string    `json:"rejectReason,omitempty"`
}

type RuntimeHelloPayload struct {
	ProtocolVersion             string   `json:"protocolVersion"`
	DeviceID                    string   `json:"deviceId"`
	ClientVersion               string   `json:"clientVersion"`
	RuntimeCapabilities         []string `json:"runtimeCapabilities"`
	LastReceivedCommandSequence int64    `json:"lastReceivedCommandSequence"`
	LastSentEventSequence       int64    `json:"lastSentEventSequence"`
	LastAppliedDesiredRevision  int64    `json:"lastAppliedDesiredRevision"`
	PendingCommandIDs           []string `json:"pendingCommandIds,omitempty"`
}

type RuntimeWelcomePayload struct {
	RuntimeInstanceID       string     `json:"runtimeInstanceId"`
	ServerTime              time.Time  `json:"serverTime"`
	AcceptedProtocolVersion string     `json:"acceptedProtocolVersion"`
	CurrentDesiredRevision  int64      `json:"currentDesiredRevision"`
	ResumeMode              ResumeMode `json:"resumeMode"`
	SessionID               string     `json:"sessionId"`
	HeartbeatIntervalMs     int        `json:"heartbeatIntervalMs"`
	HeartbeatTimeoutMs      int        `json:"heartbeatTimeoutMs"`
	MaxMessageBytes         int        `json:"maxMessageBytes"`
}

type ActualStateMeta struct {
	RuntimeInstanceID          string `json:"runtimeInstanceId"`
	LastEventSequence          int64  `json:"lastEventSequence"`
	LastCommandSequence        int64  `json:"lastCommandSequence"`
	LastAppliedDesiredRevision int64  `json:"lastAppliedDesiredRevision"`
	LastEventID                string `json:"lastEventId"`
}
