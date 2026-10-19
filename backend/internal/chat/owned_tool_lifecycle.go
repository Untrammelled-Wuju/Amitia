package chat

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
)

func (s *service) prepareOwnedToolPrompt(ctx context.Context, inference business.Inference, messages []map[string]interface{}) (func(), error) {
	lifecycle, ok := s.ownedToolRuntime.(OwnedToolLifecycleRuntime)
	if !ok || inference.ScheduledMessage {
		return func() {}, nil
	}
	prompt, finish, err := lifecycle.PrepareOwnedPrompt(ctx, inference)
	if finish == nil {
		finish = func() {}
	}
	if err != nil {
		finish()
		return func() {}, err
	}
	if len(messages) > 0 && prompt != "" {
		messages[0]["content"] = fmt.Sprint(messages[0]["content"]) + "\n\n" + prompt
	}
	return finish, coordination.ValidateCurrent(ctx)
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
	s.emitOwnedChatLifecycle(ctx, inference, messageID, "response.completed", 5)
	if lifecycle, ok := s.ownedToolRuntime.(OwnedToolLifecycleRuntime); ok {
		return lifecycle.OwnedReplyCommitted(ctx, inference, generation, messageID)
	}
	return nil
}
