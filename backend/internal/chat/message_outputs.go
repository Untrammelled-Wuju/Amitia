package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/u-ai/backend/internal/timeoutpolicy"
	"log"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/u-ai/backend/internal/artifact"
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

type MessageArtifactImporter interface {
	ImportURL(ctx context.Context, req artifact.ImportURLRequest) (artifact.Artifact, error)
}

func (s *service) planMessageOutputs(ctx context.Context, plan messageCommitPlan) ([]MessageOutput, error) {
	outputs := messageOutputsFromTurnItems(plan.TurnItems)
	if s == nil || s.toolRuntime == nil || plan.Request == nil {
		normalized, err := normalizeMessageOutputs(outputs)
		if err != nil {
			return nil, err
		}
		return s.materializeWebSearchImages(ctx, plan, normalized), nil
	}
	if planner, ok := s.toolRuntime.(MessageOutputPlanner); ok {
		requestID := plan.Request.RequestID
		if strings.TrimSpace(requestID) == "" {
			requestID = plan.Request.InteractionID
		}
		planned, err := planner.PlanMessageOutputs(ctx, SkillScope{
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
		outputs = append(outputs, planned...)
	}
	normalized, err := normalizeMessageOutputs(outputs)
	if err != nil {
		return nil, err
	}
	return s.materializeWebSearchImages(ctx, plan, normalized), nil
}

func messageOutputsFromTurnItems(items []AssistantTurnItem) []MessageOutput {
	if len(items) == 0 {
		return nil
	}
	outputs := make([]MessageOutput, 0)
	for _, item := range items {
		if item.ItemType != assistantTurnItemToolResult || item.Status != assistantTurnStatusCompleted {
			continue
		}
		raw := strings.TrimSpace(item.ResultJSON)
		if raw == "" || raw == "null" {
			continue
		}
		switch item.ToolName {
		case "media_image_generate", "media.image.generate", "send_attachment":
		case "web_run", "web.run":
			outputs = append(outputs, imageOutputsFromWebSearch(raw)...)
			continue
		default:
			continue
		}
		var envelope struct {
			MessageOutputs []MessageOutput `json:"messageOutputs"`
			Outputs        []MessageOutput `json:"outputs"`
		}
		if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
			continue
		}
		outputs = append(outputs, envelope.MessageOutputs...)
		outputs = append(outputs, envelope.Outputs...)
	}
	return outputs
}

func imageOutputsFromWebSearch(raw string) []MessageOutput {
	var envelope struct {
		Search []struct {
			Kind         string `json:"kind"`
			Title        string `json:"title"`
			URL          string `json:"url"`
			MediaURL     string `json:"media_url"`
			ThumbnailURL string `json:"thumbnail_url"`
			Width        int    `json:"width"`
			Height       int    `json:"height"`
		} `json:"search"`
	}
	if err := json.Unmarshal([]byte(raw), &envelope); err != nil {
		return nil
	}
	outputs := make([]MessageOutput, 0, 4)
	for _, hit := range envelope.Search {
		if len(outputs) >= 4 || !strings.EqualFold(strings.TrimSpace(hit.Kind), "image") {
			continue
		}
		mediaURL := strings.TrimSpace(hit.MediaURL)
		if mediaURL == "" {
			mediaURL = strings.TrimSpace(hit.ThumbnailURL)
		}
		if mediaURL == "" {
			continue
		}
		title := strings.TrimSpace(hit.Title)
		if title == "" {
			title = "搜索结果图片"
		}
		outputs = append(outputs, MessageOutput{
			OutputID:  "web_run.image",
			Placement: "after_text",
			Part: MessagePart{
				Type:     "image",
				Content:  title,
				URL:      mediaURL,
				AltText:  title,
				Width:    hit.Width,
				Height:   hit.Height,
				MIMEType: mimeTypeFromURL(mediaURL),
			},
		})
	}
	return outputs
}

func (s *service) materializeWebSearchImages(ctx context.Context, plan messageCommitPlan, outputs []MessageOutput) []MessageOutput {
	if len(outputs) == 0 || plan.Request == nil || s == nil || s.artifactImporter == nil {
		return outputs
	}
	ownerSpaceID := strings.TrimSpace(plan.Request.SpaceID)
	if ownerSpaceID == "" {
		return outputs
	}
	importCtx, cancel := timeoutpolicy.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	materialized := make([]MessageOutput, 0, len(outputs))
	for _, output := range outputs {
		if !strings.EqualFold(strings.TrimSpace(output.OutputID), "web_run.image") || !strings.EqualFold(output.Part.Type, "image") {
			materialized = append(materialized, output)
			continue
		}
		remoteURL := strings.TrimSpace(output.Part.URL)
		parsed, err := url.Parse(remoteURL)
		if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			materialized = append(materialized, output)
			continue
		}
		filename := strings.TrimSpace(output.Part.AltText)
		if filename == "" {
			filename = strings.TrimSpace(path.Base(parsed.Path))
		}
		art, importErr := s.artifactImporter.ImportURL(importCtx, artifact.ImportURLRequest{
			OwnerSpaceID: ownerSpaceID,
			URL:          remoteURL,
			Kind:         artifact.KindImage,
			MIMEType:     output.Part.MIMEType,
			Filename:     filename,
			Source:       artifact.SourceToolOutput,
			MaxBytes:     24 * 1024 * 1024,
		})
		if importErr != nil {
			log.Printf("[message_outputs] web image import failed: %v", importErr)
			continue
		}
		output.Part.FallbackURL = remoteURL
		output.Part.URL = artifact.URI(art.ID)
		if strings.TrimSpace(output.Part.MIMEType) == "" {
			output.Part.MIMEType = art.MIMEType
		}
		if output.Part.Width <= 0 {
			output.Part.Width = art.Width
		}
		if output.Part.Height <= 0 {
			output.Part.Height = art.Height
		}
		materialized = append(materialized, output)
	}
	return materialized
}

func mimeTypeFromURL(raw string) string {
	path := strings.ToLower(strings.TrimSpace(raw))
	if index := strings.IndexAny(path, "?#"); index >= 0 {
		path = path[:index]
	}
	switch {
	case strings.HasSuffix(path, ".png"):
		return "image/png"
	case strings.HasSuffix(path, ".jpg"), strings.HasSuffix(path, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(path, ".gif"):
		return "image/gif"
	case strings.HasSuffix(path, ".webp"):
		return "image/webp"
	case strings.HasSuffix(path, ".svg"):
		return "image/svg+xml"
	default:
		return ""
	}
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
