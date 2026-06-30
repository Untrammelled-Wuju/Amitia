// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package worldbook

import (
	"gorm.io/gorm"
	"time"
)

type WorldBookEntry struct {
	ID            string `gorm:"column:id;primaryKey" json:"id"`
	MatchType     string `gorm:"column:match_type;not null" json:"matchType"`
	MatchPattern  string `gorm:"column:match_pattern;not null" json:"matchPattern"`
	MatchScope    string `gorm:"column:match_scope;not null;default:full_context" json:"matchScope"`
	InjectContent string `gorm:"column:inject_content;not null" json:"injectContent"`
	Priority      int    `gorm:"column:priority;default:0" json:"priority"`
	HitCount      int    `gorm:"column:hit_count;default:0" json:"hitCount"`
	CreatedAt     string `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt     string `gorm:"column:updated_at" json:"updatedAt"`
}

func (WorldBookEntry) TableName() string { return "world_book" }

func (w *WorldBookEntry) BeforeCreate(tx *gorm.DB) error {
	now := time.Now().Format("2006-01-02 15:04:05")
	w.CreatedAt = now
	w.UpdatedAt = now
	return nil
}

type CreateWorldBookRequest struct {
	MatchType     string `json:"matchType" binding:"required"`
	MatchPattern  string `json:"matchPattern" binding:"required"`
	MatchScope    string `json:"matchScope"`
	InjectContent string `json:"injectContent" binding:"required"`
	Priority      int    `json:"priority"`
}

type UpdateWorldBookRequest struct {
	MatchType     *string `json:"matchType"`
	MatchPattern  *string `json:"matchPattern"`
	MatchScope    *string `json:"matchScope"`
	InjectContent *string `json:"injectContent"`
	Priority      *int    `json:"priority"`
}

type WorldBookListQuery struct {
	MatchType string `form:"matchType"`
	Page      int    `form:"page"`
	PageSize  int    `form:"pageSize"`
}

type WorldBookListResponse struct {
	Items      []WorldBookEntry `json:"items"`
	Total      int64            `json:"total"`
	Page       int              `json:"page"`
	PageSize   int              `json:"pageSize"`
	TotalPages int              `json:"totalPages"`
}

type TestMatchRequest struct {
	Text string `json:"text" binding:"required"`
}

type TestMatchResponse struct {
	Matches []MatchResult `json:"matches"`
}

type MatchResult struct {
	Entry      WorldBookEntry `json:"entry"`
	MatchScope string         `json:"matchScope"`
	HitText    string         `json:"hitText"`
}
