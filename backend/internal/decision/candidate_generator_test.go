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
		Now:         time.Now().UTC(),
	}

	candidates := GenerateCandidates(ctx, registry)
	if len(candidates) != len(registry.All()) {
		t.Fatalf("候选数量应与注册表一致: expected %d, got %d", len(registry.All()), len(candidates))
	}
	for _, c := range candidates {
		if c.ID == "" {
			t.Fatal("所有候选应有 ID")
		}
	}
}

func TestGenerateCandidatesWithContextEnrichment(t *testing.T) {
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
		Now: time.Now().UTC(),
	}

	candidates := GenerateCandidates(ctx, registry)
	chatCandidate := findCandidate(candidates, "chat_reply")
	if chatCandidate == nil {
		t.Fatal("chat_reply 候选丢失")
	}
	if chatCandidate.NeedScore <= 0 {
		t.Fatal("Connection 目标应为 chat_reply 增加 NeedScore")
	}
}

func TestGenerateCandidatesWithExcludes(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		UserID:      "user-1",
		CharacterID: "char-1",
		Now:         time.Now().UTC(),
	}

	candidates := GenerateCandidatesWithExcludes(ctx, registry, []string{"chat_reply", "ask_clarify"})
	for _, c := range candidates {
		if c.ID == "chat_reply" || c.ID == "ask_clarify" {
			t.Fatalf("被排除的候选不应出现: %s", c.ID)
		}
	}
}

func TestHighStressAffectsProactive(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Now: time.Now().UTC(),
		Psyche: PsycheSignalSet{
			Stress: ScalarSignal{Value: 0.85},
		},
	}

	candidates := GenerateCandidates(ctx, registry)
	proactive := findCandidate(candidates, "proactive_greet")
	if proactive == nil {
		t.Fatal("proactive_greet 候选丢失")
	}
	if proactive.RiskScore <= 0 {
		t.Fatal("高压状态下 proactive 应有风险分")
	}
}

func TestBusyLifeBlocksProactive(t *testing.T) {
	registry := DefaultCandidateRegistry()
	ctx := CandidateGenerationContext{
		Now: time.Now().UTC(),
		Life: LifeSnapshot{Busy: 0.95, Energy: 0.5},
	}

	candidates := GenerateCandidates(ctx, registry)
	proactive := findCandidate(candidates, "proactive_greet")
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

func TestIntentionBoostsGoalRelatedCandidate(t *testing.T) {
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
	}

	candidates := GenerateCandidates(ctx, registry)
	chatReply := findCandidate(candidates, "chat_reply")
	if chatReply == nil {
		t.Fatal("chat_reply 候选丢失")
	}
	if chatReply.NeedScore <= 0 {
		t.Fatal("Connection 意图应为 chat_reply 增加 NeedScore")
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
