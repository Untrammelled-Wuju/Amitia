package hook

import (
	"context"
	"encoding/json"
)

type HostHookIntegrator struct {
	Pipeline *Pipeline
}

func (h *HostHookIntegrator) invokeHook(ctx context.Context, hookPointID string, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	result := h.Pipeline.Invoke(ctx, InvokeRequest{
		HookPointID: hookPointID,
		Payload:     payload,
		Context:     hookCtx,
	})
	if result.Aborted {
		if result.Decision == DecisionReject || result.Decision == DecisionDeny {
			return result.FinalPayload, true, nil
		}
		return result.FinalPayload, true, NewHookError(ErrCodeHookRuntimeError, result.AbortReason)
	}
	return result.FinalPayload, false, nil
}

func (h *HostHookIntegrator) InvokeMessageBeforeSend(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "message.before_send/1", payload, hookCtx)
}

func (h *HostHookIntegrator) InvokeMessageBeforePersist(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "message.before_persist/1", payload, hookCtx)
}

func (h *HostHookIntegrator) InvokeMessageAfterPersist(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "message.after_persist/1", payload, hookCtx)
}

func (h *HostHookIntegrator) InvokeModelBeforeRequest(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "model.before_request/1", payload, hookCtx)
}

func (h *HostHookIntegrator) InvokeModelAfterResponse(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "model.after_response/1", payload, hookCtx)
}

func (h *HostHookIntegrator) InvokePromptBeforeAssemble(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "prompt.before_assemble/1", payload, hookCtx)
}

func (h *HostHookIntegrator) InvokePromptAfterAssemble(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "prompt.after_assemble/1", payload, hookCtx)
}

func (h *HostHookIntegrator) InvokeToolBeforeExecute(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "tool.before_execute/1", payload, hookCtx)
}

func (h *HostHookIntegrator) InvokeToolAfterExecute(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "tool.after_execute/1", payload, hookCtx)
}

func (h *HostHookIntegrator) InvokeWorkflowBeforeStart(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "workflow.before_start/1", payload, hookCtx)
}

func (h *HostHookIntegrator) InvokeWorkflowAfterFinish(ctx context.Context, payload json.RawMessage, hookCtx HookContextSnapshot) (json.RawMessage, bool, error) {
	return h.invokeHook(ctx, "workflow.after_finish/1", payload, hookCtx)
}
