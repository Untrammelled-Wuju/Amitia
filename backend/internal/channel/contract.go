package channel

import (
	"context"
	"encoding/json"
	"time"
)

type ID string

type Capabilities struct {
	Text   bool
	Image  bool
	Video  bool
	Voice  bool
	File   bool
	Group  bool
	Thread bool
	Reply  bool
}

type SidecarSpec struct {
	Kind        string
	Subdir      string
	DefaultPort int
	HealthPath  string
}

type Definition struct {
	ID            ID
	Name          string
	Description   string
	Version       string
	PublisherID   string
	Capabilities  Capabilities
	Sidecar       *SidecarSpec
	ConfigSchema  json.RawMessage
	PermissionSet []string
	Metadata      map[string]any
}

type AccountStatus struct {
	Channel      ID        `json:"channel"`
	AccountID    string    `json:"accountId"`
	Connected    bool      `json:"connected"`
	Status       string    `json:"status"`
	Running      bool      `json:"running"`
	StartedAt    time.Time `json:"startedAt,omitempty"`
	LastError    string    `json:"lastError,omitempty"`
	MessageCount int64     `json:"messageCount,omitempty"`
	ReplyCount   int64     `json:"replyCount,omitempty"`
}

type SendRequest struct {
	Channel          ID              `json:"channel"`
	AccountID        string          `json:"accountId,omitempty"`
	PeerID           string          `json:"peerId"`
	ConversationID   string          `json:"conversationId,omitempty"`
	ContentType      string          `json:"contentType"`
	Text             string          `json:"text,omitempty"`
	Payload          json.RawMessage `json:"payload,omitempty"`
	ContextToken     string          `json:"contextToken,omitempty"`
	ReplyToMessageID string          `json:"replyToMessageId,omitempty"`
	RunID            string          `json:"runId,omitempty"`
	IdempotencyKey   string          `json:"idempotencyKey"`
	Metadata         map[string]any  `json:"metadata,omitempty"`
}

type SendResult struct {
	Accepted          bool      `json:"accepted"`
	Duplicate         bool      `json:"duplicate,omitempty"`
	ProviderMessageID string    `json:"providerMessageId,omitempty"`
	DeliveredAt       time.Time `json:"deliveredAt,omitempty"`
}

type Provider interface {
	Definition() Definition
	Start(context.Context) error
	Stop(context.Context) error
	Connect(context.Context, map[string]any) error
	Disconnect(context.Context) error
	Status(context.Context, string) (AccountStatus, error)
	Send(context.Context, SendRequest) (SendResult, error)
}

type Deleter interface {
	Delete(peerID string) error
}

type ConfigProvider interface {
	Config(context.Context) (map[string]any, error)
}

type Connector interface {
	ConnectResult(context.Context, map[string]any) (map[string]any, error)
}
