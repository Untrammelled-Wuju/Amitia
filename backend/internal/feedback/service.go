// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package feedback

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/u-ai/backend/pkg/app"
	"gorm.io/gorm"
)

type Service interface {
	Create(msgID string, req *CreateFeedbackRequest) (*MessageFeedback, error)
	GetByMessage(msgID string) ([]MessageFeedback, error)
	GetStats() map[string]interface{}
	GetRecent(limit int) ([]MessageFeedback, error)
	Delete(id int) error
}

type service struct {
	repo Repository
	db   *gorm.DB
}

func NewService(repo Repository, ctx *app.AppContext) Service {
	return &service{repo: repo, db: ctx.DB}
}

func (s *service) Create(msgID string, req *CreateFeedbackRequest) (*MessageFeedback, error) {
	if !ValidFeedbackTypes[req.FeedbackType] {
		return nil, fmt.Errorf("无效的反馈类型")
	}

	role, convID, err := s.repo.GetMessage(msgID)
	if err != nil || role != "assistant" {
		return nil, fmt.Errorf("只能对 AI 回复进行反馈")
	}

	fb := &MessageFeedback{
		MessageID:    msgID,
		FeedbackType: req.FeedbackType,
		Reason:       req.Reason,
	}
	if err := s.repo.Create(fb); err != nil {
		return nil, fmt.Errorf("创建反馈失败: %w", err)
	}

	if req.FeedbackType == "unsafe" {
		s.db.Exec(`INSERT INTO safety_events (id, conversation_id, event_type, description, handled)
			VALUES (?, ?, 'user_reported_unsafe', ?, 0)`,
			uuid.New().String(), convID, "用户报告不安全内容。原因: "+req.Reason)
	}

	return fb, nil
}

func (s *service) GetByMessage(msgID string) ([]MessageFeedback, error) {
	return s.repo.GetByMessage(msgID)
}

func (s *service) GetStats() map[string]interface{} {
	total, byType, recent, _ := s.repo.GetStats()
	return map[string]interface{}{"total": total, "byType": byType, "recent": recent}
}

func (s *service) GetRecent(limit int) ([]MessageFeedback, error) {
	return s.repo.GetRecent(limit)
}

func (s *service) Delete(id int) error {
	return s.repo.Delete(id)
}
