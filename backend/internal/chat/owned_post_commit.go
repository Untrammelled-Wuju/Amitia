package chat

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (s *service) PlanOwnedTitle(ctx context.Context, inference business.Inference, generation business.Generation) (string, error) {
	if inference.Preview || inference.InternalWorker || generation.Partial || generation.Suppressed || strings.TrimSpace(inference.Message) == "" || strings.TrimSpace(generation.Text) == "" {
		return "", nil
	}
	if err := coordination.ValidateSnapshot(inference.Scope, inference.Snapshot); err != nil {
		return "", err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return "", err
	}
	for _, encoded := range inference.Snapshot.LegacyMessages {
		var message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(encoded, &message); err != nil {
			return "", err
		}
		if message.Role == "assistant" && strings.TrimSpace(message.Content) != "" {
			return "", nil
		}
	}
	for _, resource := range inference.Snapshot.Resources {
		if resource.Deleted {
			continue
		}
		if resource.Kind == "message" {
			var message struct {
				Role           string `json:"role"`
				Content        string `json:"content"`
				ConversationID string `json:"conversationId"`
			}
			if err := json.Unmarshal(resource.Body, &message); err != nil {
				return "", err
			}
			if message.ConversationID == inference.ConversationID && message.Role == "assistant" && strings.TrimSpace(message.Content) != "" {
				return "", nil
			}
		}
		if resource.Kind == "conversation" && resource.ID == inference.ConversationID {
			var conversation struct {
				Title string `json:"title"`
			}
			if err := json.Unmarshal(resource.Body, &conversation); err != nil {
				return "", err
			}
			title := strings.TrimSpace(conversation.Title)
			if title != "" && title != strings.TrimSpace(inference.Message) && title != "新对话" {
				return "", nil
			}
		}
	}
	cfg, err := s.modelForScenario("summary", 0)
	if err != nil {
		return "", err
	}
	title, err := s.requestConversationTitle(ctx, cfg, inference.Message, normalizeTitleGenerationReply(generation.Text))
	if err != nil {
		return "", err
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return "", err
	}
	return title, nil
}
