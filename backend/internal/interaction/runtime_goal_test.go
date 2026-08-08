package interaction

import (
	"testing"
	"time"

	"github.com/u-ai/backend/internal/decision"
)

func TestGoalTriggerForUserMessage(t *testing.T) {
	scope := InteractionScope{Source: "web", RequestID: "req-1", SessionID: "sess-1"}
	req := &ProcessRequest{}
	trigger := goalTriggerForRequest(scope, req)
	if trigger.Kind != decision.GoalTriggerUserMessage {
		t.Fatalf("普通用户消息应为user_message, 实际 %s", trigger.Kind)
	}
	if trigger.Source != "web" {
		t.Fatalf("Source应为web, 实际 %s", trigger.Source)
	}
	if trigger.RequestID != "req-1" {
		t.Fatalf("RequestID应为req-1, 实际 %s", trigger.RequestID)
	}
}

func TestGoalTriggerForVoice(t *testing.T) {
	scope := InteractionScope{Source: "voice", RequestID: "req-voice"}
	req := &ProcessRequest{VoiceMessage: true}
	trigger := goalTriggerForRequest(scope, req)
	if trigger.Kind != decision.GoalTriggerVoice {
		t.Fatalf("Voice消息应为voice, 实际 %s", trigger.Kind)
	}
}

func TestGoalTriggerForVoiceBySource(t *testing.T) {
	scope := InteractionScope{Source: "voice"}
	req := &ProcessRequest{}
	trigger := goalTriggerForRequest(scope, req)
	if trigger.Kind != decision.GoalTriggerVoice {
		t.Fatalf("Source=voice应为voice, 实际 %s", trigger.Kind)
	}
}

func TestGoalTriggerForProactive(t *testing.T) {
	scope := InteractionScope{Source: "proactive"}
	req := &ProcessRequest{ProactiveTaskInstruction: "do something"}
	trigger := goalTriggerForRequest(scope, req)
	if trigger.Kind != decision.GoalTriggerProactive {
		t.Fatalf("ProactiveTaskInstruction非空应为proactive, 实际 %s", trigger.Kind)
	}
}

func TestGoalTriggerForInternal(t *testing.T) {
	scope := InteractionScope{}
	req := &ProcessRequest{IsInternal: true}
	trigger := goalTriggerForRequest(scope, req)
	if trigger.Kind != decision.GoalTriggerInternal {
		t.Fatalf("IsInternal=true应为internal, 实际 %s", trigger.Kind)
	}
}

func TestCurrentInteractionGoalID(t *testing.T) {
	now := time.Now().UTC()
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1"}
	req := &ProcessRequest{InteractionID: "int-123"}
	appraisal := &AppraisalResult{EventType: "chat"}

	goal := buildCurrentInteractionGoal(scope, req, appraisal, now)
	if goal == nil {
		t.Fatal("不应返回nil")
	}
	if goal.ID != "goal:interaction:int-123" {
		t.Fatalf("Goal ID应为goal:interaction:int-123, 实际 %s", goal.ID)
	}
	if goal.UserID != "user-1" {
		t.Fatalf("UserID应为user-1, 实际 %s", goal.UserID)
	}
	if goal.CharacterID != "char-1" {
		t.Fatalf("CharacterID应为char-1, 实际 %s", goal.CharacterID)
	}
	if goal.ConversationID != "conv-1" {
		t.Fatalf("ConversationID应为conv-1, 实际 %s", goal.ConversationID)
	}
	if goal.Trigger.InteractionID != "int-123" {
		t.Fatalf("Trigger.InteractionID应为int-123, 实际 %s", goal.Trigger.InteractionID)
	}
}

func TestCurrentInteractionGoalNilWhenNoInteractionID(t *testing.T) {
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1"}
	req := &ProcessRequest{}

	goal := buildCurrentInteractionGoal(scope, req, nil, time.Now().UTC())
	if goal != nil {
		t.Fatal("InteractionID为空时应返回nil")
	}
}

func TestGoalTypeMapping(t *testing.T) {
	tests := []struct {
		eventType string
		expected  decision.GoalType
	}{
		{"help", decision.GoalTypeSupport},
		{"boundary_cross", decision.GoalTypeAutonomy},
		{"complaint", decision.GoalTypeConflictRepair},
		{"apology", decision.GoalTypeConflictRepair},
		{"cold", decision.GoalTypeConnection},
		{"emotional", decision.GoalTypeSupport},
		{"praise", decision.GoalTypeConnection},
		{"chat", decision.GoalTypeConnection},
	}

	for _, tt := range tests {
		appraisal := &AppraisalResult{EventType: tt.eventType}
		result := goalTypeForInteraction(appraisal)
		if result != tt.expected {
			t.Fatalf("EventType=%s 期望 %s, 实际 %s", tt.eventType, tt.expected, result)
		}
	}

	defaultResult := goalTypeForInteraction(nil)
	if defaultResult != decision.GoalTypeConnection {
		t.Fatalf("nil appraisal 应返回connection, 实际 %s", defaultResult)
	}
}

func TestGoalDescriptionDoesNotContainRawMessage(t *testing.T) {
	now := time.Now().UTC()
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1"}
	req := &ProcessRequest{InteractionID: "int-456", Message: "这条消息不应该出现在Goal中"}

	tests := []string{"help", "boundary_cross", "complaint", "apology", "cold", "emotional", "praise", "chat"}
	for _, eventType := range tests {
		appraisal := &AppraisalResult{EventType: eventType}
		goal := buildCurrentInteractionGoal(scope, req, appraisal, now)
		if goal == nil {
			continue
		}
		if goal.Description == req.Message {
			t.Fatalf("EventType=%s: Goal.Description不应包含原始消息", eventType)
		}
		if goal.Metadata != nil {
			if _, hasMessage := goal.Metadata["raw_message"]; hasMessage {
				t.Fatal("Goal.Metadata不应包含raw_message")
			}
		}
	}
}

func TestBuildGoalContextIncludesCurrentAndActive(t *testing.T) {
	registry := decision.NewGoalRegistry()
	now := time.Now().UTC()

	if err := registry.Register(decision.Goal{
		ID:          "long-term-1",
		UserID:      "user-1",
		CharacterID: "char-1",
		Status:      decision.GoalStatusActive,
		Type:        decision.GoalTypeSupport,
		Priority:    decision.GoalPriorityHigh,
		CreatedAt:   now,
		UpdatedAt:   now,
	}); err != nil {
		t.Fatal(err)
	}

	p := &RuntimePipeline{goalRegistry: registry}
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1"}
	req := &ProcessRequest{InteractionID: "int-789"}
	appraisal := &AppraisalResult{EventType: "help"}

	ctx := p.buildGoalContext(scope, req, appraisal, now)
	if ctx.Current == nil {
		t.Fatal("Current goal不应为nil")
	}
	if ctx.Current.ID != "goal:interaction:int-789" {
		t.Fatalf("Current.ID错误: %s", ctx.Current.ID)
	}
	if len(ctx.Active) != 2 {
		t.Fatalf("应有2个active goals(current+long-term), 实际 %d", len(ctx.Active))
	}
	if len(ctx.Intentions) == 0 {
		t.Fatal("不应没有intentions")
	}
}

func TestBuildGoalContextDeduplicatesCurrentGoal(t *testing.T) {
	registry := decision.NewGoalRegistry()
	now := time.Now().UTC()

	if err := registry.Register(decision.Goal{
		ID:             "goal:interaction:int-dup",
		UserID:         "user-1",
		CharacterID:    "char-1",
		ConversationID: "conv-1",
		Status:         decision.GoalStatusActive,
		CreatedAt:      now,
		UpdatedAt:      now,
	}); err != nil {
		t.Fatal(err)
	}

	p := &RuntimePipeline{goalRegistry: registry}
	scope := InteractionScope{UserID: "user-1", CharacterID: "char-1", ConversationID: "conv-1"}
	req := &ProcessRequest{InteractionID: "int-dup"}

	ctx := p.buildGoalContext(scope, req, nil, now)
	if ctx.Current == nil {
		t.Fatal("Current不应为nil")
	}
	count := 0
	for _, g := range ctx.Active {
		if g.ID == "goal:interaction:int-dup" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("当前Goal不应重复, 出现次数: %d", count)
	}
}

func TestGoalsForDecisionReturnsCurrentWhenNoActive(t *testing.T) {
	goalCtx := RuntimeGoalContext{
		Current: &decision.Goal{ID: "current-only"},
	}
	goals := goalsForDecision(goalCtx)
	if len(goals) != 1 || goals[0].ID != "current-only" {
		t.Fatal("没有active时应返回current")
	}
}

func TestGoalsForDecisionReturnsEmptyWhenNil(t *testing.T) {
	goals := goalsForDecision(RuntimeGoalContext{})
	if goals != nil {
		t.Fatal("空context应返回nil")
	}
}
