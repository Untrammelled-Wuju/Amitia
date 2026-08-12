// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	"gorm.io/gorm"
)

func (s *service) GetMessages(convID string, page, pageSize int) ([]Message, int64, error) {
	return s.repo.GetMessages(convID, page, pageSize)
}

func (s *service) GetMessagesScoped(convID string, characterID string, page, pageSize int) ([]Message, int64, error) {
	if err := s.requireConversationCharacter(convID, characterID); err != nil {
		return nil, 0, err
	}
	return s.repo.GetMessages(convID, page, pageSize)
}

func (s *service) DeleteMessages(convID string) error {
	if err := s.repo.DeleteMessagesByConv(convID); err != nil {
		return err
	}
	return pipelinecheckpoint.New(s.db).ResetConversation(convID)
}

func (s *service) DeleteMessagesScoped(convID string, characterID string) error {
	if err := s.requireConversationCharacter(convID, characterID); err != nil {
		return err
	}
	return s.DeleteMessages(convID)
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

func (s *service) DeleteSingleMessageScoped(id string, characterID string) error {
	var msg Message
	if err := s.db.Where("id = ?", id).First(&msg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("消息不存在")
		}
		return err
	}
	if err := s.requireConversationCharacter(msg.ConversationID, characterID); err != nil {
		return err
	}
	return s.DeleteSingleMessage(id)
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

func (s *service) SearchMessagesScoped(q MessageSearchQuery, characterID string) (*ConversationListResponse, error) {
	if q.ConversationID != "" {
		if err := s.requireConversationCharacter(q.ConversationID, characterID); err != nil {
			return nil, err
		}
	}
	return s.SearchMessages(q)
}

func (s *service) requireConversationCharacter(convID string, characterID string) error {
	convID = strings.TrimSpace(convID)
	characterID = strings.TrimSpace(characterID)
	if convID == "" || characterID == "" {
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
	if actualCharacterID != "" && actualCharacterID != characterID {
		return fmt.Errorf("%w: conversation belongs to different character", ErrConversationScopeMismatch)
	}
	return nil
}

func (s *service) Chat(req *ChatRequest) (*ChatResponse, error) {
	channel := strings.TrimSpace(req.Channel)
	if channel == "" {
		channel = "web"
	}
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "web"
	}

	resp, err := s.ProcessMessage(context.Background(), &ProcessMessageRequest{
		CharacterID:    req.CharacterID,
		Message:        req.Message,
		ConversationID: req.ConversationID,
		Channel:        channel,
		Source:         source,
		PeerID:         req.PeerID,
		UserID:         req.UserID,
		RequestID:      req.RequestID,
	})
	if err != nil {
		return nil, err
	}

	msgID := ""
	if len(resp.MessageIDs) > 0 {
		msgID = resp.MessageIDs[len(resp.MessageIDs)-1]
	}

	return &ChatResponse{
		ConversationID: resp.ConversationID,
		Sequence:       resp.Sequence,
		Message: &MessageItem{
			ID:             msgID,
			ConversationID: resp.ConversationID,
			Sequence:       resp.Sequence,
			Role:           "assistant",
			Content:        resp.Reply,
			Source:         "assistant",
			CreatedAt:      time.Now().Format("2006-01-02 15:04:05"),
		},
	}, nil
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
	if expectedCharacterID == "" {
		return fmt.Errorf("%w: character_id", ErrConversationScopeMismatch)
	}
	if actualCharacterID == "" {
		s.repo.UpdateConversation(convID, map[string]interface{}{"character_id": expectedCharacterID})
	} else if actualCharacterID != expectedCharacterID {
		return fmt.Errorf("%w: character_id", ErrConversationScopeMismatch)
	}
	actualChannel := strings.ToLower(strings.TrimSpace(conv.Channel))
	expectedChannel := strings.ToLower(strings.TrimSpace(channel))
	if actualChannel == "" || expectedChannel == "" || actualChannel != expectedChannel {
		return fmt.Errorf("%w: channel", ErrConversationScopeMismatch)
	}
	return nil
}
