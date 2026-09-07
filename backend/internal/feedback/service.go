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
	CreateForUser(msgID, userID string, req *CreateFeedbackRequest) (*MessageFeedback, error)
	GetByMessageForUser(msgID, userID string) ([]MessageFeedback, error)
	GetStats() (map[string]interface{}, error)
	GetRecent(limit int) ([]MessageFeedback, error)
	DeleteForUser(id int, userID string) error
}

type service struct {
	repo Repository
	db   *gorm.DB
}

func NewService(repo Repository, ctx *app.AppContext) Service {
	return &service{repo: repo, db: ctx.DB}
}

func (s *service) CreateForUser(msgID, userID string, req *CreateFeedbackRequest) (*MessageFeedback, error) {
	if req == nil || !ValidFeedbackTypes[req.FeedbackType] {
		return nil, fmt.Errorf("无效的反馈类型")
	}

	role, convID, err := s.repo.GetMessageForUser(msgID, userID)
	if err != nil || role != "assistant" {
		return nil, fmt.Errorf("只能对自己的 AI 回复进行反馈")
	}

	fb := &MessageFeedback{
		MessageID:    msgID,
		FeedbackType: req.FeedbackType,
		Reason:       req.Reason,
	}
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(fb).Error; err != nil {
			return fmt.Errorf("创建反馈失败: %w", err)
		}
		if req.FeedbackType == "unsafe" {
			if err := tx.Exec(`INSERT INTO safety_events (id, conversation_id, event_type, description, handled)
				VALUES (?, ?, 'user_reported_unsafe', ?, 0)`,
				uuid.New().String(), convID, "用户报告不安全内容。原因: "+req.Reason).Error; err != nil {
				return fmt.Errorf("记录安全反馈失败: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return fb, nil
}

func (s *service) GetByMessageForUser(msgID, userID string) ([]MessageFeedback, error) {
	return s.repo.GetByMessageForUser(msgID, userID)
}

func (s *service) GetStats() (map[string]interface{}, error) {
	total, byType, recent, err := s.repo.GetStats()
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"total": total, "byType": byType, "recent": recent}, nil
}

func (s *service) GetRecent(limit int) ([]MessageFeedback, error) {
	return s.repo.GetRecent(limit)
}

func (s *service) DeleteForUser(id int, userID string) error {
	return s.repo.DeleteForUser(id, userID)
}
