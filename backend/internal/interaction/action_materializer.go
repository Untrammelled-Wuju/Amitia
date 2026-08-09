package interaction

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/extension/kernel"
)

type ModelToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type ActionMaterializationScope struct {
	UserID         string
	CharacterID    string
	ConversationID string
	InteractionID  string
	RequestID      string
	SessionID      string
	Channel        string
	TraceID        string
	OperationID    string
}

type MaterializedActionKind string

const (
	MaterializedActionRespond MaterializedActionKind = "respond"
	MaterializedActionWait    MaterializedActionKind = "wait"
	MaterializedActionTool    MaterializedActionKind = "tool"
)

type MaterializedToolAction struct {
	ToolID         string `json:"toolId"`
	ModelName      string `json:"modelName"`
	ExternalCallID string `json:"externalCallID"`
	Input          json.RawMessage `json:"input"`
	ExtensionID    string `json:"extensionId,omitempty"`
	ModuleID       string `json:"moduleId,omitempty"`
	Generation     int64  `json:"generation"`
	RuntimeType    string `json:"runtimeType,omitempty"`
}

type MaterializedAction struct {
	ID             string                 `json:"id"`
	PlanID         string                 `json:"planId"`
	InteractionID  string                 `json:"interactionId"`
	CandidateID    string                 `json:"candidateId"`
	Kind           MaterializedActionKind `json:"kind"`
	Tool           *MaterializedToolAction `json:"tool,omitempty"`
	CreatedAt      time.Time              `json:"createdAt"`
}

type ActionMaterializationState string

const (
	ActionMaterializationReady          ActionMaterializationState = "ready"
	ActionMaterializationNoAction       ActionMaterializationState = "no_action"
	ActionMaterializationToolNotProduced ActionMaterializationState = "tool_not_produced"
)

type ActionMaterializationOutcome struct {
	State   ActionMaterializationState `json:"state"`
	Action  *MaterializedAction        `json:"action,omitempty"`
	Err     error                      `json:"-"`
	ErrCode decision.ActionMaterializationErrorCode `json:"errCode,omitempty"`
}

type ActionToolCatalog interface {
	ResolveModelTool(modelName string) (kernel.ResolvedToolReference, error)
}

type ActionMaterializer struct {
	toolCatalog ActionToolCatalog
}

func NewActionMaterializer(toolCatalog ActionToolCatalog) ActionMaterializer {
	return ActionMaterializer{toolCatalog: toolCatalog}
}

func BuildActionID(planID string, externalCallID string) string {
	h := sha256.New()
	h.Write([]byte(planID))
	h.Write([]byte{0x00})
	h.Write([]byte(externalCallID))
	return "action:" + hex.EncodeToString(h.Sum(nil))[:32]
}

func (m ActionMaterializer) Materialize(
	ctx context.Context,
	plan *decision.BehaviorPlan,
	directive decision.ActionDirective,
	calls []ModelToolCall,
	scope ActionMaterializationScope,
	now time.Time,
) ActionMaterializationOutcome {
	if plan == nil || directive.PlanID == "" {
		return ActionMaterializationOutcome{State: ActionMaterializationNoAction}
	}

	if err := verifyActionScope(plan, scope); err != nil {
		return ActionMaterializationOutcome{
			State:   ActionMaterializationNoAction,
			Err:     err,
			ErrCode: decision.ErrActionScopeMismatch,
		}
	}

	switch directive.Kind {
	case decision.ActionDirectiveRespond:
		return m.materializeRespond(plan, directive, scope, now)
	case decision.ActionDirectiveWait:
		return m.materializeWait(plan, directive, scope, now)
	case decision.ActionDirectiveTool:
		return m.materializeTool(plan, directive, calls, scope, now)
	}

	return ActionMaterializationOutcome{
		State:   ActionMaterializationNoAction,
		Err:     fmt.Errorf("%w: unknown directive kind: %s", decision.ErrActionPlanInvalid, directive.Kind),
		ErrCode: decision.ErrActionPlanInvalid,
	}
}

func (m ActionMaterializer) materializeRespond(
	plan *decision.BehaviorPlan,
	directive decision.ActionDirective,
	scope ActionMaterializationScope,
	now time.Time,
) ActionMaterializationOutcome {
	id := BuildActionID(plan.ID, "respond")
	action := &MaterializedAction{
		ID:             id,
		PlanID:         plan.ID,
		InteractionID:  scope.InteractionID,
		CandidateID:    directive.CandidateID,
		Kind:           MaterializedActionRespond,
		Tool:           nil,
		CreatedAt:      now,
	}
	return ActionMaterializationOutcome{State: ActionMaterializationReady, Action: action}
}

func (m ActionMaterializer) materializeWait(
	plan *decision.BehaviorPlan,
	directive decision.ActionDirective,
	scope ActionMaterializationScope,
	now time.Time,
) ActionMaterializationOutcome {
	id := BuildActionID(plan.ID, "wait")
	action := &MaterializedAction{
		ID:             id,
		PlanID:         plan.ID,
		InteractionID:  scope.InteractionID,
		CandidateID:    directive.CandidateID,
		Kind:           MaterializedActionWait,
		Tool:           nil,
		CreatedAt:      now,
	}
	return ActionMaterializationOutcome{State: ActionMaterializationReady, Action: action}
}

func (m ActionMaterializer) materializeTool(
	plan *decision.BehaviorPlan,
	directive decision.ActionDirective,
	calls []ModelToolCall,
	scope ActionMaterializationScope,
	now time.Time,
) ActionMaterializationOutcome {
	if len(calls) == 0 {
		return ActionMaterializationOutcome{State: ActionMaterializationToolNotProduced}
	}
	if len(calls) > 1 {
		return ActionMaterializationOutcome{
			State:   ActionMaterializationNoAction,
			Err:     fmt.Errorf("%w: got %d calls, max 1", decision.ErrActionMultipleToolCallsNotAllowed, len(calls)),
			ErrCode: decision.ErrActionMultipleToolCallsNotAllowed,
		}
	}
	call := calls[0]
	action, err := m.MaterializeToolCall(plan, directive, call, scope, now)
	if err != nil {
		return ActionMaterializationOutcome{
			State:   ActionMaterializationNoAction,
			Err:     err,
			ErrCode: mapActionError(err),
		}
	}
	return ActionMaterializationOutcome{State: ActionMaterializationReady, Action: &action}
}

func mapActionError(err error) decision.ActionMaterializationErrorCode {
	if err == nil {
		return ""
	}
	var code decision.ActionMaterializationErrorCode
	if dfsError(err, &code) {
		return code
	}
	return decision.ErrActionPlanInvalid
}

func dfsError(err error, code *decision.ActionMaterializationErrorCode) bool {
	if err == nil {
		return false
	}
	if ec, ok := err.(decision.ActionMaterializationErrorCode); ok {
		*code = ec
		return true
	}
	type wrapper interface{ Unwrap() error }
	if w, ok := err.(wrapper); ok {
		return dfsError(w.Unwrap(), code)
	}
	return false
}

func (m ActionMaterializer) MaterializeToolCall(
	plan *decision.BehaviorPlan,
	directive decision.ActionDirective,
	call ModelToolCall,
	scope ActionMaterializationScope,
	now time.Time,
) (MaterializedAction, error) {
	if directive.Kind != decision.ActionDirectiveTool {
		return MaterializedAction{}, fmt.Errorf("%w: directive kind is %s", decision.ErrActionNotAllowedByPlan, directive.Kind)
	}
	if call.ID == "" {
		return MaterializedAction{}, fmt.Errorf("%w: tool call ID is required", decision.ErrActionExternalCallIDMissing)
	}
	if call.Name == "" {
		return MaterializedAction{}, fmt.Errorf("%w: tool name is required", decision.ErrActionToolNameMissing)
	}
	if len(call.Arguments) > 0 && !json.Valid(call.Arguments) {
		return MaterializedAction{}, fmt.Errorf("%w: invalid JSON", decision.ErrActionInputInvalidJSON)
	}

	resolved, err := m.toolCatalog.ResolveModelTool(call.Name)
	if err != nil {
		return MaterializedAction{}, fmt.Errorf("%w: %s", decision.ErrActionToolNotFound, call.Name)
	}

	id := BuildActionID(plan.ID, call.ID)
	clonedInput := make(json.RawMessage, len(call.Arguments))
	copy(clonedInput, call.Arguments)

	action := MaterializedAction{
		ID:             id,
		PlanID:         plan.ID,
		InteractionID:  scope.InteractionID,
		CandidateID:    directive.CandidateID,
		Kind:           MaterializedActionTool,
		CreatedAt:      now,
		Tool: &MaterializedToolAction{
			ToolID:         string(resolved.ID),
			ModelName:      resolved.ModelName,
			ExternalCallID: call.ID,
			Input:          clonedInput,
			ExtensionID:    resolved.ExtensionID,
			ModuleID:       resolved.ModuleID,
			Generation:     resolved.Generation,
		},
	}
	return action, nil
}

func (m ActionMaterializer) VerifyScope(action MaterializedAction, scope ActionMaterializationScope) error {
	if action.InteractionID != scope.InteractionID {
		return fmt.Errorf("%w: interaction mismatch: action=%s scope=%s", decision.ErrActionScopeMismatch, action.InteractionID, scope.InteractionID)
	}
	return nil
}

func verifyActionScope(plan *decision.BehaviorPlan, scope ActionMaterializationScope) error {
	if plan.UserID != "" && scope.UserID != "" && plan.UserID != scope.UserID {
		return fmt.Errorf("%w: user mismatch: plan=%s scope=%s", decision.ErrActionScopeMismatch, plan.UserID, scope.UserID)
	}
	if plan.CharacterID != "" && scope.CharacterID != "" && plan.CharacterID != scope.CharacterID {
		return fmt.Errorf("%w: character mismatch: plan=%s scope=%s", decision.ErrActionScopeMismatch, plan.CharacterID, scope.CharacterID)
	}
	if plan.ConversationID != "" && scope.ConversationID != "" && plan.ConversationID != scope.ConversationID {
		return fmt.Errorf("%w: conversation mismatch: plan=%s scope=%s", decision.ErrActionScopeMismatch, plan.ConversationID, scope.ConversationID)
	}
	if plan.InteractionID != "" && scope.InteractionID != "" && plan.InteractionID != scope.InteractionID {
		return fmt.Errorf("%w: interaction mismatch: plan=%s scope=%s", decision.ErrActionScopeMismatch, plan.InteractionID, scope.InteractionID)
	}
	return nil
}
