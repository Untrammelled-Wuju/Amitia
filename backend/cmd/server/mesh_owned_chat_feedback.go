package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/feedback"
	"gorm.io/gorm"
)

func ownedFeedbackMessageKey(owner, role, message string) string {
	digest := sha256.Sum256([]byte(owner + "\x00" + role + "\x00" + message))
	return "owned-message/" + hex.EncodeToString(digest[:])
}

func (p *meshLocalDataPort) managedChatFeedback(ctx context.Context, scope coordination.ExecutionScope, request coordination.ChatManagementRequest, records []ownedChatManagementRecord) (map[string]any, error) {
	keys := make([]string, 0)
	originals := map[string]string{}
	var target *ownedChatManagementRecord
	for index, row := range records {
		if row.Kind != "message" || chatManagementString(row.Body, "role") != "assistant" {
			continue
		}
		key := row.ID
		if !row.Legacy {
			key = ownedFeedbackMessageKey(p.ownerID, scope.RoleID, row.ID)
		}
		keys = append(keys, key)
		originals[key] = row.ID
		if row.ID == request.MessageID {
			target = &records[index]
		}
	}
	query := p.services.DB.WithContext(ctx).Model(&feedback.MessageFeedback{}).Where("message_id IN ?", keys)
	if request.Operation == "feedback-stats" {
		var items []feedback.MessageFeedback
		if err := query.Order("created_at DESC,id DESC").Find(&items).Error; err != nil {
			return nil, err
		}
		byType := map[string]int64{}
		for _, item := range items {
			byType[item.FeedbackType]++
		}
		recent := items[:min(10, len(items))]
		for index := range recent {
			recent[index].MessageID = originals[recent[index].MessageID]
		}
		if recent == nil {
			recent = []feedback.MessageFeedback{}
		}
		return map[string]any{"total": len(items), "byType": byType, "recent": recent}, nil
	}
	if request.Operation == "feedback/delete" {
		if request.FeedbackID < 1 {
			return nil, errors.New("反馈编号无效")
		}
		result := query.Where("id=?", request.FeedbackID).Delete(&feedback.MessageFeedback{})
		if result.Error != nil {
			return nil, result.Error
		}
		if result.RowsAffected != 1 {
			return nil, errors.New("反馈不存在或不属于当前角色")
		}
		return map[string]any{"deleted": true, "id": request.FeedbackID}, nil
	}
	if target == nil {
		return nil, errors.New("只能操作当前Owner与角色的AI回复反馈")
	}
	key := target.ID
	if !target.Legacy {
		key = ownedFeedbackMessageKey(p.ownerID, scope.RoleID, target.ID)
	}
	if request.Operation == "feedback/list" {
		items := make([]feedback.MessageFeedback, 0)
		if err := query.Where("message_id=?", key).Order("id").Find(&items).Error; err != nil {
			return nil, err
		}
		for index := range items {
			items[index].MessageID = request.MessageID
		}
		return map[string]any{"items": items}, nil
	}
	if !feedback.ValidFeedbackTypes[request.FeedbackType] || len(request.Reason) > 8192 || strings.ContainsRune(request.Reason, 0) {
		return nil, errors.New("反馈类型或原因无效")
	}
	item := feedback.MessageFeedback{MessageID: key, FeedbackType: request.FeedbackType, Reason: request.Reason, CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	err := p.services.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&item).Error; err != nil {
			return err
		}
		if request.FeedbackType == "unsafe" {
			return tx.Exec(`INSERT INTO safety_events (id,conversation_id,event_type,description,handled) VALUES (?,?,'user_reported_unsafe',?,0)`, uuid.NewString(), chatManagementString(target.Body, "conversationId"), "用户报告不安全内容。原因: "+request.Reason).Error
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	item.MessageID = request.MessageID
	return map[string]any{"feedback": item}, nil
}
