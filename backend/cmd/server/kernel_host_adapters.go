package main

import (
	"context"
	"encoding/json"
	"strings"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/memory"
)

const (
	maxCharacterSummaryLength = 500
	maxMessageContentLength   = 200
	maxMemoryValueLength      = 300
)

type kernelCharacterReader struct {
	repo character.Repository
}

func newKernelCharacterReader(repo character.Repository) *kernelCharacterReader {
	return &kernelCharacterReader{repo: repo}
}

func (r *kernelCharacterReader) ReadCharacter(ctx context.Context, characterID string) (json.RawMessage, bool, error) {
	scopeCtx := kernel.GetHostAPIScope(ctx)
	if scopeCtx.CharacterID != "" && characterID != "" && scopeCtx.CharacterID != characterID {
		return nil, false, nil
	}
	if characterID == "" {
		characterID = scopeCtx.CharacterID
	}
	var c *character.Character
	var err error
	if characterID != "" {
		c, err = r.repo.FindByID(characterID)
	} else {
		c, err = r.repo.GetActive()
	}
	if err != nil || c == nil {
		return nil, false, nil
	}
	summary := c.Description
	if utf8.RuneCountInString(summary) > maxCharacterSummaryLength {
		summary = truncateRunes(summary, maxCharacterSummaryLength)
	}
	data, _ := json.Marshal(map[string]any{
		"id":          c.ID,
		"displayName": c.Name,
		"avatarRef":   c.Avatar,
		"summary":     summary,
	})
	return data, true, nil
}

type kernelConversationReader struct {
	chatSvc chat.Service
}

func newKernelConversationReader(chatSvc chat.Service) *kernelConversationReader {
	return &kernelConversationReader{chatSvc: chatSvc}
}

func (r *kernelConversationReader) ReadConversation(ctx context.Context, conversationID string, limit int, offset int) ([]json.RawMessage, bool, error) {
	scopeCtx := kernel.GetHostAPIScope(ctx)
	if scopeCtx.ConversationID != "" && conversationID != "" && scopeCtx.ConversationID != conversationID {
		return []json.RawMessage{}, false, nil
	}
	if conversationID == "" {
		conversationID = scopeCtx.ConversationID
	}
	if conversationID == "" {
		return []json.RawMessage{}, false, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	page := 1
	if offset > 0 && limit > 0 {
		page = offset/limit + 1
	}
	messages, total, err := r.chatSvc.GetMessages(conversationID, page, limit)
	if err != nil {
		return nil, false, err
	}
	results := make([]json.RawMessage, 0, len(messages))
	for _, msg := range messages {
		content := msg.Content
		if utf8.RuneCountInString(content) > maxMessageContentLength {
			content = truncateRunes(content, maxMessageContentLength)
		}
		entry, _ := json.Marshal(map[string]any{
			"conversationId": msg.ConversationID,
			"role":           msg.Role,
			"timestamp":      msg.CreatedAt,
			"contentSummary": content,
		})
		results = append(results, entry)
	}
	hasMore := int64(offset+limit) < total
	return results, hasMore, nil
}

type kernelMemoryQueryService struct {
	memSvc memory.Service
}

func newKernelMemoryQueryService(memSvc memory.Service) *kernelMemoryQueryService {
	return &kernelMemoryQueryService{memSvc: memSvc}
}

func (s *kernelMemoryQueryService) Query(ctx context.Context, extensionID string, query string, limit int) ([]json.RawMessage, error) {
	if strings.TrimSpace(query) == "" {
		return []json.RawMessage{}, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	scopeCtx := kernel.GetHostAPIScope(ctx)
	req := &memory.SearchMemoryRequest{
		Keyword:     query,
		CharacterID: scopeCtx.CharacterID,
		Limit:       limit,
	}
	memories, err := s.memSvc.Search(req)
	if err != nil {
		return nil, err
	}
	results := make([]json.RawMessage, 0, len(memories))
	for _, m := range memories {
		value := m.Value
		if m.SensitivityLevel == "confidential" || m.SensitivityLevel == "restricted" {
			value = "[restricted]"
		} else if utf8.RuneCountInString(value) > maxMemoryValueLength {
			value = truncateRunes(value, maxMemoryValueLength)
		}
		entry, _ := json.Marshal(map[string]any{
			"id":         m.ID,
			"memoryType": m.MemoryType,
			"key":        m.Key,
			"value":      value,
			"importance": m.Importance,
			"createdAt":  m.CreatedAt,
		})
		results = append(results, entry)
	}
	return results, nil
}

func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
