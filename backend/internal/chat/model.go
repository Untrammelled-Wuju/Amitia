// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"github.com/u-ai/backend/internal/interaction"
	"gorm.io/gorm"
)

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

type Message struct {
	ID             string  `gorm:"column:id;primaryKey" json:"id"`
	ConversationID string  `gorm:"column:conversation_id;not null;index" json:"conversationId"`
	Sequence       int64   `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Role           string  `gorm:"column:role;not null" json:"role"`
	Content        string  `gorm:"column:content;not null" json:"content"`
	MsgType        string  `gorm:"column:msg_type;default:text" json:"msgType"`
	Tokens         int     `gorm:"column:tokens;default:0" json:"tokens"`
	Source         string  `gorm:"column:source;default:manual" json:"source"`
	SafetyLevel    string  `gorm:"column:safety_level;default:normal" json:"safetyLevel"`
	Status         string  `gorm:"column:status;default:sent" json:"status"`
	IncludeInCtx   int     `gorm:"column:include_in_context;default:1" json:"includeInContext"`
	AudioUrl       string  `gorm:"column:audio_url;default:" json:"audioUrl"`
	AudioDuration  float64 `gorm:"column:audio_duration;default:0" json:"audioDuration"`
	ImageUrl       string  `gorm:"column:image_url;default:" json:"imageUrl"`
	VideoUrl       string  `gorm:"column:video_url;default:" json:"videoUrl"`
	RequestID      string  `gorm:"column:request_id;default:" json:"requestId"`
	CreatedAt      string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (Message) TableName() string { return "messages" }

func (m *Message) BeforeCreate(tx *gorm.DB) error {
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
	ID              int     `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	Name            string  `gorm:"column:name" json:"name"`
	APIType         string  `gorm:"column:api_type" json:"apiType"`
	BaseURL         string  `gorm:"column:base_url" json:"baseUrl"`
	APIKey          string  `gorm:"column:api_key" json:"apiKey"`
	ModelName       string  `gorm:"column:model_name" json:"modelName"`
	Temperature     float64 `gorm:"column:temperature;default:0.7" json:"temperature"`
	MaxTokens       int     `gorm:"column:max_tokens;default:4096" json:"maxTokens"`
	TopP            float64 `gorm:"column:top_p;default:1" json:"topP"`
	TimeoutSeconds  int     `gorm:"column:timeout_seconds;default:60" json:"timeoutSeconds"`
	RetryCount      int     `gorm:"column:retry_count;default:1" json:"retryCount"`
	IsActive        int     `gorm:"column:is_active;default:0" json:"isActive"`
	LastTestStatus  string  `gorm:"column:last_test_status" json:"lastTestStatus"`
	LastTestMessage string  `gorm:"column:last_test_message" json:"lastTestMessage"`
	LastTestAt      string  `gorm:"column:last_test_at" json:"lastTestAt"`
	HasAPIKey       bool    `gorm:"-" json:"hasApiKey"`
	CreatedAt       string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       string  `gorm:"column:updated_at" json:"updatedAt"`
}

type ProviderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
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
	SessionID      string `json:"sessionId"`
	RequestID      string `json:"requestId"`
}

type WebChatRequest struct {
	CharacterID    string `json:"characterId" binding:"required"`
	Message        string `json:"message" binding:"required"`
	ConversationID string `json:"conversationId"`
	Sequence       int64  `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	UserID         string `json:"userId"`
	SessionID      string `json:"sessionId"`
	RequestID      string `json:"requestId"`
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
	ID             string `json:"id"`
	ConversationID string `json:"conversationId"`
	Sequence       int64  `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Role           string `json:"role"`
	Content        string `json:"content"`
	Tokens         int    `json:"tokens"`
	Source         string `json:"source"`
	CreatedAt      string `json:"createdAt"`
}

type ConversationListResponse struct {
	Items      []Conversation `json:"items"`
	Total      int64          `json:"total"`
	Page       int            `json:"page"`
	PageSize   int            `json:"pageSize"`
	TotalPages int            `json:"totalPages"`
}

type ProcessMessageRequest struct {
	CharacterID           string                       `json:"characterId"`
	Message               string                       `json:"message"`
	ConversationID        string                       `json:"conversationId"`
	Sequence              int64                        `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Channel               string                       `json:"channel"`
	Source                string                       `json:"source"`
	PeerID                string                       `json:"peerId"`
	AudioUrl              string                       `json:"audioUrl"`
	AudioDuration         float64                      `json:"audioDuration"`
	VoiceMessage          bool                         `json:"voiceMessage"`
	ImageUrl              string                       `json:"imageUrl"`
	VideoUrl              string                       `json:"videoUrl"`
	RequestID             string                       `json:"requestId"`
	ImageContext          string                       `json:"-"`
	UserID                string                       `json:"-"`
	InteractionID         string                       `json:"-"`
	ExpectedStatusVersion int64                        `json:"-"`
	Runtime               *interaction.RuntimeAssembly `json:"-"`
	IsInternal            bool                         `json:"-"`
}

type ProcessMessageResponse struct {
	ConversationID string                     `json:"conversationId"`
	Sequence       int64                      `gorm:"column:sequence;not null;default:0;index" json:"sequence"`
	Reply          string                     `json:"reply"`
	CharacterID    string                     `json:"characterId"`
	CharacterName  string                     `json:"characterName"`
	MessageIDs     []string                   `json:"messageIds"`
	ForceVoice     bool                       `json:"forceVoice"`
	AudioUrls      []string                   `json:"audioUrls"`
	UserMessage    *MessageItem               `json:"userMessage"`
	UserMessageID  string                     `json:"userMessageId"`
	RequestID      string                     `json:"requestId"`
	Events         []interaction.OutboxRecord `json:"-"`
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
