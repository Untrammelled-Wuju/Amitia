package interaction

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
	"github.com/u-ai/backend/internal/extension/kernel"
)

type fakeToolCatalog struct {
	tools map[string]kernel.ResolvedToolReference
}

func (f fakeToolCatalog) ResolveModelTool(modelName string) (kernel.ResolvedToolReference, error) {
	r, ok := f.tools[modelName]
	if !ok {
		return kernel.ResolvedToolReference{}, errors.New("not found")
	}
	return r, nil
}

func TestBuildActionIDIsDeterministic(t *testing.T) {
	id1 := BuildActionID("plan-a", "call_1")
	id2 := BuildActionID("plan-a", "call_1")
	if id1 != id2 {
		t.Fatalf("action id should be deterministic: %s vs %s", id1, id2)
	}
	if id1 == "" || id1[:6] != "action" {
		t.Fatalf("action id should start with 'action:', got %s", id1)
	}
}

func TestBuildActionIDChangesWithDifferentPlan(t *testing.T) {
	id1 := BuildActionID("plan-a", "call_1")
	id2 := BuildActionID("plan-b", "call_1")
	if id1 == id2 {
		t.Fatal("different plans should produce different action ids")
	}
}

func TestMaterializeReturnsNoActionForNilPlan(t *testing.T) {
	cat := fakeToolCatalog{tools: map[string]kernel.ResolvedToolReference{}}
	m := NewActionMaterializer(cat)
	outcome := m.Materialize(nil, nil, decision.ActionDirective{}, nil, ActionMaterializationScope{}, time.Now())
	if outcome.State != ActionMaterializationNoAction {
		t.Fatalf("expected no_action for nil plan, got %s", outcome.State)
	}
}

func TestMaterializeRejectsMultipleToolCalls(t *testing.T) {
	plan := &decision.BehaviorPlan{
		ID:       "plan-1",
		Version:  decision.PlanVersionV2,
		Selected: decision.BehaviorCandidate{ID: "tool_search", ActionType: decision.CandidateActionToolCall},
	}
	directive := decision.ActionDirective{Kind: decision.ActionDirectiveTool, PlanID: "plan-1"}
	calls := []ModelToolCall{
		{ID: "call_1", Name: "browser_search", Arguments: json.RawMessage(`{}`)},
		{ID: "call_2", Name: "browser_search", Arguments: json.RawMessage(`{}`)},
	}
	cat := fakeToolCatalog{tools: map[string]kernel.ResolvedToolReference{"browser_search": {ID: "builtin/browser/search"}}}
	m := NewActionMaterializer(cat)
	outcome := m.Materialize(nil, plan, directive, calls, ActionMaterializationScope{}, time.Now())
	if outcome.State != ActionMaterializationNoAction {
		t.Fatalf("expected no_action for multiple calls, got %s", outcome.State)
	}
	if outcome.ErrCode != decision.ErrActionMultipleToolCallsNotAllowed {
		t.Fatalf("expected MULTIPLE_TOOL_CALLS_NOT_ALLOWED, got %v", outcome.ErrCode)
	}
}

func TestMaterializeToolNotProducedForEmptyCalls(t *testing.T) {
	plan := &decision.BehaviorPlan{
		ID:       "plan-2",
		Version:  decision.PlanVersionV2,
		Selected: decision.BehaviorCandidate{ID: "tool_search", ActionType: decision.CandidateActionToolCall},
	}
	directive := decision.ActionDirective{Kind: decision.ActionDirectiveTool, PlanID: "plan-2"}
	cat := fakeToolCatalog{tools: map[string]kernel.ResolvedToolReference{}}
	m := NewActionMaterializer(cat)
	outcome := m.Materialize(nil, plan, directive, nil, ActionMaterializationScope{}, time.Now())
	if outcome.State != ActionMaterializationToolNotProduced {
		t.Fatalf("expected tool_not_produced, got %s", outcome.State)
	}
}

func TestMaterializeToolCallClonesInput(t *testing.T) {
	plan := &decision.BehaviorPlan{
		ID:       "plan-3",
		Version:  decision.PlanVersionV2,
		Selected: decision.BehaviorCandidate{ID: "tool_search", ActionType: decision.CandidateActionToolCall},
	}
	directive := decision.ActionDirective{Kind: decision.ActionDirectiveTool, PlanID: "plan-3"}
	args := json.RawMessage(`{"query":"test"}`)
	calls := []ModelToolCall{{ID: "call_x", Name: "browser_search", Arguments: args}}
	cat := fakeToolCatalog{tools: map[string]kernel.ResolvedToolReference{
		"browser_search": {ID: "builtin/browser/search", ModelName: "browser_search"},
	}}
	m := NewActionMaterializer(cat)
	outcome := m.Materialize(nil, plan, directive, calls, ActionMaterializationScope{InteractionID: "i-1"}, time.Now())
	if outcome.State != ActionMaterializationReady {
		t.Fatalf("expected ready, got %s err=%v", outcome.State, outcome.Err)
	}
	if outcome.Action == nil {
		t.Fatal("expected action")
	}
	if outcome.Action.Tool == nil {
		t.Fatal("expected tool action")
	}
	if outcome.Action.Tool.ToolID != "builtin/browser/search" {
		t.Fatalf("expected canonical id, got %s", outcome.Action.Tool.ToolID)
	}
	if outcome.Action.Tool.ExternalCallID != "call_x" {
		t.Fatalf("external call id mismatch: %s", outcome.Action.Tool.ExternalCallID)
	}
	args[0] = 'X'
	if outcome.Action.Tool.Input[0] != '{' {
		t.Fatal("input should be cloned, not shared")
	}
}

func TestMaterializeToolCallRejectsMissingCallID(t *testing.T) {
	plan := &decision.BehaviorPlan{
		ID:       "plan-4",
		Version:  decision.PlanVersionV2,
		Selected: decision.BehaviorCandidate{ID: "tool_search", ActionType: decision.CandidateActionToolCall},
	}
	directive := decision.ActionDirective{Kind: decision.ActionDirectiveTool, PlanID: "plan-4"}
	calls := []ModelToolCall{{Name: "browser_search", Arguments: json.RawMessage(`{}`)}}
	cat := fakeToolCatalog{tools: map[string]kernel.ResolvedToolReference{"browser_search": {ID: "b"}}}
	m := NewActionMaterializer(cat)
	outcome := m.Materialize(nil, plan, directive, calls, ActionMaterializationScope{}, time.Now())
	if outcome.ErrCode != decision.ErrActionExternalCallIDMissing {
		t.Fatalf("expected EXTERNAL_CALL_ID_MISSING, got %v", outcome.ErrCode)
	}
}

func TestMaterializeToolCallRejectsInvalidJSON(t *testing.T) {
	plan := &decision.BehaviorPlan{
		ID:       "plan-5",
		Version:  decision.PlanVersionV2,
		Selected: decision.BehaviorCandidate{ID: "tool_search", ActionType: decision.CandidateActionToolCall},
	}
	directive := decision.ActionDirective{Kind: decision.ActionDirectiveTool, PlanID: "plan-5"}
	calls := []ModelToolCall{{ID: "c1", Name: "browser_search", Arguments: json.RawMessage(`{bad`)}}
	cat := fakeToolCatalog{tools: map[string]kernel.ResolvedToolReference{"browser_search": {ID: "b"}}}
	m := NewActionMaterializer(cat)
	outcome := m.Materialize(nil, plan, directive, calls, ActionMaterializationScope{}, time.Now())
	if outcome.ErrCode != decision.ErrActionInputInvalidJSON {
		t.Fatalf("expected INPUT_INVALID_JSON, got %v", outcome.ErrCode)
	}
}

func TestMaterializeRespondActionHasNoTool(t *testing.T) {
	plan := &decision.BehaviorPlan{
		ID:       "plan-6",
		Version:  decision.PlanVersionV2,
		Selected: decision.BehaviorCandidate{ID: "chat_reply", ActionType: decision.CandidateActionChat},
	}
	directive := decision.ActionDirective{Kind: decision.ActionDirectiveRespond, PlanID: "plan-6"}
	cat := fakeToolCatalog{tools: map[string]kernel.ResolvedToolReference{}}
	m := NewActionMaterializer(cat)
	outcome := m.Materialize(nil, plan, directive, nil, ActionMaterializationScope{InteractionID: "i-2"}, time.Now())
	if outcome.State != ActionMaterializationReady {
		t.Fatalf("expected ready, got %s", outcome.State)
	}
	if outcome.Action.Kind != MaterializedActionRespond {
		t.Fatalf("expected respond action, got %s", outcome.Action.Kind)
	}
	if outcome.Action.Tool != nil {
		t.Fatal("respond action should not have tool")
	}
	if outcome.Action.PlanID != "plan-6" {
		t.Fatal("plan id mismatch")
	}
}

func TestMaterializeScopeMismatch(t *testing.T) {
	plan := &decision.BehaviorPlan{
		ID:             "plan-7",
		Version:        decision.PlanVersionV2,
		UserID:         "user-a",
		ConversationID: "conv-a",
		InteractionID:  "intr-a",
		Selected:       decision.BehaviorCandidate{ID: "chat_reply", ActionType: decision.CandidateActionChat},
	}
	directive := decision.ActionDirective{Kind: decision.ActionDirectiveRespond, PlanID: "plan-7"}
	cat := fakeToolCatalog{tools: map[string]kernel.ResolvedToolReference{}}
	m := NewActionMaterializer(cat)
	scope := ActionMaterializationScope{
		UserID:         "user-b",
		ConversationID: "conv-b",
		InteractionID:  "intr-b",
	}
	outcome := m.Materialize(nil, plan, directive, nil, scope, time.Now())
	if outcome.ErrCode != decision.ErrActionScopeMismatch {
		t.Fatalf("expected SCOPE_MISMATCH, got %v", outcome.ErrCode)
	}
}

func TestMaterializeCannotBypassPlanWithTool(t *testing.T) {
	plan := &decision.BehaviorPlan{
		ID:       "plan-8",
		Version:  decision.PlanVersionV2,
		Selected: decision.BehaviorCandidate{ID: "chat_reply", ActionType: decision.CandidateActionChat},
	}
	directive := decision.ActionDirective{Kind: decision.ActionDirectiveRespond, PlanID: "plan-8"}
	calls := []ModelToolCall{{ID: "c1", Name: "browser_search", Arguments: json.RawMessage(`{}`)}}
	cat := fakeToolCatalog{tools: map[string]kernel.ResolvedToolReference{"browser_search": {ID: "b"}}}
	m := NewActionMaterializer(cat)
	outcome := m.Materialize(nil, plan, directive, calls, ActionMaterializationScope{}, time.Now())
	if outcome.State != ActionMaterializationReady {
		t.Fatalf("expected ready, got %s", outcome.State)
	}
	if outcome.Action.Kind != MaterializedActionRespond {
		t.Fatal("model tool call should be silently ignored when plan says respond")
	}
}

func TestMaterializeToolNotFound(t *testing.T) {
	plan := &decision.BehaviorPlan{
		ID:       "plan-9",
		Version:  decision.PlanVersionV2,
		Selected: decision.BehaviorCandidate{ID: "tool_search", ActionType: decision.CandidateActionToolCall},
	}
	directive := decision.ActionDirective{Kind: decision.ActionDirectiveTool, PlanID: "plan-9"}
	calls := []ModelToolCall{{ID: "c1", Name: "unknown_tool", Arguments: json.RawMessage(`{}`)}}
	cat := fakeToolCatalog{tools: map[string]kernel.ResolvedToolReference{}}
	m := NewActionMaterializer(cat)
	outcome := m.Materialize(nil, plan, directive, calls, ActionMaterializationScope{}, time.Now())
	if outcome.ErrCode != decision.ErrActionToolNotFound {
		t.Fatalf("expected TOOL_NOT_FOUND, got %v", outcome.ErrCode)
	}
}
