// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package memory

type Memory struct {
	ID             string  `gorm:"column:id;primaryKey" json:"id"`
	CharacterID    string  `gorm:"column:character_id" json:"characterId"`
	MemoryType     string  `gorm:"column:memory_type;default:custom" json:"memoryType"`
	Source         string  `gorm:"column:source;default:manual" json:"source"`
	Scope          string  `gorm:"column:scope;default:character" json:"scope"`
	Key            string  `gorm:"column:key;not null" json:"key"`
	Value          string  `gorm:"column:value;not null" json:"value"`
	Importance     int     `gorm:"column:importance;default:0" json:"importance"`
	Confidence     int     `gorm:"column:confidence;default:50" json:"confidence"`
	ExpiresAt      *string `gorm:"column:expires_at" json:"expiresAt"`
	EntityID       string  `gorm:"column:entity_id" json:"entityId"`
	EntityType     string  `gorm:"column:entity_type" json:"entityType"`
	SourceMsgID    string  `gorm:"column:source_msg_id" json:"sourceMsgId"`
	SourceConvID   string  `gorm:"column:source_conv_id" json:"sourceConvId"`
	VerifiedStatus string  `gorm:"column:verified_status;default:unverified" json:"verifiedStatus"`
	LastVerifiedAt *string `gorm:"column:last_verified_at" json:"lastVerifiedAt"`
	UseCount       int     `gorm:"column:use_count;default:0" json:"useCount"`
	LastUsedAt     *string `gorm:"column:last_used_at" json:"lastUsedAt"`
	CreatedAt      string  `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      string  `gorm:"column:updated_at" json:"updatedAt"`
}

func (Memory) TableName() string { return "memories" }

type CreateMemoryRequest struct {
	CharacterID    string `json:"characterId"`
	MemoryType     string `json:"memoryType"`
	Key            string `json:"key" binding:"required"`
	Value          string `json:"value" binding:"required"`
	Importance     int    `json:"importance"`
	Confidence     int    `json:"confidence"`
	ExpiresAt      string `json:"expiresAt"`
	EntityID       string `json:"entityId"`
	EntityType     string `json:"entityType"`
	SourceMsgID    string `json:"sourceMsgId"`
	SourceConvID   string `json:"sourceConvId"`
	VerifiedStatus string `json:"verifiedStatus"`
	Source         string `json:"source"`
	Scope          string `json:"scope"`
}

type UpdateMemoryRequest struct {
	Key            *string `json:"key"`
	Value          *string `json:"value"`
	MemoryType     *string `json:"memoryType"`
	CharacterID    *string `json:"characterId"`
	Importance     *int    `json:"importance"`
	Confidence     *int    `json:"confidence"`
	ExpiresAt      *string `json:"expiresAt"`
	EntityID       *string `json:"entityId"`
	EntityType     *string `json:"entityType"`
	VerifiedStatus *string `json:"verifiedStatus"`
	Scope          *string `json:"scope"`
}

type SearchMemoryRequest struct {
	Keyword     string `json:"keyword" binding:"required"`
	CharacterID string `json:"characterId"`
	Limit       int    `json:"limit"`
}

type VectorSearchRequest struct {
	Keyword     string `json:"keyword"`
	Query       string `json:"query"`
	CharacterID string `json:"characterId"`
	Limit       int    `json:"limit"`
}

type MemoryListQuery struct {
	Page           int    `form:"page"`
	PageSize       int    `form:"pageSize"`
	CharacterID    string `form:"characterId"`
	Source         string `form:"source"`
	MemoryType     string `form:"memoryType"`
	Type           string `form:"type"`
	Keyword        string `form:"keyword"`
	SortBy         string `form:"sortBy"`
	Sort           string `form:"sort"`
	VerifiedStatus string `form:"verifiedStatus"`
	MinConfidence  int    `form:"minConfidence"`
}

type MemoryListResponse struct {
	Items    []Memory `json:"items"`
	Total    int64    `json:"total"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

type MemoryCandidateModel struct {
	ID             string `gorm:"column:id;primaryKey" json:"id"`
	Key            string `gorm:"column:key" json:"key"`
	Value          string `gorm:"column:value" json:"value"`
	MemoryType     string `gorm:"column:memory_type;default:custom" json:"memoryType"`
	Importance     int    `gorm:"column:importance;default:5" json:"importance"`
	SourceText     string `gorm:"column:source_text" json:"sourceText"`
	ConversationID string `gorm:"column:conversation_id" json:"conversationId"`
	CharacterID    string `gorm:"column:character_id" json:"characterId"`
	CreatedAt      string `gorm:"column:created_at" json:"createdAt"`
}

func (MemoryCandidateModel) TableName() string { return "memory_candidates" }
