package chat

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode"

	syncapi "github.com/u-ai/backend/internal/sync"
	"gorm.io/gorm"
)

type ConversationTitleUpdatedEvent struct {
	ConversationID string
	Title          string
	Channel        string
}

var conversationTitleUpdatedPublisher func(ConversationTitleUpdatedEvent)

func SetConversationTitleUpdatedPublisher(publisher func(ConversationTitleUpdatedEvent)) {
	conversationTitleUpdatedPublisher = publisher
}

func publishConversationTitleUpdated(event ConversationTitleUpdatedEvent) {
	if conversationTitleUpdatedPublisher == nil {
		return
	}
	conversationTitleUpdatedPublisher(event)
}

func (s *service) generateConversationTitle(
	conversationID string,
	modelConfigID int,
	userMessage string,
	assistantReply string,
) {
	if s == nil || s.db == nil {
		return
	}
	conversationID = strings.TrimSpace(conversationID)
	userMessage = strings.TrimSpace(userMessage)
	assistantReply = normalizeTitleGenerationReply(assistantReply)
	if conversationID == "" || userMessage == "" || assistantReply == "" {
		return
	}

	var conversation Conversation
	if err := s.db.Where("id = ? AND deleted_at IS NULL", conversationID).First(&conversation).Error; err != nil {
		return
	}
	originalTitle := strings.TrimSpace(conversation.Title)
	if originalTitle != "" && originalTitle != userMessage && originalTitle != "新对话" {
		return
	}

	if !s.isFirstCompletedAssistantTurn(conversationID) {
		return
	}

	cfg, err := s.repo.GetActiveModel()
	if modelConfigID > 0 {
		cfg, err = s.repo.GetModelByID(modelConfigID)
	}
	if err != nil || cfg == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	messages := []map[string]interface{}{
		{
			"role": "system",
			"content": "你是会话标题生成器。根据首轮用户消息和助手回复生成一个简洁准确的中文标题。" +
				"只输出标题本身，不要引号、解释、前缀或标点，标题长度控制在 2 到 24 个字符。",
		},
		{
			"role": "user",
			"content": fmt.Sprintf(
				"用户首条消息：\n%s\n\n助手首条回复：\n%s",
				userMessage,
				assistantReply,
			),
		},
	}
	title, _, err := s.callLLMWithoutThinking(ctx, cfg, messages)
	if err != nil {
		return
	}
	title = normalizeGeneratedConversationTitle(title)
	if title == "" {
		return
	}
	_, _ = s.updateGeneratedConversationTitle(conversationID, originalTitle, title)
}

func normalizeTitleGenerationReply(value string) string {
	lines := strings.Split(value, "\n")
	for index, line := range lines {
		lines[index] = strings.TrimSpace(line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func (s *service) isFirstCompletedAssistantTurn(conversationID string) bool {
	if s == nil || s.db == nil {
		return false
	}
	var count int64
	if err := s.db.Model(&AssistantTurn{}).
		Where("conversation_id = ? AND status = ?", strings.TrimSpace(conversationID), assistantTurnStatusCompleted).
		Count(&count).Error; err != nil {
		return false
	}
	return count == 1
}

func normalizeGeneratedConversationTitle(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "\"'“”‘’")
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "标题:")
	value = strings.TrimPrefix(value, "标题：")
	value = strings.TrimPrefix(value, "Title:")
	value = strings.Join(strings.Fields(value), " ")
	value = strings.Trim(value, "。！？!?,，；;：:")
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) > 24 {
		runes = runes[:24]
	}
	for len(runes) > 0 && unicode.IsSpace(runes[len(runes)-1]) {
		runes = runes[:len(runes)-1]
	}
	if len(runes) < 2 {
		return ""
	}
	return string(runes)
}

func (s *service) updateGeneratedConversationTitle(
	conversationID string,
	originalTitle string,
	generatedTitle string,
) (bool, error) {
	updated := false
	channel := ""
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var conversation Conversation
		if err := tx.Where("id = ? AND deleted_at IS NULL", conversationID).First(&conversation).Error; err != nil {
			return err
		}
		if strings.TrimSpace(conversation.Title) != strings.TrimSpace(originalTitle) {
			return nil
		}
		nextRevision := conversation.Revision + 1
		now := time.Now().Format("2006-01-02 15:04:05")
		result := tx.Model(&Conversation{}).
			Where("id = ? AND title = ? AND revision = ?", conversationID, originalTitle, conversation.Revision).
			Updates(map[string]interface{}{
				"title":      generatedTitle,
				"revision":   nextRevision,
				"updated_at": now,
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		conversation.Title = generatedTitle
		conversation.Revision = nextRevision
		conversation.UpdatedAt = now
		channel = conversation.Channel
		if err := s.recordConversationChangeTx(tx, &conversation, syncapi.OpUpdate, nextRevision, conversation.SpaceID); err != nil {
			return err
		}
		updated = true
		return nil
	})
	if updated {
		publishConversationTitleUpdated(ConversationTitleUpdatedEvent{
			ConversationID: conversationID,
			Title:          generatedTitle,
			Channel:        channel,
		})
	}
	return updated, err
}
