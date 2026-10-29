package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/chat"
	"github.com/u-ai/backend/internal/devicemesh/business"
	"github.com/u-ai/backend/internal/devicemesh/coordination"
	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
	"github.com/u-ai/backend/log"
)

func (r *meshOwnedToolRuntime) validateLifecycle(ctx context.Context, inference business.Inference) error {
	if r == nil || r.services == nil || r.services.DeviceMesh == nil || r.services.KernelContainer == nil || r.services.KernelContainer.ToolFacade == nil {
		return errors.New("统一聊天工具生命周期尚未就绪")
	}
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if err := r.services.DeviceMesh.Coordination.Validate(ctx, inference.Scope); err != nil {
		return err
	}
	return coordination.ValidateRoleRevision(ctx, r.services.DeviceMesh, inference.Scope)
}

func (r *meshOwnedToolRuntime) PrepareOwnedPrompt(ctx context.Context, inference business.Inference) (string, func(), error) {
	if err := r.validateLifecycle(ctx, inference); err != nil {
		return "", nil, err
	}
	facade := r.services.KernelContainer.ToolFacade
	scope := ownedToolScope(inference, "")
	finish := func() { facade.EndAgentSkillRound(scope) }
	catalog, activated, activationErrors := facade.PrepareAgentSkillPrompt(ctx, scope, inference.Message)
	parts := []string{catalog}
	for _, skill := range activated {
		parts = append(parts, skill.Prompt)
	}
	if len(activationErrors) > 0 {
		parts = append(parts, "<agent_skill_activation_errors>"+strings.Join(activationErrors, "; ")+"</agent_skill_activation_errors>")
	}
	for _, contribution := range facade.BeforePrompt(ctx, scope) {
		if contribution.ExpiresAt != nil && !contribution.ExpiresAt.After(time.Now()) {
			continue
		}
		if strings.TrimSpace(contribution.Content) != "" {
			parts = append(parts, contribution.Content)
		}
	}
	if err := r.validateLifecycle(ctx, inference); err != nil {
		return "", finish, err
	}
	return strings.Join(parts, "\n\n"), finish, nil
}

func (r *meshOwnedToolRuntime) PlanOwnedOutputs(ctx context.Context, inference business.Inference, generation business.Generation) ([]chat.MessageOutput, error) {
	if err := r.validateLifecycle(ctx, inference); err != nil {
		return nil, err
	}
	input, err := json.Marshal(map[string]any{"conversationId": inference.ConversationID, "characterId": inference.Scope.RoleID, "channel": "device-mesh", "source": "owned", "userMessage": inference.Message, "reply": generation.Text, "spaceId": inference.Scope.SpaceID, "requestId": inference.Scope.RequestID, "forceVoice": generation.ForceVoice})
	if err != nil {
		return nil, err
	}
	outputs := make([]chat.MessageOutput, 0)
	for _, provider := range r.services.KernelContainer.ToolFacade.ListMessageOutputProviders(ctx) {
		definition, err := r.authorizeDefinition(ctx, inference, provider)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256([]byte(provider.ID))
		result, err := r.executeDefinition(ctx, inference, "output/"+hex.EncodeToString(digest[:]), input, definition)
		if err != nil {
			return nil, err
		}
		if result.Error != nil || !strings.EqualFold(result.Status, "SUCCESS") {
			log.Warn("owned message output provider skipped", "provider", provider.ID, "status", result.Status, "error", result.Error)
			continue
		}
		if len(result.Output) == 0 {
			continue
		}
		var envelope struct {
			Outputs []chat.MessageOutput `json:"outputs"`
		}
		if err := json.Unmarshal(result.Output, &envelope); err != nil {
			if err := json.Unmarshal(result.Output, &envelope.Outputs); err != nil {
				log.Warn("owned message output provider returned invalid output", "provider", provider.ID, "error", err, "output", string(result.Output))
				continue
			}
		}
		for _, output := range envelope.Outputs {
			if output.ExtensionID == "" {
				output.ExtensionID = provider.ExtensionID
			}
			if output.OutputID == "" {
				output.OutputID = provider.ID
			}
			outputs = append(outputs, output)
		}
	}
	return outputs, r.validateLifecycle(ctx, inference)
}

func (r *meshOwnedToolRuntime) OwnedReplyCommitted(ctx context.Context, inference business.Inference, generation business.Generation, messageID string) error {
	if err := r.validateLifecycle(ctx, inference); err != nil {
		return err
	}
	r.services.KernelContainer.ToolFacade.AfterReplyContext(ctx, ownedToolScope(inference, ""), kernel.ReplyView{MessageID: messageID, CharacterID: inference.Scope.RoleID, ConversationID: inference.ConversationID, Channel: "device-mesh", Content: generation.Text, CreatedAt: time.Now().UTC()})
	if inference.Group != nil {
		if err := r.services.OwnedBusiness.AcceptGroupInvitations(ctx, inference, generation, messageID, r.takeGroupInvites(inference.Scope.ExecutionID)); err != nil {
			return err
		}
	}
	return r.validateLifecycle(ctx, inference)
}

var _ chat.OwnedToolLifecycleRuntime = (*meshOwnedToolRuntime)(nil)

type ownedToolProgressSink struct {
	inference business.Inference
	call      string
	name      string
}

func (s ownedToolProgressSink) Emit(ctx context.Context, event capability.ToolStreamEvent) error {
	if err := coordination.ValidateCurrent(ctx); err != nil {
		return err
	}
	if s.inference.Emit == nil || event.Type != capability.ToolStreamEventProgress || event.Progress == nil {
		return nil
	}
	encoded, err := json.Marshal(map[string]any{"toolCallId": s.call, "toolName": s.name, "fraction": event.Progress.Fraction, "indeterminate": event.Progress.Indeterminate, "message": event.Progress.Message, "metadata": event.Metadata})
	if err != nil {
		return err
	}
	return s.inference.Emit(business.Event{Type: "tool-progress", Progress: encoded})
}
