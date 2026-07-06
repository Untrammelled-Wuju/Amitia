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
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	"github.com/u-ai/backend/internal/prompt"
	"gorm.io/gorm"
)

func (s *service) GetMessages(convID string, page, pageSize int) ([]Message, int64, error) {
	return s.repo.GetMessages(convID, page, pageSize)
}

func (s *service) DeleteMessages(convID string) error {
	if err := s.repo.DeleteMessagesByConv(convID); err != nil {
		return err
	}
	return pipelinecheckpoint.New(s.db).ResetConversation(convID)
}

func (s *service) DeleteSingleMessage(id string) error {
	var msg Message
	if err := s.db.Where("id = ?", id).First(&msg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("消息不存在")
		}
		return err
	}
	if err := s.repo.DeleteMessage(id); err != nil {
		return err
	}
	return pipelinecheckpoint.New(s.db).ResetConversation(msg.ConversationID)
}

func (s *service) SearchMessages(q MessageSearchQuery) (*ConversationListResponse, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	_, total, err := s.repo.SearchMessages(q)
	if err != nil {
		return nil, err
	}
	totalPages := int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	items := make([]Conversation, 0)
	return &ConversationListResponse{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize, TotalPages: totalPages}, nil
}

func (s *service) Chat(req *ChatRequest) (*ChatResponse, error) {
	safeMessage := prompt.SanitizeCurrentUserMessage(req.Message)

	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = "web"
	}
	runtimeProfile, err := s.getRoleRuntimeProfile(req.CharacterID)
	if err != nil {
		if req.CharacterID != "" {
			return nil, fmt.Errorf("角色不存在")
		}
		return nil, fmt.Errorf("没有可用角色")
	}
	convID := req.ConversationID
	if convID == "" {
		convID = uuid.New().String()
		s.repo.CreateConversation(&Conversation{ID: convID, CharacterID: runtimeProfile.CharacterID, Title: req.Message, Channel: channel})
	} else if err := s.validateConversationScope(convID, runtimeProfile.CharacterID, channel); err != nil {
		return nil, fmt.Errorf("会话与角色或渠道不匹配")
	}
	cfg, err := s.repo.GetActiveModel()
	if err != nil {
		return nil, fmt.Errorf("没有可用的模型配置")
	}

	systemParts := buildRoleSystemParts(runtimeProfile, nil)
	apiMessages := []map[string]interface{}{}
	apiMessages = append(apiMessages, map[string]interface{}{"role": "system", "content": strings.Join(systemParts, "\n\n")})
	apiMessages = append(apiMessages, map[string]interface{}{"role": "system", "content": s.compiledSystemInstruction(channel)})
	apiMessages = append(apiMessages, map[string]interface{}{"role": "user", "content": safeMessage})

	content, tokens, err := s.callLLM(context.Background(), cfg, apiMessages)
	if err != nil {
		return nil, err
	}

	s.repo.CreateMessage(&Message{ID: uuid.New().String(), ConversationID: convID, Role: "user", Content: req.Message})
	aiMsg := &Message{ID: uuid.New().String(), ConversationID: convID, Role: "assistant", Content: content, Tokens: tokens}
	s.repo.CreateMessage(aiMsg)
	s.db.Exec("UPDATE conversations SET updated_at = ?, message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ?", time.Now().Format("2006-01-02 15:04:05"), convID, convID)

	return &ChatResponse{ConversationID: convID, Message: &MessageItem{ID: aiMsg.ID, ConversationID: convID, Sequence: aiMsg.Sequence, Role: "assistant", Content: content, Tokens: tokens}}, nil
}

func (s *service) validateConversationScope(convID, characterID, channel string) error {
	convID = strings.TrimSpace(convID)
	if convID == "" {
		return nil
	}
	conv, err := s.repo.GetConversation(convID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("会话不存在")
		}
		return err
	}
	actualCharacterID := strings.TrimSpace(conv.CharacterID)
	expectedCharacterID := strings.TrimSpace(characterID)
	if actualCharacterID == "" || expectedCharacterID == "" || actualCharacterID != expectedCharacterID {
		return fmt.Errorf("%w: character_id", ErrConversationScopeMismatch)
	}
	actualChannel := strings.ToLower(strings.TrimSpace(conv.Channel))
	expectedChannel := strings.ToLower(strings.TrimSpace(channel))
	if actualChannel == "" || expectedChannel == "" || actualChannel != expectedChannel {
		return fmt.Errorf("%w: channel", ErrConversationScopeMismatch)
	}
	return nil
}
