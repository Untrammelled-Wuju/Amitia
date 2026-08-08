package interaction

import (
	"context"
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

func TestRuntimePipelineAssembleIncludesGoals(t *testing.T) {
	registry := decision.NewGoalRegistry()
	now := time.Now().UTC()

	if err := registry.Register(decision.Goal{
		ID:             "long-term-goal",
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Status:         decision.GoalStatusActive,
		Type:           decision.GoalTypeClarification,
		Priority:       decision.GoalPriorityNormal,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatal(err)
	}

	ctxRegistry := NewContextLoaderRegistry()
	p := NewRuntimePipeline(ctxRegistry, NewPathClassifier(), NewTokenBudgetManager(1200))
	p.SetGoalRegistry(registry)
	p.SetDecisionLayer(decision.DefaultCandidateRegistry(), decision.DefaultArbitrationLayer())

	scope := InteractionScope{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		RequestID:      "req-pipeline",
	}
	req := &ProcessRequest{
		InteractionID: "int-pipeline",
		Message:       "你好",
	}

	assembly := p.Assemble(context.Background(), scope, req)

	if assembly.Goals.Current == nil {
		t.Fatal("RuntimeAssembly.Goals.Current不应为nil")
	}
	if assembly.Goals.Current.UserID != "user-1" {
		t.Fatalf("Current.UserID应为user-1, 实际 %s", assembly.Goals.Current.UserID)
	}
	if assembly.Goals.Current.CharacterID != "char-1" {
		t.Fatalf("Current.CharacterID应为char-1, 实际 %s", assembly.Goals.Current.CharacterID)
	}
	if assembly.Goals.Current.ConversationID != "conv-1" {
		t.Fatalf("Current.ConversationID应为conv-1, 实际 %s", assembly.Goals.Current.ConversationID)
	}
	if len(assembly.Goals.Active) != 2 {
		t.Fatalf("应有2个active goals, 实际 %d", len(assembly.Goals.Active))
	}
	if len(assembly.Goals.Intentions) == 0 {
		t.Fatal("不应没有intentions")
	}
}

func TestRuntimePipelineAssembleWithoutGoalRegistry(t *testing.T) {
	ctxRegistry := NewContextLoaderRegistry()
	p := NewRuntimePipeline(ctxRegistry, NewPathClassifier(), NewTokenBudgetManager(1200))

	scope := InteractionScope{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
	}
	req := &ProcessRequest{
		InteractionID: "int-no-registry",
		Message:       "hello",
	}

	assembly := p.Assemble(context.Background(), scope, req)
	if assembly.Goals.Current == nil {
		t.Fatal("即使没有GoalRegistry, Current也不应为nil(因为有InteractionID)")
	}
	if assembly.Goals.Current.ID != "goal:interaction:int-no-registry" {
		t.Fatalf("Current.ID错误: %s", assembly.Goals.Current.ID)
	}
}

func TestRuntimePipelineAssembleWithEmptyInteractionID(t *testing.T) {
	ctxRegistry := NewContextLoaderRegistry()
	p := NewRuntimePipeline(ctxRegistry, NewPathClassifier(), NewTokenBudgetManager(1200))

	scope := InteractionScope{
		UserID:      "user-1",
		CharacterID: "char-1",
	}
	req := &ProcessRequest{
		Message: "no interaction id",
	}

	assembly := p.Assemble(context.Background(), scope, req)
	if assembly.Goals.Current != nil {
		t.Fatal("InteractionID为空时Current应为nil")
	}
}

func TestRuntimePipelineSetGoalRegistryAcceptsNil(t *testing.T) {
	p := NewRuntimePipeline(nil, nil, nil)
	p.SetGoalRegistry(nil)
	if p.goalRegistry != nil {
		t.Fatal("SetGoalRegistry(nil)应设置nil")
	}
}

func TestRuntimePipelineGoalsInDecision(t *testing.T) {
	registry := decision.NewGoalRegistry()
	now := time.Now().UTC()

	if err := registry.Register(decision.Goal{
		ID:          "decision-goal",
		UserID:      "user-1",
		CharacterID: "char-1",
		Status:      decision.GoalStatusActive,
		Type:        decision.GoalTypeClarification,
		Priority:    decision.GoalPriorityNormal,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatal(err)
	}

	ctxRegistry := NewContextLoaderRegistry()
	p := NewRuntimePipeline(ctxRegistry, NewPathClassifier(), NewTokenBudgetManager(1200))
	p.SetGoalRegistry(registry)
	p.SetDecisionLayer(decision.DefaultCandidateRegistry(), decision.DefaultArbitrationLayer())

	scope := InteractionScope{
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
	}
	req := &ProcessRequest{
		InteractionID: "int-decision",
		Message:       "这个需要什么帮助",
	}

	assembly := p.Assemble(context.Background(), scope, req)
	if assembly.BehaviorPlan == nil {
		t.Fatal("应有BehaviorPlan")
	}
	if len(assembly.Goals.Active) < 2 {
		t.Fatalf("Goals.Active应包含current和长期goal, 实际 %d", len(assembly.Goals.Active))
	}
}
