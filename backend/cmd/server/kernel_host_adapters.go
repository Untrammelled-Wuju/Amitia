package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/u-ai/backend/internal/character"
	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/memory"
	"github.com/u-ai/backend/internal/system"
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
	if scopeCtx.UserID == "" || (strings.TrimSpace(c.UserID) != "" && strings.TrimSpace(c.UserID) != strings.TrimSpace(scopeCtx.UserID)) {
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

func (r *kernelCharacterReader) ListCharacters(_ context.Context, userID string, includeDisabled bool) ([]json.RawMessage, error) {
	characters, err := r.repo.List(includeDisabled)
	if err != nil {
		return nil, err
	}
	userID = strings.TrimSpace(userID)
	items := make([]json.RawMessage, 0, len(characters))
	for _, character := range characters {
		owner := strings.TrimSpace(character.UserID)
		if userID != "" && owner != "" && owner != userID && owner != "default" {
			continue
		}
		data, _ := json.Marshal(map[string]any{
			"id":          character.ID,
			"displayName": character.Name,
			"avatarRef":   character.Avatar,
			"enabled":     character.Status == "enabled",
		})
		items = append(items, data)
	}
	return items, nil
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
	if strings.TrimSpace(scopeCtx.UserID) == "" {
		return []json.RawMessage{}, false, nil
	}
	type scopedConversationReader interface {
		GetMessagesForUser(conversationID, userID string, page, pageSize int) ([]chat.Message, int64, error)
	}
	scopedChat, ok := r.chatSvc.(scopedConversationReader)
	if !ok {
		return nil, false, fmt.Errorf("chat service does not support authenticated ownership")
	}
	messages, total, err := scopedChat.GetMessagesForUser(conversationID, scopeCtx.UserID, page, limit)
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

type conversationMessageSenderAdapter struct {
	appender kernel.ConversationMessageAppender
}

func newConversationMessageSenderAdapter(appender kernel.ConversationMessageAppender) *conversationMessageSenderAdapter {
	return &conversationMessageSenderAdapter{appender: appender}
}

func (a *conversationMessageSenderAdapter) SendConversationMessage(ctx context.Context, request kernel.ConversationMessageRequest) (kernel.ConversationMessageResult, error) {
	if a == nil || a.appender == nil {
		return kernel.ConversationMessageResult{}, fmt.Errorf("conversation message appender is not configured")
	}
	_, err := a.appender.AppendConversationMessages(ctx, kernel.ConversationMessageAppendRequest{
		UserID:         request.UserID,
		CharacterID:    request.CharacterID,
		ConversationID: request.ConversationID,
		Channel:        request.Channel,
		Role:           "assistant",
		Source:         "extension",
		RequestID:      request.RequestID,
		Parts: []kernel.ConversationMessagePart{
			{Type: "text", Content: request.Content},
		},
	})
	if err != nil {
		return kernel.ConversationMessageResult{}, err
	}
	return kernel.ConversationMessageResult{
		Content:   request.Content,
		RequestID: request.RequestID,
	}, nil
}

type conversationMessageAppenderAdapter struct {
	service chat.Service
}

func newConversationMessageAppenderAdapter(service chat.Service) *conversationMessageAppenderAdapter {
	return &conversationMessageAppenderAdapter{service: service}
}

func (a *conversationMessageAppenderAdapter) AppendConversationMessages(ctx context.Context, request kernel.ConversationMessageAppendRequest) (kernel.ConversationMessageAppendResult, error) {
	if a == nil || a.service == nil {
		return kernel.ConversationMessageAppendResult{}, fmt.Errorf("chat service is not configured")
	}
	parts := make([]chat.MessagePart, len(request.Parts))
	for index, part := range request.Parts {
		parts[index] = chat.MessagePart{
			Type:          part.Type,
			Content:       part.Content,
			ExtensionType: part.ExtensionType,
			MIMEType:      part.MIMEType,
			URL:           part.URL,
			FallbackURL:   part.FallbackURL,
			AltText:       part.AltText,
			Width:         part.Width,
			Height:        part.Height,
			IsAnimated:    part.IsAnimated,
			Metadata:      part.Metadata,
		}
	}
	result, err := a.service.AppendConversationMessages(ctx, &chat.AppendConversationMessagesRequest{
		UserID:           request.UserID,
		CharacterID:      request.CharacterID,
		ConversationID:   request.ConversationID,
		Channel:          request.Channel,
		Role:             request.Role,
		Source:           request.Source,
		ReplyToMessageID: request.ReplyToMessageID,
		RequestID:        request.RequestID,
		Parts:            parts,
	})
	if err != nil {
		return kernel.ConversationMessageAppendResult{}, err
	}
	bus := system.GetMessageEventBus()
	direction := "outbound"
	if strings.EqualFold(request.Role, "user") {
		direction = "inbound"
	}
	now := time.Now().Format("2006-01-02 15:04:05")
	for index, messageID := range result.MessageIDs {
		if index >= len(parts) {
			break
		}
		part := parts[index]
		sequence := result.LastSequence
		if index < len(result.Sequences) {
			sequence = result.Sequences[index]
		}
		bus.PublishMessageCreated(
			request.ConversationID,
			messageID,
			request.Channel,
			direction,
			request.Role,
			part.Content,
			now,
			sequence,
			map[string]any{
				"type":          part.Type,
				"extensionType": part.ExtensionType,
				"url":           part.URL,
				"fallbackUrl":   part.FallbackURL,
				"altText":       part.AltText,
				"width":         part.Width,
				"height":        part.Height,
				"isAnimated":    part.IsAnimated,
			},
		)
	}
	return kernel.ConversationMessageAppendResult{
		MessageIDs:      result.MessageIDs,
		Sequences:       result.Sequences,
		ResponseGroupID: result.ResponseGroupID,
		LastSequence:    result.LastSequence,
	}, nil
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
	if strings.TrimSpace(scopeCtx.UserID) == "" {
		return []json.RawMessage{}, nil
	}
	type scopedMemoryReader interface {
		SearchForUser(req *memory.SearchMemoryRequest, userID string) ([]memory.Memory, error)
	}
	scopedMemory, ok := s.memSvc.(scopedMemoryReader)
	if !ok {
		return nil, fmt.Errorf("memory service does not support authenticated ownership")
	}
	memories, err := scopedMemory.SearchForUser(req, scopeCtx.UserID)
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
