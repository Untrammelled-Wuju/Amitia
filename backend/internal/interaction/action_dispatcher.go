package interaction

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/extension/kernel"
	"github.com/u-ai/backend/internal/extension/kernel/capability"
)

func capabilityID(s string) capability.CapabilityID {
	return capability.CapabilityID(s)
}

type ActionExecutionState string

const (
	ActionExecutionCompleted        ActionExecutionState = "completed"
	ActionExecutionSkipped          ActionExecutionState = "skipped"
	ActionExecutionFailedToDispatch ActionExecutionState = "failed_to_dispatch"
	ActionExecutionAcceptedBackground ActionExecutionState = "accepted_background"
)

type ActionExecutionResult struct {
	Action          MaterializedAction       `json:"action"`
	State           ActionExecutionState     `json:"state"`
	ToolResult      *kernel.LegacyToolResult `json:"toolResult,omitempty"`
	TaskRunID       string                   `json:"taskRunId,omitempty"`
	TaskDefinitionID string                  `json:"taskDefinitionId,omitempty"`
	Background      bool                     `json:"background,omitempty"`
	Err             error                    `json:"-"`
	ErrCode         string                   `json:"errCode,omitempty"`
	CompletedAt     time.Time                `json:"completedAt"`
}

type actionToolFacade interface {
	ExecuteTool(ctx context.Context, toolID capability.CapabilityID, input json.RawMessage, scope kernel.LegacyScope, externalCallID string, idempotencyKey string) (kernel.LegacyToolResult, bool)
}

type ActionDispatcher struct {
	toolFacade actionToolFacade
}

func NewActionDispatcher(toolFacade actionToolFacade) ActionDispatcher {
	return ActionDispatcher{toolFacade: toolFacade}
}

func (d ActionDispatcher) Dispatch(
	ctx context.Context,
	action MaterializedAction,
	scope ActionMaterializationScope,
	now time.Time,
) ActionExecutionResult {
	if scopeVerificationLost(action, scope) {
		return ActionExecutionResult{
			Action:      action,
			State:       ActionExecutionFailedToDispatch,
			Err:         fmt.Errorf("ACTION_SCOPE_MISMATCH: scope mismatch at dispatch"),
			ErrCode:     "ACTION_SCOPE_MISMATCH",
			CompletedAt: now,
		}
	}

	switch action.Kind {
	case MaterializedActionRespond:
		return ActionExecutionResult{
			Action:      action,
			State:       ActionExecutionSkipped,
			CompletedAt: now,
		}
	case MaterializedActionWait:
		return ActionExecutionResult{
			Action:      action,
			State:       ActionExecutionSkipped,
			CompletedAt: now,
		}
	case MaterializedActionTool:
		return d.dispatchTool(ctx, action, scope, now)
	}

	return ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionFailedToDispatch,
		Err:         fmt.Errorf("unknown action kind: %s", action.Kind),
		ErrCode:     "ACTION_DISPATCH_UNAVAILABLE",
		CompletedAt: now,
	}
}

func (d ActionDispatcher) dispatchTool(
	ctx context.Context,
	action MaterializedAction,
	scope ActionMaterializationScope,
	now time.Time,
) ActionExecutionResult {
	if action.Tool == nil {
		return ActionExecutionResult{
			Action:      action,
			State:       ActionExecutionFailedToDispatch,
			Err:         fmt.Errorf("tool action has no tool binding"),
			ErrCode:     "ACTION_DISPATCH_UNAVAILABLE",
			CompletedAt: now,
		}
	}

	toolScope := kernel.LegacyScope{
		UserID:         scope.UserID,
		CharacterID:    scope.CharacterID,
		ConversationID: scope.ConversationID,
		Channel:        scope.Channel,
		SessionID:      scope.SessionID,
		TraceID:        scope.TraceID,
		RequestID:      scope.RequestID,
		ToolCallID:     action.Tool.ExternalCallID,
		CorrelationID:  scope.InteractionID,
		CausationID:    action.ID,
	}

	idempotencyKey := fmt.Sprintf("agent-action:%s", action.ID)
	result, ok := d.toolFacade.ExecuteTool(
		ctx,
		capabilityID(action.Tool.ToolID),
		action.Tool.Input,
		toolScope,
		action.Tool.ExternalCallID,
		idempotencyKey,
	)
	if !ok {
		return ActionExecutionResult{
			Action:      action,
			State:       ActionExecutionFailedToDispatch,
			Err:         fmt.Errorf("tool dispatch failed: %s", action.Tool.ToolID),
			ErrCode:     "ACTION_DISPATCH_UNAVAILABLE",
			CompletedAt: now,
		}
	}

	if action.Tool != nil && action.Tool.RuntimeType == string(capability.RuntimeTypeTask) && action.Tool.AllowBackground && extractTaskRunID(&result) != "" {
		return ActionExecutionResult{
			Action:           action,
			State:            ActionExecutionAcceptedBackground,
			ToolResult:       &result,
			TaskRunID:        extractTaskRunID(&result),
			TaskDefinitionID: action.Tool.ToolID,
			Background:       true,
			CompletedAt:      now,
		}
	}

	return ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionCompleted,
		ToolResult:  &result,
		CompletedAt: now,
	}
}

func extractTaskRunID(toolResult *kernel.LegacyToolResult) string {
	if toolResult == nil || len(toolResult.Output) == 0 {
		return ""
	}
	var structured map[string]any
	if err := json.Unmarshal(toolResult.Output, &structured); err != nil {
		return ""
	}
	v, ok := structured["taskRunId"]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func scopeVerificationLost(action MaterializedAction, scope ActionMaterializationScope) bool {
	return action.InteractionID != scope.InteractionID
}
