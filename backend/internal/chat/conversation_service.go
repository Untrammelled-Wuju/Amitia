// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	"github.com/u-ai/backend/internal/requestidentity"
	"github.com/u-ai/backend/internal/sync"
	"gorm.io/gorm"
)

func (s *service) ListConversations(q ConversationQuery) (*ConversationListResponse, error) {
	if q.Page <= 0 {
		q.Page = 1
	}
	if q.PageSize <= 0 {
		q.PageSize = 20
	}
	convs, total, err := s.repo.ListConversations(q)
	if err != nil {
		return nil, err
	}
	for i := range convs {
		convs[i].MessageCount = int(s.repo.CountMessagesByConv(convs[i].ID))
	}
	totalPages := int((total + int64(q.PageSize) - 1) / int64(q.PageSize))
	return &ConversationListResponse{Items: convs, Total: total, Page: q.Page, PageSize: q.PageSize, TotalPages: totalPages}, nil
}

func (s *service) GetConversation(id string) (*Conversation, error) {
	c, err := s.repo.GetConversation(id)
	if err != nil {
		return nil, fmt.Errorf("对话不存在")
	}
	return c, nil
}

func (s *service) CreateConversation(req *CreateConversationRequest) (*Conversation, error) {
	return s.CreateConversationForUser(req, requestidentity.DefaultUserID)
}

func (s *service) CreateConversationForUser(req *CreateConversationRequest, userID string) (*Conversation, error) {
	if req.Title == "" {
		req.Title = "New Chat"
	}
	if req.Channel == "" {
		req.Channel = "web"
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	c := &Conversation{ID: uuid.New().String(), CharacterID: req.CharacterID, Title: req.Title, Channel: req.Channel, Source: req.Source, PeerID: req.PeerID}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Format("2006-01-02 15:04:05")
		if err := tx.Exec("INSERT INTO conversations (id, character_id, title, channel, source, peer_id, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, 1)",
			c.ID, c.CharacterID, c.Title, c.Channel, c.Source, c.PeerID, now, now).Error; err != nil {
			return err
		}
		if err := s.recordConversationChangeTx(tx, c, sync.OpCreate, 1, userID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *service) EnsureChannelConversation(channel string) (*Conversation, error) {
	title := "微信对话"
	if channel == "qq" {
		title = "QQ对话"
	}
	convID := "conv-" + channel

	c, err := s.repo.GetConversation(convID)
	if err == nil && c != nil && c.ID == convID {
		s.db.Exec("UPDATE conversations SET channel = ?, title = ?, source = 'system' WHERE id = ?", channel, title, convID)
		c.Channel = channel
		c.Title = title
		c.Source = "system"
		return c, nil
	}

	c, err = s.repo.GetConversationByChannel(channel)
	if err == nil && c != nil && c.ID != "" {
		s.db.Exec("UPDATE conversations SET channel = ?, title = ?, source = 'system' WHERE id = ?", channel, title, c.ID)
		c.Channel = channel
		c.Title = title
		c.Source = "system"
		return c, nil
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	c = &Conversation{
		ID:          convID,
		CharacterID: "",
		Title:       title,
		Channel:     channel,
		Source:      "system",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.repo.CreateConversation(c); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *service) RecalculateMessageCounts() (int64, error) {
	result := s.db.Exec("UPDATE conversations SET message_count = (SELECT COUNT(*) FROM messages WHERE messages.conversation_id = conversations.id)")
	return result.RowsAffected, result.Error
}

func (s *service) BackfillMissingConversations() (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	result := s.db.Exec(`INSERT OR IGNORE INTO conversations (id, title, channel, source, created_at, updated_at)
		SELECT DISTINCT m.conversation_id, m.conversation_id,
		CASE
			WHEN m.conversation_id LIKE '%wechat%' THEN 'wechat'
			WHEN m.conversation_id LIKE '%qq%' THEN 'qq'
			ELSE 'web'
		END,
		'webhook', ?, ?
		FROM messages m
		LEFT JOIN conversations c ON c.id = m.conversation_id
		WHERE c.id IS NULL AND m.conversation_id != ''`, now, now)
	return result.RowsAffected, result.Error
}

func (s *service) DeleteConversation(id string) (bool, error) {
	return s.DeleteConversationForUser(id, requestidentity.DefaultUserID)
}

func (s *service) DeleteConversationForUser(id string, userID string) (bool, error) {
	characterDeleted := false
	err := s.db.Transaction(func(tx *gorm.DB) error {
		linked, err := s.tombstoneConversationTx(tx, id, userID)
		characterDeleted = linked
		return err
	})
	if err != nil {
		return false, err
	}
	if err := pipelinecheckpoint.New(s.db).ResetConversation(id); err != nil {
		return false, err
	}
	return characterDeleted, nil
}

func (s *service) tombstoneConversationTx(tx *gorm.DB, id string, userID string) (bool, error) {
	var convRow struct {
		ID          string
		CharacterID string
		Title       string
		Channel     string
		Source      string
		PeerID      string
		Revision    int64
	}
	if err := tx.Table("conversations").Where("id = ? AND deleted_at IS NULL", id).
		Select("id", "character_id", "title", "channel", "source", "peer_id", "COALESCE(revision, 1) AS revision").Take(&convRow).Error; err != nil {
		return false, err
	}

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
	if err := tx.Table("messages").Where("conversation_id = ? AND deleted_at IS NULL", id).
		Select("id", "conversation_id", "role", "content", "sequence", "msg_type", "source", "COALESCE(revision, 1) AS revision").Scan(&messages).Error; err != nil {
		return false, err
	}
	var attachments []MessageAttachment
	if s.artifactResolver != nil {
		if err := tx.Where("message_id IN (SELECT id FROM messages WHERE conversation_id = ?)", id).Find(&attachments).Error; err != nil {
			return false, err
		}
		if err := s.removeAttachmentReferences(tx, attachments); err != nil {
			return false, err
		}
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	for _, row := range messages {
		if err := tx.Table("messages").Where("id = ? AND revision = ? AND deleted_at IS NULL", row.ID, row.Revision).Updates(map[string]interface{}{
			"deleted_at": now, "updated_at": now, "revision": row.Revision + 1,
		}).Error; err != nil {
			return false, err
		}
		m := &Message{ID: row.ID, ConversationID: row.ConversationID, Role: row.Role, Content: row.Content, Sequence: row.Sequence, MsgType: row.MsgType, Source: row.Source}
		if err := s.recordMessageChangeTx(tx, m, sync.OpDelete, row.Revision+1, userID); err != nil {
			return false, err
		}
	}
	if err := tx.Where("message_id IN (SELECT id FROM messages WHERE conversation_id = ?)", id).Delete(&MessageAttachment{}).Error; err != nil {
		return false, err
	}

	characterChanged := false
	if convRow.CharacterID != "" {
		var characterRow struct {
			ID       string
			Revision int64
		}
		if err := tx.Table("characters").Where("id = ? AND deleted_at IS NULL", convRow.CharacterID).
			Select("id", "COALESCE(revision, 1) AS revision").Take(&characterRow).Error; err == nil {
			newCharacterRevision := characterRow.Revision + 1
			result := tx.Table("characters").Where("id = ? AND revision = ?", characterRow.ID, characterRow.Revision).Updates(map[string]interface{}{
				"conversation_id": "", "updated_at": now, "revision": newCharacterRevision,
			})
			if result.Error != nil {
				return false, result.Error
			}
			if result.RowsAffected == 0 {
				return false, fmt.Errorf("角色版本冲突")
			}
			if s.changeRecorder != nil {
				payload, err := json.Marshal(map[string]interface{}{"id": characterRow.ID, "revision": newCharacterRevision, "meta": map[string]interface{}{"conversation_id": ""}})
				if err != nil {
					return false, err
				}
				if _, err := s.changeRecorder.RecordChange(tx, sync.EntityTypeCharacter, sync.EntityID(characterRow.ID), sync.OpUpdate, newCharacterRevision, newBusinessMutationID(sync.EntityTypeCharacter, characterRow.ID, sync.OpUpdate), normalizeChangeUserID(userID), sync.ScopeDevice, payload); err != nil {
					return false, err
				}
			}
			characterChanged = true
		} else if err != gorm.ErrRecordNotFound {
			return false, err
		}
	}

	newRevision := convRow.Revision + 1
	result := tx.Table("conversations").Where("id = ? AND revision = ? AND deleted_at IS NULL", id, convRow.Revision).Updates(map[string]interface{}{
		"deleted_at": now, "updated_at": now, "revision": newRevision,
	})
	if result.Error != nil {
		return false, result.Error
	}
	if result.RowsAffected == 0 {
		return false, fmt.Errorf("会话版本冲突")
	}
	conversation := &Conversation{ID: convRow.ID, CharacterID: convRow.CharacterID, Title: convRow.Title, Channel: convRow.Channel, Source: convRow.Source, PeerID: convRow.PeerID}
	if err := s.recordConversationChangeTx(tx, conversation, sync.OpDelete, newRevision, userID); err != nil {
		return false, err
	}
	return characterChanged, nil
}

func (s *service) DeleteAllConversations() error {
	return s.DeleteAllConversationsForUser(requestidentity.DefaultUserID)
}

func (s *service) DeleteAllConversationsForUser(userID string) error {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var ids []string
		if err := tx.Table("conversations").Where("deleted_at IS NULL").Pluck("id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := s.tombstoneConversationTx(tx, id, userID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return pipelinecheckpoint.New(s.db).ResetAll()
}

func (s *service) ChangeCharacter(convID, charID string) (*Conversation, error) {
	return s.ChangeCharacterForUser(convID, charID, requestidentity.DefaultUserID)
}

func (s *service) ChangeCharacterForUser(convID, charID, userID string) (*Conversation, error) {
	conv, err := s.repo.GetConversation(convID)
	if err != nil {
		return nil, fmt.Errorf("会话不存在")
	}

	actualCharacterID := strings.TrimSpace(conv.CharacterID)
	if actualCharacterID != "" && actualCharacterID != strings.TrimSpace(charID) {
		msgCount := s.repo.CountMessagesByConv(convID)
		if msgCount > 0 {
			return nil, fmt.Errorf("非空会话禁止更换角色")
		}
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		var currentRevision int64
		if err := tx.Table("conversations").Where("id = ?", convID).Select("COALESCE(revision, 1)").Scan(&currentRevision).Error; err != nil {
			return err
		}
		newRevision := currentRevision + 1
		now := time.Now().Format("2006-01-02 15:04:05")
		if err := tx.Table("conversations").Where("id = ?", convID).Updates(map[string]interface{}{
			"character_id": charID,
			"updated_at":   now,
			"revision":     newRevision,
		}).Error; err != nil {
			return err
		}
		updated := *conv
		updated.CharacterID = charID
		return s.recordConversationChangeTx(tx, &updated, sync.OpUpdate, newRevision, userID)
	})
	if err != nil {
		return nil, err
	}
	return s.repo.GetConversation(convID)
}

func (s *service) GetStats() (*ChatStatsResponse, error) {
	var todayMessages int64
	s.db.Table("messages").Where("date(created_at) = date('now', 'localtime')").Count(&todayMessages)
	var totalConvs int64
	s.db.Table("conversations").Count(&totalConvs)
	return &ChatStatsResponse{TodayMessages: todayMessages, TotalConversations: totalConvs}, nil
}

func (s *service) ExportConversation(convID string, format string) (string, error) {
	conv, err := s.repo.GetConversation(convID)
	if err != nil {
		return "", fmt.Errorf("对话不存在")
	}

	msgs, err := s.repo.GetAllMessagesByConv(convID)
	if err != nil {
		return "", err
	}

	charName := ""
	if conv.CharacterID != "" {
		var c struct {
			Name string `gorm:"column:name"`
		}
		s.db.Table("characters").Where("id = ?", conv.CharacterID).Select("name").Scan(&c)
		charName = c.Name
	}

	dataDir := "data" + "/" + "exports"
	_ = os.MkdirAll(dataDir, 0o700)

	ts := time.Now().Format("20060102_150405")
	safeTitle := strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' || r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r >= 0x4e00 {
			return r
		}
		return '_'
	}, conv.Title)
	if len([]rune(safeTitle)) > 30 {
		safeTitle = string([]rune(safeTitle)[:30])
	}

	var fileName string
	var content []byte
	switch format {
	case "json":
		fileName = fmt.Sprintf("%s_%s.json", safeTitle, ts)
		content, _ = json.MarshalIndent(gin.H{
			"conversation":  conv,
			"characterName": charName,
			"messages":      msgs,
		}, "", "  ")
	default:
		fileName = fmt.Sprintf("%s_%s.md", safeTitle, ts)
		content = buildMarkdownExport(conv, charName, msgs)
	}

	filePath := filepath.Join(dataDir, fileName)
	if err := os.WriteFile(filePath, content, 0o600); err != nil {
		return "", err
	}

	return "/exports/" + fileName, nil
}

func buildMarkdownExport(conv *Conversation, charName string, msgs []Message) []byte {
	var buf strings.Builder
	buf.WriteString("# ")
	buf.WriteString(conv.Title)
	buf.WriteString("\n\n")
	if charName != "" {
		buf.WriteString("**")
		buf.WriteString(charName)
		buf.WriteString("**  \n")
	}
	buf.WriteString("**")
	buf.WriteString(conv.CreatedAt)
	buf.WriteString("**\n\n---\n\n")
	for _, m := range msgs {
		buf.WriteString("**")
		buf.WriteString(m.Role)
		buf.WriteString("**: ")
		buf.WriteString(m.Content)
		buf.WriteString("\n\n")
	}
	return []byte(buf.String())
}
