// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package feedback

import "gorm.io/gorm"

type MessageFeedback struct {
	ID             int            `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SpaceID        string         `gorm:"column:space_id;not null;default:'';index:idx_message_feedback_scope" json:"spaceId"`
	OwnerID        string         `gorm:"column:owner_id;not null;default:'';index:idx_message_feedback_scope" json:"ownerId"`
	CoreID         string         `gorm:"column:core_id;not null;default:'';index:idx_message_feedback_scope" json:"coreId"`
	RoleID         string         `gorm:"column:role_id;not null;default:'';index:idx_message_feedback_scope" json:"roleId"`
	ConversationID string         `gorm:"column:conversation_id;not null;default:''" json:"conversationId"`
	MessageID      string         `gorm:"column:message_id;not null;default:'';index" json:"messageId"`
	RequestID      string         `gorm:"column:request_id;not null;default:'';index" json:"requestId"`
	FeedbackType   string         `gorm:"column:feedback_type;not null" json:"feedbackType"`
	Reason         string         `gorm:"column:reason" json:"reason"`
	Revision       int64          `gorm:"column:revision;not null;default:1" json:"revision"`
	CreatedAt      string         `gorm:"column:created_at" json:"createdAt"`
	UpdatedAt      string         `gorm:"column:updated_at" json:"updatedAt"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
}

func (MessageFeedback) TableName() string { return "message_feedback" }

var ValidFeedbackTypes = map[string]bool{
	"good": true, "too_long": true, "too_cold": true,
	"too_exaggerated": true, "not_understand": true, "unsafe": true, "other": true,
}
