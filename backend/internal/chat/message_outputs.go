package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

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

func boolInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
