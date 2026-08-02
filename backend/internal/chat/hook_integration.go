package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel/hook"
)

type HookInvoker interface {
	InvokeMessageBeforeSend(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error)
	InvokeMessageBeforePersist(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error)
	InvokeMessageAfterPersist(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error)
	InvokeModelBeforeRequest(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error)
	InvokeModelAfterResponse(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error)
	InvokeToolBeforeExecute(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error)
	InvokeToolAfterExecute(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error)
}

type HookAdapter struct {
	integrator *hook.HostHookIntegrator
}

func NewHookAdapter(service *hook.Service) *HookAdapter {
	if service == nil || service.Integrator == nil {
		return &HookAdapter{integrator: nil}
	}
	return &HookAdapter{integrator: service.Integrator}
}

func (a *HookAdapter) available() bool {
	return a != nil && a.integrator != nil
}

func (a *HookAdapter) InvokeMessageBeforeSend(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error) {
	if !a.available() {
		return payload, false, nil
	}
	return a.integrator.InvokeMessageBeforeSend(ctx, payload, hookCtx)
}

func (a *HookAdapter) InvokeMessageBeforePersist(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error) {
	if !a.available() {
		return payload, false, nil
	}
	return a.integrator.InvokeMessageBeforePersist(ctx, payload, hookCtx)
}

func (a *HookAdapter) InvokeMessageAfterPersist(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error) {
	if !a.available() {
		return payload, false, nil
	}
	return a.integrator.InvokeMessageAfterPersist(ctx, payload, hookCtx)
}

func (a *HookAdapter) InvokeModelBeforeRequest(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error) {
	if !a.available() {
		return payload, false, nil
	}
	return a.integrator.InvokeModelBeforeRequest(ctx, payload, hookCtx)
}

func (a *HookAdapter) InvokeModelAfterResponse(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error) {
	if !a.available() {
		return payload, false, nil
	}
	return a.integrator.InvokeModelAfterResponse(ctx, payload, hookCtx)
}

func (a *HookAdapter) InvokeToolBeforeExecute(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error) {
	if !a.available() {
		return payload, false, nil
	}
	return a.integrator.InvokeToolBeforeExecute(ctx, payload, hookCtx)
}

func (a *HookAdapter) InvokeToolAfterExecute(ctx context.Context, payload json.RawMessage, hookCtx hook.HookContextSnapshot) (json.RawMessage, bool, error) {
	if !a.available() {
		return payload, false, nil
	}
	return a.integrator.InvokeToolAfterExecute(ctx, payload, hookCtx)
}

func buildHookContext(invocationID, operationID, extensionID, userID, characterID, conversationID, sessionID, channel string) hook.HookContextSnapshot {
	var charID *string
	if characterID != "" {
		c := characterID
		charID = &c
	}
	var convID *string
	if conversationID != "" {
		c := conversationID
		convID = &c
	}
	return hook.HookContextSnapshot{
		InvocationID:   invocationID,
		OperationID:    operationID,
		ExtensionID:    extensionID,
		CharacterID:    charID,
		ConversationID: convID,
		Platform:       channel,
		Timestamp:      time.Now().UTC(),
		Depth:          0,
	}
}

func marshalPayload(data interface{}) json.RawMessage {
	if data == nil {
		return json.RawMessage(`{}`)
	}
	b, err := json.Marshal(data)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func unmarshalPayload(payload json.RawMessage, target interface{}) error {
	if len(payload) == 0 {
		return nil
	}
	return json.Unmarshal(payload, target)
}

func hookPayloadToMap(payload json.RawMessage) map[string]interface{} {
	var m map[string]interface{}
	if err := unmarshalPayload(payload, &m); err != nil {
		return map[string]interface{}{}
	}
	return m
}

func mapToHookPayload(m map[string]interface{}) json.RawMessage {
	b, err := json.Marshal(m)
	if err != nil {
		return json.RawMessage(`{}`)
	}
	return b
}

func (s *service) SetHookInvoker(invoker HookInvoker) {
	s.hookInvoker = invoker
}

func (s *service) invokeMessageHook(ctx context.Context, hookPoint string, payload map[string]interface{}, invocationID, userID, characterID, conversationID, sessionID, channel string) (map[string]interface{}, bool, error) {
	if s.hookInvoker == nil {
		return payload, false, nil
	}

	hookCtx := buildHookContext(invocationID, "", "", userID, characterID, conversationID, sessionID, channel)
	rawPayload := mapToHookPayload(payload)

	var result json.RawMessage
	var blocked bool
	var err error

	switch hookPoint {
	case "before_send":
		result, blocked, err = s.hookInvoker.InvokeMessageBeforeSend(ctx, rawPayload, hookCtx)
	case "before_persist":
		result, blocked, err = s.hookInvoker.InvokeMessageBeforePersist(ctx, rawPayload, hookCtx)
	case "after_persist":
		result, blocked, err = s.hookInvoker.InvokeMessageAfterPersist(ctx, rawPayload, hookCtx)
	case "model_before_request":
		result, blocked, err = s.hookInvoker.InvokeModelBeforeRequest(ctx, rawPayload, hookCtx)
	case "model_after_response":
		result, blocked, err = s.hookInvoker.InvokeModelAfterResponse(ctx, rawPayload, hookCtx)
	case "tool_before_execute":
		result, blocked, err = s.hookInvoker.InvokeToolBeforeExecute(ctx, rawPayload, hookCtx)
	case "tool_after_execute":
		result, blocked, err = s.hookInvoker.InvokeToolAfterExecute(ctx, rawPayload, hookCtx)
	default:
		return payload, false, fmt.Errorf("unknown hook point: %s", hookPoint)
	}

	if err != nil {
		return payload, false, err
	}

	if blocked {
		return hookPayloadToMap(result), true, nil
	}

	return hookPayloadToMap(result), false, nil
}
