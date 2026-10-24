package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/system"
)

const (
	maxCharacterSummaryLength = 500
	maxMessageContentLength   = 200
	maxMemoryValueLength      = 300
)

type kernelCharacterReader struct {
	engine  *business.Engine
	catalog coordination.RoleCatalogPort
	coreID  string
}

func newKernelCharacterReader() *kernelCharacterReader { return &kernelCharacterReader{} }
func (r *kernelCharacterReader) roleCatalog(ctx context.Context, includeDisabled bool) ([]coordination.Role, error) {
	host := kernel.GetHostAPIScope(ctx)
	request, err := ownedHostChatRequest(ctx, r.engine, r.coreID, host.ConversationID, host.CharacterID, uuid.NewString())
	if err != nil {
		return nil, err
	}
	if r.catalog == nil {
		return nil, fmt.Errorf("Owner role catalog is not configured")
	}
	var result []coordination.Role
	err = r.engine.WithExecutionScope(ctx, *request.ExpectedScope, func(current context.Context) error {
		var err error
		result, err = r.catalog.ListOwnedRoleCatalog(current, *request.ExpectedScope, includeDisabled)
		return err
	})
	return result, err
}
func (r *kernelCharacterReader) ReadCharacter(ctx context.Context, characterID string) (json.RawMessage, bool, error) {
	host := kernel.GetHostAPIScope(ctx)
	if characterID == "" {
		characterID = host.CharacterID
	}
	if characterID == "" || characterID != host.CharacterID {
		return nil, false, nil
	}
	roles, err := r.roleCatalog(ctx, false)
	if err != nil {
		return nil, false, err
	}
	for _, role := range roles {
		if role.ID == characterID {
			summary := role.Description
			if utf8.RuneCountInString(summary) > maxCharacterSummaryLength {
				summary = truncateRunes(summary, maxCharacterSummaryLength)
			}
			raw, err := json.Marshal(map[string]any{"id": role.ID, "displayName": role.Name, "avatarRef": role.Avatar, "summary": summary})
			return raw, err == nil, err
		}
	}
	return nil, false, nil
}
func (r *kernelCharacterReader) ListCharacters(ctx context.Context, spaceID string, includeDisabled bool) ([]json.RawMessage, error) {
	host := kernel.GetHostAPIScope(ctx)
	if spaceID != "" && spaceID != host.SpaceID {
		return nil, coordination.ErrWrongOwner
	}
	roles, err := r.roleCatalog(ctx, includeDisabled)
	if err != nil {
		return nil, err
	}
	items := make([]json.RawMessage, 0, len(roles))
	for _, role := range roles {
		raw, err := json.Marshal(map[string]any{"id": role.ID, "displayName": role.Name, "avatarRef": role.Avatar, "enabled": !role.Disabled})
		if err != nil {
			return nil, err
		}
		items = append(items, raw)
	}
	return items, nil
}

type kernelConversationReader struct {
	engine *business.Engine
	coreID string
}

func newKernelConversationReader() *kernelConversationReader { return &kernelConversationReader{} }
func ownedHostChatRequest(ctx context.Context, engine *business.Engine, coreID, conversationID, roleID, requestID string) (business.Request, error) {
	if engine == nil {
		return business.Request{}, fmt.Errorf("Owner chat service is not configured")
	}
	host := kernel.GetHostAPIScope(ctx)
	if host.SpaceID == "" || host.CharacterID == "" || roleID != "" && roleID != host.CharacterID || host.ConversationID != "" && host.ConversationID != conversationID {
		return business.Request{}, coordination.ErrWrongOwner
	}
	if requestID == "" {
		requestID = uuid.NewString()
	}
	request := business.Request{SpaceID: host.SpaceID, CoreID: coreID, RoleID: host.CharacterID, ConversationID: conversationID, RequestID: requestID}
	if scope, ok := coordination.FromContext(ctx); ok {
		if scope.SpaceID != host.SpaceID || scope.RoleID != host.CharacterID {
			return business.Request{}, coordination.ErrWrongOwner
		}
		request.DeviceID, request.TargetDeviceID, request.CoreID = scope.InitiatorDeviceID, scope.TargetDeviceID, scope.CoreID
		scope.RequestID = requestID
		scope.TurnID, scope.ExecutionID = "", ""
		request.ExpectedScope = &scope
		return request, nil
	}
	actor, err := auth.RequireActor(ctx)
	if err != nil {
		return business.Request{}, err
	}
	if actor.SpaceID.String() != host.SpaceID || actor.DeviceID == "" {
		return business.Request{}, coordination.ErrWrongOwner
	}
	request.DeviceID, request.TargetDeviceID = actor.DeviceID.String(), actor.DeviceID.String()
	view, err := engine.Query(ctx, request, coordination.DataQuery{ConversationID: conversationID, Management: true, ResourceKind: "conversation", Limit: 1})
	if err != nil {
		return business.Request{}, err
	}
	scope := view.Scope
	scope.RequestID = requestID
	scope.TurnID, scope.ExecutionID = "", ""
	request.ExpectedScope = &scope
	return request, nil
}
func (r *kernelConversationReader) ReadConversation(ctx context.Context, conversationID string, limit, offset int) ([]json.RawMessage, bool, error) {
	host := kernel.GetHostAPIScope(ctx)
	if conversationID == "" {
		conversationID = host.ConversationID
	}
	if conversationID == "" {
		return []json.RawMessage{}, false, nil
	}
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 || offset > 1000000 {
		return nil, false, coordination.ErrPendingLimit
	}
	request, err := ownedHostChatRequest(ctx, r.engine, r.coreID, conversationID, host.CharacterID, uuid.NewString())
	if err != nil {
		return nil, false, err
	}
	page := offset/limit + 1
	skip := offset % limit
	items := []json.RawMessage{}
	total := 0
	for len(items) < limit {
		result, err := r.engine.ManageChat(ctx, request, coordination.ChatManagementRequest{Operation: "search", ConversationOwnerID: request.ExpectedScope.ResourceOwnerID, ConversationID: conversationID, Page: page, PageSize: limit})
		if err != nil {
			return nil, false, err
		}
		raw, err := json.Marshal(result)
		if err != nil {
			return nil, false, err
		}
		var batch struct {
			Items []struct {
				ConversationID string `json:"conversationId"`
				Role           string `json:"role"`
				CreatedAt      string `json:"createdAt"`
				Content        string `json:"content"`
			} `json:"items"`
			Total int `json:"total"`
		}
		if err = json.Unmarshal(raw, &batch); err != nil {
			return nil, false, err
		}
		total = batch.Total
		for i, msg := range batch.Items {
			if i < skip {
				continue
			}
			content := msg.Content
			if utf8.RuneCountInString(content) > maxMessageContentLength {
				content = truncateRunes(content, maxMessageContentLength)
			}
			entry, err := json.Marshal(map[string]any{"conversationId": msg.ConversationID, "role": msg.Role, "timestamp": msg.CreatedAt, "contentSummary": content})
			if err != nil {
				return nil, false, err
			}
			items = append(items, entry)
			if len(items) == limit {
				break
			}
		}
		if len(batch.Items) < limit || page*limit >= total {
			break
		}
		page++
		skip = 0
	}
	return items, offset+len(items) < total, nil
}

type kernelMemoryQueryService struct {
	engine *business.Engine
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
		SpaceID:        request.SpaceID,
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
	engine *business.Engine
	coreID string
}

func newConversationMessageAppenderAdapter() *conversationMessageAppenderAdapter {
	return &conversationMessageAppenderAdapter{}
}

func (a *conversationMessageAppenderAdapter) AppendConversationMessages(ctx context.Context, request kernel.ConversationMessageAppendRequest) (kernel.ConversationMessageAppendResult, error) {
	if a == nil || a.engine == nil {
		return kernel.ConversationMessageAppendResult{}, fmt.Errorf("chat service is not configured")
	}
	parts := make([]business.AppendedMessagePart, len(request.Parts))
	for index, part := range request.Parts {
		parts[index] = business.AppendedMessagePart{
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
	if request.SpaceID != kernel.GetHostAPIScope(ctx).SpaceID {
		return kernel.ConversationMessageAppendResult{}, coordination.ErrWrongOwner
	}
	if request.RequestID == "" {
		request.RequestID = uuid.NewString()
	}
	ownedRequest, err := ownedHostChatRequest(ctx, a.engine, a.coreID, request.ConversationID, request.CharacterID, request.RequestID)
	if err != nil {
		return kernel.ConversationMessageAppendResult{}, err
	}
	if strings.EqualFold(strings.TrimSpace(request.Role), "user") {
		for i, part := range parts {
			media := &interaction.UnifiedEntryRequest{}
			switch strings.ToLower(strings.TrimSpace(part.Type)) {
			case "image":
				media.ImageUrl = part.URL
			case "audio":
				media.AudioUrl = part.URL
			case "video":
				media.VideoUrl = part.URL
			default:
				continue
			}
			parts[i].Attachments, err = ownedIngressAttachments(ctx, media)
			if err != nil {
				return kernel.ConversationMessageAppendResult{}, err
			}
		}
	}
	result, err := a.engine.AppendOwnedMessages(ctx, ownedRequest, business.AppendOwnedMessagesRequest{ConversationID: request.ConversationID, Role: request.Role, Source: request.Source, Channel: request.Channel, ReplyToMessageID: request.ReplyToMessageID, Parts: parts})
	if err != nil {
		return kernel.ConversationMessageAppendResult{}, err
	}
	bus := system.GetMessageEventBus()
	direction := "outbound"
	if result.Role == "user" {
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
			result.Channel,
			direction,
			result.Role,
			result.Status,
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
		DeliveryGroupID: result.DeliveryGroupID,
		LastSequence:    result.LastSequence,
	}, nil
}

func newKernelMemoryQueryService() *kernelMemoryQueryService {
	return &kernelMemoryQueryService{}
}

func (s *kernelMemoryQueryService) Query(ctx context.Context, extensionID string, query string, limit int) ([]json.RawMessage, error) {
	scope, owned := coordination.FromContext(ctx)
	host := kernel.GetHostAPIScope(ctx)
	if !owned || s.engine == nil || scope.SpaceID != host.SpaceID || scope.RoleID != host.CharacterID {
		return nil, fmt.Errorf("记忆查询缺少当前数据归属")
	}
	if strings.TrimSpace(query) == "" {
		return []json.RawMessage{}, nil
	}
	if limit < 1 || limit > 50 {
		limit = 10
	}
	result, err := s.engine.SearchOwnedMemories(ctx, business.Request{SpaceID: scope.SpaceID, DeviceID: scope.InitiatorDeviceID, TargetDeviceID: scope.TargetDeviceID, CoreID: scope.CoreID, RoleID: scope.RoleID, RequestID: scope.RequestID + "|host-memory"}, business.MemoryManagementQuery{Query: query, Mode: "keyword", Limit: limit, ContextUsable: true})
	if err != nil {
		return nil, err
	}
	results := []json.RawMessage{}
	for _, row := range result.Resources {
		if !coordination.ResourceUsable(row.Body, time.Now()) {
			continue
		}
		var document struct {
			Key     string `json:"key"`
			Content struct {
				Value       string `json:"value"`
				Text        string `json:"text"`
				MemoryType  string `json:"memoryType"`
				Importance  int    `json:"importance"`
				Sensitivity string `json:"sensitivityLevel"`
			} `json:"content"`
		}
		if json.Unmarshal(row.Body, &document) != nil {
			return nil, fmt.Errorf("记忆记录格式无效")
		}
		text := document.Content.Value
		if text == "" {
			text = document.Content.Text
		}
		if document.Content.Sensitivity == "confidential" || document.Content.Sensitivity == "restricted" {
			text = "[restricted]"
		} else if utf8.RuneCountInString(text) > maxMemoryValueLength {
			text = truncateRunes(text, maxMemoryValueLength)
		}
		entry, err := json.Marshal(map[string]any{"id": row.ID, "key": document.Key, "value": text, "memoryType": document.Content.MemoryType, "importance": document.Content.Importance})
		if err != nil {
			return nil, err
		}
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
