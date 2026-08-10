package interaction

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/extension/kernel"
)

type ObservationBuildScope struct {
	UserID         string
	CharacterID    string
	ConversationID string
	InteractionID  string
}

type ObservationBuildInput struct {
	Plan      *decision.BehaviorPlan
	Directive decision.ActionDirective
	Execution ActionExecutionResult
	Scope     ObservationBuildScope
}

type ObservationBuilder struct{}

func NewObservationBuilder() ObservationBuilder {
	return ObservationBuilder{}
}

func (b ObservationBuilder) Build(input ObservationBuildInput) (*decision.Observation, error) {
	if input.Plan == nil {
		return nil, nil
	}
	if !b.verifyScope(input) {
		return nil, decision.ObservationBuildError{Code: decision.ErrObservationScopeMismatch, Err: fmt.Errorf("observation scope mismatch: plan interaction=%s, scope interaction=%s", safeInteractionID(input.Execution.Action), input.Scope.InteractionID)}
	}

	switch input.Execution.State {
	case ActionExecutionCompleted:
		return b.buildCompleted(input)
	case ActionExecutionSkipped:
		return b.buildSkipped(input)
	case ActionExecutionFailedToDispatch:
		return b.buildDispatchFailure(input)
	default:
		return nil, decision.ObservationBuildError{Code: decision.ErrObservationActionInvalid, Err: fmt.Errorf("unknown action execution state: %s", input.Execution.State)}
	}
}

func (b ObservationBuilder) buildCompleted(input ObservationBuildInput) (*decision.Observation, error) {
	action := input.Execution.Action
	observedAt := input.Execution.CompletedAt
	if observedAt.IsZero() {
		return nil, decision.ObservationBuildError{Code: decision.ErrObservationTimeMissing, Err: fmt.Errorf("completed action missing CompletedAt")}
	}

	switch action.Kind {
	case MaterializedActionRespond:
		return b.buildSkippedNoAction(input, decision.ObservationOutcomeSkipped, observedAt), nil
	case MaterializedActionWait:
		return b.buildSkippedNoAction(input, decision.ObservationOutcomeSkipped, observedAt), nil
	case MaterializedActionTool:
		return b.buildToolResult(input, observedAt)
	default:
		return nil, decision.ObservationBuildError{Code: decision.ErrObservationActionInvalid, Err: fmt.Errorf("unknown materialized action kind: %s", action.Kind)}
	}
}

func (b ObservationBuilder) buildSkipped(input ObservationBuildInput) (*decision.Observation, error) {
	action := input.Execution.Action
	observedAt := input.Execution.CompletedAt
	if observedAt.IsZero() {
		return nil, decision.ObservationBuildError{Code: decision.ErrObservationTimeMissing, Err: fmt.Errorf("skipped action missing CompletedAt")}
	}
	switch action.Kind {
	case MaterializedActionRespond:
		return b.buildSkippedNoAction(input, decision.ObservationOutcomeSkipped, observedAt), nil
	case MaterializedActionWait:
		return b.buildSkippedNoAction(input, decision.ObservationOutcomeSkipped, observedAt), nil
	default:
		return b.buildToolResult(input, observedAt)
	}
}

func (b ObservationBuilder) buildSkippedNoAction(input ObservationBuildInput, outcome decision.ObservationOutcome, observedAt time.Time) *decision.Observation {
	action := input.Execution.Action
	obs := &decision.Observation{
		Version:        decision.ObservationVersionV1,
		ID:             decision.BuildObservationID(action.ID),
		PlanID:         action.PlanID,
		ActionID:       action.ID,
		InteractionID:  action.InteractionID,
		UserID:         input.Scope.UserID,
		CharacterID:    input.Scope.CharacterID,
		ConversationID: input.Scope.ConversationID,
		CandidateID:    action.CandidateID,
		GoalIDs:        copyStringSlice(input.Plan.GoalIDs),
		GoalRefs:       copyGoalRefs(input.Plan.GoalRefs),
		Kind:           decision.ObservationKindNoAction,
		TargetKind:     decision.ObservationTargetNone,
		Outcome:        outcome,
		ObservedAt:     observedAt,
	}
	if obs.InteractionID == "" {
		obs.InteractionID = input.Scope.InteractionID
	}
	return obs
}

func (b ObservationBuilder) buildToolResult(input ObservationBuildInput, observedAt time.Time) (*decision.Observation, error) {
	action := input.Execution.Action
	if action.Tool == nil {
		return nil, decision.ObservationBuildError{Code: decision.ErrObservationToolResultMissing, Err: fmt.Errorf("materialized tool action missing tool binding")}
	}
	toolResult := input.Execution.ToolResult
	if toolResult == nil {
		return nil, decision.ObservationBuildError{Code: decision.ErrObservationToolResultMissing, Err: fmt.Errorf("completed tool action missing ToolResult")}
	}
	normalizedStatus := strings.ToUpper(toolResult.Status)
	switch normalizedStatus {
	case "SUCCESS", "FAILED", "CANCELLED", "TIMED_OUT":
	default:
		return nil, decision.ObservationBuildError{Code: decision.ErrObservationResultInvalid, Err: fmt.Errorf("unknown tool result status: %s", toolResult.Status)}
	}
	if normalizedStatus != "SUCCESS" && toolResult.Error == nil {
		return nil, decision.ObservationBuildError{Code: decision.ErrObservationResultInvalid, Err: fmt.Errorf("failed/cancelled/timed_out tool result missing canonical error")}
	}

	outcome := mapStatusOutcome(normalizedStatus)
	obs := &decision.Observation{
		Version:        decision.ObservationVersionV1,
		ID:             decision.BuildObservationID(action.ID),
		PlanID:         action.PlanID,
		ActionID:       action.ID,
		InteractionID:  safeInteractionID(action),
		UserID:         input.Scope.UserID,
		CharacterID:    input.Scope.CharacterID,
		ConversationID: input.Scope.ConversationID,
		CandidateID:    action.CandidateID,
		GoalIDs:        copyStringSlice(input.Plan.GoalIDs),
		GoalRefs:       copyGoalRefs(input.Plan.GoalRefs),
		Kind:           decision.ObservationKindToolResult,
		TargetKind:     decision.ObservationTargetTool,
		Outcome:        outcome,
		InvocationID:   toolResult.RunID,
		ExternalCallID: action.Tool.ExternalCallID,
		ToolID:         string(action.Tool.ToolID),
		ObservedAt:     observedAt,
	}

	if obs.InteractionID == "" {
		obs.InteractionID = input.Scope.InteractionID
	}
	obs.Evidence = buildEvidence(toolResult)
	if err := decision.ValidateObservation(*obs); err != nil {
		return nil, err
	}
	return obs, nil
}

func (b ObservationBuilder) buildDispatchFailure(input ObservationBuildInput) (*decision.Observation, error) {
	action := input.Execution.Action
	observedAt := input.Execution.CompletedAt
	if observedAt.IsZero() {
		return nil, decision.ObservationBuildError{Code: decision.ErrObservationTimeMissing, Err: fmt.Errorf("failed_to_dispatch action missing CompletedAt")}
	}
	obs := &decision.Observation{
		Version:        decision.ObservationVersionV1,
		ID:             decision.BuildObservationID(action.ID),
		PlanID:         action.PlanID,
		ActionID:       action.ID,
		InteractionID:  safeInteractionID(action),
		UserID:         input.Scope.UserID,
		CharacterID:    input.Scope.CharacterID,
		ConversationID: input.Scope.ConversationID,
		CandidateID:    action.CandidateID,
		GoalIDs:        copyStringSlice(input.Plan.GoalIDs),
		GoalRefs:       copyGoalRefs(input.Plan.GoalRefs),
		Kind:           decision.ObservationKindDispatchFailure,
		TargetKind:     decision.ObservationTargetNone,
		Outcome:        decision.ObservationOutcomeNotDispatched,
		ObservedAt:     observedAt,
	}
	if obs.InteractionID == "" {
		obs.InteractionID = input.Scope.InteractionID
	}
	if action.Tool != nil {
		obs.ToolID = string(action.Tool.ToolID)
		obs.ExternalCallID = action.Tool.ExternalCallID
	}
	obs.Evidence = decision.ObservationEvidence{}
	if input.Execution.Err != nil {
		obs.Evidence.Metadata = map[string]any{"error": input.Execution.Err.Error()}
	}
	return obs, nil
}

func buildEvidence(toolResult *kernel.LegacyToolResult) decision.ObservationEvidence {
	evidence := decision.ObservationEvidence{}
	if toolResult.VisibleText != "" {
		evidence.Contents = append(evidence.Contents, decision.ObservationContent{
			Kind: decision.ObservationContentText,
			Text: toolResult.VisibleText,
		})
	}
	if len(toolResult.Output) > 0 {
		if isJSONStructured(toolResult.Output) {
			evidence.Structured = json.RawMessage(append([]byte(nil), toolResult.Output...))
		} else {
			evidence.Contents = append(evidence.Contents, decision.ObservationContent{
				Kind: decision.ObservationContentText,
				Text: string(toolResult.Output),
			})
		}
	}
	if toolResult.Error != nil {
		evidence.Error = &decision.ObservationError{
			Code:       toolResult.Error.Code,
			Message:    toolResult.Error.Message,
			DomainCode: toolResult.Error.Detail,
			Retryable:  toolResult.Error.Retryable,
		}
	}
	if toolResult.DurationMS > 0 {
		if evidence.Metadata == nil {
			evidence.Metadata = map[string]any{}
		}
		evidence.Metadata["durationMs"] = toolResult.DurationMS
	}
	return evidence
}

func mapStatusOutcome(status string) decision.ObservationOutcome {
	switch status {
	case "SUCCESS":
		return decision.ObservationOutcomeSucceeded
	case "FAILED":
		return decision.ObservationOutcomeFailed
	case "CANCELLED":
		return decision.ObservationOutcomeCancelled
	case "TIMED_OUT":
		return decision.ObservationOutcomeTimedOut
	default:
		return decision.ObservationOutcomeFailed
	}
}

func (ObservationBuilder) verifyScope(input ObservationBuildInput) bool {
	expectedInteraction := input.Scope.InteractionID
	if expectedInteraction == "" {
		return true
	}
	actualInteraction := safeInteractionID(input.Execution.Action)
	if actualInteraction == "" {
		return true
	}
	return actualInteraction == expectedInteraction
}

func safeInteractionID(action MaterializedAction) string {
	return action.InteractionID
}

func copyGoalRefs(src []decision.GoalRef) []decision.GoalRef {
	if len(src) == 0 {
		return nil
	}
	dst := make([]decision.GoalRef, len(src))
	copy(dst, src)
	return dst
}

func copyStringSlice(src []string) []string {
	if len(src) == 0 {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

func isJSONStructured(data []byte) bool {
	trimmed := strings.TrimSpace(string(data))
	if len(trimmed) == 0 {
		return false
	}
	if trimmed[0] == '{' || trimmed[0] == '[' {
		var probe any
		return json.Unmarshal(data, &probe) == nil
	}
	return false
}
