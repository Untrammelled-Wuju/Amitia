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

func (s *service) ListConversationsForSpace(q ConversationQuery, spaceID string) (*ConversationListResponse, error) {
	q.SpaceID = normalizeConversationOwner(spaceID)
	q.IncludeLegacyDefault = chatLocalSingleUserMode()
	return s.ListConversations(q)
}

func (s *service) GetConversation(id string) (*Conversation, error) {
	c, err := s.repo.GetConversation(id)
	if err != nil {
		return nil, fmt.Errorf("对话不存在")
	}
	return c, nil
}

func (s *service) GetConversationForSpace(id, spaceID string) (*Conversation, error) {
	c, err := s.requireConversationOwner(id, spaceID)
	if err != nil {
		return nil, fmt.Errorf("对话不存在")
	}
	return c, nil
}

func (s *service) CreateConversation(req *CreateConversationRequest) (*Conversation, error) {
	return s.CreateConversationForSpace(req, requestidentity.CanonicalSpaceID())
}

func (s *service) CreateConversationForSpace(req *CreateConversationRequest, spaceID string) (*Conversation, error) {
	if req == nil {
		return nil, fmt.Errorf("conversation request is required")
	}
	projectID := strings.TrimSpace(req.ProjectID)
	if existing, err := s.findReusableEmptyConversationForSpace(projectID, req.Channel, req.Source, spaceID); err != nil {
		return nil, err
	} else if existing != nil {
		return existing, nil
	}
	if projectID != "" {
		if _, err := s.requireProjectForSpace(projectID, spaceID); err != nil {
			return nil, err
		}
	}
	if req.Title == "" {
		req.Title = "新对话"
	}
	if req.Channel == "" {
		req.Channel = "web"
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	owner := normalizeConversationOwner(spaceID)
	c := &Conversation{ID: uuid.New().String(), SpaceID: owner, ProjectID: projectID, Title: req.Title, Channel: req.Channel, Source: req.Source, PeerID: req.PeerID}

	err := s.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now().Format("2006-01-02 15:04:05")
		if err := tx.Exec("INSERT INTO conversations (id, space_id, project_id, title, channel, source, peer_id, created_at, updated_at, revision) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 1)",
			c.ID, c.SpaceID, c.ProjectID, c.Title, c.Channel, c.Source, c.PeerID, now, now).Error; err != nil {
			return err
		}
		if err := s.recordConversationChangeTx(tx, c, sync.OpCreate, 1, spaceID); err != nil {
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
	return s.EnsureChannelConversationForSpace(channel, requestidentity.CanonicalSpaceID())
}

func (s *service) EnsureChannelConversationForSpace(channel, spaceID string) (*Conversation, error) {
	owner := normalizeConversationOwner(spaceID)
	title := channel + "对话"

	var c Conversation
	query := s.db.Where("channel = ? AND deleted_at IS NULL", channel)
	query = applyConversationOwnerScope(query, spaceID)
	if err := query.Order("CASE WHEN source = 'system' THEN 0 ELSE 1 END, updated_at DESC").First(&c).Error; err == nil {
		if err := s.db.Model(&Conversation{}).Where("id = ?", c.ID).Updates(map[string]interface{}{
			"space_id": owner, "channel": channel, "title": title, "source": "system",
		}).Error; err != nil {
			return nil, err
		}
		c.SpaceID = owner
		c.Channel = channel
		c.Title = title
		c.Source = "system"
		return &c, nil
	} else if err != gorm.ErrRecordNotFound {
		return nil, err
	}

	now := time.Now().Format("2006-01-02 15:04:05")
	c = Conversation{
		ID:        uuid.New().String(),
		SpaceID:   owner,
		Title:     title,
		Channel:   channel,
		Source:    "system",
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.persistConversationWithChange(&c, owner); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *service) RecalculateMessageCounts() (int64, error) {
	result := s.db.Exec("UPDATE conversations SET message_count = (SELECT COUNT(*) FROM messages WHERE messages.conversation_id = conversations.id)")
	return result.RowsAffected, result.Error
}

func (s *service) BackfillMissingConversations() (int64, error) {
	now := time.Now().Format("2006-01-02 15:04:05")
	result := s.db.Exec(`INSERT OR IGNORE INTO conversations (id, space_id, title, channel, source, created_at, updated_at)
		SELECT DISTINCT m.conversation_id, 'default', m.conversation_id, 'web',
		'webhook', ?, ?
		FROM messages m
		LEFT JOIN conversations c ON c.id = m.conversation_id
		WHERE c.id IS NULL AND m.conversation_id != ''`, now, now)
	return result.RowsAffected, result.Error
}

func (s *service) DeleteConversation(id string) (bool, error) {
	return s.DeleteConversationForSpace(id, requestidentity.CanonicalSpaceID())
}

func (s *service) DeleteConversationForSpace(id string, spaceID string) (bool, error) {
	err := s.db.Transaction(func(tx *gorm.DB) error {
		_, err := s.tombstoneConversationTx(tx, id, spaceID)
		return err
	})
	if err != nil {
		return false, err
	}
	if err := pipelinecheckpoint.New(s.db).ResetConversation(id); err != nil {
		return false, err
	}
	return false, nil
}

func (s *service) tombstoneConversationTx(tx *gorm.DB, id string, spaceID string) (bool, error) {
	var convRow struct {
		ID        string
		SpaceID   string
		ProjectID string
		Title     string
		Channel   string
		Source    string
		PeerID    string
		Revision  int64
	}
	if err := tx.Table("conversations").Where("id = ? AND deleted_at IS NULL", id).
		Select("id", "space_id", "project_id", "title", "channel", "source", "peer_id", "COALESCE(revision, 1) AS revision").Take(&convRow).Error; err != nil {
		return false, err
	}
	if !conversationOwnerMatches(convRow.SpaceID, spaceID) {
		return false, gorm.ErrRecordNotFound
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
		if err := s.recordMessageChangeTx(tx, m, sync.OpDelete, row.Revision+1, spaceID); err != nil {
			return false, err
		}
	}
	if err := tx.Where("message_id IN (SELECT id FROM messages WHERE conversation_id = ?)", id).Delete(&MessageAttachment{}).Error; err != nil {
		return false, err
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
	conversation := &Conversation{ID: convRow.ID, SpaceID: convRow.SpaceID, ProjectID: convRow.ProjectID, Title: convRow.Title, Channel: convRow.Channel, Source: convRow.Source, PeerID: convRow.PeerID}
	if err := s.recordConversationChangeTx(tx, conversation, sync.OpDelete, newRevision, spaceID); err != nil {
		return false, err
	}
	return false, nil
}

func (s *service) DeleteAllConversations() error {
	return s.DeleteAllConversationsForSpace(requestidentity.CanonicalSpaceID())
}

func (s *service) DeleteAllConversationsForSpace(spaceID string) error {
	var ids []string
	err := s.db.Transaction(func(tx *gorm.DB) error {
		query := tx.Table("conversations").Where("deleted_at IS NULL")
		query = applyConversationOwnerScope(query, spaceID)
		if err := query.Pluck("id", &ids).Error; err != nil {
			return err
		}
		for _, id := range ids {
			if _, err := s.tombstoneConversationTx(tx, id, spaceID); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	checkpoint := pipelinecheckpoint.New(s.db)
	for _, id := range ids {
		if err := checkpoint.ResetConversation(id); err != nil {
			return err
		}
	}
	return nil
}

func (s *service) GetStats() (*ChatStatsResponse, error) {
	var todayMessages int64
	s.db.Table("messages").Where("date(created_at) = date('now', 'localtime')").Count(&todayMessages)
	var totalConvs int64
	s.db.Table("conversations").Count(&totalConvs)
	return &ChatStatsResponse{TodayMessages: todayMessages, TotalConversations: totalConvs}, nil
}

func (s *service) GetStatsForSpace(spaceID string) (*ChatStatsResponse, error) {
	convQuery := s.db.Table("conversations").Where("deleted_at IS NULL")
	convQuery = applyConversationOwnerScope(convQuery, spaceID)
	var totalConvs int64
	if err := convQuery.Count(&totalConvs).Error; err != nil {
		return nil, err
	}
	msgQuery := s.db.Table("messages AS m").Joins("JOIN conversations AS c ON c.id = m.conversation_id").
		Where("m.deleted_at IS NULL AND c.deleted_at IS NULL AND date(m.created_at) = date('now', 'localtime')")
	owner := normalizeConversationOwner(spaceID)
	if chatLocalSingleUserMode() {
		msgQuery = msgQuery.Where("c.space_id = ? OR c.space_id = '' OR c.space_id IS NULL OR c.space_id = ?", owner, requestidentity.LegacySpaceID)
	} else {
		msgQuery = msgQuery.Where("c.space_id = ?", owner)
	}
	var todayMessages int64
	if err := msgQuery.Count(&todayMessages).Error; err != nil {
		return nil, err
	}
	return &ChatStatsResponse{TodayMessages: todayMessages, TotalConversations: totalConvs}, nil
}

func (s *service) ExportConversationForSpace(convID, format, spaceID string) (string, error) {
	if _, err := s.requireConversationOwner(convID, spaceID); err != nil {
		return "", fmt.Errorf("对话不存在")
	}
	return s.ExportConversation(convID, format)
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
