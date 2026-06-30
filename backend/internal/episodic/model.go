// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package episodic

import (
	"gorm.io/gorm"
	"time"
)

type EpisodicMemory struct {
	ID              string `gorm:"column:id;primaryKey" json:"id"`
	UserID          string `gorm:"column:user_id;not null;default:default" json:"userId"`
	SceneType       string `gorm:"column:scene_type;not null" json:"sceneType"`
	Title           string `gorm:"column:title;not null" json:"title"`
	Content         string `gorm:"column:content;not null" json:"content"`
	ContextBefore   string `gorm:"column:context_before" json:"contextBefore"`
	ContextAfter    string `gorm:"column:context_after" json:"contextAfter"`
	TriggerKeywords string `gorm:"column:trigger_keywords" json:"triggerKeywords"`
	SentimentScore  int    `gorm:"column:sentiment_score;default:0" json:"sentimentScore"`
	MessageIDStart  string `gorm:"column:message_id_start" json:"messageIdStart"`
	MessageIDEnd    string `gorm:"column:message_id_end" json:"messageIdEnd"`
	SourceConvID    string `gorm:"column:source_conv_id" json:"sourceConvId"`
	CreatedAt       string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt       string `gorm:"column:updated_at" json:"updatedAt"`
}

func (EpisodicMemory) TableName() string { return "episodic_memories" }

func (e *EpisodicMemory) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	e.CreatedAt = now
	e.UpdatedAt = now
	return nil
}

type CreateEpisodicRequest struct {
	UserID          string `json:"userId"`
	SceneType       string `json:"sceneType" binding:"required"`
	Title           string `json:"title" binding:"required"`
	Content         string `json:"content" binding:"required"`
	ContextBefore   string `json:"contextBefore"`
	ContextAfter    string `json:"contextAfter"`
	TriggerKeywords string `json:"triggerKeywords"`
	SentimentScore  int    `json:"sentimentScore"`
	MessageIDStart  string `json:"messageIdStart"`
	MessageIDEnd    string `json:"messageIdEnd"`
	SourceConvID    string `json:"sourceConvId"`
}

type EpisodicListQuery struct {
	UserID    string `form:"userId"`
	SceneType string `form:"sceneType"`
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
}

type EpisodicListResponse struct {
	Items      []EpisodicMemory `json:"items"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
}
