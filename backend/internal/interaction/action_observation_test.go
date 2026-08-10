package interaction

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/extension/kernel"
)

func TestBuildReturnsNilForNilPlan(t *testing.T) {
	b := NewObservationBuilder()
	obs, err := b.Build(ObservationBuildInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs != nil {
		t.Fatalf("expected nil observation for nil plan")
	}
}

func TestBuildRejectsScopeMismatch(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{
		ID:            "a1",
		PlanID:        "p1",
		InteractionID: "I1",
		Kind:          MaterializedActionTool,
	}
	exec := ActionExecutionResult{Action: action, State: ActionExecutionSkipped, CompletedAt: time.Now().UTC()}
	obs, err := b.Build(ObservationBuildInput{
		Plan:      plan,
		Execution: exec,
		Scope:     ObservationBuildScope{InteractionID: "I2"},
	})
	if err == nil {
		t.Fatal("expected scope mismatch error")
	}
	if obs != nil {
		t.Fatalf("expected nil observation on scope mismatch")
	}
	if !strings.Contains(err.Error(), string(decision.ErrObservationScopeMismatch)) {
		t.Fatalf("expected ErrObservationScopeMismatch, got %v", err)
	}
}

func TestBuildToolSuccess(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1", GoalIDs: []string{"g1"}}
	now := time.Now().UTC()
	action := MaterializedAction{
		ID:            "a1",
		PlanID:        "p1",
		InteractionID: "I1",
		Kind:          MaterializedActionTool,
		Tool:          &MaterializedToolAction{ToolID: "builtin/browser/search", ExternalCallID: "call-1"},
	}
	exec := ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionCompleted,
		CompletedAt: now,
		ToolResult: &kernel.LegacyToolResult{
			RunID:       "inv-1",
			Status:      "SUCCESS",
			VisibleText: "search result",
			Output:      json.RawMessage(`{"count":5}`),
			DurationMS:  100,
		},
	}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{InteractionID: "I1", UserID: "u-1", CharacterID: "c-1", ConversationID: "conv-1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs == nil {
		t.Fatal("expected non-nil observation")
	}
	if obs.Kind != decision.ObservationKindToolResult {
		t.Fatalf("expected tool_result, got %s", obs.Kind)
	}
	if obs.Outcome != decision.ObservationOutcomeSucceeded {
		t.Fatalf("expected succeeded, got %s", obs.Outcome)
	}
	if obs.ToolID != "builtin/browser/search" {
		t.Fatalf("expected correct toolId, got %s", obs.ToolID)
	}
	if obs.InvocationID != "inv-1" {
		t.Fatalf("expected invocation id inv-1, got %s", obs.InvocationID)
	}
	if obs.ExternalCallID != "call-1" {
		t.Fatalf("expected external call id call-1")
	}
	if len(obs.Evidence.Contents) != 1 || obs.Evidence.Contents[0].Text != "search result" {
		t.Fatalf("expected visible text in contents, got %+v", obs.Evidence.Contents)
	}
	if len(obs.Evidence.Structured) == 0 {
		t.Fatalf("expected structured evidence")
	}
	if len(obs.GoalIDs) != 1 || obs.GoalIDs[0] != "g1" {
		t.Fatalf("expected goal ids preserved")
	}
	expectedID := decision.BuildObservationID("a1")
	if obs.ID != expectedID {
		t.Fatalf("expected observation id %s, got %s", expectedID, obs.ID)
	}
}

func TestBuildIDStable(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	for i := 0; i < 100; i++ {
		action := MaterializedAction{ID: "action-same", PlanID: "p1", Kind: MaterializedActionWait}
		exec := ActionExecutionResult{Action: action, State: ActionExecutionSkipped, CompletedAt: time.Now().UTC()}
		obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if obs.ID != decision.BuildObservationID("action-same") {
			t.Fatalf("id differs at iteration %d", i)
		}
	}
}

func TestBuildIDDiffersAcrossActions(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	ids := map[string]bool{}
	for _, id := range []string{"a1", "a2", "a3"} {
		action := MaterializedAction{ID: id, PlanID: "p1", Kind: MaterializedActionWait}
		exec := ActionExecutionResult{Action: action, State: ActionExecutionSkipped, CompletedAt: time.Now().UTC()}
		obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ids[obs.ID] = true
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 distinct ids, got %d", len(ids))
	}
}

func TestBuildRespondNoAction(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionRespond}
	exec := ActionExecutionResult{Action: action, State: ActionExecutionSkipped, CompletedAt: time.Now().UTC()}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.Kind != decision.ObservationKindNoAction {
		t.Fatalf("expected no_action, got %s", obs.Kind)
	}
	if obs.Outcome != decision.ObservationOutcomeSkipped {
		t.Fatalf("expected skipped, got %s", obs.Outcome)
	}
	if obs.TargetKind != decision.ObservationTargetNone {
		t.Fatalf("expected targetKind=none, got %s", obs.TargetKind)
	}
	if obs.ToolID != "" || obs.InvocationID != "" {
		t.Fatalf("no_action should not set toolId or invocationId")
	}
}

func TestBuildWaitNoAction(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionWait}
	exec := ActionExecutionResult{Action: action, State: ActionExecutionSkipped, CompletedAt: time.Now().UTC()}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.Kind != decision.ObservationKindNoAction {
		t.Fatalf("expected no_action, got %s", obs.Kind)
	}
	if obs.Outcome != decision.ObservationOutcomeSkipped {
		t.Fatalf("expected skipped, got %s", obs.Outcome)
	}
}

func TestBuildToolFailed(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", InteractionID: "I1", Kind: MaterializedActionTool, Tool: &MaterializedToolAction{ToolID: "builtin/x/1"}}
	exec := ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionCompleted,
		CompletedAt: time.Now().UTC(),
		ToolResult: &kernel.LegacyToolResult{
			RunID:  "inv-1",
			Status: "FAILED",
			Error:  &kernel.LegacyToolError{Code: "PROVIDER_ERROR", Message: "provider failed", Retryable: false},
		},
	}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{InteractionID: "I1", ConversationID: "c1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.Outcome != decision.ObservationOutcomeFailed {
		t.Fatalf("expected failed, got %s", obs.Outcome)
	}
	if obs.Evidence.Error == nil || obs.Evidence.Error.Code != "PROVIDER_ERROR" {
		t.Fatalf("expected error to be preserved, got %+v", obs.Evidence.Error)
	}
}

func TestBuildToolCancelled(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", InteractionID: "I1", Kind: MaterializedActionTool, Tool: &MaterializedToolAction{ToolID: "builtin/x/1"}}
	exec := ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionCompleted,
		CompletedAt: time.Now().UTC(),
		ToolResult: &kernel.LegacyToolResult{
			RunID:       "inv-1",
			Status:      "CANCELLED",
			Error:       &kernel.LegacyToolError{Code: "CANCELLED", Message: "user cancelled"},
			VisibleText: "partial output",
		},
	}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{InteractionID: "I1", ConversationID: "c1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.Outcome != decision.ObservationOutcomeCancelled {
		t.Fatalf("expected cancelled, got %s", obs.Outcome)
	}
	if len(obs.Evidence.Contents) != 1 || obs.Evidence.Contents[0].Text != "partial output" {
		t.Fatalf("expected text preserved on cancelled")
	}
}

func TestBuildToolTimedOut(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", InteractionID: "I1", Kind: MaterializedActionTool, Tool: &MaterializedToolAction{ToolID: "builtin/x/1"}}
	exec := ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionCompleted,
		CompletedAt: time.Now().UTC(),
		ToolResult: &kernel.LegacyToolResult{
			RunID:  "inv-1",
			Status: "TIMED_OUT",
			Error:  &kernel.LegacyToolError{Code: "TIMEOUT", Message: "timed out"},
		},
	}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{InteractionID: "I1", ConversationID: "c1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.Outcome != decision.ObservationOutcomeTimedOut {
		t.Fatalf("expected timed_out, got %s", obs.Outcome)
	}
}

func TestBuildDispatchFailure(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionTool, Tool: &MaterializedToolAction{ToolID: "nonexistent/tool", ExternalCallID: "call-x"}}
	exec := ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionFailedToDispatch,
		CompletedAt: time.Now().UTC(),
		Err:         fmt.Errorf("tool not found"),
	}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.Kind != decision.ObservationKindDispatchFailure {
		t.Fatalf("expected dispatch_failure, got %s", obs.Kind)
	}
	if obs.Outcome != decision.ObservationOutcomeNotDispatched {
		t.Fatalf("expected not_dispatched, got %s", obs.Outcome)
	}
	if obs.ToolID != "nonexistent/tool" {
		t.Fatalf("expected toolId from materialized action")
	}
	if obs.Evidence.Error != nil {
		t.Fatalf("dispatch failure should not set Evidence.Error since no kernel invocation")
	}
}

func TestBuildToolResultMissing(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionTool, Tool: &MaterializedToolAction{ToolID: "builtin/x/1"}}
	exec := ActionExecutionResult{Action: action, State: ActionExecutionCompleted, CompletedAt: time.Now().UTC()}
	_, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
	if err == nil {
		t.Fatal("expected error for nil tool result")
	}
}

func TestBuildUnknownStatus(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionTool, Tool: &MaterializedToolAction{ToolID: "builtin/x/1"}}
	exec := ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionCompleted,
		CompletedAt: time.Now().UTC(),
		ToolResult:  &kernel.LegacyToolResult{Status: "WTF", Error: &kernel.LegacyToolError{Code: "X"}},
	}
	_, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
	if err == nil {
		t.Fatal("expected error for unknown status")
	}
}

func TestBuildMissingCompletedAt(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionWait}
	exec := ActionExecutionResult{Action: action, State: ActionExecutionSkipped}
	_, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
	if err == nil {
		t.Fatal("expected error for missing CompletedAt")
	}
	if !strings.Contains(err.Error(), string(decision.ErrObservationTimeMissing)) {
		t.Fatalf("expected ErrObservationTimeMissing, got %v", err)
	}
}

func TestBuildDeepCopyEvidence(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	original := json.RawMessage(`{"key":"value"}`)
	action := MaterializedAction{ID: "a1", PlanID: "p1", InteractionID: "I1", Kind: MaterializedActionTool, Tool: &MaterializedToolAction{ToolID: "builtin/x/1"}}
	exec := ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionCompleted,
		CompletedAt: time.Now().UTC(),
		ToolResult:  &kernel.LegacyToolResult{Status: "SUCCESS", Output: original, RunID: "inv-1"},
	}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{InteractionID: "I1", ConversationID: "c1"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(obs.Evidence.Structured) == 0 {
		t.Skip("output may be stored as content instead of structured")
	}
	original[2] = 'X'
	if obs.Evidence.Structured[2] == 'X' {
		t.Fatalf("evidence was not deep copied")
	}
}

func TestBuildDeepCopyGoalIDs(t *testing.T) {
	b := NewObservationBuilder()
	goals := []string{"g1", "g2"}
	plan := &decision.BehaviorPlan{ID: "p1", GoalIDs: goals}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionWait}
	exec := ActionExecutionResult{Action: action, State: ActionExecutionSkipped, CompletedAt: time.Now().UTC()}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	goals[0] = "mutated"
	if obs.GoalIDs[0] == "mutated" {
		t.Fatalf("goal ids were not deep copied")
	}
}

func TestBuildPreservesAllScopeFields(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionWait}
	exec := ActionExecutionResult{Action: action, State: ActionExecutionSkipped, CompletedAt: time.Now().UTC()}
	scope := ObservationBuildScope{
		UserID:         "u-1",
		CharacterID:    "c-1",
		ConversationID: "conv-1",
		InteractionID:  "I1",
	}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: scope})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.UserID != "u-1" || obs.CharacterID != "c-1" || obs.ConversationID != "conv-1" || obs.InteractionID != "I1" {
		t.Fatalf("expected scope fields preserved, got %+v", obs)
	}
}

func TestBuildMaterializationFailureAsDispatch(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionTool}
	exec := ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionFailedToDispatch,
		CompletedAt: time.Now().UTC(),
		ErrCode:     "ACTION_TOOL_NOT_FOUND",
	}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.Kind != decision.ObservationKindDispatchFailure {
		t.Fatalf("expected dispatch_failure, got %s", obs.Kind)
	}
}

func TestBuildEmptyInteractionFallsBackToScope(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionWait}
	exec := ActionExecutionResult{Action: action, State: ActionExecutionSkipped, CompletedAt: time.Now().UTC()}
	obs, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{InteractionID: "I-scope"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if obs.InteractionID != "I-scope" {
		t.Fatalf("expected InteractionID to fall back to scope, got %s", obs.InteractionID)
	}
}

func TestBuildMissingErrorForFailedStatus(t *testing.T) {
	b := NewObservationBuilder()
	plan := &decision.BehaviorPlan{ID: "p1"}
	action := MaterializedAction{ID: "a1", PlanID: "p1", Kind: MaterializedActionTool, Tool: &MaterializedToolAction{ToolID: "x/1"}}
	exec := ActionExecutionResult{
		Action:      action,
		State:       ActionExecutionCompleted,
		CompletedAt: time.Now().UTC(),
		ToolResult:  &kernel.LegacyToolResult{Status: "FAILED"},
	}
	_, err := b.Build(ObservationBuildInput{Plan: plan, Execution: exec, Scope: ObservationBuildScope{}})
	if err == nil || !strings.Contains(err.Error(), string(decision.ErrObservationResultInvalid)) {
		t.Fatalf("expected ErrObservationResultInvalid for failed without error, got %v", err)
	}
}
