package decision

import (
	"testing"
	"time"
)

func TestApplyCandidateContextSignalsScoring(t *testing.T) {
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
			Stress: ScalarSignal{Value: 0.3},
		},
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
		Now:     time.Now().UTC(),
	}

	candidates := GenerateCandidates(ctx, registry)
	scored := ApplyCandidateContextSignals(candidates, ctx)

	chatCandidate := findCandidate(scored, "chat_reply")
	if chatCandidate == nil {
		t.Fatal("chat_reply 候选丢失")
	}
	if chatCandidate.NeedScore <= 0 {
		t.Fatal("Connection 目标应为 chat_reply 增加 NeedScore")
	}
}

func TestApplyCandidateContextSignalsHighStressAffectsProactive(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Now: time.Now().UTC(),
		Psyche: PsycheSignalSet{
			Stress: ScalarSignal{Value: 0.85},
		},
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
	}

	candidates := GenerateCandidates(ctx, registry)
	scored := ApplyCandidateContextSignals(candidates, ctx)

	proactive := findCandidate(scored, "proactive_greet")
	if proactive == nil {
		t.Fatal("proactive_greet 候选丢失")
	}
	if proactive.RiskScore <= 0 {
		t.Fatal("高压状态下 proactive 应有风险分")
	}
}

func TestApplyCandidateContextSignalsBusyLifeBlocksProactive(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Now:     time.Now().UTC(),
		Life:    LifeSnapshot{Busy: 0.95, Energy: 0.5},
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
	}

	candidates := GenerateCandidates(ctx, registry)
	scored := ApplyCandidateContextSignals(candidates, ctx)

	proactive := findCandidate(scored, "proactive_greet")
	if proactive == nil {
		t.Fatal("proactive_greet 候选丢失")
	}
	hasBusyBlock := false
	for _, c := range proactive.Constraints {
		if c.Kind == "busy_block" && c.Hard {
			hasBusyBlock = true
		}
	}
	if !hasBusyBlock {
		t.Fatal("高忙状态应为 proactive 添加 hard 约束")
	}
}

func TestApplyCandidateContextSignalsIntentionBoostsGoalRelatedCandidate(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Now: time.Now().UTC(),
		Intentions: []Intention{
			{
				GoalID:     "goal-conn",
				GoalType:   GoalTypeConnection,
				Commitment: CommitmentStrong,
				Status:     IntentionStatusExecuting,
			},
		},
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
	}

	candidates := GenerateCandidates(ctx, registry)
	scored := ApplyCandidateContextSignals(candidates, ctx)

	chatReply := findCandidate(scored, "chat_reply")
	if chatReply == nil {
		t.Fatal("chat_reply 候选丢失")
	}
	if chatReply.NeedScore <= 0 {
		t.Fatal("Connection 意图应为 chat_reply 增加 NeedScore")
	}
}

func TestApplyCandidateContextSignalsDoesNotMutateInput(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Now: time.Now().UTC(),
		Goals: []Goal{
			{ID: "g1", Type: GoalTypeSupport, Status: GoalStatusActive},
		},
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
	}

	candidates := GenerateCandidates(ctx, registry)
	original := make([]BehaviorCandidate, len(candidates))
	copy(original, candidates)

	_ = ApplyCandidateContextSignals(candidates, ctx)

	for i := range candidates {
		if candidates[i].NeedScore != original[i].NeedScore {
			t.Fatalf("ApplyCandidateContextSignals 修改了输入 slice, 位置 %d: NeedScore 从 %f 变为 %f", i, original[i].NeedScore, candidates[i].NeedScore)
		}
	}
}

func TestApplyCandidateContextSignalsDoesNotSetFinalScore(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Now: time.Now().UTC(),
		Goals: []Goal{
			{ID: "g1", Type: GoalTypeConnection, Status: GoalStatusActive},
		},
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
	}

	candidates := GenerateCandidates(ctx, registry)
	scored := ApplyCandidateContextSignals(candidates, ctx)

	for _, c := range scored {
		if c.FinalScore != 0 {
			t.Fatalf("ApplyCandidateContextSignals 不应设置 FinalScore, 实际 %f", c.FinalScore)
		}
	}
}

func TestApplyCandidateContextSignalsEmptyCandidates(t *testing.T) {
	ctx := CandidateGenerationContext{
		Now:     time.Now().UTC(),
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
	}

	result := ApplyCandidateContextSignals(nil, ctx)
	if result == nil {
		t.Fatal("空输入应返回非 nil 空 slice")
	}
	if len(result) != 0 {
		t.Fatal("空输入应返回空 slice")
	}
}

func TestApplyCandidateContextSignalsPersonalityScore(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Now: time.Now().UTC(),
		PersonalityWeights: map[BehaviorTag]float64{
			BehaviorTagReply: 0.3,
		},
		Trigger: GoalTrigger{Kind: GoalTriggerInternal},
	}

	candidates := GenerateCandidates(ctx, registry)
	scored := ApplyCandidateContextSignals(candidates, ctx)

	chatReply := findCandidate(scored, "chat_reply")
	if chatReply == nil {
		t.Fatal("chat_reply 候选丢失")
	}
	if chatReply.PersonalityScore == 0 {
		t.Fatal("PersonalityWeights 应影响 PersonalityScore")
	}
}
