// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package feedback

type MessageFeedback struct {
	ID           int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	MessageID    string `gorm:"column:message_id;not null;index" json:"messageId"`
	FeedbackType string `gorm:"column:feedback_type;not null" json:"feedbackType"`
	Reason       string `gorm:"column:reason" json:"reason"`
	CreatedAt    string `gorm:"column:created_at" json:"createdAt"`
}

func (MessageFeedback) TableName() string { return "message_feedback" }

type CreateFeedbackRequest struct {
	FeedbackType string `json:"feedbackType" binding:"required"`
	Reason       string `json:"reason"`
}

var ValidFeedbackTypes = map[string]bool{
	"good": true, "too_long": true, "too_cold": true,
	"too_exaggerated": true, "not_understand": true, "unsafe": true, "other": true,
}
