// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"encoding/json"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/chat/modelprotocol"
	newoutbox "github.com/u-ai/backend/internal/outbox"
	"gorm.io/gorm"
	"time"
)

type ModelProtocol = modelprotocol.ModelProtocol

const (
	ProtocolOpenAIChat        = modelprotocol.ProtocolOpenAIChat
	ProtocolOpenAIResponses   = modelprotocol.ProtocolOpenAIResponses
	ProtocolAnthropicMessages = modelprotocol.ProtocolAnthropicMessages
	ProtocolGeminiGenerate    = modelprotocol.ProtocolGeminiGenerate
	ProtocolOllamaChat        = modelprotocol.ProtocolOllamaChat
)

type ModelContentType = modelprotocol.ModelContentType

const (
	ContentTypeText  = modelprotocol.ContentTypeText
	ContentTypeImage = modelprotocol.ContentTypeImage
	ContentTypeFile  = modelprotocol.ContentTypeFile
	ContentTypeAudio = modelprotocol.ContentTypeAudio
	ContentTypeVideo = modelprotocol.ContentTypeVideo
)

type ModelContentPart = modelprotocol.ModelContentPart
type ModelMessage = modelprotocol.ModelMessage
type ModelToolDefinition = modelprotocol.ModelToolDefinition
type ModelToolCall = modelprotocol.ModelToolCall
type ModelToolResult = modelprotocol.ModelToolResult
type ModelResponseFormat = modelprotocol.ModelResponseFormat
type ModelContinuationState = modelprotocol.ModelContinuationState
type ModelRequest = modelprotocol.ModelRequest
type ModelUsage = modelprotocol.ModelUsage
type ModelError = modelprotocol.ModelError
type ModelResult = modelprotocol.ModelResult
type ModelEventType = modelprotocol.ModelEventType

const (
	ModelEventResponseStarted        = modelprotocol.ModelEventResponseStarted
	ModelEventTextDelta              = modelprotocol.ModelEventTextDelta
	ModelEventTextDone               = modelprotocol.ModelEventTextDone
	ModelEventRefusalDelta           = modelprotocol.ModelEventRefusalDelta
	ModelEventRefusalDone            = modelprotocol.ModelEventRefusalDone
	ModelEventToolCallStarted        = modelprotocol.ModelEventToolCallStarted
	ModelEventToolCallArgumentsDelta = modelprotocol.ModelEventToolCallArgumentsDelta
	ModelEventToolCallDone           = modelprotocol.ModelEventToolCallDone
	ModelEventReasoningSummaryDelta  = modelprotocol.ModelEventReasoningSummaryDelta
	ModelEventReasoningSummaryDone   = modelprotocol.ModelEventReasoningSummaryDone
	ModelEventUsage                  = modelprotocol.ModelEventUsage
	ModelEventCompleted              = modelprotocol.ModelEventCompleted
	ModelEventFailed                 = modelprotocol.ModelEventFailed
	ModelEventCancelled              = modelprotocol.ModelEventCancelled
)

type ModelEvent = modelprotocol.ModelEvent
type ModelEventSink = modelprotocol.ModelEventSink
type ModelCapabilities = modelprotocol.ModelCapabilities

type Conversation struct {
	ID           string `gorm:"column:id;primaryKey" json:"id"`
	CharacterID  string `gorm:"column:character_id" json:"characterId"`
	Title        string `gorm:"column:title" json:"title"`
	Channel      string `gorm:"column:channel;default:web" json:"channel"`
	Source       string `gorm:"column:source;default:manual" json:"source"`
	PeerID       string `gorm:"column:peer_id" json:"peerId"`
	MessageCount int    `gorm:"column:message_count;default:0" json:"messageCount"`
	StateVersion string `gorm:"column:state_version" json:"stateVersion"`
	CreatedAt    string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt    string `gorm:"column:updated_at" json:"updatedAt"`
}

func (Conversation) TableName() string { return "conversations" }

func (c *Conversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	if c.CreatedAt == "" {
		c.CreatedAt = now
	}
	if c.UpdatedAt == "" {
		c.UpdatedAt = now
	}
	return nil
}

type Message struct {
	ID                  string  `gorm:"column:id;primaryKey" json:"id"`
	ConversationID      string  `gorm:"column:conversation_id;not null;index" json:"conversationId"`
	Sequence            int64   `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Role                string  `gorm:"column:role;not null" json:"role"`
	Content             string  `gorm:"column:content;not null" json:"content"`
	MsgType             string  `gorm:"column:msg_type;default:text" json:"msgType"`
	Tokens              int     `gorm:"column:tokens;default:0" json:"tokens"`
	Source              string  `gorm:"column:source;default:manual" json:"source"`
	SafetyLevel         string  `gorm:"column:safety_level;default:normal" json:"safetyLevel"`
	Status              string  `gorm:"column:status;default:sent" json:"status"`
	IncludeInCtx        int     `gorm:"column:include_in_context;default:1" json:"includeInContext"`
	AudioUrl            string  `gorm:"column:audio_url;default:" json:"audioUrl"`
	AudioDuration       float64 `gorm:"column:audio_duration;default:0" json:"audioDuration"`
	ImageUrl            string  `gorm:"column:image_url;default:" json:"imageUrl"`
	VideoUrl            string  `gorm:"column:video_url;default:" json:"videoUrl"`
	EmoteID             string  `gorm:"column:emote_id;default:" json:"emoteId"`
	AltText             string  `gorm:"column:alt_text;default:" json:"altText"`
	IsAnimated          int     `gorm:"column:is_animated;default:0" json:"isAnimated"`
	MediaWidth          int     `gorm:"column:media_width;default:0" json:"width"`
	MediaHeight         int     `gorm:"column:media_height;default:0" json:"height"`
	OriginalAsset       string  `gorm:"column:original_asset_reference;default:" json:"originalAssetReference"`
	FallbackAsset       string  `gorm:"column:fallback_asset_reference;default:" json:"fallbackAssetReference"`
	ResponseGroupID     string  `gorm:"column:response_group_id;default:" json:"responseGroupId"`
	DeliverySequence    int     `gorm:"column:delivery_sequence;default:0" json:"deliverySequence"`
	EmoteDecisionStatus string  `gorm:"column:emote_decision_status;default:none" json:"emoteDecisionStatus"`
	RequestID           string  `gorm:"column:request_id;default:" json:"requestId"`
	ReplyToMessageID    *string `gorm:"column:reply_to_message_id" json:"replyToMessageId,omitempty"`
	ReplyToRole         *string `gorm:"column:reply_to_role" json:"replyToRole,omitempty"`
	ReplyToExcerpt      *string `gorm:"column:reply_to_excerpt" json:"replyToExcerpt,omitempty"`
	CreatedAt           string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt           string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (Message) TableName() string { return "messages" }

func (m *Message) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	if m.CreatedAt == "" {
		m.CreatedAt = now
	}
	if m.UpdatedAt == "" {
		m.UpdatedAt = now
	}
	if m.Sequence > 0 || m.ConversationID == "" {
		return nil
	}
	var maxSequence int64
	if err := tx.Model(&Message{}).Where("conversation_id = ?", m.ConversationID).Select("COALESCE(MAX(sequence), 0)").Scan(&maxSequence).Error; err != nil {
		return err
	}
	m.Sequence = maxSequence + 1
	return nil
}

type ModelConfig struct {
	ID                int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name              string  `gorm:"column:name" json:"name"`
	APIType           string  `gorm:"column:api_type" json:"apiType"`
	Protocol          string  `gorm:"column:protocol" json:"protocol"`
	BaseURL           string  `gorm:"column:base_url" json:"baseUrl"`
	APIKey            string  `gorm:"column:api_key" json:"apiKey"`
	ModelName         string  `gorm:"column:model_name" json:"modelName"`
	Temperature       float64 `gorm:"column:temperature;default:0.7" json:"temperature"`
	MaxTokens         int     `gorm:"column:max_tokens;default:4096" json:"maxTokens"`
	ContextWindow     int     `gorm:"column:context_window;default:0" json:"contextWindow"`
	MaxOutputTokens   int     `gorm:"column:max_output_tokens;default:0" json:"maxOutputTokens"`
	CapabilitiesJSON  string  `gorm:"column:capabilities_json" json:"capabilitiesJson"`
	TopP              float64 `gorm:"column:top_p;default:1" json:"topP"`
	TimeoutSeconds    int     `gorm:"column:timeout_seconds;default:60" json:"timeoutSeconds"`
	RetryCount        int     `gorm:"column:retry_count;default:1" json:"retryCount"`
	IsActive          int     `gorm:"column:is_active;default:0" json:"isActive"`
	LastTestStatus    string  `gorm:"column:last_test_status" json:"lastTestStatus"`
	LastTestMessage   string  `gorm:"column:last_test_message" json:"lastTestMessage"`
	LastTestAt        string  `gorm:"column:last_test_at" json:"lastTestAt"`
	HasAPIKey         bool    `gorm:"-" json:"hasApiKey"`
	CreatedAt         string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt         string  `gorm:"column:updated_at" json:"updatedAt"`
}

type MessageAttachment struct {
	ID           string `gorm:"column:id;primaryKey" json:"id"`
	MessageID    string `gorm:"column:message_id;not null;index" json:"messageId"`
	Sequence     int    `gorm:"column:sequence;not null;default:0" json:"sequence"`
	Type         string `gorm:"column:type;not null" json:"type"`
	ResourceURI  string `gorm:"column:resource_uri;not null" json:"resourceUri"`
	MIMEType     string `gorm:"column:mime_type" json:"mimeType"`
	Filename     string `gorm:"column:filename" json:"filename"`
	SizeBytes    int64  `gorm:"column:size_bytes;default:0" json:"sizeBytes"`
	ContentHash  string `gorm:"column:content_hash" json:"contentHash"`
	Width        int    `gorm:"column:width;default:0" json:"width"`
	Height       int    `gorm:"column:height;default:0" json:"height"`
	DurationMS   int64  `gorm:"column:duration_ms;default:0" json:"durationMs"`
	CreatedAt    string `gorm:"column:created_at" json:"createdAt"`
}

func (MessageAttachment) TableName() string { return "message_attachments" }

type MessageAttachmentInput struct {
	Type        string `json:"type"`
	ResourceURI string `json:"resourceUri"`
	MIMEType    string `json:"mimeType,omitempty"`
	Filename    string `json:"filename,omitempty"`
}

type ProviderInfo struct {
	ID                 string   `json:"id"`
	Name               string   `json:"name"`
	Protocol           string   `json:"protocol"`
	DefaultProtocol    string   `json:"defaultProtocol"`
	SupportedProtocols []string `json:"supportedProtocols"`
	DefaultBaseURL     string   `json:"defaultBaseUrl"`
	DefaultModel       string   `json:"defaultModel"`
	DocsURL            string   `json:"docsUrl"`
}

func (ModelConfig) TableName() string { return "model_configs" }

type ChatRequest struct {
	CharacterID    string `json:"characterId" binding:"required"`
	Message        string `json:"message" binding:"required"`
	ConversationID string `json:"conversationId"`
	Sequence       int64  `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Channel        string `json:"channel"`
	Source         string `json:"source"`
	PeerID         string `json:"peerId"`
	UserID         string `json:"userId"`
	DeviceTimezone string `json:"deviceTimezone"`
	SessionID      string `json:"sessionId"`
	RequestID      string `json:"requestId"`
}

type WebChatRequest struct {
	CharacterID      string  `json:"characterId" binding:"required"`
	Message          string  `json:"message" binding:"required"`
	ConversationID   string  `json:"conversationId"`
	Sequence         int64   `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	UserID           string  `json:"userId"`
	DeviceTimezone   string  `json:"deviceTimezone"`
	SessionID        string  `json:"sessionId"`
	RequestID        string  `json:"requestId"`
	ReplyToMessageID *string `json:"replyToMessageId,omitempty"`
}

type CreateConversationRequest struct {
	CharacterID string `json:"characterId" binding:"required"`
	Title       string `json:"title"`
	Channel     string `json:"channel"`
	Source      string `json:"source"`
	PeerID      string `json:"peerId"`
}

type ConversationQuery struct {
	Page        int    `form:"page"`
	PageSize    int    `form:"pageSize"`
	Channel     string `form:"channel"`
	Source      string `form:"source"`
	CharacterID string `form:"characterId"`
	Keyword     string `form:"keyword"`
}

type MessageSearchQuery struct {
	Keyword        string `form:"keyword" binding:"required"`
	ConversationID string `form:"conversationId"`
	Sequence       int64  `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Page           int    `form:"page"`
	PageSize       int    `form:"pageSize"`
}

type ChatResponse struct {
	ConversationID string       `json:"conversationId"`
	Sequence       int64        `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Message        *MessageItem `json:"message"`
}

type ContextStructureLog struct {
	ConversationID string `json:"conversationId"`
	Sequence       int64  `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Round          int    `json:"round"`
	Sys1Tokens     int    `json:"sys1Tokens"`
	Sys2Tokens     int    `json:"sys2Tokens"`
	HistoryTokens  int    `json:"historyTokens"`
	UserTokens     int    `json:"userTokens"`
	TotalMessages  int    `json:"totalMessages"`
	CompressedFrom int    `json:"compressedFrom"`
	CompressedTo   int    `json:"compressedTo"`
}

type MessageItem struct {
	ID               string  `json:"id"`
	ConversationID   string  `json:"conversationId"`
	Sequence         int64   `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Role             string  `json:"role"`
	Content          string  `json:"content"`
	Tokens           int     `json:"tokens"`
	Source           string  `json:"source"`
	CreatedAt        string  `json:"createdAt"`
	ReplyToMessageID *string `json:"replyToMessageId,omitempty"`
	ReplyToRole      *string `json:"replyToRole,omitempty"`
	ReplyToExcerpt   *string `json:"replyToExcerpt,omitempty"`
}

type ConversationListResponse struct {
	Items      []Conversation `json:"items"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}

type ProcessMessageRequest struct {
	CharacterID              string                       `json:"characterId"`
	Message                  string                       `json:"message"`
	ConversationID           string                       `json:"conversationId"`
	Sequence                 int64                        `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Channel                  string                       `json:"channel"`
	Source                   string                       `json:"source"`
	ProactiveTaskInstruction string                       `json:"-"`
	ProactiveTimeContext     string                       `json:"-"`
	ProactiveRecentContext   string                       `json:"-"`
	ProactiveRelationship    string                       `json:"-"`
	ProactiveEmotion         string                       `json:"-"`
	ProactiveMemory          string                       `json:"-"`
	PeerID                   string                       `json:"peerId"`
	AudioUrl                 string                       `json:"audioUrl"`
	AudioDuration            float64                      `json:"audioDuration"`
	VoiceMessage             bool                         `json:"voiceMessage"`
	ImageUrl                 string                       `json:"imageUrl"`
	VideoUrl                 string                       `json:"videoUrl"`
	Attachments              []MessageAttachmentInput     `json:"attachments,omitempty"`
	RequestID                string                       `json:"requestId"`
	ReplyToMessageID         *string                      `json:"replyToMessageId,omitempty"`
	ImageContext             string                       `json:"-"`
	UserID                   string                       `json:"-"`
	DeviceTimezone           string                       `json:"-"`
	SessionID                string                       `json:"-"`
	InteractionID            string                       `json:"-"`
	ExpectedStatusVersion    int64                        `json:"-"`
	Runtime                  *interaction.RuntimeAssembly `json:"-"`
	IsInternal               bool                         `json:"-"`
}

type ProcessMessageResponse struct {
	ConversationID string                   `json:"conversationId"`
	Sequence       int64                    `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Reply          string                   `json:"reply"`
	Lines          []string                 `json:"lines"`
	CharacterID    string                   `json:"characterId"`
	CharacterName  string                   `json:"characterName"`
	MessageIDs     []string                 `json:"messageIds"`
	ForceVoice     bool                     `json:"forceVoice"`
	AudioUrls      []string                 `json:"audioUrls"`
	UserMessage    *MessageItem             `json:"userMessage"`
	UserMessageID  string                   `json:"userMessageId"`
	RequestID      string                   `json:"requestId"`
	MessagePlan    *interaction.MessagePlan `json:"messagePlan,omitempty"`
	Events         []newoutbox.OutboxRecord `json:"-"`
}

type ChatStatsResponse struct {
	TodayMessages      int64 `json:"todayMessages"`
	TotalConversations int64 `json:"totalConversations"`
}

type WorkingMemoryState struct {
	ConversationID string   `json:"conversationId"`
	Sequence       int64    `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Summary        string   `json:"summary"`
	KeyPoints      []string `json:"keyPoints"`
	UpdatedAt      string   `json:"updatedAt"`
}
