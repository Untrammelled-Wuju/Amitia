package chat

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/google/uuid"
)

const MaxMessageOutputsPerReply = 8

type MessageOutputPlanningEvent struct {
	ConversationID string
	CharacterID    string
	Channel        string
	Source         string
	UserMessage    string
	Reply          string
	SpaceID        string
	PeerID         string
	RequestID      string
	ForceVoice     bool
}

type MessagePart struct {
	Type          string         `json:"type"`
	Content       string         `json:"content,omitempty"`
	ExtensionType string         `json:"extensionType,omitempty"`
	MIMEType      string         `json:"mimeType,omitempty"`
	URL           string         `json:"url,omitempty"`
	FallbackURL   string         `json:"fallbackUrl,omitempty"`
	AltText       string         `json:"altText,omitempty"`
	Width         int            `json:"width,omitempty"`
	Height        int            `json:"height,omitempty"`
	IsAnimated    bool           `json:"isAnimated,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type MessageOutput struct {
	OutputID    string      `json:"outputId,omitempty"`
	ExtensionID string      `json:"extensionId,omitempty"`
	Placement   string      `json:"placement,omitempty"`
	Part        MessagePart `json:"part"`
}

type MessageOutputPlanner interface {
	PlanMessageOutputs(ctx context.Context, scope SkillScope, event *MessageOutputPlanningEvent) ([]MessageOutput, error)
}

func (s *service) planMessageOutputs(ctx context.Context, plan messageCommitPlan) ([]MessageOutput, error) {
	if s == nil || s.toolRuntime == nil || plan.Request == nil {
		return nil, nil
	}
	planner, ok := s.toolRuntime.(MessageOutputPlanner)
	if !ok {
		return nil, nil
	}
	requestID := plan.Request.RequestID
	if strings.TrimSpace(requestID) == "" {
		requestID = plan.Request.InteractionID
	}
	outputs, err := planner.PlanMessageOutputs(ctx, SkillScope{
		SpaceID:        plan.Request.SpaceID,
		CharacterID:    plan.Character,
		ConversationID: plan.Conversation,
		Channel:        plan.Request.Channel,
		SessionID:      plan.Request.SessionID,
		Message:        plan.Request.Message,
		Source:         plan.Source,
		IsInternal:     plan.Request.IsInternal,
		TraceID:        requestID,
		RequestID:      requestID,
	}, &MessageOutputPlanningEvent{
		ConversationID: plan.Conversation,
		CharacterID:    plan.Character,
		Channel:        plan.Request.Channel,
		Source:         plan.Source,
		UserMessage:    plan.Request.Message,
		Reply:          plan.Reply,
		SpaceID:        plan.Request.SpaceID,
		PeerID:         plan.Request.PeerID,
		RequestID:      requestID,
		ForceVoice:     plan.ForceVoice,
	})
	if err != nil {
		return nil, err
	}
	return normalizeMessageOutputs(outputs)
}

func normalizeMessageOutputs(outputs []MessageOutput) ([]MessageOutput, error) {
	if len(outputs) == 0 {
		return nil, nil
	}
	if len(outputs) > MaxMessageOutputsPerReply {
		return nil, fmt.Errorf("message outputs exceed %d", MaxMessageOutputsPerReply)
	}
	normalized := make([]MessageOutput, 0, len(outputs))
	for index, output := range outputs {
		part := output.Part
		part.Type = strings.ToLower(strings.TrimSpace(part.Type))
		part.Content = strings.TrimSpace(part.Content)
		part.ExtensionType = strings.TrimSpace(part.ExtensionType)
		part.MIMEType = strings.TrimSpace(part.MIMEType)
		part.URL = strings.TrimSpace(part.URL)
		part.FallbackURL = strings.TrimSpace(part.FallbackURL)
		part.AltText = strings.TrimSpace(part.AltText)
		if !validMessageOutputPartType(part.Type) {
			return nil, fmt.Errorf("message output %d has invalid part type %q; assistant text must remain in the turn text block", index, part.Type)
		}
		if part.URL == "" {
			return nil, fmt.Errorf("message output %d %s part requires url", index, part.Type)
		}
		output.Placement = strings.ToLower(strings.TrimSpace(output.Placement))
		if output.Placement == "" {
			output.Placement = "after_text"
		}
		switch output.Placement {
		case "before_text", "after_text":
		default:
			return nil, fmt.Errorf("message output %d has invalid placement %q", index, output.Placement)
		}
		output.Part = part
		normalized = append(normalized, output)
	}
	sort.SliceStable(normalized, func(i, j int) bool {
		if normalized[i].Placement == normalized[j].Placement {
			return false
		}
		return normalized[i].Placement == "before_text"
	})
	return normalized, nil
}

func validMessagePartType(value string) bool {
	switch value {
	case "text", "image", "audio", "video", "file":
		return true
	default:
		return false
	}
}

func validMessageOutputPartType(value string) bool {
	switch value {
	case "image", "audio", "video", "file":
		return true
	default:
		return false
	}
}

func buildMessageFromOutput(plan messageCommitPlan, deliveryGroupID string, sequence int, output MessageOutput) *Message {
	part := output.Part
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
	if !strings.EqualFold(plan.Request.Channel, "web") {
		status = "sending"
	}
	source := strings.TrimSpace(plan.Source)
	if strings.TrimSpace(output.ExtensionID) != "" {
		source = "extension:" + strings.TrimSpace(output.ExtensionID)
	}
	message := &Message{
		ID:               uuid.New().String(),
		ConversationID:   plan.Conversation,
		Role:             "assistant",
		Content:          content,
		MsgType:          part.Type,
		ExtensionType:    part.ExtensionType,
		Source:           source,
		Status:           status,
		AltText:          part.AltText,
		IsAnimated:       boolInt(part.IsAnimated),
		MediaWidth:       part.Width,
		MediaHeight:      part.Height,
		OriginalAsset:    part.URL,
		FallbackAsset:    part.FallbackURL,
		DeliveryGroupID:  deliveryGroupID,
		DeliverySequence: sequence,
		RequestID:        plan.Request.RequestID,
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

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
