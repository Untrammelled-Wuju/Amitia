// SPDX-FileCopyrightText: 2026 彭旭
// SPDX-License-Identifier: AGPL-3.0-only
package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/u-ai/backend/internal/pipelinecheckpoint"
	"time"
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
	if req.Title == "" {
		req.Title = "New Chat"
	}
	if req.Channel == "" {
		req.Channel = "web"
	}
	if req.Source == "" {
		req.Source = "manual"
	}
	c := &Conversation{CharacterID: req.CharacterID, Title: req.Title, Channel: req.Channel, Source: req.Source, PeerID: req.PeerID}
	if err := s.repo.CreateConversation(c); err != nil {
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

func (s *service) DeleteConversation(id string) error {
	if err := s.repo.DeleteConversation(id); err != nil {
		return err
	}
	return pipelinecheckpoint.New(s.db).ResetConversation(id)
}

func (s *service) DeleteAllConversations() error {
	if err := s.repo.DeleteAllConversations(); err != nil {
		return err
	}
	return pipelinecheckpoint.New(s.db).ResetAll()
}

func (s *service) ChangeCharacter(convID, charID string) (*Conversation, error) {
	s.db.Exec("UPDATE conversations SET character_id = ?, updated_at = ? WHERE id = ?", charID, time.Now().Format("2006-01-02 15:04:05"), convID)
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
	_ = os.MkdirAll(dataDir, 0755)

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
			"conversation": conv,
			"characterName": charName,
			"messages":      msgs,
		}, "", "  ")
	default:
		fileName = fmt.Sprintf("%s_%s.md", safeTitle, ts)
		content = buildMarkdownExport(conv, charName, msgs)
	}

	filePath := filepath.Join(dataDir, fileName)
	if err := os.WriteFile(filePath, content, 0644); err != nil {
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
