// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

func (s *service) GetConversationSummary(convID string) (*ConversationSummary, error) {
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	if _, err := s.GetConversation(convID); err != nil {
		return nil, err
	}
	var summary ConversationSummary
	err := s.db.Where("conversation_id = ?", convID).Order("round_end DESC, compressed_at DESC").First(&summary).Error
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

func (s *service) UpdateConversationSummary(convID, summaryText string) (*ConversationSummary, error) {
	convID = strings.TrimSpace(convID)
	summaryText = strings.TrimSpace(summaryText)
	if convID == "" || summaryText == "" {
		return nil, fmt.Errorf("conversation id and summary text are required")
	}
	if _, err := s.GetConversation(convID); err != nil {
		return nil, err
	}

	var existing ConversationSummary
	err := s.db.Where("conversation_id = ?", convID).Order("round_end DESC, compressed_at DESC").First(&existing).Error
	if err == nil {
		if err := s.db.Model(&existing).Updates(map[string]interface{}{
			"summary_text":  summaryText,
			"compressed_at": time.Now().Format("2006-01-02 15:04:05"),
		}).Error; err != nil {
			return nil, err
		}
		existing.SummaryText = summaryText
		existing.CompressedAt = time.Now().Format("2006-01-02 15:04:05")
		return &existing, nil
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	var messageCount int64
	if err := s.db.Model(&Message{}).Where("conversation_id = ? AND role IN ('user','assistant')", convID).Count(&messageCount).Error; err != nil {
		return nil, err
	}
	summary := &ConversationSummary{
		ID:             uuid.NewString(),
		ConversationID: convID,
		RoundStart:     1,
		RoundEnd:       int((messageCount + 1) / 2),
		SummaryText:    summaryText,
		CompressedAt:   time.Now().Format("2006-01-02 15:04:05"),
	}
	if err := s.db.Create(summary).Error; err != nil {
		return nil, err
	}
	return summary, nil
}

func (s *service) DeleteConversationSummary(convID string) error {
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return fmt.Errorf("conversation id is required")
	}
	if _, err := s.GetConversation(convID); err != nil {
		return err
	}
	return s.db.Where("conversation_id = ?", convID).Delete(&ConversationSummary{}).Error
}

func (s *service) GenerateConversationSummary(ctx context.Context, convID string) (*ConversationSummary, error) {
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return nil, fmt.Errorf("conversation id is required")
	}
	if _, err := s.GetConversation(convID); err != nil {
		return nil, err
	}
	if s.compressor == nil {
		return nil, fmt.Errorf("conversation compressor is unavailable")
	}

	var messages []Message
	if err := s.db.WithContext(ctx).
		Where("conversation_id = ? AND role IN ('user','assistant')", convID).
		Order("sequence ASC, created_at ASC").
		Limit(200).
		Find(&messages).Error; err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("conversation has no messages")
	}

	var text strings.Builder
	for _, message := range messages {
		content := strings.TrimSpace(message.Content)
		if content == "" {
			continue
		}
		text.WriteString(message.Role)
		text.WriteString(": ")
		text.WriteString(content)
		text.WriteByte('\n')
	}
	if text.Len() == 0 {
		return nil, fmt.Errorf("conversation has no summarizable content")
	}

	parent := ""
	if current, err := s.GetConversationSummary(convID); err == nil {
		parent = current.SummaryText
	}
	summaryText := s.compressor.generateSummary(ctx, text.String(), parent)
	if strings.TrimSpace(summaryText) == "" {
		return nil, fmt.Errorf("summary generation failed")
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	summary := &ConversationSummary{
		ID:             uuid.NewString(),
		ConversationID: convID,
		RoundStart:     1,
		RoundEnd:       (len(messages) + 1) / 2,
		SummaryText:    strings.TrimSpace(summaryText),
		CompressedAt:   now,
	}
	if err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", convID).Delete(&ConversationSummary{}).Error; err != nil {
			return err
		}
		return tx.Create(summary).Error
	}); err != nil {
		return nil, err
	}
	return summary, nil
}
