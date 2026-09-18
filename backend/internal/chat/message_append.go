package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	syncapi "github.com/u-ai/backend/internal/sync"
	"gorm.io/gorm"
)

const MaxConversationMessageParts = 16

type AppendConversationMessagesRequest struct {
	SpaceID          string
	CharacterID      string
	ConversationID   string
	Channel          string
	Role             string
	Source           string
	ReplyToMessageID *string
	RequestID        string
	Parts            []MessagePart
}

type AppendConversationMessagesResult struct {
	MessageIDs      []string `json:"messageIds"`
	Sequences       []int64  `json:"sequences"`
	ResponseGroupID string   `json:"responseGroupId"`
	LastSequence    int64    `json:"lastSequence"`
}

func (s *service) AppendConversationMessages(ctx context.Context, request *AppendConversationMessagesRequest) (*AppendConversationMessagesResult, error) {
	if request == nil {
		return nil, fmt.Errorf("append message request is required")
	}
	request.SpaceID = strings.TrimSpace(request.SpaceID)
	request.CharacterID = strings.TrimSpace(request.CharacterID)
	request.ConversationID = strings.TrimSpace(request.ConversationID)
	request.Channel = strings.TrimSpace(request.Channel)
	request.Role = strings.ToLower(strings.TrimSpace(request.Role))
	request.Source = strings.TrimSpace(request.Source)
	request.RequestID = strings.TrimSpace(request.RequestID)
	if request.ConversationID == "" || request.CharacterID == "" || request.SpaceID == "" {
		return nil, fmt.Errorf("conversationId, characterId and spaceId are required")
	}
	if request.Role == "" {
		request.Role = "assistant"
	}
	switch request.Role {
	case "user", "assistant":
	default:
		return nil, fmt.Errorf("role must be user or assistant")
	}
	if len(request.Parts) == 0 || len(request.Parts) > MaxConversationMessageParts {
		return nil, fmt.Errorf("parts must contain between 1 and %d items", MaxConversationMessageParts)
	}
	parts := make([]MessagePart, len(request.Parts))
	for index, part := range request.Parts {
		normalized, err := normalizeAppendMessagePart(index, part)
		if err != nil {
			return nil, err
		}
		parts[index] = normalized
	}

	var conversation Conversation
	if err := s.db.WithContext(ctx).Where("id = ?", request.ConversationID).First(&conversation).Error; err != nil {
		return nil, err
	}
	if !conversationOwnerMatches(conversation.SpaceID, request.SpaceID) {
		return nil, gorm.ErrRecordNotFound
	}
	if conversation.CharacterID != request.CharacterID {
		return nil, ErrConversationScopeMismatch
	}
	if request.Channel == "" {
		request.Channel = conversation.Channel
	}
	if request.Source == "" {
		request.Source = "extension"
	}
	responseGroupID := request.RequestID
	if responseGroupID == "" {
		responseGroupID = uuid.New().String()
	}
	messageIDs := make([]string, 0, len(parts))
	sequences := make([]int64, 0, len(parts))
	deliveryPayloads := make([][]byte, 0, len(parts))
	var lastSequence int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for index, part := range parts {
			message := buildAppendMessage(request, responseGroupID, index+1, part)
			if err := tx.Create(message).Error; err != nil {
				return err
			}
			if err := s.recordMessageChangeTx(tx, message, syncapi.OpCreate, 1, request.SpaceID); err != nil {
				return err
			}
			messageIDs = append(messageIDs, message.ID)
			sequences = append(sequences, message.Sequence)
			lastSequence = message.Sequence
			if !strings.EqualFold(request.Channel, "web") {
				payload, err := json.Marshal(map[string]any{
					"messageId":        message.ID,
					"conversationId":   request.ConversationID,
					"characterId":      request.CharacterID,
					"responseGroupId":  responseGroupID,
					"deliverySequence": index + 1,
					"content":          message.Content,
					"mimeType":         part.MIMEType,
					"altText":          part.AltText,
					"originalPath":     part.URL,
					"fallbackPath":     part.FallbackURL,
					"isAnimated":       part.IsAnimated,
				})
				if err != nil {
					return err
				}
				deliveryPayloads = append(deliveryPayloads, payload)
			}
		}
		now := formatChatTime()
		if err := tx.Exec("UPDATE conversations SET updated_at = ?, message_count = (SELECT COUNT(*) FROM messages WHERE conversation_id = ?) WHERE id = ?", now, request.ConversationID, request.ConversationID).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if s.deliveryStore != nil && len(deliveryPayloads) > 0 {
		peerID := conversation.PeerID
		for index, payload := range deliveryPayloads {
			if err := s.deliveryStore.CreateDeliveryIntent(responseGroupID, request.Channel, peerID, parts[index].Type, payload); err != nil {
				return nil, err
			}
		}
	}
	return &AppendConversationMessagesResult{
		MessageIDs:      messageIDs,
		Sequences:       sequences,
		ResponseGroupID: responseGroupID,
		LastSequence:    lastSequence,
	}, nil
}

func normalizeAppendMessagePart(index int, part MessagePart) (MessagePart, error) {
	part.Type = strings.ToLower(strings.TrimSpace(part.Type))
	part.Content = strings.TrimSpace(part.Content)
	part.ExtensionType = strings.TrimSpace(part.ExtensionType)
	part.MIMEType = strings.TrimSpace(part.MIMEType)
	part.URL = strings.TrimSpace(part.URL)
	part.FallbackURL = strings.TrimSpace(part.FallbackURL)
	part.AltText = strings.TrimSpace(part.AltText)
	if !validMessagePartType(part.Type) {
		return MessagePart{}, fmt.Errorf("part %d has invalid type %q", index, part.Type)
	}
	if part.Type == "text" && part.Content == "" {
		return MessagePart{}, fmt.Errorf("part %d text content is required", index)
	}
	if part.Type != "text" && part.URL == "" {
		return MessagePart{}, fmt.Errorf("part %d %s url is required", index, part.Type)
	}
	return part, nil
}

func buildAppendMessage(request *AppendConversationMessagesRequest, responseGroupID string, sequence int, part MessagePart) *Message {
	content := part.Content
	if content == "" {
		content = part.AltText
	}
	if content == "" {
		switch part.Type {
		case "image":
			content = "[图片]"
		case "audio":
			content = "[音频]"
		case "video":
			content = "[视频]"
		case "file":
			content = "[文件]"
		}
	}
	status := "sent"
	if !strings.EqualFold(request.Channel, "web") {
		status = "sending"
	}
	message := &Message{
		ID:               uuid.New().String(),
		ConversationID:   request.ConversationID,
		Role:             request.Role,
		Content:          content,
		MsgType:          part.Type,
		ExtensionType:    part.ExtensionType,
		Source:           request.Source,
		Status:           status,
		AltText:          part.AltText,
		IsAnimated:       boolInt(part.IsAnimated),
		MediaWidth:       part.Width,
		MediaHeight:      part.Height,
		OriginalAsset:    part.URL,
		FallbackAsset:    part.FallbackURL,
		ResponseGroupID:  responseGroupID,
		DeliverySequence: sequence,
		RequestID:        request.RequestID,
		ReplyToMessageID: request.ReplyToMessageID,
	}
	switch part.Type {
	case "image":
		message.ImageUrl = part.URL
	case "audio":
		message.AudioUrl = part.URL
	case "video":
		message.VideoUrl = part.URL
	}
	return message
}

func formatChatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
