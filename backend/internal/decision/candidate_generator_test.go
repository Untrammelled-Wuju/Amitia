package decision

import (
	"testing"
	"time"
)

func TestGenerateCandidatesProducesAllActions(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		UserID:      "user-1",
		CharacterID: "char-1",
		Trigger:     GoalTrigger{Kind: GoalTriggerInternal},
		Now:         time.Now().UTC(),
	}

	candidates := GenerateCandidates(ctx, registry)
	if len(candidates) == 0 {
		t.Fatal("应生成至少一些候选")
	}
	for _, c := range candidates {
		if c.ID == "" {
			t.Fatal("所有候选应有 ID")
		}
	}
}

func TestGenerateCandidatesDoesNotScoreCandidates(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		UserID:      "user-1",
		CharacterID: "char-1",
		Goals: []Goal{
			{ID: "g1", Type: GoalTypeConnection, Status: GoalStatusActive, Priority: GoalPriorityHigh},
		},
		Relationship: RelationshipSnapshot{
			Dimensions: map[RelationshipDimension]RelationshipDimensionValue{
				RelationshipTrust: {Value: 0.8},
			},
		},
		Psyche: PsycheSignalSet{
			Stress: ScalarSignal{Value: 0.95},
		},
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
		Now:     time.Now().UTC(),
	}

	candidates := GenerateCandidates(ctx, registry)
	for _, c := range candidates {
		if c.NeedScore != 0 {
			t.Fatalf("Generator 不应写 NeedScore, 实际 %f", c.NeedScore)
		}
		if c.PersonalityScore != 0 {
			t.Fatalf("Generator 不应写 PersonalityScore, 实际 %f", c.PersonalityScore)
		}
		if c.RelationshipScore != 0 {
			t.Fatalf("Generator 不应写 RelationshipScore, 实际 %f", c.RelationshipScore)
		}
		if c.AffectScore != 0 {
			t.Fatalf("Generator 不应写 AffectScore, 实际 %f", c.AffectScore)
		}
		if c.RiskScore != 0 {
			t.Fatalf("Generator 不应写 RiskScore, 实际 %f", c.RiskScore)
		}
		if c.FinalScore != 0 {
			t.Fatalf("Generator 不应写 FinalScore, 实际 %f", c.FinalScore)
		}
		if c.Reasons != nil {
			t.Fatal("Generator 不应写 Reasons")
		}
	}
}

func TestGenerateCandidatesWithExcludes(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		UserID:      "user-1",
		CharacterID: "char-1",
		Trigger:     GoalTrigger{Kind: GoalTriggerInternal},
		Now:         time.Now().UTC(),
	}

	candidates := GenerateCandidatesWithExcludes(ctx, registry, []string{"chat_reply", "ask_clarify"})
	for _, c := range candidates {
		if c.ID == "chat_reply" || c.ID == "ask_clarify" {
			t.Fatalf("被排除的候选不应出现: %s", c.ID)
		}
	}
}

func TestGenerateCandidatesExcludesDuplicateAndUnknown(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		UserID:      "user-1",
		CharacterID: "char-1",
		Trigger:     GoalTrigger{Kind: GoalTriggerInternal},
		Now:         time.Now().UTC(),
	}

	candidates := GenerateCandidatesWithExcludes(ctx, registry, []string{"chat_reply", "chat_reply", "not-exist"})
	if len(candidates) == 0 {
		t.Fatal("排除重复/未知 ID 后仍应有候选")
	}
	for _, c := range candidates {
		if c.ID == "chat_reply" {
			t.Fatal("被排除的候选不应出现")
		}
	}
}

func TestGenerateCandidatesNilRegistry(t *testing.T) {
	ctx := CandidateGenerationContext{
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
		Now:     time.Now().UTC(),
	}
	candidates := GenerateCandidates(ctx, nil)
	if candidates != nil {
		t.Fatal("nil Registry 应返回 nil")
	}
}

func TestGenerateCandidatesTriggerUserMessage(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Trigger: GoalTrigger{Kind: GoalTriggerUserMessage},
		Now:     time.Now().UTC(),
	}

	candidates := GenerateCandidates(ctx, registry)
	hasChatReply := false
	hasProactiveGreet := false
	for _, c := range candidates {
		if c.ID == "chat_reply" {
			hasChatReply = true
		}
		if c.ID == "proactive_greet" {
			hasProactiveGreet = true
		}
	}
	if !hasChatReply {
		t.Fatal("user_message 应包含 chat_reply")
	}
	if hasProactiveGreet {
		t.Fatal("user_message 不应包含 proactive_greet")
	}
}

func TestGenerateCandidatesTriggerProactive(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Trigger: GoalTrigger{Kind: GoalTriggerProactive},
		Now:     time.Now().UTC(),
	}

	candidates := GenerateCandidates(ctx, registry)
	hasProactiveGreet := false
	for _, c := range candidates {
		if c.ID == "proactive_greet" {
			hasProactiveGreet = true
		}
	}
	if !hasProactiveGreet {
		t.Fatal("proactive 应包含 proactive_greet")
	}
}

func TestGenerateCandidatesTriggerVoice(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Trigger: GoalTrigger{Kind: GoalTriggerVoice},
		Now:     time.Now().UTC(),
	}

	candidates := GenerateCandidates(ctx, registry)
	hasChatReply := false
	hasAskClarify := false
	hasOfferSupport := false
	for _, c := range candidates {
		if c.ID == "chat_reply" {
			hasChatReply = true
		}
		if c.ID == "ask_clarify" {
			hasAskClarify = true
		}
		if c.ID == "offer_support" {
			hasOfferSupport = true
		}
	}
	if !hasChatReply || !hasAskClarify || !hasOfferSupport {
		t.Fatal("voice 应包含 chat_reply, ask_clarify, offer_support")
	}
}

func TestGenerateCandidatesPreconditionBoundaryCrossed(t *testing.T) {
	registry := DefaultCandidateRegistry()

	ctxNoAutonomy := CandidateGenerationContext{
		Goals:   []Goal{{ID: "g1", Type: GoalTypeConnection, Status: GoalStatusActive}},
		Trigger: GoalTrigger{Kind: GoalTriggerUserMessage},
		Now:     time.Now().UTC(),
	}
	candidates := GenerateCandidates(ctxNoAutonomy, registry)
	for _, c := range candidates {
		if c.ID == "set_boundary" {
			t.Fatal("无 Autonomy Goal 时不应出现 set_boundary")
		}
	}

	ctxWithAutonomy := CandidateGenerationContext{
		Goals:   []Goal{{ID: "g1", Type: GoalTypeAutonomy, Status: GoalStatusActive}},
		Trigger: GoalTrigger{Kind: GoalTriggerUserMessage},
		Now:     time.Now().UTC(),
	}
	candidates = GenerateCandidates(ctxWithAutonomy, registry)
	hasBoundary := false
	for _, c := range candidates {
		if c.ID == "set_boundary" {
			hasBoundary = true
		}
	}
	if !hasBoundary {
		t.Fatal("有 Autonomy Goal 时应出现 set_boundary")
	}
}

func TestGenerateCandidatesPreconditionInformationGoal(t *testing.T) {
	registry := DefaultCandidateRegistry()

	ctxNoInfo := CandidateGenerationContext{
		Goals:   []Goal{{ID: "g1", Type: GoalTypeConnection, Status: GoalStatusActive}},
		Trigger: GoalTrigger{Kind: GoalTriggerUserMessage},
		Now:     time.Now().UTC(),
	}
	candidates := GenerateCandidates(ctxNoInfo, registry)
	for _, c := range candidates {
		if c.ID == "tool_search" {
			t.Fatal("无 Information Goal 时不应出现 tool_search")
		}
	}

	ctxWithInfo := CandidateGenerationContext{
		Goals:   []Goal{{ID: "g1", Type: GoalTypeInformation, Status: GoalStatusActive}},
		Trigger: GoalTrigger{Kind: GoalTriggerUserMessage},
		Now:     time.Now().UTC(),
	}
	candidates = GenerateCandidates(ctxWithInfo, registry)
	hasToolSearch := false
	for _, c := range candidates {
		if c.ID == "tool_search" {
			hasToolSearch = true
		}
	}
	if !hasToolSearch {
		t.Fatal("有 Information Goal 时应出现 tool_search")
	}
}

func TestGenerateCandidatesStableOrder(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
		Now:     time.Now().UTC(),
	}

	var firstOrder []string
	for i := 0; i < 100; i++ {
		candidates := GenerateCandidates(ctx, registry)
		ids := make([]string, len(candidates))
		for j, c := range candidates {
			ids[j] = c.ID
		}
		if i == 0 {
			firstOrder = ids
		} else {
			for j := range ids {
				if ids[j] != firstOrder[j] {
					t.Fatalf("第 %d 次执行顺序不一致: 位置 %d 期望 %s 实际 %s", i, j, firstOrder[j], ids[j])
				}
			}
		}
	}
}

func TestGenerateCandidatesActionTypeAndChannel(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
		Now:     time.Now().UTC(),
	}

	candidates := GenerateCandidates(ctx, registry)
	for _, c := range candidates {
		if c.ActionType == "" {
			t.Fatalf("候选 %s 的 ActionType 不应为空", c.ID)
		}
		if c.Channel == "" {
			t.Fatalf("候选 %s 的 Channel 不应为空", c.ID)
		}
	}

	tests := []struct {
		id         string
		actionType CandidateActionType
		channel    BehaviorChannel
	}{
		{"chat_reply", CandidateActionChat, BehaviorChannelChat},
		{"proactive_greet", CandidateActionProactive, BehaviorChannelProactive},
		{"wait_observe", CandidateActionWait, BehaviorChannelSystem},
		{"tool_search", CandidateActionToolCall, BehaviorChannelChat},
	}

	candidateMap := make(map[string]BehaviorCandidate)
	for _, c := range candidates {
		candidateMap[c.ID] = c
	}

	for _, tt := range tests {
		c, ok := candidateMap[tt.id]
		if !ok {
			continue
		}
		if c.ActionType != tt.actionType {
			t.Fatalf("%s ActionType 期望 %s, 实际 %s", tt.id, tt.actionType, c.ActionType)
		}
		if c.Channel != tt.channel {
			t.Fatalf("%s Channel 期望 %s, 实际 %s", tt.id, tt.channel, c.Channel)
		}
	}
}

func findCandidate(candidates []BehaviorCandidate, id string) *BehaviorCandidate {
	for i := range candidates {
		if candidates[i].ID == id {
			return &candidates[i]
		}
	}
	return nil
}
