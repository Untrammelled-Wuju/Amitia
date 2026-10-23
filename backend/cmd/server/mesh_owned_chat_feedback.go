package main

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/feedback"
	"github.com/u-ai/backend/internal/safety"
	"gorm.io/gorm"
)

func (p *meshLocalDataPort) managedChatFeedback(ctx context.Context, scope coordination.ExecutionScope, request coordination.ChatManagementRequest, records []ownedChatManagementRecord) (map[string]any, error) {
	keys := make([]string, 0)
	var target *ownedChatManagementRecord
	for index, row := range records {
		if row.Kind != "message" || chatManagementString(row.Body, "role") != "assistant" {
			continue
		}
		keys = append(keys, row.ID)
		if row.ID == request.MessageID {
			target = &records[index]
		}
	}
	feedbackScope := feedback.Scope{SpaceID: p.legacySpaceID, OwnerID: p.ownerID, CoreID: scope.CoreID, RoleID: scope.RoleID}
	repo := feedback.NewScopedRepository(p.services.DB)
	if request.Operation == "feedback-stats" {
		return repo.Stats(ctx, feedbackScope, keys)
	}
	if request.Operation == "feedback/delete" {
		item, err := repo.Delete(ctx, feedbackScope, request.FeedbackID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"deleted": true, "id": item.ID, "revision": item.Revision}, nil
	}
	if target == nil {
		return nil, errors.New("当前回复尚未保存、已删除或不属于当前Owner与角色，暂不可反馈")
	}
	if request.Operation == "feedback/list" {
		items, err := repo.ListMessage(ctx, feedbackScope, target.ID)
		if err != nil {
			return nil, err
		}
		return map[string]any{"items": items}, nil
	}
	item := feedback.MessageFeedback{ConversationID: chatManagementString(target.Body, "conversationId"), MessageID: target.ID, RequestID: request.RequestID, FeedbackType: request.FeedbackType, Reason: request.Reason}
	err := p.services.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		created, inserted, err := repo.WithDB(tx).Create(ctx, feedbackScope, item)
		if err != nil {
			return err
		}
		item = *created
		if inserted && request.FeedbackType == "unsafe" {
			policy, err := safety.NewRepository(tx).Policy(ctx, safety.PolicyScope{SpaceID: feedbackScope.SpaceID, OwnerID: feedbackScope.OwnerID, CoreID: feedbackScope.CoreID})
			if err != nil {
				return err
			}
			feedbackID := int64(item.ID)
			details, _ := json.Marshal(map[string]any{"source": "message_feedback", "reason": request.Reason})
			return safety.NewRepository(tx).RecordIncident(ctx, safety.Incident{
				ID:             "safety-incident-" + uuid.NewString(),
				SpaceID:        feedbackScope.SpaceID,
				OwnerID:        feedbackScope.OwnerID,
				CoreID:         feedbackScope.CoreID,
				RoleID:         feedbackScope.RoleID,
				ConversationID: item.ConversationID,
				MessageID:      item.MessageID,
				FeedbackID:     &feedbackID,
				RequestID:      request.RequestID,
				EventType:      "user_reported_unsafe",
				Severity:       "high",
				Description:    "用户报告不安全内容。原因: " + request.Reason,
				Status:         "open",
				PolicyRevision: policy.Revision,
				Revision:       1,
				DetailsJSON:    string(details),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{"feedback": item}, nil
}
