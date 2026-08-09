package decision

import "fmt"

type ActionDirectiveKind string

const (
	ActionDirectiveRespond ActionDirectiveKind = "respond"
	ActionDirectiveWait    ActionDirectiveKind = "wait"
	ActionDirectiveTool    ActionDirectiveKind = "tool"
)

type ActionDirective struct {
	PlanID      string               `json:"planId"`
	CandidateID string               `json:"candidateId"`
	Kind        ActionDirectiveKind  `json:"kind"`
	Required    bool                 `json:"required"`
	Intent      BehaviorPlanIntent   `json:"intent,omitempty"`
	Strategy    BehaviorPlanStrategy `json:"strategy,omitempty"`
}

type ActionMaterializationErrorCode string

const (
	ErrActionPlanInvalid                 ActionMaterializationErrorCode = "ACTION_PLAN_INVALID"
	ErrActionScopeMismatch               ActionMaterializationErrorCode = "ACTION_SCOPE_MISMATCH"
	ErrActionNotAllowedByPlan            ActionMaterializationErrorCode = "ACTION_NOT_ALLOWED_BY_PLAN"
	ErrActionMultipleToolCallsNotAllowed ActionMaterializationErrorCode = "ACTION_MULTIPLE_TOOL_CALLS_NOT_ALLOWED"
	ErrActionExternalCallIDMissing       ActionMaterializationErrorCode = "ACTION_EXTERNAL_CALL_ID_MISSING"
	ErrActionToolNameMissing             ActionMaterializationErrorCode = "ACTION_TOOL_NAME_MISSING"
	ErrActionToolNotFound                ActionMaterializationErrorCode = "ACTION_TOOL_NOT_FOUND"
	ErrActionInputInvalidJSON            ActionMaterializationErrorCode = "ACTION_INPUT_INVALID_JSON"
	ErrActionTargetStale                 ActionMaterializationErrorCode = "ACTION_TARGET_STALE"
)

type ActionDispatchErrorCode string

const (
	ErrActionDispatchUnavailable ActionDispatchErrorCode = "ACTION_DISPATCH_UNAVAILABLE"
)

func (e ActionMaterializationErrorCode) Error() string {
	return string(e)
}

func (e ActionDispatchErrorCode) Error() string {
	return string(e)
}

func BuildActionDirective(plan *BehaviorPlan) (ActionDirective, error) {
	if plan == nil {
		return ActionDirective{}, nil
	}
	if plan.ID == "" {
		return ActionDirective{}, fmt.Errorf("%w: plan ID is required", ErrActionPlanInvalid)
	}
	if plan.Selected.ID == "" {
		return ActionDirective{}, fmt.Errorf("%w: candidate ID is required", ErrActionPlanInvalid)
	}
	if plan.Version != PlanVersionV2 {
		return ActionDirective{}, fmt.Errorf("%w: expected V2 plan, got %s", ErrActionPlanInvalid, plan.Version)
	}

	if plan.SafetyLevel == BehaviorSafetyLevelBlocked {
		return ActionDirective{
			PlanID:      plan.ID,
			CandidateID: plan.Selected.ID,
			Kind:        ActionDirectiveWait,
			Required:    false,
			Intent:      plan.Intent,
			Strategy:    plan.Strategy,
		}, nil
	}

	switch plan.Selected.ActionType {
	case CandidateActionWait:
		return ActionDirective{
			PlanID:      plan.ID,
			CandidateID: plan.Selected.ID,
			Kind:        ActionDirectiveWait,
			Required:    false,
			Intent:      plan.Intent,
			Strategy:    plan.Strategy,
		}, nil
	case CandidateActionToolCall:
		return ActionDirective{
			PlanID:      plan.ID,
			CandidateID: plan.Selected.ID,
			Kind:        ActionDirectiveTool,
			Required:    true,
			Intent:      plan.Intent,
			Strategy:    plan.Strategy,
		}, nil
	}

	return ActionDirective{
		PlanID:      plan.ID,
		CandidateID: plan.Selected.ID,
		Kind:        ActionDirectiveRespond,
		Required:    false,
		Intent:      plan.Intent,
		Strategy:    plan.Strategy,
	}, nil
}
