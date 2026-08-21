// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/artifact"
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	"github.com/u-ai/backend/internal/requestidentity"
	syncapi "github.com/u-ai/backend/internal/sync"
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

func (s *service) removeAttachmentReferences(tx *gorm.DB, attachments []MessageAttachment) error {
	if s.artifactResolver == nil {
		return nil
	}
	for _, att := range attachments {
		id, parseErr := artifact.ParseURI(att.ResourceURI)
		if parseErr == nil {
			if err := s.artifactResolver.UnregisterReferenceGormTx(tx, string(id), "message_attachment", att.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *service) DeleteMessages(convID string) error {
	return s.DeleteMessagesForUser(convID, requestidentity.DefaultUserID)
}

func (s *service) DeleteMessagesForUser(convID string, userID string) error {
	var attachments []MessageAttachment
	if s.artifactResolver != nil {
		attachments, _ = s.repo.GetAttachmentsByConv(convID)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var messages []struct {
			ID             string
			ConversationID string
			Role           string
			Content        string
			Sequence       int64
			MsgType        string
			Source         string
			Revision       int64
		}
		if err := tx.Table("messages").Where("conversation_id = ?", convID).
			Select("id", "conversation_id", "role", "content", "sequence", "msg_type", "source", "COALESCE(revision, 1) AS revision").Scan(&messages).Error; err != nil {
			return err
		}
		if err := s.removeAttachmentReferences(tx, attachments); err != nil {
			return err
		}
		for _, row := range messages {
			message := &Message{ID: row.ID, ConversationID: row.ConversationID, Role: row.Role, Content: row.Content, Sequence: row.Sequence, MsgType: row.MsgType, Source: row.Source}
			now := time.Now().Format("2006-01-02 15:04:05")
			result := tx.Table("messages").Where("id = ? AND revision = ? AND deleted_at IS NULL", row.ID, row.Revision).Updates(map[string]interface{}{
				"deleted_at": now,
				"updated_at": now,
				"revision":   row.Revision + 1,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected == 0 {
				return fmt.Errorf("消息版本冲突")
			}
			if err := s.recordMessageChangeTx(tx, message, syncapi.OpDelete, row.Revision+1, userID); err != nil {
				return err
			}
		}
		if err := tx.Where("conversation_id = ?", convID).Delete(&MessageAttachment{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
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
	return s.DeleteSingleMessageForUser(id, requestidentity.DefaultUserID)
}

func (s *service) DeleteSingleMessageForUser(id string, userID string) error {
	var msg Message
	if err := s.db.Where("id = ?", id).First(&msg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("消息不存在")
		}
		return err
	}
	var attachments []MessageAttachment
	if s.artifactResolver != nil {
		attachments, _ = s.repo.GetMessageAttachments(id)
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var revision int64
		if err := tx.Table("messages").Where("id = ?", id).Select("COALESCE(revision, 1)").Scan(&revision).Error; err != nil {
			return err
		}
		if err := s.removeAttachmentReferences(tx, attachments); err != nil {
			return err
		}
		now := time.Now().Format("2006-01-02 15:04:05")
		result := tx.Table("messages").Where("id = ? AND revision = ? AND deleted_at IS NULL", id, revision).Updates(map[string]interface{}{
			"deleted_at": now,
			"updated_at": now,
			"revision":   revision + 1,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return fmt.Errorf("消息版本冲突")
		}
		if err := s.recordMessageChangeTx(tx, &msg, syncapi.OpDelete, revision+1, userID); err != nil {
			return err
		}
		if err := tx.Where("message_id = ?", id).Delete(&MessageAttachment{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
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

func (s *service) SearchMessages(q MessageSearchQuery) (*MessageSearchResponse, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	items, total, err := s.repo.SearchMessages(q)
	if err != nil {
		return nil, err
	}
	totalPages := int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	if items == nil {
		items = []Message{}
	}
	return &MessageSearchResponse{Items: items, Total: total, Page: q.Page, PageSize: q.PageSize, TotalPages: totalPages}, nil
}

func (s *service) SearchMessagesScoped(q MessageSearchQuery, characterID string) (*MessageSearchResponse, error) {
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
