package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/u-ai/backend/internal/agentplatform"
	"github.com/u-ai/backend/internal/auth"
	"github.com/u-ai/backend/internal/interaction"
	"github.com/u-ai/backend/internal/runtimeidentity"
)

type a2aUnifiedEntryExecutor struct {
	platform *agentplatform.Service
	entry    *interaction.UnifiedEntry
	deviceID string
}

func (e *a2aUnifiedEntryExecutor) ExecuteA2ATask(ctx context.Context, spaceID, agentID string, input any) (any, error) {
	if e == nil || e.platform == nil || e.entry == nil {
		return nil, errors.New("A2A 执行链路不可用")
	}
	detail, err := e.platform.GetAgent(ctx, spaceID, agentID)
	if err != nil {
		return nil, fmt.Errorf("读取 A2A 目标 Agent: %w", err)
	}
	message := ""
	if value, ok := input.(map[string]any); ok {
		for _, key := range []string{"message", "text", "prompt"} {
			if text, ok := value[key].(string); ok && strings.TrimSpace(text) != "" {
				message = text
				break
			}
		}
	}
	if message == "" {
		encoded, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("编码 A2A 输入: %w", err)
		}
		message = string(encoded)
	}
	actor := &auth.ActorContext{
		PrincipalType: auth.PrincipalSystemWorker,
		SpaceID:       runtimeidentity.SpaceID(spaceID),
		DeviceID:      runtimeidentity.DeviceID(e.deviceID),
		AuthMethod:    "a2a",
	}
	request := &interaction.UnifiedEntryRequest{
		Channel: "a2a", Source: "runtime", SpaceID: spaceID,
		CharacterID:    detail.Profile.CharacterID,
		ConversationID: "a2a-" + agentID,
		Message:        message, AdmissionExplicit: true, AdmissionTrigger: "direct",
		PermissionMode: "request_approval", IsInternal: true,
	}
	result, err := e.entry.Handle(auth.WithActor(ctx, actor), request)
	if err != nil {
		return nil, fmt.Errorf("A2A 统一执行: %w", err)
	}
	output := map[string]any{
		"interactionId": result.InteractionID,
		"outcome":       string(result.Outcome),
		"error":         result.Error,
	}
	if result.Response != nil {
		output["reply"] = result.Response.Reply
		output["conversationId"] = result.Response.ConversationID
		output["messageIds"] = result.Response.MessageIDs
	}
	return output, nil
}
