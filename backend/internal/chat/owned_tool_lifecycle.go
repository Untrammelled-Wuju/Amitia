package chat

import (
	"context"
	"encoding/json"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (s *service) prepareOwnedToolPrompt(ctx context.Context, inference business.Inference, messages []map[string]interface{}) ([]map[string]interface{}, func(), error) {
	lifecycle, ok := s.ownedToolRuntime.(OwnedToolLifecycleRuntime)
	if !ok || inference.ScheduledMessage {
		return messages, func() {}, nil
	}
	prompt, finish, err := lifecycle.PrepareOwnedPrompt(ctx, inference)
	if finish == nil {
		finish = func() {}
	}
	if err != nil {
		finish()
		return messages, func() {}, err
	}
	if len(messages) > 0 && prompt != "" {
		messages = append(messages[:len(messages)-1:len(messages)-1], map[string]interface{}{"role": "system", "content": prompt}, messages[len(messages)-1])
	}
	return messages, finish, coordination.ValidateCurrent(ctx)
}

func (s *service) finishOwnedOutputs(ctx context.Context, inference business.Inference, generation business.Generation, toolOutputs []MessageOutput) (business.Generation, error) {
	if generation.Partial {
		return generation, nil
	}
	if lifecycle, ok := s.ownedToolRuntime.(OwnedToolLifecycleRuntime); ok && !inference.ScheduledMessage {
		planned, err := lifecycle.PlanOwnedOutputs(ctx, inference, generation)
		if err != nil {
			generation.Partial = true
			return generation, err
		}
		toolOutputs = append(toolOutputs, planned...)
	}
	outputs, err := normalizeMessageOutputs(toolOutputs)
	if err != nil {
		generation.Partial = true
		return generation, err
	}
	if len(outputs) > 0 {
		generation.Outputs, err = json.Marshal(outputs)
	}
	if err == nil {
		err = coordination.ValidateCurrent(ctx)
	}
	if err != nil {
		generation.Partial = true
	}
	return generation, err
}

func (s *service) OwnedReplyCommitted(ctx context.Context, inference business.Inference, generation business.Generation, messageID string) error {
	if generation.Partial || messageID == "" {
		return nil
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if lifecycle, ok := s.ownedToolRuntime.(OwnedToolLifecycleRuntime); ok {
		return lifecycle.OwnedReplyCommitted(ctx, inference, generation, messageID)
	}
	return nil
}
